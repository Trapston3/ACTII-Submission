// Package config handles environment variable resolution and validation.
// It fails fast at startup if any required configuration is missing,
// preventing cryptic runtime errors deep in the execution pipeline.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Config holds all validated runtime configuration derived from environment variables.
type Config struct {
	APIKey     string
	BaseURL    string
	Models     []string
	ModelTiers map[string]int // model name → tier (1=cheap, 2=standard, 3=powerful)
}

// Load reads environment variables, validates them, and returns a Config.
// Returns a non-nil error if any required variable is missing or malformed.
// Callers should treat a non-nil error as fatal and exit immediately.
func Load() (*Config, error) {
	apiKey := stripQuotes(os.Getenv("FIREWORKS_API_KEY"))
	if apiKey == "" {
		return nil, fmt.Errorf("FIREWORKS_API_KEY is required but not set")
	}

	baseURL := stripQuotes(os.Getenv("FIREWORKS_BASE_URL"))
	if baseURL == "" {
		return nil, fmt.Errorf("FIREWORKS_BASE_URL is required but not set")
	}
	// Normalize: strip trailing slash for consistent URL construction downstream
	baseURL = strings.TrimRight(baseURL, "/")

	modelsRaw := stripQuotes(os.Getenv("ALLOWED_MODELS"))
	if modelsRaw == "" {
		return nil, fmt.Errorf("ALLOWED_MODELS is required but not set")
	}

	models := parseModels(modelsRaw)
	if len(models) == 0 {
		return nil, fmt.Errorf("ALLOWED_MODELS parsed to zero models (raw value: %q)", modelsRaw)
	}

	tiers := classifyModelTiers(models)

	return &Config{
		APIKey:     apiKey,
		BaseURL:    baseURL,
		Models:     models,
		ModelTiers: tiers,
	}, nil
}

// parseModels handles three common formats from the evaluation harness:
//   - JSON array:          ["model-a","model-b"]
//   - Comma-separated:     model-a,model-b,model-c
//   - Semicolon-separated: model-a;model-b
//   - Single model:        accounts/fireworks/models/llama-v3p1-8b-instruct
func parseModels(raw string) []string {
	raw = strings.TrimSpace(raw)

	// JSON array (starts with '[')
	if strings.HasPrefix(raw, "[") {
		var models []string
		if err := json.Unmarshal([]byte(raw), &models); err == nil {
			return filterEmpty(models)
		}
		// Fall through if JSON parse fails — try string splitting
	}

	// Comma-separated
	if strings.Contains(raw, ",") {
		return filterEmpty(strings.Split(raw, ","))
	}

	// Semicolon-separated
	if strings.Contains(raw, ";") {
		return filterEmpty(strings.Split(raw, ";"))
	}

	// Single model name (no delimiter)
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}
	return []string{trimmed}
}

// classifyModelTiers assigns each model to a cost tier based on name substrings.
// Tier 1 = cheapest small models (route sentiment, NER)
// Tier 2 = mid-range models (route factual, summarization, math)
// Tier 3 = most capable models (route code gen, code debug, logical)
func classifyModelTiers(models []string) map[string]int {
	tiers := make(map[string]int, len(models))
	for _, m := range models {
		tiers[m] = tierForModel(m)
	}
	return tiers
}

// tierForModel classifies a single model name into tier 1, 2, or 3.
func tierForModel(model string) int {
	lower := strings.ToLower(model)

	// Tier 1: Small, cheap, fast models
	if containsAny(lower, "1.7b", "3b", "7b", "8b", "small", "mini", "tiny", "nano", "glm-5p1") {
		return 1
	}

	// Tier 3: Most capable, expensive models (reasoning, large-scale code)
	if containsAny(lower, "r1", "k2", "405b", "large", "-code", "kimi", "deepseek", "v4-pro", "glm-5p2") {
		return 3
	}

	// Tier 2: Default mid-range (70b, mixtral, qwen-large, gpt-oss, etc.)
	return 2
}

// containsAny returns true if s contains any of the provided substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// filterEmpty trims whitespace and removes empty strings from a slice.
func filterEmpty(ss []string) []string {
	result := make([]string, 0, len(ss))
	for _, s := range ss {
		s = strings.TrimSpace(s)
		if s != "" {
			result = append(result, s)
		}
	}
	return result
}

// stripQuotes removes surrounding double or single quotes from a string.
// Handles the common case where .env files wrap values in quotes that get
// passed literally into environment variables by some loaders.
func stripQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			s = s[1 : len(s)-1]
		}
	}
	return s
}
