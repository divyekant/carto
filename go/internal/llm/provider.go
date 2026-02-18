package llm

import (
	"context"
	"fmt"
)

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

// NewProvider creates the appropriate Provider based on the provider name.
func NewProvider(name string, opts Options) (Provider, error) {
	switch name {
	case "anthropic", "":
		c := NewClient(opts)
		return NewAnthropicProvider(c), nil
	case "openai", "openrouter":
		baseURL := opts.BaseURL
		if baseURL == "" {
			if name == "openrouter" {
				baseURL = "https://openrouter.ai/api"
			} else {
				baseURL = "https://api.openai.com"
			}
		}
		return NewOpenAIProvider(baseURL, opts.APIKey, opts.HaikuModel, opts.OpusModel), nil
	case "ollama":
		baseURL := opts.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return NewOllamaProvider(baseURL, opts.HaikuModel, opts.OpusModel), nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q (supported: anthropic, openai, openrouter, ollama)", name)
	}
}
