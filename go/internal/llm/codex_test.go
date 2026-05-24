package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestCodexAuth(t *testing.T, codexHome, accessToken, refreshToken, accountID string) {
	t.Helper()
	if err := os.MkdirAll(codexHome, 0o700); err != nil {
		t.Fatalf("mkdir codex home: %v", err)
	}
	auth := map[string]any{
		"auth_mode": "chatgpt",
		"tokens": map[string]any{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"account_id":    accountID,
		},
	}
	data, err := json.Marshal(auth)
	if err != nil {
		t.Fatalf("marshal auth: %v", err)
	}
	if err := os.WriteFile(filepath.Join(codexHome, "auth.json"), data, 0o600); err != nil {
		t.Fatalf("write auth: %v", err)
	}
}

func TestCodexProvider_CompleteUsesCodexSessionAuthAndStreamsText(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	writeTestCodexAuth(t, codexHome, "access-token", "refresh-token", "account-123")

	var gotAuth string
	var gotAccount string
	var gotReq struct {
		Model string `json:"model"`
		Input []struct {
			Type    string `json:"type"`
			Role    string `json:"role"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"input"`
		Stream bool `json:"stream"`
		Store  bool `json:"store"`
	}
	var gotRaw map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		gotAuth = r.Header.Get("Authorization")
		gotAccount = r.Header.Get("ChatGPT-Account-ID")
		if err := json.NewDecoder(r.Body).Decode(&gotRaw); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		payload, _ := json.Marshal(gotRaw)
		if err := json.Unmarshal(payload, &gotReq); err != nil {
			t.Fatalf("decode typed request: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: response.output_text.delta\n"))
		w.Write([]byte(`data: {"type":"response.output_text.delta","delta":"{\"ok\""}` + "\n\n"))
		w.Write([]byte("event: response.output_text.delta\n"))
		w.Write([]byte(`data: {"type":"response.output_text.delta","delta":":true}"}` + "\n\n"))
		w.Write([]byte("event: response.completed\n"))
		w.Write([]byte(`data: {"type":"response.completed","response":{"status":"completed"}}` + "\n\n"))
	}))
	defer srv.Close()

	provider := NewCodexProvider(srv.URL, "gpt-5.4-mini", "gpt-5.5")
	got, err := provider.Complete(context.Background(), CompletionRequest{
		User:      "Return JSON.",
		MaxTokens: 128,
	})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}

	if got != `{"ok":true}` {
		t.Errorf("got %q, want JSON text", got)
	}
	if gotAuth != "Bearer access-token" {
		t.Errorf("Authorization = %q, want bearer token", gotAuth)
	}
	if gotAccount != "account-123" {
		t.Errorf("ChatGPT-Account-ID = %q, want account id", gotAccount)
	}
	if gotReq.Model != "gpt-5.4-mini" {
		t.Errorf("model = %q, want fast model", gotReq.Model)
	}
	if !gotReq.Stream || gotReq.Store {
		t.Errorf("stream/store = %v/%v, want true/false", gotReq.Stream, gotReq.Store)
	}
	if _, ok := gotRaw["max_output_tokens"]; ok {
		t.Errorf("codex request must omit max_output_tokens because the Codex backend rejects it")
	}
	if len(gotReq.Input) != 1 || gotReq.Input[0].Role != "user" || gotReq.Input[0].Content[0].Text != "Return JSON." {
		t.Errorf("unexpected input payload: %+v", gotReq.Input)
	}
}

func TestCodexProvider_CompleteRefreshesExpiredTokenAndPersistsAuth(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("CODEX_REFRESH_TOKEN_URL_OVERRIDE", "")
	writeTestCodexAuth(t, codexHome, expiredJWT(), "old-refresh", "account-123")

	refreshSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got struct {
			ClientID     string `json:"client_id"`
			GrantType    string `json:"grant_type"`
			RefreshToken string `json:"refresh_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatalf("decode refresh request: %v", err)
		}
		if got.GrantType != "refresh_token" || got.RefreshToken != "old-refresh" {
			t.Fatalf("unexpected refresh request: %+v", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"new-access","refresh_token":"new-refresh"}`))
	}))
	defer refreshSrv.Close()
	t.Setenv("CODEX_REFRESH_TOKEN_URL_OVERRIDE", refreshSrv.URL)

	apiSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer new-access" {
			t.Fatalf("Authorization = %q, want refreshed token", got)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: response.output_text.done\n"))
		w.Write([]byte(`data: {"type":"response.output_text.done","text":"ok"}` + "\n\n"))
	}))
	defer apiSrv.Close()

	provider := NewCodexProvider(apiSrv.URL, "gpt-5.4-mini", "gpt-5.5")
	got, err := provider.Complete(context.Background(), CompletionRequest{User: "hello"})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if got != "ok" {
		t.Errorf("got %q, want ok", got)
	}

	data, err := os.ReadFile(filepath.Join(codexHome, "auth.json"))
	if err != nil {
		t.Fatalf("read auth: %v", err)
	}
	if !strings.Contains(string(data), "new-access") || !strings.Contains(string(data), "new-refresh") {
		t.Fatalf("auth.json did not persist refreshed tokens: %s", data)
	}
}

func TestCodexProvider_RetriesRateLimitWithRetryAfter(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	writeTestCodexAuth(t, codexHome, "access-token", "refresh-token", "account-123")

	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"detail":"Rate limit exceeded"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("event: response.output_text.done\n"))
		w.Write([]byte(`data: {"type":"response.output_text.done","text":"ok after retry"}` + "\n\n"))
	}))
	defer srv.Close()

	provider := NewCodexProvider(srv.URL, "gpt-5.4-mini", "gpt-5.5")
	got, err := provider.Complete(context.Background(), CompletionRequest{User: "hello"})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if got != "ok after retry" {
		t.Errorf("got %q, want retry result", got)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestCodexProvider_RetriesTransientStreamReadError(t *testing.T) {
	codexHome := t.TempDir()
	t.Setenv("CODEX_HOME", codexHome)
	writeTestCodexAuth(t, codexHome, "access-token", "refresh-token", "account-123")

	attempts := 0
	provider := NewCodexProvider("http://codex.test", "gpt-5.4-mini", "gpt-5.5")
	provider.http = http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		attempts++
		if attempts == 1 {
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Retry-After": []string{"0"}},
				Body:       errReadCloser{err: errors.New("stream error: stream ID 3; INTERNAL_ERROR; received from peer")},
				Request:    req,
			}, nil
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body: io.NopCloser(strings.NewReader(
				"event: response.output_text.done\n" +
					`data: {"type":"response.output_text.done","text":"ok after stream retry"}` + "\n\n",
			)),
			Request: req,
		}, nil
	})}

	got, err := provider.Complete(context.Background(), CompletionRequest{User: "hello"})
	if err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	if got != "ok after stream retry" {
		t.Errorf("got %q, want retry result", got)
	}
	if attempts != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type errReadCloser struct {
	err error
}

func (r errReadCloser) Read([]byte) (int, error) { return 0, r.err }
func (r errReadCloser) Close() error             { return nil }

func expiredJWT() string {
	return "eyJhbGciOiJub25lIn0.eyJleHAiOjF9."
}
