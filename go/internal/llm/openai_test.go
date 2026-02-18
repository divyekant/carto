package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAIProvider_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("unexpected auth: %s", r.Header.Get("Authorization"))
		}

		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "gpt-4o-mini" {
			t.Errorf("expected model 'gpt-4o-mini', got '%v'", body["model"])
		}

		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": `{"answer":"42"}`}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "test-key", "gpt-4o-mini", "gpt-4o")
	result, err := p.Complete(context.Background(), CompletionRequest{User: "test", MaxTokens: 100})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"answer":"42"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestOpenAIProvider_DeepTier(t *testing.T) {
	var receivedModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		receivedModel = body["model"].(string)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "ok"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := NewOpenAIProvider(srv.URL, "test-key", "gpt-4o-mini", "gpt-4o")
	p.Complete(context.Background(), CompletionRequest{User: "deep", IsDeepTier: true})

	if receivedModel != "gpt-4o" {
		t.Errorf("expected 'gpt-4o' for deep tier, got '%s'", receivedModel)
	}
}
