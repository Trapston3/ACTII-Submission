// Package router handles model selection based on task category
// and the set of ALLOWED_MODELS resolved from the environment.
package router

import (
	"math"

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
// Category mappings:
//  - sentiment, ner                 -> Tier 1 (cheapest)
//  - factual, summarization, general -> Tier 2 (standard)
//  - math, logical                  -> Tier 2-3 (complex)
//  - code_generation, code_debugging -> Tier 3 (powerful)
//
// If preferred tier is unavailable, it finds the closest available tier
// (preferring cheaper models first to preserve tokens).
func (r *Router) SelectModel(category string) string {
	// If only one model is allowed, we have no routing choices.
	if len(r.config.Models) == 1 {
		return r.config.Models[0]
	}

	preferredTier := 2
	switch category {
	case models.CategorySentiment, models.CategoryNER:
		preferredTier = 1
	case models.CategoryFactual, models.CategorySummarize, models.CategoryGeneral:
		preferredTier = 2
	case models.CategoryMath, models.CategoryLogical:
		preferredTier = 2
	case models.CategoryCodeGen, models.CategoryCodeDebug:
		preferredTier = 3
	}

	// 1. Try to find a model matching the exact preferred tier.
	for _, m := range r.config.Models {
		if r.config.ModelTiers[m] == preferredTier {
			return m
		}
	}

	// 2. Try to find a cheaper model (preferredTier - 1, preferredTier - 2).
	for t := preferredTier - 1; t >= 1; t-- {
		for _, m := range r.config.Models {
			if r.config.ModelTiers[m] == t {
				return m
			}
		}
	}

	// 3. Try to find a more expensive model (preferredTier + 1, preferredTier + 2).
	for t := preferredTier + 1; t <= 3; t++ {
		for _, m := range r.config.Models {
			if r.config.ModelTiers[m] == t {
				return m
			}
		}
	}

	// 4. Default to first model.
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
