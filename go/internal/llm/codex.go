package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	defaultCodexBaseURL        = "https://chatgpt.com/backend-api/codex"
	defaultCodexFastModel      = "gpt-5.4-mini"
	defaultCodexDeepModel      = "gpt-5.5"
	codexRefreshTokenURL       = "https://auth.openai.com/oauth/token"
	codexRefreshTokenURLVar    = "CODEX_REFRESH_TOKEN_URL_OVERRIDE"
	codexOAuthClientID         = "app_EMoamEEZ73f0CkXaXp7hrann"
	codexAuthRefreshLeeway     = time.Minute
	codexCompletionHTTPTimeout = 5 * time.Minute
	codexMaxCompletionAttempts = 5
	codexMaxRetryDelay         = 30 * time.Second
)

// CodexProvider sends completion requests through the user's Codex ChatGPT
// session stored in ~/.codex/auth.json. It does not use provider API keys.
type CodexProvider struct {
	baseURL   string
	fastModel string
	deepModel string
	http      http.Client
	mu        sync.Mutex
}

func NewCodexProvider(baseURL, fastModel, deepModel string) *CodexProvider {
	if baseURL == "" {
		baseURL = defaultCodexBaseURL
	}
	if fastModel == "" {
		fastModel = defaultCodexFastModel
	}
	if deepModel == "" {
		deepModel = defaultCodexDeepModel
	}
	return &CodexProvider{
		baseURL:   strings.TrimRight(baseURL, "/"),
		fastModel: fastModel,
		deepModel: deepModel,
		http:      http.Client{Timeout: codexCompletionHTTPTimeout},
	}
}

func (p *CodexProvider) Name() string { return "codex" }

func (p *CodexProvider) Complete(ctx context.Context, req CompletionRequest) (string, error) {
	auth, err := p.loadUsableAuth()
	if err != nil {
		return "", err
	}

	var lastErr error
	refreshed := false
	for attempt := 0; attempt < codexMaxCompletionAttempts; attempt++ {
		text, status, retryAfter, err := p.completeWithAuth(ctx, req, auth)
		if err == nil {
			return text, nil
		}
		lastErr = err

		if (status == http.StatusUnauthorized || status == http.StatusForbidden) && !refreshed {
			auth, err = p.refreshAuth()
			if err != nil {
				return "", err
			}
			refreshed = true
			continue
		}

		if !isRetryableCodexError(status, err) || attempt == codexMaxCompletionAttempts-1 {
			break
		}
		if err := sleepCodexRetry(ctx, codexRetryDelay(attempt, retryAfter)); err != nil {
			return "", err
		}
	}
	return "", lastErr
}

type codexAuthFile struct {
	AuthMode    string           `json:"auth_mode"`
	OpenAIKey   *string          `json:"OPENAI_API_KEY,omitempty"`
	Tokens      codexAuthTokens  `json:"tokens"`
	LastRefresh *time.Time       `json:"last_refresh,omitempty"`
	Extra       *json.RawMessage `json:"-"`
}

