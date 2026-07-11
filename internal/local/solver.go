// Package local provides zero-token local task solving for deterministic categories.
// It implements a strict activation gate: the math solver ONLY fires when the
// expression contains exclusively digits, operators, parentheses, and the sqrt
// keyword — no alphabetic word-math that could cause miscalculation.
package local

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"devfleet-agent/internal/models"
)

// Solve attempts to answer a task locally without any API call.
// Returns a SolverResult with Solved=false if the task cannot be handled
// safely and deterministically at this tier.
func Solve(category string, prompt string) models.SolverResult {
	prompt = strings.TrimSpace(prompt)
	switch category {
	case models.CategoryMath:
		return solveMath(prompt)
	case models.CategorySentiment:
		return solveSentiment(prompt)
	default:
		return models.SolverResult{Solved: false}
	}
}

// ─── MATH SOLVER ─────────────────────────────────────────────────────────────

// strictNumericalRe matches tokens that are safe for local evaluation:
// digits, decimals, operators, parentheses, and the sqrt keyword only.
var strictNumericalRe = regexp.MustCompile(`^[\d\s\.\+\-\*\/\^\(\)]+$|^[\d\s\.\+\-\*\/\^\(\)]*\bsqrt\b[\d\s\.\+\-\*\/\^\(\)]*$`)

// hasWordContent returns true if the string contains any sequence of 2+
// consecutive alphabetic characters (other than "sqrt") — signals word math.
var wordContentRe = regexp.MustCompile(`[a-zA-Z]{2,}`)

func solveMath(prompt string) models.SolverResult {
	// Normalize: collapse whitespace, lowercase sqrt
	expr := strings.TrimSpace(prompt)
	expr = regexp.MustCompile(`\s+`).ReplaceAllString(expr, " ")
	exprLower := strings.ToLower(expr)

	// STRICT GATE: detect any alphabetic word content beyond "sqrt"
	// Replace sqrt with a placeholder to check what remains
	withoutSqrt := regexp.MustCompile(`(?i)\bsqrt\b`).ReplaceAllString(exprLower, "")
	if wordContentRe.MatchString(withoutSqrt) {
		// Alphabetic content detected beyond sqrt → word math → skip
		return models.SolverResult{Solved: false}
	}

	// Secondary gate: expression must only contain allowed characters
	allowed := regexp.MustCompile(`^[\d\s\.\+\-\*\/\^\(\)]*$`)
	if !allowed.MatchString(withoutSqrt) {
		return models.SolverResult{Solved: false}
	}

	// At this point the expression is safe — evaluate it
	val, err := evaluate(strings.ToLower(expr))
	if err != nil {
		return models.SolverResult{Solved: false}
	}

	answer := formatNumber(val)
	return models.SolverResult{
		Answer:     answer,
		Solved:     true,
		Confidence: 1.0,
	}
}

// formatNumber formats a float64 as a clean string:
// integer results lose the ".0", decimals keep up to 6 significant figures.
func formatNumber(v float64) string {
	if v == math.Trunc(v) && !math.IsInf(v, 0) {
		return strconv.FormatInt(int64(v), 10)
	}
	s := strconv.FormatFloat(v, 'f', 6, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

// ─── EXPRESSION EVALUATOR (recursive descent, no dependencies) ────────────────

// evaluate parses and computes a mathematical expression string.
// Supports: +, -, *, /, ^, unary minus, parentheses, sqrt().
func evaluate(expr string) (float64, error) {
	p := &parser{input: strings.TrimSpace(expr), pos: 0}
	val, err := p.parseExpr()
	if err != nil {
		return 0, err
	}
	if p.pos < len(p.input) {
		remaining := strings.TrimSpace(p.input[p.pos:])
		if remaining != "" {
			return 0, fmt.Errorf("unexpected token: %q", remaining)
		}
	}
	return val, nil
}

type parser struct {
	input string
	pos   int
}

func (p *parser) peek() byte {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return 0
	}
	return p.input[p.pos]
}

func (p *parser) skipSpaces() {
	for p.pos < len(p.input) && p.input[p.pos] == ' ' {
		p.pos++
	}
}

// parseExpr handles + and - (lowest precedence)
func (p *parser) parseExpr() (float64, error) {
	left, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		ch := p.peek()
		if ch != '+' && ch != '-' {
			break
		}
		p.pos++
		right, err := p.parseTerm()
		if err != nil {
			return 0, err
		}
		if ch == '+' {
			left += right
		} else {
			left -= right
		}
	}
	return left, nil
}

// parseTerm handles * and / (medium precedence)
func (p *parser) parseTerm() (float64, error) {
	left, err := p.parsePower()
	if err != nil {
		return 0, err
	}
	for {
		ch := p.peek()
		if ch != '*' && ch != '/' {
			break
		}
		p.pos++
		right, err := p.parsePower()
		if err != nil {
			return 0, err
		}
		if ch == '*' {
			left *= right
		} else {
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			left /= right
		}
	}
	return left, nil
}

