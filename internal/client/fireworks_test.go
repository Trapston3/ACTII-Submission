package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"devfleet-agent/internal/config"
	"devfleet-agent/internal/models"
)

func TestClient_Complete_Success(t *testing.T) {
	// Set up mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected authorization header: %s", r.Header.Get("Authorization"))
		}

		// Verify request body
		var req models.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if req.Temperature != 0.0 {
			t.Errorf("expected temperature 0.0, got %.2f", req.Temperature)
		}

		// Respond with mock data containing think tags
		resp := models.ChatResponse{
			Choices: []models.ChatChoice{
				{
					Message: models.ChatMessage{
						Role:    "assistant",
						Content: "<think>\nThinking about the problem...\nEvaluating calculations...\n</think>42",
					},
				},
			},
			Usage: models.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 25,
				TotalTokens:      35,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	c := NewClient(cfg)
	ans, usage, err := c.Complete(context.Background(), "test-model", "System Prompt", "User Prompt", 0, "general")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify think tags are stripped
	if ans != "42" {
		t.Errorf("expected answer %q, got %q", "42", ans)
	}

	// Verify token tracking
	if usage.TotalTokens != 35 {
		t.Errorf("expected usage 35, got %d", usage.TotalTokens)
	}
	if c.TotalTokens() != 35 {
		t.Errorf("expected client cumulative tokens 35, got %d", c.TotalTokens())
	}
}

func TestClient_Complete_RetryOnRateLimit(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			// Fail first two times with 429 Rate Limit
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}

		// Succeed on third attempt
		resp := models.ChatResponse{
			Choices: []models.ChatChoice{
				{Message: models.ChatMessage{Role: "assistant", Content: "success"}},
			},
			Usage: models.TokenUsage{TotalTokens: 12},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	}

	c := NewClient(cfg)
	// Override delay function for testing to avoid wasting time
	c.retryDelay = func(attempt int) {}

	ans, _, err := c.Complete(context.Background(), "test-model", "sys", "usr", 0, "general")
	if err != nil {
		t.Fatalf("unexpected error after retries: %v", err)
	}
	if ans != "success" {
		t.Errorf("expected success, got %q", ans)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
}

func TestClient_ValidateAnswer(t *testing.T) {
	tests := []struct {
		name       string
		apiResp    string
		wantResult bool
	}{
		{"valid YES response", "YES", true},
		{"valid YES with spaces", "  yes  ", true},
		{"valid YES with details", "YES, the proposed answer is correct.", true},
		{"NO response", "NO", false},
		{"random response", "unrelated text", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req models.ChatRequest
				json.NewDecoder(r.Body).Decode(&req)
				if req.MaxTokens != 3 {
					t.Errorf("expected max_tokens 3 for validation gate, got %d", req.MaxTokens)
				}

				resp := models.ChatResponse{
					Choices: []models.ChatChoice{
						{Message: models.ChatMessage{Role: "assistant", Content: tt.apiResp}},
					},
					Usage: models.TokenUsage{TotalTokens: 5},
				}
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			cfg := &config.Config{
				APIKey:  "test-key",
				BaseURL: server.URL,
			}

			c := NewClient(cfg)
			c.retryDelay = func(attempt int) {}

			valid, _, err := c.ValidateAnswer(context.Background(), "cheap-model", "2 + 2", "4")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if valid != tt.wantResult {
				t.Errorf("ValidateAnswer() = %v, want %v (api response: %q)", valid, tt.wantResult, tt.apiResp)
			}
		})
	}
}

func TestClient_SanitizeOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"markdown code block",
			"```python\ndef foo():\n    return 42\n```",
			"def foo():\n    return 42",
		},
		{
			"preamble and markdown block",
			"Here's the code:\n```go\nfmt.Println(\"hello\")\n```",
			"fmt.Println(\"hello\")",
		},
		{
			"preamble, code, and signoff",
			"Here is the answer: 42. Hope this helps!",
			"42.",
		},
		{
			"let me know signoff",
			"The capital is Paris. Let me know if you need anything else.",
			"The capital is Paris.",
		},
		{
			"no artifacts",
			"Just a plain answer without extra fluff.",
			"Just a plain answer without extra fluff.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeOutput(tt.input, "general")
			if got != tt.expected {
				t.Errorf("sanitizeOutput(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

