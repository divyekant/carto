package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Version is the semantic version embedded by the build pipeline.
// It is used in audit log entries and the /api/health response.
var Version = "2.0.0"

type Config struct {
	MemoriesURL   string
	MemoriesKey   string
	AnthropicKey  string
	FastModel     string
	DeepModel     string
	MaxConcurrent int
	LLMProvider   string
	LLMApiKey     string
	LLMBaseURL    string
	FastMaxTokens int
	DeepMaxTokens int
	GitHubToken   string
	JiraToken     string
	JiraEmail     string
	JiraBaseURL   string
	LinearToken   string
	NotionToken   string
	SlackToken    string
	// B2B SaaS security fields.
	ServerToken string // CARTO_SERVER_TOKEN — empty disables auth (dev mode)
	CORSOrigins string // CARTO_CORS_ORIGINS — comma-separated allowed origins
	// Observability fields.
	AuditLogFile string // CARTO_AUDIT_LOG — file path for structured audit logs
	// Profile name — selects a named section in the config file.
	Profile string // CARTO_PROFILE — defaults to "default"
}

// ValidationError holds one or more human-readable config problems.
type ValidationError struct {
	Fields []string
}

func (e *ValidationError) Error() string {
	return "config validation failed: " + strings.Join(e.Fields, "; ")
}

// Validate checks that the Config is internally consistent and has the minimum
// required values set for normal operation. It returns nil when valid.
func (c Config) Validate() error {
	var errs []string

	// LLM provider must be one of the known values.
	switch c.LLMProvider {
	case "anthropic", "openai", "ollama", "":
		// acceptable
	default:
		errs = append(errs, fmt.Sprintf("unknown llm_provider %q (expected anthropic|openai|ollama)", c.LLMProvider))
	}

	// API key required for cloud providers.
	if c.LLMProvider == "anthropic" || c.LLMProvider == "" {
		if c.AnthropicKey == "" && c.LLMApiKey == "" {
			errs = append(errs, "no API key set — configure ANTHROPIC_API_KEY or LLM_API_KEY")
		}
	} else if c.LLMProvider == "openai" {
		if c.LLMApiKey == "" {
			errs = append(errs, "LLM_API_KEY is required for openai provider")
		}
		if c.LLMBaseURL == "" {
			errs = append(errs, "LLM_BASE_URL is required for openai provider")
		}
	}

	// MemoriesURL must start with http/https.
	if c.MemoriesURL != "" && !strings.HasPrefix(c.MemoriesURL, "http://") && !strings.HasPrefix(c.MemoriesURL, "https://") {
		errs = append(errs, "memories_url must start with http:// or https://")
	}

	// MaxConcurrent must be positive.
	if c.MaxConcurrent < 1 {
		errs = append(errs, fmt.Sprintf("max_concurrent must be ≥ 1, got %d", c.MaxConcurrent))
	}

	if len(errs) > 0 {
		return &ValidationError{Fields: errs}
	}
	return nil
}

// ConfigDir returns the XDG-compliant directory for Carto config files.
// On Linux/macOS this resolves to ~/.config/carto; on other platforms it
// falls back to ~/.carto.
func ConfigDir() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "carto")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "carto")
	}
	return ".carto"
}

// DefaultConfigFilePath returns the path to the default per-user config file.
func DefaultConfigFilePath() string {
	return filepath.Join(ConfigDir(), "config.json")
}

// persistedConfig is the JSON shape written to the config file.
type persistedConfig struct {
	MemoriesURL   string `json:"memories_url,omitempty"`
	MemoriesKey   string `json:"memories_key,omitempty"`
	AnthropicKey  string `json:"anthropic_key,omitempty"`
	FastModel     string `json:"fast_model,omitempty"`
	DeepModel     string `json:"deep_model,omitempty"`
	MaxConcurrent int    `json:"max_concurrent,omitempty"`
	FastMaxTokens int    `json:"fast_max_tokens,omitempty"`
	DeepMaxTokens int    `json:"deep_max_tokens,omitempty"`
	LLMProvider   string `json:"llm_provider,omitempty"`
	LLMApiKey     string `json:"llm_api_key,omitempty"`
	LLMBaseURL    string `json:"llm_base_url,omitempty"`
	GitHubToken   string `json:"github_token,omitempty"`
	JiraToken     string `json:"jira_token,omitempty"`
	JiraEmail     string `json:"jira_email,omitempty"`
	JiraBaseURL   string `json:"jira_base_url,omitempty"`
	LinearToken   string `json:"linear_token,omitempty"`
	NotionToken   string `json:"notion_token,omitempty"`
	SlackToken    string `json:"slack_token,omitempty"`
}

