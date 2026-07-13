// Package client contains system prompts optimized to minimize input tokens
// and force direct, plain-text target responses from Fireworks models.
package client

// terseSuffix is appended to every system prompt to prevent models from
// generating markdown, explanations, or reasoning artifacts that the LLM
// judge will penalize.
const terseSuffix = " Do not explain. Do not use markdown. Plain text only."

// SystemPrompt returns a clear system prompt for the specified task category.
// Enforces formatting rules without contradictory or confusing constraints.
func SystemPrompt(category string) string {
	switch category {
	case "math":
		return "Solve the math problem. Output only the final numerical answer or expression. No explanations or markdown."
	case "sentiment":
		return "Classify sentiment as Positive, Negative, Neutral, or Mixed. Format: 'Label: One-sentence justification'. No conversational filler, intro, or markdown."
	case "ner":
		return "Extract named entities in format: Name (TYPE). Example: John Doe (PERSON), Paris (LOCATION). Comma-separated. No intro, filler, or markdown."
	case "factual":
		return "Answer directly, accurately, and concisely. Plain text only. No conversational filler, intro, or markdown."
	case "summarization":
		return "Summarize the text in 2-3 concise sentences. Plain text only. No conversational filler, intro, or markdown."
	case "code_generation":
		return "Write the requested code. Return ONLY the raw code. No markdown blocks, intro, or explanations."
	case "code_debugging":
		return "Fix the bug. Return ONLY the corrected code. No markdown blocks, intro, or explanations."
	case "logical":
		return "Solve the logic puzzle. Output ONLY the direct answer. Plain text only."
	default:
		return "Answer concisely. Plain text only."
	}
}

// MaxTokensForCategory returns the maximum completion tokens allowed for a
// given task category. These caps prevent models from burning tokens on
// verbose, rambling answers. Values are tuned for the evaluation harness.
func MaxTokensForCategory(category string) int {
	switch category {
	case "math":
		return 150 // Allow enough space for reasoning + expression output
	case "sentiment":
		return 300 // Label + one-sentence justification
	case "ner":
		return 300 // List of entities with types
	case "factual":
		return 300 // Concise 1-2 sentence response
	case "summarization":
		return 350 // Concise 2-3 sentence summary
	case "code_generation":
		return 800 // Code can be longer
	case "code_debugging":
		return 800 // Code can be longer
	case "logical":
		return 150 // Logic puzzles can have structured responses
	default:
		return 200 // General catch-all
	}
}
