package client

import (
	"strings"
	"testing"
)

func TestAuditOutput_Math(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"plain number", "42", "42"},
		{"preamble padding", "The final answer is 42", "42"},
		{"expression", "2 + 3", "2 + 3"},
		{"negative number", "Result: -15.5", "-15.5"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuditOutput("math", tt.input, "")
			if got != tt.expected {
				t.Errorf("AuditOutput(math, %q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAuditOutput_NER(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"numbered list", "1. Apple\n2. Google\n3. AMD", "Apple, Google, AMD"},
		{"bulleted list", "- France\n* Germany\n• Italy", "France, Germany, Italy"},
		{"comma separated already", "John, Paul, George, Ringo", "John, Paul, George, Ringo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuditOutput("ner", tt.input, "")
			if got != tt.expected {
				t.Errorf("AuditOutput(ner, %q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAuditOutput_Sentiment(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"valid label and description",
			"positive. The customer is highly satisfied with the fast shipping.",
			"Positive: The customer is highly satisfied with the fast shipping.",
		},
		{
			"missing label",
			"The customer expresses frustration and wants a refund.",
			"Neutral: The customer expresses frustration and wants a refund.",
		},
		{
			"long justification sentence limiting",
			"Negative: The food was cold. The service was also bad. I will never eat here again.",
			"Negative: The food was cold. The service was also bad.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuditOutput("sentiment", tt.input, "")
			if got != tt.expected {
				t.Errorf("AuditOutput(sentiment, %q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestAuditOutput_BulletedList(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			"three compliant bullets",
			"- First bullet point under fifteen words.\n- Second bullet point is also short.\n- Third compliant point.",
			"- First bullet point under fifteen words.\n- Second bullet point is also short.\n- Third compliant point.",
		},
		{
			"bullet point too long hard truncation",
			"- First bullet point is extremely long and will be truncated because it has more than fifteen words in total.\n- Second short bullet.\n- Third short bullet.",
			"- First bullet point is extremely long and will be truncated because it has more.\n- Second short bullet.\n- Third short bullet.",
		},
		{
			"bullet point trailing stop word truncation",
			"- First bullet point is extremely long and will be truncated because it has to wait until next week.\n- Second short bullet.\n- Third short bullet.",
			"- First bullet point is extremely long and will be truncated because it has.\n- Second short bullet.\n- Third short bullet.",
		},
		{
			"too many bullets",
			"- One\n- Two\n- Three\n- Four\n- Five",
			"- One.\n- Two.\n- Three.",
		},
		{
			"too few bullets",
			"- One\n- Two",
			"- One.\n- Two.\n- Additional relevant point.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuditOutput("factual", tt.input, "")
			// Normalize newlines for Windows/Unix compatibility in comparison
			gotNorm := strings.ReplaceAll(got, "\r\n", "\n")
			expNorm := strings.ReplaceAll(tt.expected, "\r\n", "\n")
			if gotNorm != expNorm {
				t.Errorf("AuditOutput(bulled list, %q) =\n%q\nwant\n%q", tt.input, gotNorm, expNorm)
			}
		})
	}
}

func TestAuditOutput_SentenceLimiting(t *testing.T) {
	tests := []struct {
		name     string
		category string
		input    string
		expected string
	}{
		{
			"factual limit 2",
			"factual",
			"Paris is the capital of France. It is the most populous city in the country. It has a rich history.",
			"Paris is the capital of France. It is the most populous city in the country.",
		},
		{
			"summarization limit 3",
			"summarization",
			"Sentence one. Sentence two. Sentence three. Sentence four. Sentence five.",
			"Sentence one. Sentence two. Sentence three.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuditOutput(tt.category, tt.input, "")
			if got != tt.expected {
				t.Errorf("AuditOutput(%s, %q) = %q, want %q", tt.category, tt.input, got, tt.expected)
			}
		})
	}
}

func TestAuditOutput_DynamicSentenceLimiting(t *testing.T) {
	tests := []struct {
		name     string
		category string
		prompt   string
		input    string
		expected string
	}{
		{
			"dynamic limit 2 from prompt word",
			"summarization",
			"Summarize in exactly two sentences: ...",
			"First sentence. Second sentence. Third sentence.",
			"First sentence. Second sentence.",
		},
		{
			"dynamic limit 1 from prompt number",
			"factual",
			"Answer in 1 sentence only.",
			"First sentence. Second sentence.",
			"First sentence.",
		},
		{
			"dynamic limit 4 from prompt word",
			"general",
			"Write exactly four sentences.",
			"One. Two. Three. Four. Five. Six.",
			"One. Two. Three. Four.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AuditOutput(tt.category, tt.input, tt.prompt)
			if got != tt.expected {
				t.Errorf("AuditOutput(%s, %q, %q) = %q, want %q", tt.category, tt.input, tt.prompt, got, tt.expected)
			}
		})
	}
}
