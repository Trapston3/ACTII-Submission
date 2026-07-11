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
			"selects cheapest model for sentiment/ner (Tier 1)",
			[]string{"llama-8b-small", "llama-70b-std", "deepseek-r1-heavy"},
			"sentiment",
			"llama-8b-small",
		},
		{
			"selects mid-range model for factual/summarization (Tier 2)",
			[]string{"llama-8b-small", "llama-70b-std", "deepseek-r1-heavy"},
			"summarization",
			"llama-70b-std",
		},
		{
			"selects powerful model for code_generation/code_debugging (Tier 3)",
			[]string{"llama-8b-small", "llama-70b-std", "deepseek-r1-heavy"},
			"code_generation",
			"deepseek-r1-heavy",
		},
		{
			"fallback to closest tier when preferred tier is missing (Tier 3 requested, Tier 2 closest)",
			[]string{"llama-8b-small", "llama-70b-std"},
			"code_debugging",
			"llama-70b-std",
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
			// Construct config manually
			cfg := &config.Config{
				APIKey:     "test",
				BaseURL:    "test",
				Models:     tt.models,
				ModelTiers: make(map[string]int),
			}
			// Reclassify tiers to match config Load logic
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
	if m == "deepseek-r1-heavy" {
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

