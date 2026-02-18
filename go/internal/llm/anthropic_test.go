package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAnthropicProvider_Name(t *testing.T) {
	c := NewClient(Options{APIKey: "test", HaikuModel: "h", OpusModel: "o", MaxConcurrent: 1})
	p := NewAnthropicProvider(c)
	if p.Name() != "anthropic" {
		t.Errorf("expected name 'anthropic', got '%s'", p.Name())
	}
}

func TestAnthropicProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":[{"type":"text","text":"{\"key\":\"value\"}"}],"stop_reason":"end_turn"}`))
	}))
	defer srv.Close()

	c := NewClient(Options{APIKey: "test", HaikuModel: "h", OpusModel: "o", MaxConcurrent: 1, BaseURL: srv.URL})
	p := NewAnthropicProvider(c)

	result, err := p.Complete(context.Background(), CompletionRequest{
		User:      "test prompt",
		MaxTokens: 100,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}
