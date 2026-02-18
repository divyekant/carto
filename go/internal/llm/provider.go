package llm

import "context"

// Provider abstracts an LLM backend (Anthropic, OpenAI, Ollama, etc.).
type Provider interface {
	// Complete sends a prompt and returns the text response.
	Complete(ctx context.Context, req CompletionRequest) (string, error)
	// Name returns the provider identifier (e.g., "anthropic", "openai").
	Name() string
}

// CompletionRequest is a provider-agnostic request.
type CompletionRequest struct {
	Model     string
	System    string
	User      string
	MaxTokens int
	// IsDeepTier signals this is an expensive/deep analysis call.
	IsDeepTier bool
}
