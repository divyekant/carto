package llm

import (
	"context"
	"fmt"
)

// AnthropicProvider implements Provider for the Anthropic API.
type AnthropicProvider struct {
	client *Client
}

// NewAnthropicProvider wraps an existing Client as a Provider.
func NewAnthropicProvider(c *Client) *AnthropicProvider {
	return &AnthropicProvider{client: c}
}

func (p *AnthropicProvider) Name() string { return "anthropic" }

func (p *AnthropicProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	tier := TierHaiku
	if req.IsDeepTier {
		tier = TierOpus
	}

	opts := &CompleteOptions{
		System:    req.System,
		MaxTokens: req.MaxTokens,
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = 4096
	}

	raw, err := p.client.CompleteJSON(req.User, tier, opts)
	if err != nil {
		return "", fmt.Errorf("anthropic: %w", err)
	}
	return string(raw), nil
}
