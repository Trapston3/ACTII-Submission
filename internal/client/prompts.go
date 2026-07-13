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
		return "Solve the mathematical problem. Return ONLY the final numerical answer or expression. Do not show work or explanations unless requested by the prompt. Plain text only. No markdown."
	case "sentiment":
		return "Classify the sentiment of the text. State the label (Positive, Negative, Mixed, or Neutral), followed by a colon and a one-sentence justification. Example: 'Positive: The tone is highly enthusiastic and appreciative.' Plain text only."
	case "ner":
		return "Identify all named entities mentioned in the input text and extract them with their corresponding type labels. Format the output as a comma-separated list where each entity is followed by its type in parentheses, like: Entity Name (TYPE). Example: Barack Obama (PERSON), Hawaii (LOCATION). Plain text only."
	case "factual":
		return "Answer the question directly, accurately, and concisely. Fulfill all parts of the question. Plain text only."
	case "summarization":
		return "Summarize the text in 2-3 concise sentences. Plain text only."
	case "code_generation":
		return "Write the requested code. Return ONLY the raw code. Do not wrap in markdown code blocks. Do not write any explanations or introduction."
	case "code_debugging":
		return "Fix the bug in the provided code. Return ONLY the corrected code. Do not wrap in markdown code blocks. Do not write any explanations."
	case "logical":
		return "Solve the logic puzzle. Return ONLY the direct answer. Plain text only."
	default:
		return "Answer the prompt concisely and directly. Plain text only."
	}
}

// MaxTokensForCategory returns the maximum completion tokens allowed for a
// given task category. These caps prevent models from burning tokens on
// verbose, rambling answers. Values are tuned for the evaluation harness.
func MaxTokensForCategory(category string) int {
	switch category {
	case "math":
		return 150 // Allow sufficient tokens for expression outputs
	case "sentiment":
		return 500 // Label + one-sentence justification (bumped to allow reasoning space)
	case "ner":
		return 200 // List of entities with types
	case "factual":
		return 250 // 1-2 sentences with explanation if needed
	case "summarization":
		return 300 // 2-3 sentences
	case "code_generation":
		return 800 // Code can be longer
	case "code_debugging":
		return 800 // Code can be longer
	case "logical":
		return 200 // Logic puzzles can have structured responses
	default:
		return 250 // General catch-all
	}
}
