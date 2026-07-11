package classifier

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name     string
		prompt   string
		expected string
	}{
		// ── MATH ────────────────────────────────────────────────────────────────
		{"math: calculate keyword", "Calculate the sum of 15 and 27", "math"},
		{"math: what is X plus Y", "What is 144 plus 56?", "math"},
		{"math: sqrt word", "What is the square root of 144?", "math"},
		{"math: percentage", "What percentage of 200 is 50?", "math"},
		{"math: equation", "Solve the equation: 3x + 7 = 22", "math"},
		{"math: how many digits/operators", "How much is 7 multiplied by 8?", "math"},
		{"math: average", "Find the average of 10, 20, and 30", "math"},
		{"math: prime", "Is 97 a prime number?", "math"},
		{"math: area formula", "What is the area of a rectangle 5 by 8?", "math"},

		// ── SENTIMENT ───────────────────────────────────────────────────────────
		{"sentiment: classify keyword", "Classify the sentiment of this review: I love it!", "sentiment"},
		{"sentiment: positive or negative", "Is this statement positive or negative?", "sentiment"},
		{"sentiment: opinion keyword", "What is the overall opinion expressed here?", "sentiment"},
		{"sentiment: analyze feeling", "Analyze the feeling conveyed in this text", "sentiment"},
		{"sentiment: tone", "What tone does this passage convey?", "sentiment"},

		// ── NER ─────────────────────────────────────────────────────────────────
		{"ner: extract entities", "Extract all named entities from the following text", "ner"},
		{"ner: identify persons", "Identify all person names mentioned in this passage", "ner"},
		{"ner: organizations", "List the organizations referenced in the article", "ner"},
		{"ner: locations", "What locations are mentioned in the text?", "ner"},
		{"ner: entity recognition", "Perform named entity recognition on this paragraph", "ner"},

		// ── FACTUAL ─────────────────────────────────────────────────────────────
		{"factual: who is", "Who is the current president of France?", "factual"},
		{"factual: what is", "What is the capital of Japan?", "factual"},
		{"factual: when did", "When did World War II end?", "factual"},
		{"factual: define", "Define the term photosynthesis", "factual"},
		{"factual: how does", "How does the immune system work?", "factual"},
		{"factual: what year", "What year was the Eiffel Tower built?", "factual"},

		// ── SUMMARIZATION ───────────────────────────────────────────────────────
		{"summary: summarize keyword", "Summarize the following article in 3 sentences", "summarization"},
		{"summary: summary of", "Provide a summary of this document", "summarization"},
		{"summary: tldr", "TL;DR of the following passage:", "summarization"},
		{"summary: brief", "Give a brief overview of this text", "summarization"},
		{"summary: condense", "Condense the following into key points", "summarization"},
		{"summary: main points", "What are the main points of this article?", "summarization"},
		{"summary: key takeaways", "List the key takeaways from this report", "summarization"},

		// ── CODE GENERATION ─────────────────────────────────────────────────────
		{"codegen: write function", "Write a Python function to reverse a string", "code_generation"},
		{"codegen: implement", "Implement a binary search algorithm in Go", "code_generation"},
		{"codegen: create class", "Create a class in Java that represents a bank account", "code_generation"},
		{"codegen: write code", "Write code that sorts a list of numbers", "code_generation"},
		{"codegen: generate", "Generate a SQL query to find all users created this month", "code_generation"},
		{"codegen: program", "Write a program to calculate Fibonacci numbers", "code_generation"},

		// ── CODE DEBUGGING ──────────────────────────────────────────────────────
		{"debug: fix bug", "Fix the bug in this Python code snippet", "code_debugging"},
		{"debug: why error", "Why is this code throwing a NullPointerException?", "code_debugging"},
		{"debug: what wrong", "What is wrong with this JavaScript function?", "code_debugging"},
		{"debug: debug keyword", "Debug the following code and explain the issue", "code_debugging"},
		{"debug: error in code", "There is an error in this code, can you find it?", "code_debugging"},
		{"debug: not working", "This code is not working as expected, what's the problem?", "code_debugging"},

		// ── LOGICAL REASONING ───────────────────────────────────────────────────
		{"logical: if then", "If all mammals are warm-blooded and whales are mammals, are whales warm-blooded?", "logical"},
		{"logical: deduce", "Deduce the conclusion from these premises", "logical"},
		{"logical: infer", "What can we infer from the following statements?", "logical"},
		{"logical: logical", "Use logical reasoning to determine the answer", "logical"},
		{"logical: syllogism", "Evaluate this syllogism for validity", "logical"},
		{"logical: which must be true", "Which of the following must be true given these facts?", "logical"},
		{"logical: reasoning", "Apply deductive reasoning to solve this puzzle", "logical"},

		// ── GENERAL (AMBIGUOUS FALLBACK) ────────────────────────────────────────
		{"general: vague request", "Tell me something interesting", "general"},
		{"general: no keyword", "Give me your thoughts on this topic", "general"},
		{"general: completely ambiguous", "Help me with my project", "general"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Classify(tt.prompt)
			if got != tt.expected {
				t.Errorf("Classify(%q)\n  got:  %q\n  want: %q", tt.prompt, got, tt.expected)
			}
		})
	}
}
