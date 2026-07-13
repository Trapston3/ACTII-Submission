package local

import "testing"

func TestSolve_MathStrictNumerical(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		wantSolved bool
		wantAnswer string
	}{
		// ── MUST SOLVE (strict numerical only) ──────────────────────────────────
		{"integer addition", "2 + 3", true, "5"},
		{"integer subtraction", "50 - 23", true, "27"},
		{"integer multiplication", "15 * 7", true, "105"},
		{"integer division exact", "100 / 4", true, "25"},
		{"decimal addition", "3.5 + 1.5", true, "5"},
		{"decimal multiplication", "2.5 * 4", true, "10"},
		{"parentheses grouping", "(2 + 3) * 4", true, "20"},
		{"nested parentheses", "(10 - 4) * (2 + 1)", true, "18"},
		{"exponent caret", "2 ^ 10", true, "1024"},
		{"sqrt function lowercase", "sqrt(144)", true, "12"},
		{"sqrt function uppercase", "SQRT(64)", true, "8"},
		{"negative numbers", "-5 + 10", true, "5"},
		{"chained operations", "1 + 2 + 3 + 4", true, "10"},
		{"whitespace variation", "  7   *   6  ", true, "42"},
		{"single number", "42", true, "42"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Solve("math", tt.prompt)
			if result.Solved != tt.wantSolved {
				t.Errorf("Solve(math, %q).Solved = %v, want %v", tt.prompt, result.Solved, tt.wantSolved)
				return
			}
			if tt.wantSolved && result.Answer != tt.wantAnswer {
				t.Errorf("Solve(math, %q).Answer = %q, want %q", tt.prompt, result.Answer, tt.wantAnswer)
			}
		})
	}
}

func TestSolve_MathWordProblems_MustNOTSolve(t *testing.T) {
	// These MUST return solved:false — word math risks miscalculation
	wordProblems := []string{
		"If a campus drone flies 5 miles north then 3 miles east, how far is it from start?",
		"A train leaves at 3pm going 60mph. Another train leaves at 4pm going 80mph. When do they meet?",
		"Calculate the area of a circle with radius 7",
		"What is 15 percent of 200 dollars?",
		"John has 5 apples and gives 2 to Mary. How many does he have?",
		"How many miles is it from London to Paris?",
		"If there are 12 months in a year, how many weeks are in 3 years?",
		"A rectangle has a width of 4 meters and a height of 6 meters. What is its area?",
		"What is the square root of 144 plus 5?", // contains alphabetic words mixed in
		"Calculate the sum of the first 10 natural numbers",
	}

	for _, prompt := range wordProblems {
		t.Run(prompt[:min(len(prompt), 50)], func(t *testing.T) {
			result := Solve("math", prompt)
			if result.Solved {
				t.Errorf("Solve(math, %q) should NOT solve word problems, but got answer: %q",
					prompt, result.Answer)
			}
		})
	}
}

func TestSolve_SentimentHighConfidence(t *testing.T) {
	tests := []struct {
		name       string
		prompt     string
		wantSolved bool
		wantAnswer string
	}{
		// ── Strong positive (2+ positive signals, 0 negative) ───────────────────
		{"strong love+amazing", "I absolutely love this amazing product!", true, "Positive: The statement contains highly positive and enthusiastic wording."},
		{"strong excellent+fantastic", "The service was excellent and fantastic", true, "Positive: The statement contains highly positive and enthusiastic wording."},
		{"strong wonderful+great", "This is a wonderful and great experience", true, "Positive: The statement contains highly positive and enthusiastic wording."},
		{"strong best+perfect", "This is the best and most perfect meal ever", true, "Positive: The statement contains highly positive and enthusiastic wording."},

		// ── Strong negative (2+ negative signals, 0 positive) ───────────────────
		{"strong terrible+hate", "This is terrible and I hate it with passion", true, "Negative: The statement contains critical and negative wording."},
		{"strong awful+horrible", "Awful and horrible experience, never returning", true, "Negative: The statement contains critical and negative wording."},
		{"strong worst+disgusting", "The worst, most disgusting service I've had", true, "Negative: The statement contains critical and negative wording."},

		// ── Ambiguous/mixed — MUST NOT solve locally ────────────────────────────
		{"mixed positive+negative", "The food was great but the service was terrible", false, ""},
		{"weak single signal", "It was okay I guess", false, ""},
		{"neutral statement", "The package arrived on Tuesday", false, ""},
		{"single positive only", "I love this", false, ""},   // only 1 signal
		{"descriptive no emotion", "The product is blue and weighs 2kg", false, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Solve("sentiment", tt.prompt)
			if result.Solved != tt.wantSolved {
				t.Errorf("Solve(sentiment, %q).Solved = %v, want %v",
					tt.prompt, result.Solved, tt.wantSolved)
				return
			}
			if tt.wantSolved && result.Answer != tt.wantAnswer {
				t.Errorf("Solve(sentiment, %q).Answer = %q, want %q",
					tt.prompt, result.Answer, tt.wantAnswer)
			}
		})
	}
}

func TestSolve_WrongCategory_NeverSolves(t *testing.T) {
	// Local solver only handles math and sentiment — everything else skips
	skippedCategories := []string{"ner", "factual", "summarization", "code_generation", "code_debugging", "logical", "general"}
	for _, cat := range skippedCategories {
		t.Run("skip_"+cat, func(t *testing.T) {
			result := Solve(cat, "some prompt text")
			if result.Solved {
				t.Errorf("Solve(%q, ...) should never solve non-math/sentiment categories", cat)
			}
		})
	}
}

func TestSolve_ConfidenceGate(t *testing.T) {
	// Solved math should always have confidence 1.0
	result := Solve("math", "7 * 6")
	if !result.Solved {
		t.Fatal("expected math to be solved")
	}
	if result.Confidence < 0.99 {
		t.Errorf("solved math should have confidence ~1.0, got %.2f", result.Confidence)
	}

	// Strong sentiment should have confidence >= 0.9
	result = Solve("sentiment", "This is the most terrible and horrible thing ever")
	if !result.Solved {
		t.Fatal("expected high-confidence sentiment to be solved")
	}
	if result.Confidence < 0.9 {
		t.Errorf("strong sentiment should have confidence >= 0.9, got %.2f", result.Confidence)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
