package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "llama3.2" {
			t.Errorf("expected model 'llama3.2', got '%v'", body["model"])
		}
		if body["stream"] != false {
			t.Errorf("expected stream=false")
		}

		resp := map[string]any{"response": `{"answer":"hello"}`}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "llama3.2", "llama3.2:70b")
	result, err := p.Complete(context.Background(), CompletionRequest{User: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"answer":"hello"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestOllamaProvider_NoAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			t.Error("Ollama should not send Authorization header")
		}
		resp := map[string]any{"response": "ok"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOllamaProvider(srv.URL, "llama3.2", "llama3.2:70b")
	p.Complete(context.Background(), CompletionRequest{User: "test"})
}
