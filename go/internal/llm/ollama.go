package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// OllamaProvider implements Provider for local Ollama instances.
type OllamaProvider struct {
	baseURL   string
	fastModel string
	deepModel string
	http      http.Client
}

// NewOllamaProvider creates a provider for a local Ollama server.
func NewOllamaProvider(baseURL, fastModel, deepModel string) *OllamaProvider {
	return &OllamaProvider{
		baseURL:   baseURL,
		fastModel: fastModel,
		deepModel: deepModel,
	}
}

func (p *OllamaProvider) Name() string { return "ollama" }

func (p *OllamaProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	model := p.fastModel
	if req.IsDeepTier {
		model = p.deepModel
	}

	prompt := req.User
	if req.System != "" {
		prompt = req.System + "\n\n" + prompt
	}

	body := map[string]any{
		"model":  model,
		"prompt": prompt,
		"stream": false,
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("ollama: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/generate", bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("ollama: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("ollama: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ollama: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama: API error %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Response string `json:"response"`
	}

	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("ollama: unmarshal response: %w", err)
	}

	return result.Response, nil
}
