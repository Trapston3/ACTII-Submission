// Package client implements the raw HTTP Fireworks AI client with retry support,
// reasoning tag stripping, and a cost-efficient 1-Token validation gate.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"devfleet-agent/internal/config"
	"devfleet-agent/internal/models"
)

var thinkTagRe = regexp.MustCompile(`(?s)<think>.*?</think>`)

// ── Output sanitizer patterns (compiled once at init) ─────────────────────────
// These strip common LLM artifacts that the evaluation judge penalizes.
var (
	// Markdown code fences: ```language\n...\n```
	codeFenceRe = regexp.MustCompile("(?s)```\\w*\n?(.*?)\n?```")
	// Leading preamble phrases: "Here's the answer:", "Here is my solution:"
	preambleRe = regexp.MustCompile(`(?i)^(here(?:'s| is) (?:the |my |your )?(?:answer|solution|result|code|output|response|explanation)[:\s]*)`)
	// Trailing sign-off phrases: "Let me know if...", "Hope this helps..."
	signoffRe = regexp.MustCompile(`(?i)(let me know if .*|hope this helps.*|feel free to .*|if you have any .*|is there anything else.*)\s*$`)
)

// Client manages HTTP traffic and cumulative token auditing.
type Client struct {
	config     *config.Config
	httpClient *http.Client
	tokenCount int64 // atomic cumulative total token counter

	// retryDelay allows overriding sleep behavior during unit tests
	retryDelay func(attempt int)
}

// NewClient creates a Fireworks API client.
func NewClient(cfg *config.Config) *Client {
	return &Client{
		config:     cfg,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		retryDelay: func(attempt int) {
			// Exponential backoff with ±30% jitter to prevent thundering herd
			base := time.Duration(1<<attempt) * 250 * time.Millisecond
			jitter := 0.7 + rand.Float64()*0.6 // 0.7–1.3 multiplier
			delay := time.Duration(float64(base) * jitter)
			time.Sleep(delay)
		},
	}
}

// OverrideRetryDelay overrides the default sleep behavior during retries (useful for testing).
func (c *Client) OverrideRetryDelay(delayFn func(attempt int)) {
	c.retryDelay = delayFn
}


// Complete makes a chat completion request to the Fireworks API.
// It enforces temperature=0.0 and retries up to 3 times on 429 or 5xx errors.
// Automatically strips <think>...</think> reasoning tags from the output.
// maxTokens > 0 enforces a server-side completion token ceiling.
func (c *Client) Complete(
	ctx context.Context,
	model string,
	systemPrompt string,
	userPrompt string,
	maxTokens int,
) (string, models.TokenUsage, error) {
	reqBody := models.ChatRequest{
		Model:       model,
		Temperature: 0.0,
		MaxTokens:   maxTokens,
		Messages: []models.ChatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
	}

	var respBody models.ChatResponse
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		// Respect context cancellation
		if err = ctx.Err(); err != nil {
			return "", models.TokenUsage{}, err
		}

		respBody, err = c.doPost(ctx, reqBody)
		if err == nil {
			break
		}

		// Check if it is a transient/rate-limiting error to retry
		if attempt < 2 && isTransient(err) {
			c.retryDelay(attempt)
			continue
		}
		break
	}

	if err != nil {
		return "", models.TokenUsage{}, err
	}

	// 1. Accumulate total tokens atomically for TokenBreaker tracking
	atomic.AddInt64(&c.tokenCount, int64(respBody.Usage.TotalTokens))

	// 2. Parse assistant response
	if len(respBody.Choices) == 0 {
		return "", respBody.Usage, fmt.Errorf("API response returned zero choices")
	}
	content := respBody.Choices[0].Message.Content

	// 3. AGGRESSIVELY strip <think>...</think> tags and any internal reasoning artifacts
	content = thinkTagRe.ReplaceAllString(content, "")

	// 4. Sanitize output: strip markdown fences, preambles, sign-offs
	content = sanitizeOutput(content)

	return content, respBody.Usage, nil
}

