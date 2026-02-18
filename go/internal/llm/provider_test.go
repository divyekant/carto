package llm

import "testing"

func TestNewProvider_Anthropic(t *testing.T) {
	p, err := NewProvider("anthropic", Options{APIKey: "test", HaikuModel: "h", OpusModel: "o", MaxConcurrent: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("expected 'anthropic', got '%s'", p.Name())
	}
}

func TestNewProvider_Empty(t *testing.T) {
	p, err := NewProvider("", Options{APIKey: "test", HaikuModel: "h", OpusModel: "o", MaxConcurrent: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "anthropic" {
		t.Errorf("expected 'anthropic' for empty provider, got '%s'", p.Name())
	}
}

func TestNewProvider_OpenAI(t *testing.T) {
	p, err := NewProvider("openai", Options{APIKey: "test", HaikuModel: "gpt-4o-mini", OpusModel: "gpt-4o"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "openai" {
		t.Errorf("expected 'openai', got '%s'", p.Name())
	}
}

func TestNewProvider_Ollama(t *testing.T) {
	p, err := NewProvider("ollama", Options{HaikuModel: "llama3.2", OpusModel: "llama3.2:70b"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.Name() != "ollama" {
		t.Errorf("expected 'ollama', got '%s'", p.Name())
	}
}

func TestNewProvider_Unknown(t *testing.T) {
	_, err := NewProvider("gemini", Options{})
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}
