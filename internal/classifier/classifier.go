// Package classifier derives one of 8 task categories from raw prompt text.
// It uses compiled regex patterns with aggressive keyword matching.
// Category detection is order-dependent: first match wins.
// Falls back to "general" when no pattern fires confidently.
package classifier

import (
	"regexp"
	"strings"

	"devfleet-agent/internal/models"
)

// rule pairs a compiled pattern with the category it signals.
type rule struct {
	re       *regexp.Regexp
	category string
}

// rules are evaluated in priority order. More specific patterns come first
// to prevent over-matching by broader patterns.
var rules []rule

func init() {
	// Each pattern is case-insensitive ((?i) prefix).
	// Word boundaries (\b) prevent partial-word false matches.
	definitions := []struct {
		pattern  string
		category string
	}{
		// ── Code Debugging (before code_gen to catch "fix"/"bug" first) ───────
		{
			`(?i)\b(bug|debug|debugg|error\s+in|fix\s+(the\s+)?(bug|code|error|issue)|what('s|s| is)\s+wrong|not\s+working|why\s+is\s+(this|the)\s+code|syntax\s+error|traceback|exception|stack\s+trace|failing\s+test)\b`,
			models.CategoryCodeDebug,
		},
		// ── Code Generation ──────────────────────────────────────────────────
		{
			`(?i)\b(write\s+(?:(an?|the)\s+)?(?:([\w\-\+#]+)\s+)?(function|class|method|program|script|code)|implement(ation)?|create\s+(?:(an?|the)\s+)?(?:([\w\-\+#]+)\s+)?(class|function|method)|generate\s+(?:(an?|the)\s+)?(?:([\w\-\+#]+)\s+)?(code|function|query|script)|code\s+that|algorithm\s+(for|that|to)|write\s+code)\b`,
			models.CategoryCodeGen,
		},

		// ── Named Entity Recognition ──────────────────────────────────────────
		{
			`(?i)\b(named\s+entit\w*|extract\s+(all\s+)?(named|entit\w*|person|organization|location|place|name)s?|identify\s+(all\s+)?(person|name|entit\w*|organization)s?|list\s+(the\s+)?(organization|person|location|entit\w*|name)s?\b|entity\s+recogni\w*|ner\b|locations?\b|persons?\b|organizations?\b)\b`,
			models.CategoryNER,
		},

		// ── Summarization ─────────────────────────────────────────────────────
		{
			`(?i)\b(summari[sz]e|summary|summarisation|tl;?dr|brief\s+(overview|description|summary)|condense|key\s+(points|takeaways|ideas)|main\s+(points|ideas|takeaways)|overview\s+of|abstract\s+of|give\s+(me\s+)?(the\s+)?(gist|highlights))\b`,
			models.CategorySummarize,
		},

		// ── Sentiment ─────────────────────────────────────────────────────────
		{
			`(?i)\b(sentiment|positive\s+or\s+negative|negative\s+or\s+positive|classify\s+(the\s+)?(feeling|emotion|tone|opinion|sentiment)|overall\s+(feeling|opinion|sentiment|tone)|analyze\s+(the\s+)?(feeling|emotion|tone|sentiment)|what\s+(tone|feeling|emotion|sentiment|opinion))\b`,
			models.CategorySentiment,
		},

		// ── Math ─────────────────────────────────────────────────────────────
		{
			`(?i)\b(calculat(e|ion)|comput(e|ation)|solv(e|ing)\s+(the\s+)?(equation|expression|formula)|what\s+is\s+[-+]?\d+|how\s+(much|many)\s+is\s+[-+]?\d+|\bsum\b|\bproduct\b|\bdifference\b|\bquotient\b|square\s+root|sqrt|\bpercent(age)?\b|\baverage\b|\bmean\b|\bmedian\b|\bmode\b|prime\s+number|factorial|fibonacci|\barea\b|\bvolume\b|\bperimeter\b|multiply|divide|subtract|add)\b`,
			models.CategoryMath,
		},

		// ── Logical Reasoning ─────────────────────────────────────────────────
		{
			`(?i)\b(deduc(e|tion|tive)|infer(ence)?|logical\s+(reasoning|conclusion|deduction)|syllogism|if\s+all\s+\w+\s+are|which\s+(of\s+the\s+following\s+)?(must|can|cannot)\s+be\s+true|what\s+can\s+we\s+(infer|conclude|deduce)|apply\s+(logical|deductive)|reasoning\s+(puzzle|problem|exercise)|given\s+these\s+(facts|premises|statements))\b`,
			models.CategoryLogical,
		},

		// ── Factual Knowledge ─────────────────────────────────────────────────
		{
			`(?i)\b(who\s+(is|was|are|were)|what\s+(is|was|are|were|year)\b|when\s+(did|was|were|is)|where\s+(is|was|are|were)|how\s+(does|do|did|is)\s+[^?]+\s+work|define\b|definition\s+of|capital\s+of|what\s+country|in\s+what\s+year|invented\s+by|discovered\s+by|founded\s+in|born\s+in|died\s+in)\b`,
			models.CategoryFactual,
		},
	}

	rules = make([]rule, 0, len(definitions))
	for _, d := range definitions {
		rules = append(rules, rule{
			re:       regexp.MustCompile(d.pattern),
			category: d.category,
		})
	}
}

// Classify derives a task category from raw prompt text.
// It does NOT rely on any structural category field from the input JSON.
// If no pattern confidently matches, it returns CategoryGeneral.
func Classify(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return models.CategoryGeneral
	}

	for _, r := range rules {
		if r.re.MatchString(trimmed) {
			return r.category
		}
	}

	return models.CategoryGeneral
}