// parsePower handles ^ (exponentiation, right-associative)
func (p *parser) parsePower() (float64, error) {
	base, err := p.parseUnary()
	if err != nil {
		return 0, err
	}
	if p.peek() == '^' {
		p.pos++
		exp, err := p.parseUnary()
		if err != nil {
			return 0, err
		}
		return math.Pow(base, exp), nil
	}
	return base, nil
}

// parseUnary handles unary minus and the sqrt() function
func (p *parser) parseUnary() (float64, error) {
	p.skipSpaces()
	if p.pos < len(p.input) && p.input[p.pos] == '-' {
		p.pos++
		val, err := p.parsePrimary()
		if err != nil {
			return 0, err
		}
		return -val, nil
	}
	// sqrt function
	if strings.HasPrefix(p.input[p.pos:], "sqrt") {
		p.pos += 4
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != '(' {
			return 0, fmt.Errorf("sqrt requires parentheses")
		}
		p.pos++ // consume '('
		inner, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, fmt.Errorf("missing closing parenthesis after sqrt")
		}
		p.pos++ // consume ')'
		if inner < 0 {
			return 0, fmt.Errorf("sqrt of negative number")
		}
		return math.Sqrt(inner), nil
	}
	return p.parsePrimary()
}

// parsePrimary handles numbers and parenthesized sub-expressions
func (p *parser) parsePrimary() (float64, error) {
	p.skipSpaces()
	if p.pos >= len(p.input) {
		return 0, fmt.Errorf("unexpected end of expression")
	}

	// Parenthesized sub-expression
	if p.input[p.pos] == '(' {
		p.pos++ // consume '('
		val, err := p.parseExpr()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if p.pos >= len(p.input) || p.input[p.pos] != ')' {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		p.pos++ // consume ')'
		return val, nil
	}

	// Number (integer or decimal)
	start := p.pos
	if p.input[p.pos] == '.' {
		p.pos++
	}
	for p.pos < len(p.input) && (unicode.IsDigit(rune(p.input[p.pos])) || p.input[p.pos] == '.') {
		p.pos++
	}
	if start == p.pos {
		return 0, fmt.Errorf("expected number at position %d: %q", p.pos, p.input)
	}
	return strconv.ParseFloat(p.input[start:p.pos], 64)
}

// ─── SENTIMENT SOLVER ─────────────────────────────────────────────────────────

// contrastiveRe detects conjunctions that signal mixed or nuanced sentiment.
// When present, the local solver must NOT attempt classification — these
// require cloud models that understand contextual polarity shifts.
var contrastiveRe = regexp.MustCompile(`(?i)\b(but|however|although|though|yet|nevertheless|on the other hand|despite|in spite of|conversely|whereas|while|still)\b`)

// positiveWords are strong unambiguous positive sentiment signals.
var positiveWords = []string{
	"love", "amazing", "excellent", "fantastic", "wonderful",
	"great", "best", "perfect", "outstanding", "superb",
	"brilliant", "magnificent", "exceptional", "incredible", "awesome",
	"delightful", "terrific", "fabulous", "marvelous",
}

// negativeWords are strong unambiguous negative sentiment signals.
var negativeWords = []string{
	"terrible", "awful", "hate", "horrible", "worst",
	"disgusting", "pathetic", "dreadful", "atrocious", "deplorable",
	"appalling", "revolting", "abysmal", "dismal", "horrendous",
	"despise", "loathe", "ghastly", "hideous",
}

func solveSentiment(prompt string) models.SolverResult {
	lower := strings.ToLower(prompt)

	// CONTRASTIVE GATE: if the prompt contains a contrastive conjunction,
	// the sentiment is likely mixed/nuanced and must be routed to cloud.
	if contrastiveRe.MatchString(lower) {
		return models.SolverResult{Solved: false}
	}

	posCount := countSignals(lower, positiveWords)
	negCount := countSignals(lower, negativeWords)

	// Require 2+ strong signals on one side AND 0 on the other
	// to avoid misclassifying mixed-sentiment text
	if posCount >= 2 && negCount == 0 {
		return models.SolverResult{
			Answer:     "positive",
			Solved:     true,
			Confidence: 0.95,
		}
	}
	if negCount >= 2 && posCount == 0 {
		return models.SolverResult{
			Answer:     "negative",
			Solved:     true,
			Confidence: 0.95,
		}
	}

	// Ambiguous, mixed, or insufficient signals → route to cloud
	return models.SolverResult{Solved: false}
}

// countSignals counts how many words from the signal list appear in the text.
func countSignals(text string, signals []string) int {
	count := 0
	for _, w := range signals {
		if strings.Contains(text, w) {
			count++
		}
	}
	return count
}
