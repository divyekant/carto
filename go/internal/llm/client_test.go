package llm

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeMessagesHandler returns a handler that responds with a valid Anthropic
// Messages API response containing the given text.
func fakeMessagesHandler(text string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": text},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func TestClient_Complete(t *testing.T) {
	var gotReq apiRequest
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()

		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotReq)

		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "hello back"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(Options{
		APIKey:  "sk-test-key",
		BaseURL: srv.URL,
	})

	result, err := c.Complete("hello", TierHaiku, &CompleteOptions{
		System:    "you are helpful",
		MaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if result != "hello back" {
		t.Errorf("got result %q, want %q", result, "hello back")
	}

	// Verify request body.
	if gotReq.Model != "claude-haiku-4-5-20251001" {
		t.Errorf("got model %q, want %q", gotReq.Model, "claude-haiku-4-5-20251001")
	}
	if gotReq.MaxTokens != 1024 {
		t.Errorf("got max_tokens %d, want 1024", gotReq.MaxTokens)
	}
	if gotReq.System != "you are helpful" {
		t.Errorf("got system %q, want %q", gotReq.System, "you are helpful")
	}
	if len(gotReq.Messages) != 1 || gotReq.Messages[0].Content != "hello" {
		t.Errorf("unexpected messages: %+v", gotReq.Messages)
	}

	// Verify headers for API key mode (not OAuth).
	if got := gotHeaders.Get("X-Api-Key"); got != "sk-test-key" {
		t.Errorf("got X-Api-Key %q, want %q", got, "sk-test-key")
	}
	if got := gotHeaders.Get("Content-Type"); got != "application/json" {
		t.Errorf("got Content-Type %q, want %q", got, "application/json")
	}
	if got := gotHeaders.Get("Anthropic-Version"); got != "2023-06-01" {
		t.Errorf("got Anthropic-Version %q, want %q", got, "2023-06-01")
	}
	if got := gotHeaders.Get("Authorization"); got != "" {
		t.Errorf("API key mode should not set Authorization, got %q", got)
	}
}

func TestClient_Semaphore(t *testing.T) {
	var inflight atomic.Int32
	var maxInflight atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inflight.Add(1)
		// Track maximum concurrent requests observed.
		for {
			old := maxInflight.Load()
			if cur <= old || maxInflight.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		inflight.Add(-1)

		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "ok"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(Options{
		APIKey:        "sk-test",
		BaseURL:       srv.URL,
		MaxConcurrent: 2,
	})

	var wg sync.WaitGroup
	errs := make([]error, 5)
	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = c.Complete("test", TierHaiku, nil)
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("request %d failed: %v", i, err)
		}
	}

	if peak := maxInflight.Load(); peak > 2 {
		t.Errorf("max inflight was %d, want <= 2", peak)
	}
}

func TestClient_OAuthHeaders(t *testing.T) {
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()

		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "ok"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(Options{
		APIKey:  "oauth-token-123",
		BaseURL: srv.URL,
		IsOAuth: true,
	})

	_, err := c.Complete("hi", TierOpus, nil)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	// Must have Bearer token.
	if got := gotHeaders.Get("Authorization"); got != "Bearer oauth-token-123" {
		t.Errorf("got Authorization %q, want %q", got, "Bearer oauth-token-123")
	}

	// Must NOT have X-Api-Key.
	if got := gotHeaders.Get("X-Api-Key"); got != "" {
		t.Errorf("OAuth mode should not set X-Api-Key, got %q", got)
	}

	// Must have the beta header.
	if got := gotHeaders.Get("Anthropic-Beta"); got != "oauth-2025-04-20,interleaved-thinking-2025-05-14" {
		t.Errorf("got Anthropic-Beta %q, want %q", got, "oauth-2025-04-20,interleaved-thinking-2025-05-14")
	}
}

func TestClient_OAuthHeaders_HaikuExcludesThinking(t *testing.T) {
	var gotHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeaders = r.Header.Clone()

		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "ok"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(Options{
		APIKey:  "oauth-token-456",
		BaseURL: srv.URL,
		IsOAuth: true,
	})

	_, err := c.Complete("hi", TierHaiku, nil)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	// Haiku must have OAuth beta but NOT thinking beta.
	got := gotHeaders.Get("Anthropic-Beta")
	if got != OAuthBeta {
		t.Errorf("got Anthropic-Beta %q, want %q (OAuth only, no thinking)", got, OAuthBeta)
	}
}

func TestClient_CompleteJSON(t *testing.T) {
	cases := []struct {
		name     string
		response string
		wantKey  string
		wantVal  string
	}{
		{
			name:     "plain json",
			response: `{"key": "value"}`,
			wantKey:  "key",
			wantVal:  "value",
		},
		{
			name:     "markdown fenced",
			response: "```json\n{\"key\": \"fenced\"}\n```",
			wantKey:  "key",
			wantVal:  "fenced",
		},
		{
			name:     "text before json",
			response: "Here is the result:\n{\"key\": \"after-text\"}",
			wantKey:  "key",
			wantVal:  "after-text",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(fakeMessagesHandler(tc.response))
			defer srv.Close()

			c := NewClient(Options{APIKey: "sk-test", BaseURL: srv.URL})

			raw, err := c.CompleteJSON("give json", TierHaiku, nil)
			if err != nil {
				t.Fatalf("CompleteJSON returned error: %v", err)
			}

			var m map[string]string
			if err := json.Unmarshal(raw, &m); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}
			if m[tc.wantKey] != tc.wantVal {
				t.Errorf("got %q=%q, want %q", tc.wantKey, m[tc.wantKey], tc.wantVal)
			}
		})
	}
}

func TestClient_OpusModel(t *testing.T) {
	var gotReq apiRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &gotReq)

		resp := map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "ok"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	c := NewClient(Options{APIKey: "sk-test", BaseURL: srv.URL})

	_, err := c.Complete("hi", TierOpus, nil)
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if gotReq.Model != "claude-opus-4-6" {
		t.Errorf("got model %q, want %q", gotReq.Model, "claude-opus-4-6")
	}
}
