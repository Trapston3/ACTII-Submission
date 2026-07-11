// Package client implements the raw HTTP Fireworks AI client with retry support,
// reasoning tag stripping, and a cost-efficient 1-Token validation gate.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	"devfleet-agent/internal/config"
	"devfleet-agent/internal/models"
)

var thinkTagRe = regexp.MustCompile(`(?s)<think>.*?</think>`)

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
			// Exponential backoff: 500ms, 1s, 2s
			delay := time.Duration(1<<attempt) * 250 * time.Millisecond
			time.Sleep(delay)
		},
	}
}

// Complete makes a chat completion request to the Fireworks API.
// It enforces temperature=0.0 and retries up to 3 times on 429 or 5xx errors.
// Automatically strips <think>...</think> reasoning tags from the output.
func (c *Client) Complete(
	ctx context.Context,
	model string,
	systemPrompt string,
	userPrompt string,
) (string, models.TokenUsage, error) {
	reqBody := models.ChatRequest{
		Model:       model,
		Temperature: 0.0,
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
	content = strings.TrimSpace(content)

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
	url := fmt.Sprintf("%s/v1/chat/completions", c.config.BaseURL)

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
		return models.ChatResponse{}, &apiError{
			statusCode: resp.StatusCode,
			message:    fmt.Sprintf("Fireworks API returned status %d %s", resp.StatusCode, resp.Status),
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