type codexAuthTokens struct {
	IDToken      string `json:"id_token,omitempty"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	AccountID    string `json:"account_id"`
}

func (p *CodexProvider) loadUsableAuth() (codexAuthTokens, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	auth, path, err := readCodexAuth()
	if err != nil {
		return codexAuthTokens{}, err
	}
	if auth.Tokens.AccessToken == "" || auth.Tokens.AccountID == "" {
		return codexAuthTokens{}, fmt.Errorf("codex: ChatGPT auth is missing access token or account id; run `codex login`")
	}
	if tokenExpiresSoon(auth.Tokens.AccessToken, codexAuthRefreshLeeway) {
		refreshed, err := refreshCodexAuth(auth)
		if err != nil {
			return codexAuthTokens{}, err
		}
		if err := writeCodexAuth(path, refreshed); err != nil {
			return codexAuthTokens{}, err
		}
		auth = refreshed
	}
	return auth.Tokens, nil
}

func (p *CodexProvider) refreshAuth() (codexAuthTokens, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	auth, path, err := readCodexAuth()
	if err != nil {
		return codexAuthTokens{}, err
	}
	refreshed, err := refreshCodexAuth(auth)
	if err != nil {
		return codexAuthTokens{}, err
	}
	if err := writeCodexAuth(path, refreshed); err != nil {
		return codexAuthTokens{}, err
	}
	return refreshed.Tokens, nil
}

func (p *CodexProvider) completeWithAuth(ctx context.Context, req CompletionRequest, auth codexAuthTokens) (string, int, time.Duration, error) {
	model := p.fastModel
	if req.IsDeepTier {
		model = p.deepModel
	}
	body := codexResponseRequest{
		Model:             model,
		Instructions:      req.System,
		Input:             []codexResponseItem{newCodexUserMessage(req.User)},
		Tools:             []any{},
		ToolChoice:        "auto",
		ParallelToolCalls: false,
		Reasoning: &codexReasoning{
			Effort: "low",
		},
		Store:   false,
		Stream:  true,
		Include: []string{},
	}
	if req.IsDeepTier {
		body.Reasoning.Effort = "medium"
	}

	data, err := json.Marshal(body)
	if err != nil {
		return "", 0, 0, fmt.Errorf("codex: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(data))
	if err != nil {
		return "", 0, 0, fmt.Errorf("codex: create request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+auth.AccessToken)
	httpReq.Header.Set("ChatGPT-Account-ID", auth.AccountID)
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("User-Agent", UserAgent)

	resp, err := p.http.Do(httpReq)
	if err != nil {
		return "", 0, 0, fmt.Errorf("codex: request failed: %w", err)
	}
	defer resp.Body.Close()

	retryAfter, _ := parseRetryAfter(resp.Header.Get("Retry-After"))
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", resp.StatusCode, retryAfter, fmt.Errorf("codex: API error %d: %s", resp.StatusCode, string(respBody))
	}

	text, err := parseCodexSSE(resp.Body)
	if err != nil {
		return "", resp.StatusCode, retryAfter, err
	}
	return text, resp.StatusCode, retryAfter, nil
}

func isRetryableCodexError(status int, err error) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	if status == http.StatusTooManyRequests || status == http.StatusRequestTimeout || status == http.StatusConflict || status >= 500 {
		return true
	}

	msg := strings.ToLower(err.Error())
	if status == 0 {
		return strings.Contains(msg, "request failed") ||
			strings.Contains(msg, "connection reset") ||
			strings.Contains(msg, "server disconnected") ||
			strings.Contains(msg, "timeout") ||
			strings.Contains(msg, "temporarily unavailable")
	}
	if status == http.StatusOK {
		return strings.Contains(msg, "read sse") &&
			(strings.Contains(msg, "stream error") ||
				strings.Contains(msg, "internal_error") ||
				strings.Contains(msg, "unexpected eof") ||
				strings.Contains(msg, "connection reset") ||
				strings.Contains(msg, "server disconnected") ||
				strings.Contains(msg, "http2") ||
				strings.Contains(msg, "rate limit"))
	}
	return false
}

func codexRetryDelay(attempt int, retryAfter time.Duration) time.Duration {
	if retryAfter >= 0 {
		return retryAfter
	}
	delay := time.Duration(1<<uint(attempt)) * time.Second
	if delay > codexMaxRetryDelay {
		return codexMaxRetryDelay
	}
	return delay
}

func sleepCodexRetry(ctx context.Context, delay time.Duration) error {
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseRetryAfter(value string) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return -1, false
	}
	if seconds, err := time.ParseDuration(value + "s"); err == nil {
		return seconds, true
	}
	if when, err := http.ParseTime(value); err == nil {
		delay := time.Until(when)
		if delay < 0 {
			return 0, true
		}
		return delay, true
	}
	return -1, false
}

type codexResponseRequest struct {
	Model             string              `json:"model"`
	Instructions      string              `json:"instructions,omitempty"`
	Input             []codexResponseItem `json:"input"`
	Tools             []any               `json:"tools"`
	ToolChoice        string              `json:"tool_choice"`
	ParallelToolCalls bool                `json:"parallel_tool_calls"`
	Reasoning         *codexReasoning     `json:"reasoning,omitempty"`
	Store             bool                `json:"store"`
	Stream            bool                `json:"stream"`
	Include           []string            `json:"include"`
}

type codexReasoning struct {
	Effort string `json:"effort,omitempty"`
}

type codexResponseItem struct {
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Content []codexContentItem `json:"content"`
}

type codexContentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func newCodexUserMessage(text string) codexResponseItem {
	return codexResponseItem{
		Type: "message",
		Role: "user",
		Content: []codexContentItem{{
			Type: "input_text",
			Text: text,
		}},
	}
}

func readCodexAuth() (codexAuthFile, string, error) {
	path, err := codexAuthPath()
	if err != nil {
		return codexAuthFile{}, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return codexAuthFile{}, "", fmt.Errorf("codex: read auth file %s: %w; run `codex login`", path, err)
	}
	var auth codexAuthFile
	if err := json.Unmarshal(data, &auth); err != nil {
		return codexAuthFile{}, "", fmt.Errorf("codex: parse auth file %s: %w", path, err)
	}
	return auth, path, nil
}

func writeCodexAuth(path string, auth codexAuthFile) error {
	now := time.Now().UTC()
	raw := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &raw)
	}
	if raw == nil {
		raw = map[string]any{}
	}
	if auth.AuthMode != "" {
		raw["auth_mode"] = auth.AuthMode
	}
	tokens, _ := raw["tokens"].(map[string]any)
	if tokens == nil {
		tokens = map[string]any{}
	}
	if auth.Tokens.IDToken != "" {
		tokens["id_token"] = auth.Tokens.IDToken
	}
	tokens["access_token"] = auth.Tokens.AccessToken
	tokens["refresh_token"] = auth.Tokens.RefreshToken
	tokens["account_id"] = auth.Tokens.AccountID
	raw["tokens"] = tokens
	raw["last_refresh"] = now.Format(time.RFC3339Nano)

	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return fmt.Errorf("codex: marshal refreshed auth: %w", err)
	}
	return os.WriteFile(path, data, 0o600)
}

func codexAuthPath() (string, error) {
	if home := os.Getenv("CODEX_HOME"); home != "" {
		return filepath.Join(home, "auth.json"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("codex: resolve home directory: %w", err)
	}
	return filepath.Join(home, ".codex", "auth.json"), nil
}

func refreshCodexAuth(auth codexAuthFile) (codexAuthFile, error) {
	if auth.Tokens.RefreshToken == "" {
		return auth, fmt.Errorf("codex: ChatGPT auth has no refresh token; run `codex login`")
	}
	payload := map[string]string{
		"client_id":     codexOAuthClientID,
		"grant_type":    "refresh_token",
		"refresh_token": auth.Tokens.RefreshToken,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return auth, fmt.Errorf("codex: marshal refresh request: %w", err)
	}
	endpoint := os.Getenv(codexRefreshTokenURLVar)
	if endpoint == "" {
		endpoint = codexRefreshTokenURL
	}
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return auth, fmt.Errorf("codex: create refresh request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return auth, fmt.Errorf("codex: refresh token request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return auth, fmt.Errorf("codex: read refresh response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return auth, fmt.Errorf("codex: refresh token failed %d: %s", resp.StatusCode, string(respBody))
	}

	var refresh struct {
		IDToken      string `json:"id_token"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.Unmarshal(respBody, &refresh); err != nil {
		return auth, fmt.Errorf("codex: decode refresh response: %w", err)
	}
	if refresh.IDToken != "" {
		auth.Tokens.IDToken = refresh.IDToken
	}
	if refresh.AccessToken != "" {
		auth.Tokens.AccessToken = refresh.AccessToken
	}
	if refresh.RefreshToken != "" {
		auth.Tokens.RefreshToken = refresh.RefreshToken
	}
	if auth.Tokens.AccessToken == "" {
		return auth, fmt.Errorf("codex: refresh response did not include an access token")
	}
	return auth, nil
}

