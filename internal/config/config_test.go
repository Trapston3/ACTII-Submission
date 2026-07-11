package config

import (
	"os"
	"testing"
)

// helper clears and restores env vars around a test
func withEnv(t *testing.T, vars map[string]string, fn func()) {
	t.Helper()
	original := make(map[string]string, len(vars))
	for k := range vars {
		original[k] = os.Getenv(k)
		os.Unsetenv(k)
	}
	for k, v := range vars {
		if v != "" {
			os.Setenv(k, v)
		}
	}
	fn()
	for k, v := range original {
		if v == "" {
			os.Unsetenv(k)
		} else {
			os.Setenv(k, v)
		}
	}
}

func TestLoad_MissingAPIKey(t *testing.T) {
	withEnv(t, map[string]string{
		"FIREWORKS_API_KEY": "",
		"FIREWORKS_BASE_URL": "https://api.fireworks.ai/inference/v1",
		"ALLOWED_MODELS":    "model-a",
	}, func() {
		_, err := Load()
		if err == nil {
			t.Error("expected error for missing FIREWORKS_API_KEY, got nil")
		}
	})
}

func TestLoad_MissingBaseURL(t *testing.T) {
	withEnv(t, map[string]string{
		"FIREWORKS_API_KEY": "sk-test",
		"FIREWORKS_BASE_URL": "",
		"ALLOWED_MODELS":    "model-a",
	}, func() {
		_, err := Load()
		if err == nil {
			t.Error("expected error for missing FIREWORKS_BASE_URL, got nil")
		}
	})
}

func TestLoad_MissingAllowedModels(t *testing.T) {
	withEnv(t, map[string]string{
		"FIREWORKS_API_KEY": "sk-test",
		"FIREWORKS_BASE_URL": "https://api.fireworks.ai/inference/v1",
		"ALLOWED_MODELS":    "",
	}, func() {
		_, err := Load()
		if err == nil {
			t.Error("expected error for missing ALLOWED_MODELS, got nil")
		}
	})
}

func TestLoad_CommaSeparatedModels(t *testing.T) {
	withEnv(t, map[string]string{
		"FIREWORKS_API_KEY": "sk-test",
		"FIREWORKS_BASE_URL": "https://api.fireworks.ai/inference/v1",
		"ALLOWED_MODELS":    "model-a,model-b,model-c",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Models) != 3 {
			t.Errorf("got %d models, want 3: %v", len(cfg.Models), cfg.Models)
		}
		if cfg.Models[0] != "model-a" {
			t.Errorf("first model = %q, want %q", cfg.Models[0], "model-a")
		}
	})
}

func TestLoad_JSONArrayModels(t *testing.T) {
	withEnv(t, map[string]string{
		"FIREWORKS_API_KEY": "sk-test",
		"FIREWORKS_BASE_URL": "https://api.fireworks.ai/inference/v1",
		"ALLOWED_MODELS":    `["model-x","model-y"]`,
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Models) != 2 {
			t.Errorf("got %d models, want 2: %v", len(cfg.Models), cfg.Models)
		}
	})
}

func TestLoad_SemicolonSeparatedModels(t *testing.T) {
	withEnv(t, map[string]string{
		"FIREWORKS_API_KEY": "sk-test",
		"FIREWORKS_BASE_URL": "https://api.fireworks.ai/inference/v1",
		"ALLOWED_MODELS":    "model-a;model-b",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Models) != 2 {
			t.Errorf("got %d models, want 2: %v", len(cfg.Models), cfg.Models)
		}
	})
}

func TestLoad_SingleModel(t *testing.T) {
	withEnv(t, map[string]string{
		"FIREWORKS_API_KEY": "sk-test",
		"FIREWORKS_BASE_URL": "https://api.fireworks.ai/inference/v1",
		"ALLOWED_MODELS":    "accounts/fireworks/models/llama-v3p1-8b-instruct",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(cfg.Models) != 1 {
			t.Errorf("got %d models, want 1", len(cfg.Models))
		}
	})
}

func TestLoad_BaseURLTrailingSlashStripped(t *testing.T) {
	withEnv(t, map[string]string{
		"FIREWORKS_API_KEY": "sk-test",
		"FIREWORKS_BASE_URL": "https://api.fireworks.ai/inference/v1/",
		"ALLOWED_MODELS":    "model-a",
	}, func() {
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.BaseURL[len(cfg.BaseURL)-1] == '/' {
			t.Errorf("BaseURL should not have trailing slash, got: %s", cfg.BaseURL)
		}
	})
}

func TestModelTierClassification(t *testing.T) {
	tests := []struct {
		model    string
		wantTier int
	}{
		{"accounts/fireworks/models/llama-v3p1-8b-instruct", 1},
		{"accounts/fireworks/models/llama-v3p3-70b-instruct", 2},
		{"accounts/fireworks/models/deepseek-r1", 3},
		{"accounts/fireworks/models/qwen3-235b-a22b", 2}, // large param count → tier 2 default
		{"accounts/fireworks/models/kimi-k2-instruct", 3},
		{"accounts/fireworks/models/mixtral-8x22b-instruct", 2},
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			tiers := classifyModelTiers([]string{tt.model})
			got := tiers[tt.model]
			if got != tt.wantTier {
				t.Errorf("classifyModelTiers(%q) = tier %d, want tier %d", tt.model, got, tt.wantTier)
			}
		})
	}
}
