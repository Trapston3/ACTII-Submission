// Package optimizer compresses prompts before API submission to minimize token spend.
// It implements three stages:
//  1. Leading filler stripping (pleasantries, indirect phrasing)
//  2. Trailing filler stripping (sign-offs, politeness markers)
//  3. Whitespace normalization (collapse multi-spaces/newlines, trim)
//
// Each saved token directly improves leaderboard rank.
package optimizer

import (
	"regexp"
	"strings"
)

// All patterns are compiled once at package init — never per-call.
var (
	// ── Leading filler patterns ───────────────────────────────────────────────
	// Ordered from most specific to least to avoid partial-strip issues.
	leadingFillerRe = regexp.MustCompile(
		`(?i)^(` +
			`please\s+help\s+me\s+|` +
			`could\s+you\s+please\s+|` +
			`can\s+you\s+please\s+|` +
			`please\s+could\s+you\s+|` +
			`i\s*'?d\s+like\s+you\s+to\s+|` +
			`i\s+would\s+like\s+you\s+to\s+|` +
			`i\s+need\s+you\s+to\s+|` +
			`could\s+you\s+|` +
			`can\s+you\s+|` +
			`please\s+|` +
			`help\s+me\s+(understand\s+|with\s+)?` +
			`)+`,
	)

	// ── Trailing filler patterns ───────────────────────────────────────────────
	trailingFillerRe = regexp.MustCompile(
		`(?i)[,\s]*(` +
			`thanks?\s+you\s+very\s+much|` +
			`thanks?\s+in\s+advance|` +
			`thank\s+you\s+so\s+much|` +
			`thank\s+you|` +
			`thanks?\s*(a\s+lot)?|` +
			`please\s+and\s+thank\s+you|` +
			`please\b` +
			`)[.!]?\s*$`,
	)

	// ── Whitespace normalizer ─────────────────────────────────────────────────
	multiSpaceRe = regexp.MustCompile(`\s+`)
)

// Optimize compresses a prompt through all stages and returns the result.
// It is safe to call with an empty string.
func Optimize(prompt string) string {
	if prompt == "" {
		return ""
	}

	// Stage 1: Strip leading filler
	result := leadingFillerRe.ReplaceAllString(prompt, "")

	// Stage 2: Strip trailing filler
	result = trailingFillerRe.ReplaceAllString(result, "")

	// Stage 3: Normalize whitespace — collapse all runs of whitespace
	// (spaces, tabs, newlines) to a single space, then trim
	result = multiSpaceRe.ReplaceAllString(result, " ")
	result = strings.TrimSpace(result)

	return result
}