func tokenExpiresSoon(token string, leeway time.Duration) bool {
	exp, ok := jwtExpiration(token)
	if !ok {
		return false
	}
	return time.Until(exp) <= leeway
}

func jwtExpiration(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) < 2 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp int64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return time.Time{}, false
	}
	return time.Unix(claims.Exp, 0), true
}

func parseCodexSSE(r io.Reader) (string, error) {
	reader := bufio.NewReader(r)
	var event string
	var dataLines []string
	var b strings.Builder
	var doneText string

	flush := func() error {
		if len(dataLines) == 0 {
			event = ""
			return nil
		}
		data := strings.Join(dataLines, "\n")
		eventName := event
		event = ""
		dataLines = nil

		var payload struct {
			Type     string          `json:"type"`
			Delta    string          `json:"delta"`
			Text     string          `json:"text"`
			Error    json.RawMessage `json:"error"`
			Response struct {
				Status string          `json:"status"`
				Error  json.RawMessage `json:"error"`
			} `json:"response"`
		}
		if err := json.Unmarshal([]byte(data), &payload); err != nil {
			return fmt.Errorf("codex: parse SSE %s payload: %w", eventName, err)
		}
		switch payload.Type {
		case "response.output_text.delta":
			b.WriteString(payload.Delta)
		case "response.output_text.done":
			if payload.Text != "" {
				doneText = payload.Text
			}
		case "response.failed", "response.incomplete":
			if len(payload.Response.Error) > 0 {
				return fmt.Errorf("codex: response failed: %s", payload.Response.Error)
			}
			if len(payload.Error) > 0 {
				return fmt.Errorf("codex: response failed: %s", payload.Error)
			}
			return fmt.Errorf("codex: response failed")
		}
		return nil
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil && len(line) == 0 {
			if err == io.EOF {
				if flushErr := flush(); flushErr != nil {
					return "", flushErr
				}
				break
			}
			return "", fmt.Errorf("codex: read SSE: %w", err)
		}
		line = strings.TrimRight(line, "\r\n")
		if line == "" {
			if err := flush(); err != nil {
				return "", err
			}
		} else if strings.HasPrefix(line, "event:") {
			event = strings.TrimSpace(strings.TrimPrefix(line, "event:"))
		} else if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
		if err == io.EOF {
			if flushErr := flush(); flushErr != nil {
				return "", flushErr
			}
			break
		}
	}

	if b.Len() > 0 {
		return b.String(), nil
	}
	if doneText != "" {
		return doneText, nil
	}
	return "", fmt.Errorf("codex: no output text in response")
}
