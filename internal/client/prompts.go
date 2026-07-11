// Package client contains system prompts optimized to minimize input tokens
// and force direct, plain-text target responses from Fireworks models.
package client

// SystemPrompt returns a terse system prompt for the specified task category.
// Enforces formatting rules (e.g. no explanations, JSON, or reasoning).
func SystemPrompt(category string) string {
	switch category {
	case "math":
		return "Solve. Return ONLY the final numerical answer or expression. No steps, no text."
	case "sentiment":
		return "Classify sentiment as positive, negative, or neutral. Return ONLY the classification word."
	case "ner":
		return "Extract named entities. Return ONLY a comma-separated list. No other text."
	case "factual":
		return "Answer concisely. Return ONLY the direct answer. Keep it under 2 sentences."
	case "summarization":
		return "Summarize the text in 1-2 concise sentences. No introductory text."
	case "code_generation":
		return "Write the code. Return ONLY the raw code without markdown blocks or explanation."
	case "code_debugging":
		return "Fix the code. Return ONLY the corrected code. No explanations, no markdown blocks."
	case "logical":
		return "Solve. Return ONLY the final answer. No reasoning, no steps."
	default:
		return "Answer the prompt concisely and directly."
	}
}