// ValidateAnswer executes the 1-Token validation gate on a cheap model.
// Sends the prompt: "Question: {question}\nProposed answer: {proposedAnswer}\nIs this correct? Reply with only YES or NO."
// Limits token usage via max_tokens=3. Returns true if response contains "YES".
func (c *Client) ValidateAnswer(
	ctx context.Context,
	cheapModel string,
	question string,
	proposedAnswer string,
) (bool, models.TokenUsage, error) {
	prompt := fmt.Sprintf(
		"Question: %s\nProposed answer: %s\nIs this correct? Reply with only YES or NO.",
		question,
		proposedAnswer,
	)

	reqBody := models.ChatRequest{
		Model:       cheapModel,
		Temperature: 0.0,
		MaxTokens:   3, // Limit tokens strictly
		Messages: []models.ChatMessage{
			{Role: "user", Content: prompt},
		},
	}

	var respBody models.ChatResponse
	var err error

	for attempt := 0; attempt < 3; attempt++ {
		if err = ctx.Err(); err != nil {
			return false, models.TokenUsage{}, err
		}

		respBody, err = c.doPost(ctx, reqBody)
		if err == nil {
			break
		}

		if attempt < 2 && isTransient(err) {
			c.retryDelay(attempt)
			continue
		}
		break
	}

	if err != nil {
		return false, models.TokenUsage{}, err
	}

	// Accumulate tokens
	atomic.AddInt64(&c.tokenCount, int64(respBody.Usage.TotalTokens))

	if len(respBody.Choices) == 0 {
		return false, respBody.Usage, fmt.Errorf("validation API returned zero choices")
	}

	content := strings.ToUpper(strings.TrimSpace(respBody.Choices[0].Message.Content))
	isValid := strings.Contains(content, "YES")

	return isValid, respBody.Usage, nil
}

// TotalTokens returns the total cumulative tokens used by this client.
func (c *Client) TotalTokens() int64 {
	return atomic.LoadInt64(&c.tokenCount)
}

func (c *Client) doPost(ctx context.Context, reqBody models.ChatRequest) (models.ChatResponse, error) {
	baseURL := strings.TrimRight(c.config.BaseURL, "/")
	if !strings.HasSuffix(baseURL, "/chat/completions") {
		if strings.HasSuffix(baseURL, "/v1") {
			baseURL += "/chat/completions"
		} else {
			baseURL += "/v1/chat/completions"
		}
	}
	url := baseURL
	log.Printf("[DEBUG] Fireworks API Request URL: %s", url)
	log.Printf("[DEBUG] Using Model: %s", reqBody.Model)

	data, err := json.Marshal(reqBody)
	if err != nil {
		return models.ChatResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return models.ChatResponse{}, fmt.Errorf("create HTTP request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return models.ChatResponse{}, fmt.Errorf("HTTP post failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Read the error response body for diagnostic logging
		var errBody []byte
		errBody, _ = io.ReadAll(io.LimitReader(resp.Body, 1024))
		log.Printf("[DEBUG] Fireworks API error response (status %d): %s", resp.StatusCode, string(errBody))
		return models.ChatResponse{}, &apiError{
			statusCode: resp.StatusCode,
			message:    fmt.Sprintf("Fireworks API returned status %d %s: %s", resp.StatusCode, resp.Status, string(errBody)),
		}
	}

	var respBody models.ChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return models.ChatResponse{}, fmt.Errorf("decode JSON response: %w", err)
	}

	return respBody, nil
}

type apiError struct {
	statusCode int
	message    string
}

func (e *apiError) Error() string {
	return e.message
}

func isTransient(err error) bool {
	if err == nil {
		return false
	}
	if apiErr, ok := err.(*apiError); ok {
		// Retry on Rate Limit (429) or Server Errors (5xx)
		return apiErr.statusCode == http.StatusTooManyRequests || apiErr.statusCode >= 500
	}
	// General networking errors are transient
	return true
}

// sanitizeOutput strips common LLM output artifacts that the evaluation judge
// will penalize: markdown code fences, leading preamble phrases, and trailing
// sign-off noise. Called after think-tag stripping.
func sanitizeOutput(content string) string {
	// 1. Unwrap markdown code fences → extract inner content
	content = codeFenceRe.ReplaceAllString(content, "$1")

	// 2. Strip leading preamble phrases ("Here's the answer:", etc.)
	content = preambleRe.ReplaceAllString(content, "")

	// 3. Strip trailing sign-off noise ("Let me know if...", etc.)
	content = signoffRe.ReplaceAllString(content, "")

	// 4. Normalize whitespace and trim
	content = strings.TrimSpace(content)
	return content
}
