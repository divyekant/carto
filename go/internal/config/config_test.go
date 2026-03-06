package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	cfg := Load()
	if cfg.MemoriesURL != "http://localhost:8900" {
		t.Errorf("expected default Memories URL, got %s", cfg.MemoriesURL)
	}
	if cfg.FastModel != "claude-haiku-4-5-20251001" {
		t.Errorf("expected default fast model, got %s", cfg.FastModel)
	}
	if cfg.MaxConcurrent != 10 {
		t.Errorf("expected default concurrency 10, got %d", cfg.MaxConcurrent)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	os.Setenv("MEMORIES_URL", "http://custom:9999")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("MEMORIES_URL")

	cfg := Load()
	if cfg.AnthropicKey != "test-key" {
		t.Errorf("expected test-key, got %s", cfg.AnthropicKey)
	}
	if cfg.MemoriesURL != "http://custom:9999" {
		t.Errorf("expected custom URL, got %s", cfg.MemoriesURL)
	}
}

func TestLoadConfig_TokenLimitDefaults(t *testing.T) {
	cfg := Load()
	if cfg.FastMaxTokens != 4096 {
		t.Errorf("expected default FastMaxTokens 4096, got %d", cfg.FastMaxTokens)
	}
	if cfg.DeepMaxTokens != 8192 {
		t.Errorf("expected default DeepMaxTokens 8192, got %d", cfg.DeepMaxTokens)
	}
}

func TestLoadConfig_TokenLimitEnvOverrides(t *testing.T) {
	t.Setenv("CARTO_FAST_MAX_TOKENS", "8192")
	t.Setenv("CARTO_DEEP_MAX_TOKENS", "16384")

	cfg := Load()
	if cfg.FastMaxTokens != 8192 {
		t.Errorf("expected FastMaxTokens 8192, got %d", cfg.FastMaxTokens)
	}
	if cfg.DeepMaxTokens != 16384 {
		t.Errorf("expected DeepMaxTokens 16384, got %d", cfg.DeepMaxTokens)
	}
}

func TestResolveURL_NonDocker(t *testing.T) {
	url := ResolveURL("http://localhost:8900")
	if url != "http://localhost:8900" {
		t.Errorf("expected localhost unchanged, got %s", url)
	}
}

func TestResolveURL_Docker(t *testing.T) {
	tests := []struct {
		input    string
		inDocker bool
		expected string
	}{
		{"http://localhost:8900", false, "http://localhost:8900"},
		{"http://127.0.0.1:8900", false, "http://127.0.0.1:8900"},
		{"http://localhost:8900", true, "http://host.docker.internal:8900"},
		{"http://127.0.0.1:8900", true, "http://host.docker.internal:8900"},
		{"https://memories.example.com", true, "https://memories.example.com"},
		{"https://memories.example.com", false, "https://memories.example.com"},
	}
	for _, tt := range tests {
		got := resolveURLForDocker(tt.input, tt.inDocker)
		if got != tt.expected {
			t.Errorf("resolveURLForDocker(%q, %v) = %q, want %q", tt.input, tt.inDocker, got, tt.expected)
		}
	}
}

func TestIsDocker(t *testing.T) {
	result := IsDocker()
	_ = result
}

func TestIsOAuthToken(t *testing.T) {
	if !IsOAuthToken("sk-ant-oat01-abc123") {
		t.Error("should detect OAuth token")
	}
	if IsOAuthToken("sk-ant-api03-abc123") {
		t.Error("should not detect API key as OAuth")
	}
	if IsOAuthToken("") {
		t.Error("should not detect empty string as OAuth")
	}
}

// =========================================================================
// MaskSecret tests
// =========================================================================

func TestMaskSecret_EmptyValue(t *testing.T) {
	if got := MaskSecret(""); got != "" {
		t.Errorf("empty secret should return empty, got %q", got)
	}
}

func TestMaskSecret_ShortValue(t *testing.T) {
	// Values ≤ 8 chars are fully redacted to avoid leaking too much.
	for _, s := range []string{"abc", "abcdefgh", "1234"} {
		if got := MaskSecret(s); got != "****" {
			t.Errorf("short secret %q should be fully masked, got %q", s, got)
		}
	}
}