// ConfigPath is the file path where UI settings are persisted.
// It defaults to ".carto-server.json" in the projects directory.
var ConfigPath string

func Load() Config {
	cfg := Config{
		MemoriesURL:   envOr("MEMORIES_URL", "http://localhost:8900"),
		MemoriesKey:   os.Getenv("MEMORIES_API_KEY"),
		AnthropicKey:  os.Getenv("ANTHROPIC_API_KEY"),
		FastModel:     envOr("CARTO_FAST_MODEL", "claude-haiku-4-5-20251001"),
		DeepModel:     envOr("CARTO_DEEP_MODEL", "claude-opus-4-6"),
		MaxConcurrent: envOrInt("CARTO_MAX_CONCURRENT", 10),
		FastMaxTokens: envOrInt("CARTO_FAST_MAX_TOKENS", 4096),
		DeepMaxTokens: envOrInt("CARTO_DEEP_MAX_TOKENS", 8192),
		LLMProvider:   envOr("LLM_PROVIDER", "anthropic"),
		LLMApiKey:     os.Getenv("LLM_API_KEY"),
		LLMBaseURL:    os.Getenv("LLM_BASE_URL"),
		GitHubToken:   os.Getenv("GITHUB_TOKEN"),
		JiraToken:     os.Getenv("JIRA_TOKEN"),
		JiraEmail:     os.Getenv("JIRA_EMAIL"),
		JiraBaseURL:   os.Getenv("JIRA_BASE_URL"),
		LinearToken:   os.Getenv("LINEAR_TOKEN"),
		NotionToken:   os.Getenv("NOTION_TOKEN"),
		SlackToken:    os.Getenv("SLACK_TOKEN"),
		ServerToken:   os.Getenv("CARTO_SERVER_TOKEN"),
		CORSOrigins:   os.Getenv("CARTO_CORS_ORIGINS"),
		AuditLogFile:  os.Getenv("CARTO_AUDIT_LOG"),
		Profile:       envOr("CARTO_PROFILE", "default"),
	}

	// Overlay persisted settings (only non-empty values override).
	if ConfigPath != "" {
		if saved, err := loadPersistedConfig(ConfigPath); err == nil {
			mergeConfig(&cfg, saved)
		}
	}

	return cfg
}

// Save writes the current config to the persisted config file.
func Save(cfg Config) error {
	if ConfigPath == "" {
		return nil
	}
	p := persistedConfig{
		MemoriesURL:   cfg.MemoriesURL,
		MemoriesKey:   cfg.MemoriesKey,
		AnthropicKey:  cfg.AnthropicKey,
		FastModel:     cfg.FastModel,
		DeepModel:     cfg.DeepModel,
		MaxConcurrent: cfg.MaxConcurrent,
		FastMaxTokens: cfg.FastMaxTokens,
		DeepMaxTokens: cfg.DeepMaxTokens,
		LLMProvider:   cfg.LLMProvider,
		LLMApiKey:     cfg.LLMApiKey,
		LLMBaseURL:    cfg.LLMBaseURL,
		GitHubToken:   cfg.GitHubToken,
		JiraToken:     cfg.JiraToken,
		JiraEmail:     cfg.JiraEmail,
		JiraBaseURL:   cfg.JiraBaseURL,
		LinearToken:   cfg.LinearToken,
		NotionToken:   cfg.NotionToken,
		SlackToken:    cfg.SlackToken,
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ConfigPath, data, 0600)
}

func loadPersistedConfig(path string) (persistedConfig, error) {
	var p persistedConfig
	data, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(data, &p)
	return p, err
}

