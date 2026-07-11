package optimizer

import (
	"strings"
	"testing"
)

func TestOptimize_StripLeadingFiller(t *testing.T) {
	cases := []struct {
		input    string
		wantGone string
	}{
		{"Please help me calculate the sum of 2 and 3.", "Please help me"},
		{"Could you please write a Python function to reverse a string?", "Could you please"},
		{"Can you help me find the capital of France?", "Can you help me"},
		{"I'd like you to summarize this article for me.", "I'd like you to"},
		{"I would like you to explain photosynthesis.", "I would like you to"},
		{"Please write code that sorts a list.", "Please write"},
		{"Could you explain how neural networks work?", "Could you"},
		{"Can you please tell me who invented the telephone?", "Can you please"},
		{"I need you to fix this bug in my code.", "I need you to"},
		{"Help me understand recursion in programming.", "Help me understand"},
	}

	for _, tt := range cases {
		t.Run(tt.input[:min(len(tt.input), 45)], func(t *testing.T) {
			got := Optimize("general", tt.input)
			if strings.Contains(strings.ToLower(got), strings.ToLower(tt.wantGone)) {
				t.Errorf("Optimize(%q)\n  result still contains filler %q\n  got: %q",
					tt.input, tt.wantGone, got)
			}
		})
	}
}

func TestOptimize_StripTrailingFiller(t *testing.T) {
	cases := []struct {
		input    string
		wantGone string
	}{
		{"Calculate 2 + 2. Thank you!", "Thank you"},
		{"Summarize this article. Thanks in advance.", "Thanks in advance"},
		{"What is the capital of France? Thank you very much.", "Thank you very much"},
		{"Fix this bug please.", "please"},
	}

	for _, tt := range cases {
		t.Run(tt.input[:min(len(tt.input), 45)], func(t *testing.T) {
			got := Optimize("general", tt.input)
			if strings.Contains(strings.ToLower(got), strings.ToLower(tt.wantGone)) {
				t.Errorf("Optimize(%q)\n  result still contains trailing filler %q\n  got: %q",
					tt.input, tt.wantGone, got)
			}
		})
	}
}

func TestOptimize_PreservesCoreContent(t *testing.T) {
	// After stripping filler, the substantive content must remain
	cases := []struct {
		input      string
		wantCore   string
	}{
		{"Please help me calculate the square root of 144.", "square root of 144"},
		{"Could you write a Python function to reverse a string?", "Python function"},
		{"Can you tell me who is the president of France?", "president of France"},
		{"I'd like you to summarize the following article:", "summarize"},
		{"Could you debug this code and explain the error?", "debug"},
	}

	for _, tt := range cases {
		t.Run(tt.wantCore, func(t *testing.T) {
			got := Optimize("general", tt.input)
			if !strings.Contains(strings.ToLower(got), strings.ToLower(tt.wantCore)) {
				t.Errorf("Optimize(%q)\n  should preserve %q\n  got: %q",
					tt.input, tt.wantCore, got)
			}
		})
	}
}

func TestOptimize_WhitespaceNormalization(t *testing.T) {
	cases := []string{
		"Calculate    the   sum\n\nof    two   numbers",
		"What  is   the   capital   of   France?",
		"  leading and trailing spaces  ",
		"Multiple\n\nnewlines\n\nin\n\nprompt",
	}

	for _, input := range cases {
		t.Run(input[:min(len(input), 40)], func(t *testing.T) {
			got := Optimize("general", input)
			if strings.Contains(got, "  ") {
				t.Errorf("Optimize(%q) has double spaces: %q", input, got)
			}
			if strings.Contains(got, "\n") {
				t.Errorf("Optimize(%q) has newlines: %q", input, got)
			}
			if got != strings.TrimSpace(got) {
				t.Errorf("Optimize(%q) has leading/trailing whitespace: %q", input, got)
			}
		})
	}
}

func TestOptimize_VaultGuard_EmailScrubbing(t *testing.T) {
	cases := []struct {
		input string
	}{
		{"Send the results to john.doe@example.com and process the data"},
		{"Contact us at support@company.org for assistance"},
		{"Email admin@test.co.uk with your response"},
	}

	for _, tt := range cases {
		t.Run(tt.input[:min(len(tt.input), 45)], func(t *testing.T) {
			got := Optimize("general", tt.input)
			if strings.Contains(got, "@") {
				t.Errorf("Optimize(%q)\n  email not scrubbed, still contains '@'\n  got: %q",
					tt.input, got)
			}
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("Optimize(%q)\n  expected [REDACTED] placeholder\n  got: %q",
					tt.input, got)
			}
		})
	}
}

func TestOptimize_VaultGuard_PhoneScrubbing(t *testing.T) {
	cases := []struct {
		input string
	}{
		{"Call me at 555-123-4567 for the answer"},
		{"My number is (800) 555-0100 please call"},
		{"Reach us at 555.867.5309 anytime"},
		{"Phone: 18005550100 is our hotline"},
	}

	for _, tt := range cases {
		t.Run(tt.input[:min(len(tt.input), 45)], func(t *testing.T) {
			got := Optimize("general", tt.input)
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("Optimize(%q)\n  phone not scrubbed, expected [REDACTED]\n  got: %q",
					tt.input, got)
			}
		})
	}
}

func TestOptimize_VaultGuard_SSNScrubbing(t *testing.T) {
	cases := []struct {
		input string
	}{
		{"My SSN is 123-45-6789 please verify"},
		{"Social security number: 987-65-4321"},
	}

	for _, tt := range cases {
		t.Run(tt.input[:min(len(tt.input), 45)], func(t *testing.T) {
			got := Optimize("general", tt.input)
			if !strings.Contains(got, "[REDACTED]") {
				t.Errorf("Optimize(%q)\n  SSN not scrubbed, expected [REDACTED]\n  got: %q",
					tt.input, got)
			}
		})
	}
}

func TestOptimize_EmptyAndPassthrough(t *testing.T) {
	// Empty input → empty output (no panic)
	if got := Optimize("general", ""); got != "" {
		t.Errorf("Optimize('') = %q, want ''", got)
	}

	// Pure technical prompts with no filler should survive intact (modulo whitespace)
	technical := "Write a function: func add(a, b int) int"
	got := Optimize("general", technical)
	if !strings.Contains(got, "func add") {
		t.Errorf("Optimize(%q) destroyed technical content, got: %q", technical, got)
	}
}

func TestOptimize_PIIScrubbingBypass(t *testing.T) {
	tests := []struct {
		name       string
		category   string
		input      string
		shouldScrub bool
	}{
		{"general email", "general", "Send report to test@email.com.", true},
		{"ner email", "ner", "Extract email test@email.com from here.", false},
		{"factual phone", "factual", "Find the owner of 555-123-4567 please.", false},
		{"code_debugging ssn", "code_debugging", "Debug this SSN check function for 123-45-6789", false},
		{"math email", "math", "Calculate value for user test@email.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Optimize(tt.category, tt.input)
			hasScrubbed := strings.Contains(got, "[REDACTED]")
			if tt.shouldScrub && !hasScrubbed {
				t.Errorf("expected PII to be scrubbed for category %q, got: %q", tt.category, got)
			}
			if !tt.shouldScrub && hasScrubbed {
				t.Errorf("expected PII bypass (NOT scrubbed) for category %q, got: %q", tt.category, got)
			}
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
