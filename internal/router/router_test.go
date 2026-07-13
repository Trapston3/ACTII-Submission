package router

import (
	"testing"

	"devfleet-agent/internal/config"
)

func TestRouter_SelectModel(t *testing.T) {
	tests := []struct {
		name       string
		models     []string
		category   string
		wantModel  string
	}{
		{
			"single model configuration always returns that model",
			[]string{"only-model"},
			"math",
			"only-model",
		},
		{
			"Gemma-First override for text tasks",
			[]string{"deepseek-v4-pro", "gemma-2-9b-it", "minimax"},
			"sentiment",
			"gemma-2-9b-it",
		},
		{
			"Tier 1 cheapest prioritization (gemma-2-9b-it first)",
			[]string{"llama-v3p1-8b-instruct", "glm-5p1", "gemma-2-9b-it"},
			"sentiment",
			"gemma-2-9b-it",
		},
		{
			"Tier 1 cheapest prioritization (llama-v3p1-8b-instruct second)",
			[]string{"glm-5p1", "llama-v3p1-8b-instruct"},
			"ner",
			"llama-v3p1-8b-instruct",
		},
		{
			"Tier 2 mid-range prioritization (llama-v3p3-70b-instruct first)",
			[]string{"glm-5p1", "qwen2.5-72b-instruct", "llama-v3p3-70b-instruct"},
			"summarization",
			"llama-v3p3-70b-instruct",
		},
		{
			"Tier 2 mid-range prioritization (qwen2.5-72b-instruct second)",
			[]string{"glm-5p1", "qwen2.5-72b-instruct"},
			"factual",
			"qwen2.5-72b-instruct",
		},
		{
			"Tier 3 heavy prioritization (deepseek-v4-pro first)",
			[]string{"kimi-k2p7-code", "glm-5p2", "deepseek-v4-pro"},
			"math",
			"deepseek-v4-pro",
		},
		{
			"Tier 3 heavy does NOT use gemma",
			[]string{"gemma-2-9b-it", "kimi-k2p7-code", "glm-5p2"},
			"code_generation",
			"glm-5p2",
		},
		{
			"fallback to closest tier when preferred tier is missing (Tier 1 requested, Tier 2 closest)",
			[]string{"llama-70b-std", "deepseek-r1-heavy"},
			"sentiment",
			"llama-70b-std",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{
				APIKey:     "test",
				BaseURL:    "test",
				Models:     tt.models,
				ModelTiers: make(map[string]int),
			}
			for _, m := range tt.models {
				cfg.ModelTiers[m] = tierForTest(m)
			}

			r := NewRouter(cfg)
			got := r.SelectModel(tt.category)
			if got != tt.wantModel {
				t.Errorf("SelectModel(%q) = %q, want %q", tt.category, got, tt.wantModel)
			}
		})
	}
}

func TestRouter_CheapestModel(t *testing.T) {
	// Should return the model with the lowest tier
	cfg := &config.Config{
		Models:     []string{"llama-70b-std", "llama-8b-small", "deepseek-r1-heavy"},
		ModelTiers: map[string]int{"llama-70b-std": 2, "llama-8b-small": 1, "deepseek-r1-heavy": 3},
	}
	r := NewRouter(cfg)
	got := r.CheapestModel()
	if got != "llama-8b-small" {
		t.Errorf("CheapestModel() = %q, want %q", got, "llama-8b-small")
	}
}

// tierForTest duplicates the tierForModel substring heuristics for test simplicity
func tierForTest(m string) int {
	// Simple matching mapping for test cases
	if m == "llama-8b-small" {
		return 1
	}
	if m == "deepseek-r1-heavy" || m == "glm-5p2" {
		return 3
	}
	return 2
}

func TestRouter_GetNextFallbackModel(t *testing.T) {
	cfg := &config.Config{
		Models:     []string{"llama-70b-std", "llama-8b-small", "deepseek-r1-heavy"},
		ModelTiers: map[string]int{"llama-70b-std": 2, "llama-8b-small": 1, "deepseek-r1-heavy": 3},
	}
	r := NewRouter(cfg)

	tests := []struct {
		failedModel string
		wantModel   string
	}{
		{"llama-8b-small", "llama-70b-std"},      // Cheapest model other than small is standard (Tier 2)
		{"llama-70b-std", "llama-8b-small"},      // Cheapest model other than standard is small (Tier 1)
		{"deepseek-r1-heavy", "llama-8b-small"},  // Cheapest model other than heavy is small (Tier 1)
	}

	for _, tt := range tests {
		got := r.GetNextFallbackModel(tt.failedModel)
		if got != tt.wantModel {
			t.Errorf("GetNextFallbackModel(%q) = %q, want %q", tt.failedModel, got, tt.wantModel)
		}
	}

	// Single model test
	cfgSingle := &config.Config{
		Models:     []string{"only-model"},
		ModelTiers: map[string]int{"only-model": 1},
	}
	rSingle := NewRouter(cfgSingle)
	gotSingle := rSingle.GetNextFallbackModel("only-model")
	if gotSingle != "" {
		t.Errorf("expected empty string when no other model is available, got %q", gotSingle)
	}
}