func mergeConfig(cfg *Config, p persistedConfig) {
	if p.MemoriesURL != "" {
		cfg.MemoriesURL = p.MemoriesURL
	}
	if p.MemoriesKey != "" {
		cfg.MemoriesKey = p.MemoriesKey
	}
	if p.AnthropicKey != "" {
		cfg.AnthropicKey = p.AnthropicKey
	}
	if p.FastModel != "" {
		cfg.FastModel = p.FastModel
	}
	if p.DeepModel != "" {
		cfg.DeepModel = p.DeepModel
	}
	if p.MaxConcurrent != 0 {
		cfg.MaxConcurrent = p.MaxConcurrent
	}
	if p.FastMaxTokens != 0 {
		cfg.FastMaxTokens = p.FastMaxTokens
	}
	if p.DeepMaxTokens != 0 {
		cfg.DeepMaxTokens = p.DeepMaxTokens
	}
	if p.LLMProvider != "" {
		cfg.LLMProvider = p.LLMProvider
	}
	if p.LLMApiKey != "" {
		cfg.LLMApiKey = p.LLMApiKey
	}
	if p.LLMBaseURL != "" {
		cfg.LLMBaseURL = p.LLMBaseURL
	}
	if p.GitHubToken != "" {
		cfg.GitHubToken = p.GitHubToken
	}
	if p.JiraToken != "" {
		cfg.JiraToken = p.JiraToken
	}
	if p.JiraEmail != "" {
		cfg.JiraEmail = p.JiraEmail
	}
	if p.JiraBaseURL != "" {
		cfg.JiraBaseURL = p.JiraBaseURL
	}
	if p.LinearToken != "" {
		cfg.LinearToken = p.LinearToken
	}
	if p.NotionToken != "" {
		cfg.NotionToken = p.NotionToken
	}
	if p.SlackToken != "" {
		cfg.SlackToken = p.SlackToken
	}
}

// IsDocker returns true when running inside a Docker container.
func IsDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// ResolveURL rewrites localhost/127.0.0.1 URLs to host.docker.internal
// when running inside Docker. Remote URLs pass through unchanged.
func ResolveURL(rawURL string) string {
	return resolveURLForDocker(rawURL, IsDocker())
}

func resolveURLForDocker(rawURL string, inDocker bool) string {
	if !inDocker {
		return rawURL
	}
	u := strings.Replace(rawURL, "localhost", "host.docker.internal", 1)
	u = strings.Replace(u, "127.0.0.1", "host.docker.internal", 1)
	return u
}

// MaskSecret returns a partially-masked representation of a secret value safe
// for display in UIs and logs. Values ≤ 8 chars are fully redacted.
// Example: "sk-ant-api03-abcdef" → "sk-ant-a********cdef"
func MaskSecret(s string) string {
	if s == "" {
		return ""
	}
	if len(s) <= 8 {
		return "****"
	}
	prefix := s[:4]
	suffix := s[len(s)-4:]
	return prefix + strings.Repeat("*", 8) + suffix
}

// Redacted returns a copy of Config with all secret fields replaced by masked
// values. Use this copy for logging and diagnostics — never log a raw Config.
func (c Config) Redacted() Config {
	r := c
	r.AnthropicKey = MaskSecret(c.AnthropicKey)
	r.LLMApiKey = MaskSecret(c.LLMApiKey)
	r.MemoriesKey = MaskSecret(c.MemoriesKey)
	r.GitHubToken = MaskSecret(c.GitHubToken)
	r.JiraToken = MaskSecret(c.JiraToken)
	r.LinearToken = MaskSecret(c.LinearToken)
	r.NotionToken = MaskSecret(c.NotionToken)
	r.SlackToken = MaskSecret(c.SlackToken)
	r.ServerToken = MaskSecret(c.ServerToken)
	return r
}

// EffectiveAPIKey returns the API key that will actually be used for LLM calls.
// LLMApiKey takes priority over the Anthropic-specific AnthropicKey.
func (c Config) EffectiveAPIKey() string {
	if c.LLMApiKey != "" {
		return c.LLMApiKey
	}
	return c.AnthropicKey
}

func IsOAuthToken(key string) bool {
	return len(key) > 0 && strings.HasPrefix(key, "sk-ant-oat01-")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
