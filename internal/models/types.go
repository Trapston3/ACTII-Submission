package models

// Task represents a single task from /input/tasks.json.
// The category field is optional — the classifier derives it from prompt text.
type Task struct {
	TaskID   string `json:"task_id"`
	Category string `json:"category,omitempty"`
	Prompt   string `json:"prompt"`
}

// Result represents a single result entry for /output/results.json.
type Result struct {
	TaskID string `json:"task_id"`
	Answer string `json:"answer"`
}

// Category constants for the 8 evaluation domains.
const (
	CategoryMath      = "math"
	CategorySentiment = "sentiment"
	CategoryNER       = "ner"
	CategoryFactual   = "factual"
	CategorySummarize = "summarization"
	CategoryCodeGen   = "code_generation"
	CategoryCodeDebug = "code_debugging"
	CategoryLogical   = "logical"
	CategoryGeneral   = "general"
)

// SolverResult holds the output of the local zero-token solver.
// Confidence below 0.85 triggers the 1-token cloud validation gate.
type SolverResult struct {
	Answer     string
	Solved     bool
	Confidence float64 // 0.0–1.0
}

// ChatMessage is a single message in an OpenAI-compatible chat request.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatRequest is the OpenAI-compatible request body for the Fireworks API.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// ChatResponse is the OpenAI-compatible response from the Fireworks API.
type ChatResponse struct {
	Choices []ChatChoice `json:"choices"`
	Usage   TokenUsage   `json:"usage"`
}

// ChatChoice is a single generation choice in the chat response.
type ChatChoice struct {
	Message ChatMessage `json:"message"`
}

// TokenUsage tracks token consumption returned by the API.
// Accumulated by the TokenBreaker in the main orchestrator.
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}
