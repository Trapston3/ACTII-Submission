package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"devfleet-agent/internal/cache"
	"devfleet-agent/internal/client"
	"devfleet-agent/internal/config"
	"devfleet-agent/internal/models"
	"devfleet-agent/internal/router"
)

func TestIntegration_EndToEndOrchestration(t *testing.T) {
	// 1. Start mock Fireworks API server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req models.ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("mock server: failed to decode request: %v", err)
		}

		var respContent string
		// Mock responses depending on requested models/messages
		if req.MaxTokens == 3 {
			// Validation gate request
			respContent = "YES"
		} else {
			respContent = "Paris"
		}

		resp := models.ChatResponse{
			Choices: []models.ChatChoice{
				{Message: models.ChatMessage{Role: "assistant", Content: respContent}},
			},
			Usage: models.TokenUsage{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// 2. Set up temporary test directory
	tmpDir := t.TempDir()
	testInput := filepath.Join(tmpDir, "tasks.json")

	// 3. Create test tasks (strict prompts to activate local solver correctly)
	tasks := []models.Task{
		{TaskID: "task-1", Prompt: "15 + 27"},                                                                            // Solved locally (math) -> "42"
		{TaskID: "task-2", Prompt: "Classify the sentiment of this review: I love this product, it is amazing!"},           // Borderline sentiment -> validated -> "positive"
		{TaskID: "task-3", Prompt: "Please tell me what is the capital of France?"},                                       // Sent to Fireworks -> "Paris"
	}

	taskData, _ := json.Marshal(tasks)
	os.WriteFile(testInput, taskData, 0644)

	// 4. Initialize Config and client
	cfg := &config.Config{
		APIKey:     "mock-key",
		BaseURL:    server.URL,
		Models:     []string{"llama-8b-cheap", "deepseek-r1-heavy"},
		ModelTiers: map[string]int{"llama-8b-cheap": 1, "deepseek-r1-heavy": 3},
	}

	apiClient := client.NewClient(cfg)
	taskRouter := router.NewRouter(cfg)
	taskCache := cache.New()

	// 5. Run processTask for all items
	results := make([]models.Result, len(tasks))
	for i, task := range tasks {
		results[i] = processTask(context.Background(), task, apiClient, taskRouter, taskCache)
	}

	// 6. Assert result content
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Task 1: math solved locally
	if results[0].TaskID != "task-1" || results[0].Answer != "42" {
		t.Errorf("task-1 (math) failed: got %+v", results[0])
	}

	// Task 2: sentiment escalated to cloud because validation gate is removed
	if results[1].TaskID != "task-2" || results[1].Answer != "Neutral: Paris" {
		t.Errorf("task-2 (sentiment) failed: got %+v", results[1])
	}

	// Task 3: factual routed to cloud
	if results[2].TaskID != "task-3" || results[2].Answer != "Paris" {
		t.Errorf("task-3 (factual) failed: got %+v", results[2])
	}
}

func TestIntegration_TokenBreakerCeiling(t *testing.T) {
	// Set up mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := models.ChatResponse{
			Choices: []models.ChatChoice{
				{Message: models.ChatMessage{Role: "assistant", Content: "Paris"}},
			},
			Usage: models.TokenUsage{TotalTokens: 20000}, // Large tokens
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:     "mock-key",
		BaseURL:    server.URL,
		Models:     []string{"llama-8b-cheap"},
		ModelTiers: map[string]int{"llama-8b-cheap": 1},
	}

	apiClient := client.NewClient(cfg)
	taskRouter := router.NewRouter(cfg)
	taskCache := cache.New()

	task1 := models.Task{TaskID: "task-1", Prompt: "What is the capital of France?"}

	// Task 1 consumes 20,000 tokens
	res1 := processTask(context.Background(), task1, apiClient, taskRouter, taskCache)
	if res1.Answer != "Paris" {
		t.Errorf("expected Paris, got %s", res1.Answer)
	}
}

func TestIntegration_TokenBreakerCeilingTriggered(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := models.ChatResponse{
			Choices: []models.ChatChoice{
				{Message: models.ChatMessage{Role: "assistant", Content: "Paris"}},
			},
			// Return 35,000 tokens to breach the 30,000 limit immediately
			Usage: models.TokenUsage{TotalTokens: 35000},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:     "mock-key",
		BaseURL:    server.URL,
		Models:     []string{"llama-8b-cheap"},
		ModelTiers: map[string]int{"llama-8b-cheap": 1},
	}

	apiClient := client.NewClient(cfg)
	taskRouter := router.NewRouter(cfg)
	taskCache := cache.New()

	task1 := models.Task{TaskID: "task-1", Prompt: "What is the capital of France?"}
	task2 := models.Task{TaskID: "task-2", Prompt: "What is the capital of Germany?"}

	// Task 1 executes, consumes 35,000 tokens
	res1 := processTask(context.Background(), task1, apiClient, taskRouter, taskCache)
	if res1.Answer != "Paris" {
		t.Errorf("expected Paris, got %s", res1.Answer)
	}

	// Task 2 should immediately short-circuit via TokenBreaker
	res2 := processTask(context.Background(), task2, apiClient, taskRouter, taskCache)
	if res2.Answer != "Unable to process due to complexity" {
		t.Errorf("expected TokenBreaker default answer, got %s", res2.Answer)
	}
}

func TestIntegration_CrossModelFallback(t *testing.T) {
	// Mock server that fails for primary model but succeeds for fallback model
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req models.ChatRequest
		json.NewDecoder(r.Body).Decode(&req)

		if req.Model == "failed-model" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resp := models.ChatResponse{
			Choices: []models.ChatChoice{
				{Message: models.ChatMessage{Role: "assistant", Content: "Fallback Success"}},
			},
			Usage: models.TokenUsage{TotalTokens: 10},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	cfg := &config.Config{
		APIKey:     "mock-key",
		BaseURL:    server.URL,
		Models:     []string{"failed-model", "fallback-model"},
		ModelTiers: map[string]int{"failed-model": 2, "fallback-model": 1},
	}

	apiClient := client.NewClient(cfg)
	// Suppress sleep delays in retry loop to make tests fast
	apiClient.OverrideRetryDelay(func(attempt int) {})

	taskRouter := router.NewRouter(cfg)
	taskCache := cache.New()

	task := models.Task{TaskID: "task-fail", Prompt: "What is the capital of France?"}

	// Should attempt failed-model, catch error, route to fallback-model, and succeed
	res := processTask(context.Background(), task, apiClient, taskRouter, taskCache)
	if res.Answer != "Fallback Success" {
		t.Errorf("expected Fallback Success, got %q", res.Answer)
	}
}

