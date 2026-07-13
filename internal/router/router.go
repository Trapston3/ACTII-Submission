// Package router handles model selection based on task category
// and the set of ALLOWED_MODELS resolved from the environment.
package router

import (
	"math"
	"strings"

	"devfleet-agent/internal/config"
	"devfleet-agent/internal/models"
)

// Router maps task categories to preferred model cost tiers.
type Router struct {
	config *config.Config
}

// NewRouter creates a new Router with the resolved environment config.
func NewRouter(cfg *config.Config) *Router {
	return &Router{config: cfg}
}

// SelectModel maps a task category to the best available model in ALLOWED_MODELS.
func (r *Router) SelectModel(category string) string {
	// If only one model is allowed, we have no routing choices.
	if len(r.config.Models) == 1 {
		return r.config.Models[0]
	}

	findModel := func(sub string) (string, bool) {
		for _, m := range r.config.Models {
			if strings.Contains(strings.ToLower(m), strings.ToLower(sub)) {
				return m, true
			}
		}
		return "", false
	}

	// 1. Gemma-First Override: text tasks, if any allowed model contains "gemma", select it immediately.
	isTextTask := category == models.CategorySentiment || category == models.CategoryNER || category == models.CategorySummarize || category == models.CategoryFactual
	if isTextTask {
		if gemmaModel, found := findModel("gemma"); found {
			return gemmaModel
		}
	}

	// 2. Specific Tier Hierarchies
	switch category {
	case models.CategorySentiment, models.CategoryNER:
		// Tier 1 (Cheapest - Sentiment, NER): Prioritize "gpt-oss-20b", then "deepseek-v4-flash", then "minimax".
		for _, name := range []string{"gpt-oss-20b", "deepseek-v4-flash", "minimax"} {
			if m, found := findModel(name); found {
				return m
			}
		}
	case models.CategorySummarize, models.CategoryFactual:
		// Tier 2 (Mid/Fast - Summarization, Factual): Prioritize "gpt-oss-120b", then "qwen3", then "deepseek-v4-flash".
		for _, name := range []string{"gpt-oss-120b", "qwen3", "deepseek-v4-flash"} {
			if m, found := findModel(name); found {
				return m
			}
		}
	case models.CategoryCodeGen, models.CategoryCodeDebug, models.CategoryMath, models.CategoryLogical:
		// Tier 3 (Heavy - Code, Math, Logic): Do not use Gemma. Prioritize "deepseek-v4-pro", then "glm-5p2", then "kimi-k2p7-code".
		for _, name := range []string{"deepseek-v4-pro", "glm-5p2", "kimi-k2p7-code"} {
			if m, found := findModel(name); found {
				return m
			}
		}
	}

	// 3. Fallback: If no prioritized model matches, use tier-based fallback
	preferredTier := 2
	switch category {
	case models.CategorySentiment, models.CategoryNER:
		preferredTier = 1
	case models.CategoryFactual, models.CategorySummarize, models.CategoryGeneral:
		preferredTier = 2
	case models.CategoryMath, models.CategoryLogical:
		preferredTier = 3
	case models.CategoryCodeGen, models.CategoryCodeDebug:
		preferredTier = 3
	}

	// Try exact match
	for _, m := range r.config.Models {
		if r.config.ModelTiers[m] == preferredTier {
			return m
		}
	}
	// Try fallback (prefer cheaper models first)
	for t := preferredTier - 1; t >= 1; t-- {
		for _, m := range r.config.Models {
			if r.config.ModelTiers[m] == t {
				return m
			}
		}
	}
	for t := preferredTier + 1; t <= 3; t++ {
		for _, m := range r.config.Models {
			if r.config.ModelTiers[m] == t {
				return m
			}
		}
	}
	return r.config.Models[0]
}

// CheapestModel returns the model with the lowest cost tier.
// Used by the 1-Token validation gate to minimize token spend.
func (r *Router) CheapestModel() string {
	if len(r.config.Models) == 0 {
		return ""
	}

	cheapest := r.config.Models[0]
	minTier := math.MaxInt32

	for _, m := range r.config.Models {
		tier := r.config.ModelTiers[m]
		if tier < minTier {
			minTier = tier
			cheapest = m
		}
	}

	return cheapest
}

// MostCapableModel returns the model with the highest cost tier.
// Used for sentiment escalation when contrastive prompts require
// nuanced reasoning that cheaper models hallucinate on.
func (r *Router) MostCapableModel() string {
	if len(r.config.Models) == 0 {
		return ""
	}

	best := r.config.Models[0]
	maxTier := 0

	for _, m := range r.config.Models {
		tier := r.config.ModelTiers[m]
		if tier > maxTier {
			maxTier = tier
			best = m
		}
	}

	return best
}

// GetNextFallbackModel returns the next cheapest available model from
// ALLOWED_MODELS that is different from the failed model. Returns empty string
// if no other models are configured.
func (r *Router) GetNextFallbackModel(failedModel string) string {
	var bestFallback string
	minTier := math.MaxInt32

	for _, m := range r.config.Models {
		if m == failedModel {
			continue
		}
		tier := r.config.ModelTiers[m]
		if tier < minTier {
			minTier = tier
			bestFallback = m
		}
	}

	return bestFallback
}

