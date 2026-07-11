// Package client contains system prompts optimized to minimize input tokens
// and force direct, plain-text target responses from Fireworks models.
package client

// terseSuffix is appended to every system prompt to prevent models from
// generating markdown, explanations, or reasoning artifacts that the LLM
// judge will penalize.
const terseSuffix = " Do not explain. Do not use markdown. Plain text only."

// SystemPrompt returns a terse system prompt for the specified task category.
// Enforces formatting rules (e.g. no explanations, JSON, or reasoning).
func SystemPrompt(category string) string {
	switch category {
	case "math":
		return "Solve. Return ONLY the final numerical answer or expression. No steps, no text." + terseSuffix
	case "sentiment":
		return "Classify sentiment as Positive, Negative, Mixed, or Neutral. State the label, then give a one-sentence reason." + terseSuffix
	case "ner":
		return "Extract named entities. Return ONLY a comma-separated list. No other text." + terseSuffix
	case "factual":
		return "Answer concisely. Return ONLY the direct answer. Keep it under 2 sentences." + terseSuffix
	case "summarization":
		return "Summarize the text in 2-3 concise sentences. No introductory text." + terseSuffix
	case "code_generation":
		return "Write the code. Return ONLY the raw code without markdown blocks or explanation." + terseSuffix
	case "code_debugging":
		return "Fix the code. Return ONLY the corrected code. No explanations, no markdown blocks." + terseSuffix
	case "logical":
		return "Solve. Return ONLY the final answer. No reasoning, no steps." + terseSuffix
	default:
		return "Answer the prompt concisely and directly." + terseSuffix
	}
}

// MaxTokensForCategory returns the maximum completion tokens allowed for a
// given task category. These caps prevent models from burning tokens on
// verbose, rambling answers. Values are tuned for the evaluation harness.
func MaxTokensForCategory(category string) int {
	switch category {
	case "math":
		return 30 // Single number or expression
	case "sentiment":
		return 50 // Label + one-sentence justification
	case "ner":
		return 80 // Comma-separated entity list
	case "factual":
		return 150 // 1-2 sentences (bumped per judge requirements)
	case "summarization":
		return 200 // 2-3 sentences (bumped per judge requirements)
	case "code_generation":
		return 300 // Raw code block
	case "code_debugging":
		return 300 // Corrected code
	case "logical":
		return 80 // Direct conclusion
	default:
		return 150 // General catch-all
	}
}
