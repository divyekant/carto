package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// ProviderAdapter wraps a Provider to satisfy the pipeline's LLMClient
// interface (CompleteJSON). For Anthropic this returns the native Client
// which has CompleteJSON built-in. For other providers, it wraps
// Provider.Complete and extracts JSON from the text response.
type ProviderAdapter struct {
	provider Provider
	opts     Options
}

func (a *ProviderAdapter) CompleteJSON(prompt string, tier Tier, copts *CompleteOptions) (json.RawMessage, error) {
	req := CompletionRequest{
		User:       prompt,
		IsDeepTier: tier == TierDeep,
	}
	if copts != nil {
		req.System = copts.System
		req.MaxTokens = copts.MaxTokens
	}

	text, err := a.provider.Complete(context.Background(), req)
	if err != nil {
		return nil, err
	}

	// Extract the first JSON object from the response text.
	cleaned := stripMarkdownFences(text)
	start := strings.Index(cleaned, "{")
	if start == -1 {
		return nil, fmt.Errorf("llm: no JSON object found in provider response")
	}
	// Find matching closing brace.
	depth := 0
	for i := start; i < len(cleaned); i++ {
		if cleaned[i] == '{' {
			depth++
		} else if cleaned[i] == '}' {
			depth--
			if depth == 0 {
				raw := json.RawMessage(cleaned[start : i+1])
				if json.Valid(raw) {
					return raw, nil
				}
				return nil, fmt.Errorf("llm: extracted JSON is invalid")
			}
		}
	}
	return nil, fmt.Errorf("llm: unclosed JSON object in provider response")
}

// NewPipelineClient creates an LLM client suitable for the pipeline based on
// the provider name. For Anthropic it returns the native *Client (with
// CompleteJSON). For other providers it wraps them in a ProviderAdapter.
func NewPipelineClient(providerName string, opts Options) (interface {
	CompleteJSON(prompt string, tier Tier, opts *CompleteOptions) (json.RawMessage, error)
}, error) {
	switch providerName {
	case "anthropic", "":
		return NewClient(opts), nil
	default:
		p, err := NewProvider(providerName, opts)
		if err != nil {
			return nil, err
		}
		return &ProviderAdapter{provider: p, opts: opts}, nil
	}
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
		return NewOpenAIProvider(baseURL, opts.APIKey, opts.FastModel, opts.DeepModel), nil
	case "ollama":
		baseURL := opts.BaseURL
		if baseURL == "" {
			baseURL = "http://localhost:11434"
		}
		return NewOllamaProvider(baseURL, opts.FastModel, opts.DeepModel), nil
	default:
		return nil, fmt.Errorf("llm: unknown provider %q (supported: anthropic, openai, openrouter, ollama)", name)
	}
}
