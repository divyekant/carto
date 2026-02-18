package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Tier selects which model class to use.
type Tier string

const (
	TierHaiku Tier = "haiku"
	TierOpus  Tier = "opus"
)

// OAuth constants matching the WebChat/Claude CLI pattern.
const (
	OAuthClientID = "9d1c250a-e61b-44d9-88ed-5944d1962f5e"
	OAuthTokenURL = "https://console.anthropic.com/v1/oauth/token"
	OAuthBeta     = "oauth-2025-04-20"
	ThinkingBeta  = "interleaved-thinking-2025-05-14"
	UserAgent     = "carto/0.2.0 (external, cli)"
)

// Options configures the Anthropic API client.
type Options struct {
	APIKey        string
	BaseURL       string
	HaikuModel    string
	OpusModel     string
	MaxConcurrent int
	IsOAuth       bool
}

// CompleteOptions provides per-request overrides.
type CompleteOptions struct {
	System    string
	MaxTokens int
}

// oauthState tracks a refreshable OAuth token.
type oauthState struct {
	mu           sync.Mutex
	accessToken  string
	refreshToken string
	expiresAt    time.Time
}

// Client is an HTTP-based Anthropic API client.
type Client struct {
	opts  Options
	sem   chan struct{}
	http  http.Client
	oauth *oauthState // non-nil when using OAuth tokens
}

// NewClient creates a Client with sensible defaults.
func NewClient(opts Options) *Client {
	if opts.BaseURL == "" {
		opts.BaseURL = "https://api.anthropic.com"
	}
	if opts.HaikuModel == "" {
		opts.HaikuModel = "claude-haiku-4-5-20251001"
	}
	if opts.OpusModel == "" {
		opts.OpusModel = "claude-opus-4-6"
	}
	if opts.MaxConcurrent <= 0 {
		opts.MaxConcurrent = 10
	}

	sem := make(chan struct{}, opts.MaxConcurrent)
	c := &Client{opts: opts, sem: sem}

	if opts.IsOAuth {
		c.oauth = &oauthState{
			accessToken: opts.APIKey,
		}
	}

	return c
}

// refreshOAuthToken exchanges the refresh token for a new access token.
// All checks happen inside the lock to prevent multiple goroutines from
// triggering redundant refreshes.
func (c *Client) refreshOAuthToken() error {
	if c.oauth == nil {
		return nil
	}

	c.oauth.mu.Lock()
	defer c.oauth.mu.Unlock()

	// Token might have been refreshed by another goroutine while we waited.
	if c.oauth.refreshToken == "" {
		return nil
	}
	if !c.oauth.expiresAt.IsZero() && time.Now().Before(c.oauth.expiresAt) {
		return nil
	}

	payload := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": c.oauth.refreshToken,
		"client_id":     OAuthClientID,
	}
	body, _ := json.Marshal(payload)

	resp, err := c.http.Post(OAuthTokenURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("llm: oauth refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("llm: oauth refresh failed %d: %s", resp.StatusCode, text)
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("llm: oauth refresh decode: %w", err)
	}

	c.oauth.accessToken = result.AccessToken
	c.oauth.refreshToken = result.RefreshToken
	c.oauth.expiresAt = time.Now().Add(time.Duration(result.ExpiresIn) * time.Second)
	return nil
}

// apiRequest is the JSON body sent to /v1/messages.
type apiRequest struct {
	Model     string       `json:"model"`
	MaxTokens int          `json:"max_tokens"`
	System    string       `json:"system,omitempty"`
	Messages  []apiMessage `json:"messages"`
}

type apiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// apiResponse is the top-level JSON returned by /v1/messages.
type apiResponse struct {
	Content []contentBlock `json:"content"`
}

type contentBlock struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

// Complete sends a prompt to the Anthropic Messages API and returns the text
// from the first text content block.
func (c *Client) Complete(prompt string, tier Tier, opts *CompleteOptions) (string, error) {
	// Acquire semaphore slot.
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	model := c.opts.HaikuModel
	if tier == TierOpus {
		model = c.opts.OpusModel
	}

	maxTokens := 4096
	var system string
	if opts != nil {
		if opts.MaxTokens > 0 {
			maxTokens = opts.MaxTokens
		}
		system = opts.System
	}

	reqBody := apiRequest{
		Model:     model,
		MaxTokens: maxTokens,
		System:    system,
		Messages: []apiMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("llm: marshal request: %w", err)
	}

	endpoint := strings.TrimRight(c.opts.BaseURL, "/") + "/v1/messages"

	// OAuth: add ?beta=true query param (matches WebChat/Claude CLI pattern).
	if c.opts.IsOAuth {
		endpoint += "?beta=true"
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("llm: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Anthropic-Version", "2023-06-01")

	if c.opts.IsOAuth {
		// Refresh token if needed (check is inside the lock to avoid races).
		if err := c.refreshOAuthToken(); err != nil {
			return "", fmt.Errorf("oauth refresh: %w", err)
		}

		// Use current access token.
		token := c.opts.APIKey
		if c.oauth != nil {
			c.oauth.mu.Lock()
			token = c.oauth.accessToken
			c.oauth.mu.Unlock()
		}

		req.Header.Set("Authorization", "Bearer "+token)
		beta := OAuthBeta
		if tier == TierOpus {
			beta += "," + ThinkingBeta
		}
		req.Header.Set("Anthropic-Beta", beta)
		req.Header.Set("User-Agent", UserAgent)
		// Remove x-api-key if present (belt-and-suspenders).
		req.Header.Del("X-Api-Key")
	} else {
		req.Header.Set("X-Api-Key", c.opts.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("llm: send request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("llm: read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("llm: API returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return "", fmt.Errorf("llm: unmarshal response: %w", err)
	}

	for _, block := range apiResp.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}

	return "", fmt.Errorf("llm: no text block in response")
}

// CompleteJSON calls Complete and extracts the first JSON object from the
// response, stripping any surrounding markdown fences.
func (c *Client) CompleteJSON(prompt string, tier Tier, opts *CompleteOptions) (json.RawMessage, error) {
	text, err := c.Complete(prompt, tier, opts)
	if err != nil {
		return nil, err
	}

	cleaned := stripMarkdownFences(text)

	// Find the first JSON object in the cleaned text.
	start := strings.Index(cleaned, "{")
	if start == -1 {
		return nil, fmt.Errorf("llm: no JSON object found in response")
	}

	// Walk forward to find the matching closing brace.
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(cleaned); i++ {
		ch := cleaned[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				raw := json.RawMessage(cleaned[start : i+1])
				// Validate it's actually valid JSON.
				if !json.Valid(raw) {
					return nil, fmt.Errorf("llm: extracted JSON is invalid")
				}
				return raw, nil
			}
		}
	}

	return nil, fmt.Errorf("llm: incomplete JSON object in response")
}

// stripMarkdownFences removes ```json ... ``` or ``` ... ``` wrappers.
func stripMarkdownFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	}
	if strings.HasSuffix(s, "```") {
		s = strings.TrimSuffix(s, "```")
	}
	return strings.TrimSpace(s)
}
