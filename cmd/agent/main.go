// Entrypoint for the DevFleet routing agent container.
// Orchestrates reading tasks, concurrency pooling, local zero-token solvers,
// prompt optimization, model routing, and output JSON writing under strict timeouts.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"golang.org/x/sync/errgroup"

	"devfleet-agent/internal/cache"
	"devfleet-agent/internal/classifier"
	"devfleet-agent/internal/client"
	"devfleet-agent/internal/config"
	"devfleet-agent/internal/local"
	"devfleet-agent/internal/models"
	"devfleet-agent/internal/optimizer"
	"devfleet-agent/internal/output"
	"devfleet-agent/internal/router"
)

const (
	inputPath           = "/input/tasks.json"
	outputPath          = "/output/results.json"
	globalTimeout       = 9 * time.Minute // 1-minute safety buffer for 10-minute hard cap
	maxConcurrency      = 4              // Keep concurrency conservative to avoid 429 penalties
	tokenBreakerCap     = 30000          // Keep cumulative budget under 30,000 tokens
	validationThreshold = 0.98           // Math (1.0) bypasses, sentiment (0.95) triggers 1-token gate
)

func main() {
	log.Println("[DevFleet] Starting Orchestrator...")

	// 1. Load and validate runtime environment configurations
	cfg, err := config.Load()
	if err != nil {
		log.Printf("[DevFleet] Configuration error: %v", err)
		os.Exit(1) // Fail fast on environment issues
	}
	log.Printf("[DevFleet] Configuration loaded. %d allowed models. Base URL: %s", len(cfg.Models), cfg.BaseURL)

	// 2. Read evaluation batch tasks
	tasks, err := readTasks(inputPath)
	if err != nil {
		log.Printf("[DevFleet] Failed to read input tasks: %v", err)
		os.Exit(1)
	}
	log.Printf("[DevFleet] Loaded %d tasks for execution", len(tasks))

	// 3. Initialize pipeline components
	apiClient := client.NewClient(cfg)
	taskRouter := router.NewRouter(cfg)
	taskCache := cache.New()

	// 4. Establish global context timeout (9-minute guillotine)
	ctx, cancel := context.WithTimeout(context.Background(), globalTimeout)
	defer cancel()

	results := make([]models.Result, len(tasks))

	// 5. Concurrent execution pool using errgroup
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxConcurrency)

	for i, task := range tasks {
		i, task := i, task
		g.Go(func() error {
			// Early context cancellation check
			if ctx.Err() != nil {
				results[i] = models.Result{
					TaskID: task.TaskID,
					Answer: "Unable to process",
				}
				return nil
			}

			results[i] = processTask(ctx, task, apiClient, taskRouter, taskCache)
			return nil // Never return error to group to guarantee we collect all results
		})
	}

	// Wait for all workers to complete or context to expire
	_ = g.Wait()

	// 6. Write final payloads atomically
	log.Printf("[DevFleet] Writing %d results to %s...", len(results), outputPath)
	if err := output.WriteResults(outputPath, results); err != nil {
		log.Printf("[DevFleet] Failed to write results: %v", err)
		os.Exit(1)
	}

	log.Printf("[DevFleet] Execution complete. Total tokens consumed: %d", apiClient.TotalTokens())
}

func processTask(
	ctx context.Context,
	task models.Task,
	apiClient *client.Client,
	taskRouter *router.Router,
	taskCache *cache.Cache,
) models.Result {
	taskID := task.TaskID
	prompt := task.Prompt

	// Category classification fallback
	category := task.Category
	if category == "" {
		category = classifier.Classify(prompt)
	}

	// A. Cache check (0 tokens)
	if cached, ok := taskCache.Get(prompt); ok {
		log.Printf("[Task %s] Cache HIT", taskID)
		return models.Result{TaskID: taskID, Answer: cached}
	}

	// B. TokenBreaker Ceiling check
	if apiClient.TotalTokens() >= tokenBreakerCap {
		log.Printf("[Task %s] TokenBreaker threshold breached (%d >= %d). Defaulting.",
			taskID, apiClient.TotalTokens(), tokenBreakerCap)
		return models.Result{
			TaskID: taskID,
			Answer: "Unable to process due to complexity",
		}
	}

	// C. Local solver attempt
	solverResult := local.Solve(category, prompt)
	if solverResult.Solved {
		if solverResult.Confidence >= validationThreshold {
			log.Printf("[Task %s] Local Solve SUCCESS (confidence: %.2f)", taskID, solverResult.Confidence)
			taskCache.Set(prompt, solverResult.Answer)
			return models.Result{TaskID: taskID, Answer: solverResult.Answer}
		}

		// Borderline confidence (e.g. sentiment) -> 1-Token validation gate
		cheapModel := taskRouter.CheapestModel()
		log.Printf("[Task %s] Local Solve borderline (confidence: %.2f). Validating via 1-token gate...",
			taskID, solverResult.Confidence)
		valid, _, err := apiClient.ValidateAnswer(ctx, cheapModel, prompt, solverResult.Answer)
		if err == nil && valid {
			log.Printf("[Task %s] 1-Token validation gate CONFIRMED answer", taskID)
			taskCache.Set(prompt, solverResult.Answer)
			return models.Result{TaskID: taskID, Answer: solverResult.Answer}
		}
		log.Printf("[Task %s] 1-Token validation REJECTED local answer (or error: %v). Escaling to cloud...",
			taskID, err)
	}

	// D. Prompt compression & optimization
	optimizedPrompt := optimizer.Optimize(prompt)

	// E. Router model selection (with Tier 3 escalation for rejected sentiment)
	var model string
	if category == models.CategorySentiment {
		// Sentiment that failed local solver or 1-token gate requires nuanced
		// understanding of contrastive structure → force Tier 3 model
		model = taskRouter.MostCapableModel()
		log.Printf("[Task %s] Sentiment escalated to Tier 3 model: %s", taskID, model)
	} else {
		model = taskRouter.SelectModel(category)
	}

	// F. Dynamic max_tokens cap based on category
	maxTokens := client.MaxTokensForCategory(category)

	// G. Fireworks API invocation
	log.Printf("[Task %s] Routing to Fireworks (%s, category: %s, maxTokens: %d)", taskID, model, category, maxTokens)
	systemPrompt := client.SystemPrompt(category)

	ans, _, err := apiClient.Complete(ctx, model, systemPrompt, optimizedPrompt, maxTokens)
	if err != nil {
		log.Printf("[Task %s] Fireworks API completion failed: %v. Returning fallback.", taskID, err)
		return models.Result{
			TaskID: taskID,
			Answer: "Unable to process",
		}
	}

	// H. Write to Cache
	taskCache.Set(prompt, ans)
	return models.Result{TaskID: taskID, Answer: ans}
}

func readTasks(path string) ([]models.Task, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	var tasks []models.Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, fmt.Errorf("parse JSON: %w", err)
	}

	return tasks, nil
}