func TestMaskSecret_LongValue_PreservesPrefix(t *testing.T) {
	got := MaskSecret("sk-ant-api03-abcdefghijklmn")
	if !strings.HasPrefix(got, "sk-a") {
		t.Errorf("masked secret should preserve 4-char prefix, got %q", got)
	}
}

func TestMaskSecret_LongValue_PreservesSuffix(t *testing.T) {
	got := MaskSecret("sk-ant-api03-abcdefghijklmn")
	if !strings.HasSuffix(got, "klmn") {
		t.Errorf("masked secret should preserve 4-char suffix, got %q", got)
	}
}

func TestMaskSecret_LongValue_HidesMiddle(t *testing.T) {
	original := "sk-ant-api03-abcdefghijklmn"
	got := MaskSecret(original)
	// The middle portion must not appear in the masked output.
	if strings.Contains(got, "api03") {
		t.Errorf("masked secret must not contain original middle, got %q", got)
	}
	if !strings.Contains(got, "****") {
		t.Errorf("masked secret must contain **** placeholder, got %q", got)
	}
}

// =========================================================================
// Redacted() tests
// =========================================================================

func TestRedacted_MasksSecretFields(t *testing.T) {
	cfg := Config{
		AnthropicKey: "sk-ant-real-secret-key-value",
		LLMApiKey:    "sk-llm-api-key-value",
		MemoriesKey:  "mem-secret-key-value",
		GitHubToken:  "ghp_realtoken123456",
		JiraToken:    "jira-token-abc123",
		LinearToken:  "lin_realtoken123",
		NotionToken:  "secret_notion_token",
		SlackToken:   "xoxb-realslacktoken",
		ServerToken:  "carto-server-token",
	}

	r := cfg.Redacted()

	secrets := map[string]string{
		"AnthropicKey": r.AnthropicKey,
		"LLMApiKey":    r.LLMApiKey,
		"MemoriesKey":  r.MemoriesKey,
		"GitHubToken":  r.GitHubToken,
		"JiraToken":    r.JiraToken,
		"LinearToken":  r.LinearToken,
		"NotionToken":  r.NotionToken,
		"SlackToken":   r.SlackToken,
		"ServerToken":  r.ServerToken,
	}

	originals := map[string]string{
		"AnthropicKey": cfg.AnthropicKey,
		"LLMApiKey":    cfg.LLMApiKey,
		"MemoriesKey":  cfg.MemoriesKey,
		"GitHubToken":  cfg.GitHubToken,
		"JiraToken":    cfg.JiraToken,
		"LinearToken":  cfg.LinearToken,
		"NotionToken":  cfg.NotionToken,
		"SlackToken":   cfg.SlackToken,
		"ServerToken":  cfg.ServerToken,
	}

	for field, masked := range secrets {
		if masked == originals[field] {
			t.Errorf("Redacted() must mask %s, but it was returned in plain text", field)
		}
	}
}

func TestRedacted_PreservesNonSecretFields(t *testing.T) {
	cfg := Config{
		MemoriesURL:   "http://localhost:8900",
		FastModel:     "claude-haiku-4-5-20251001",
		DeepModel:     "claude-opus-4-6",
		MaxConcurrent: 10,
		LLMProvider:   "anthropic",
	}

	r := cfg.Redacted()

	if r.MemoriesURL != cfg.MemoriesURL {
		t.Errorf("MemoriesURL should not be redacted, got %q", r.MemoriesURL)
	}
	if r.FastModel != cfg.FastModel {
		t.Errorf("FastModel should not be redacted, got %q", r.FastModel)
	}
	if r.MaxConcurrent != cfg.MaxConcurrent {
		t.Errorf("MaxConcurrent should not be redacted, got %d", r.MaxConcurrent)
	}
}

func TestLoadConfig_ServerTokenFromEnv(t *testing.T) {
	t.Setenv("CARTO_SERVER_TOKEN", "my-secret-token")
	cfg := Load()
	if cfg.ServerToken != "my-secret-token" {
		t.Errorf("expected ServerToken from env, got %q", cfg.ServerToken)
	}
}

func TestLoadConfig_CORSOriginsFromEnv(t *testing.T) {
	t.Setenv("CARTO_CORS_ORIGINS", "https://myapp.com,https://dashboard.example.com")
	cfg := Load()
	if cfg.CORSOrigins != "https://myapp.com,https://dashboard.example.com" {
		t.Errorf("expected CORSOrigins from env, got %q", cfg.CORSOrigins)
	}
}
