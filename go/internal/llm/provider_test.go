package llm

import (
	"context"
	"encoding/json"
	"testing"
)

func TestNewProvider_Anthropic(t *testing.T) {
	p, err := NewProvider("anthropic", Options{APIKey: "test", FastModel: "h", DeepModel: "o", MaxConcurrent: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("expected 'anthropic', got '%s'", p.Name())
	}
}

func TestNewProvider_Empty(t *testing.T) {
	p, err := NewProvider("", Options{APIKey: "test", FastModel: "h", DeepModel: "o", MaxConcurrent: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("expected 'anthropic' for empty provider, got '%s'", p.Name())
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	p, err := NewProvider("openai", Options{APIKey: "test", FastModel: "gpt-4o-mini", DeepModel: "gpt-4o"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected 'openai', got '%s'", p.Name())
	}
}

func TestNewProvider_Ollama(t *testing.T) {
	p, err := NewProvider("ollama", Options{FastModel: "llama3.2", DeepModel: "llama3.2:70b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "ollama" {
		t.Errorf("expected 'ollama', got '%s'", p.Name())
	}
}

func TestNewProvider_Codex(t *testing.T) {
	p, err := NewProvider("codex", Options{FastModel: "gpt-5.4-mini", DeepModel: "gpt-5.5"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "codex" {
		t.Errorf("expected 'codex', got '%s'", p.Name())
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	_, err := NewProvider("gemini", Options{})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

type textProvider struct {
	text string
}

func (p textProvider) Complete(_ context.Context, _ CompletionRequest) (string, error) {
	return p.text, nil
}

func (p textProvider) Name() string { return "text" }

func TestProviderAdapter_CompleteJSONHandlesBracesInsideStrings(t *testing.T) {
	adapter := &ProviderAdapter{provider: textProvider{text: "```json\n{\"summary\":\"uses { braces } in text\",\"ok\":true}\n```"}}

	raw, err := adapter.CompleteJSON("prompt", TierFast, nil)
	if err != nil {
		t.Fatalf("CompleteJSON returned error: %v", err)
	}

	var got struct {
		Summary string `json:"summary"`
		OK      bool   `json:"ok"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.Summary != "uses { braces } in text" || !got.OK {
		t.Fatalf("unexpected JSON: %+v", got)
	}
}

func TestProviderAdapter_CompleteJSONCanExtractArrays(t *testing.T) {
	adapter := &ProviderAdapter{provider: textProvider{text: "```json\n[{\"index\":0,\"summary\":\"one\"},{\"index\":1,\"summary\":\"two\"}]\n```"}}

	raw, err := adapter.CompleteJSON("prompt", TierFast, nil)
	if err != nil {
		t.Fatalf("CompleteJSON returned error: %v", err)
	}

	var got []struct {
		Index   int    `json:"index"`
		Summary string `json:"summary"`
	}
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got) != 2 || got[1].Summary != "two" {
		t.Fatalf("unexpected JSON array: %+v", got)
	}
}
