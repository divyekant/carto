package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

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
}

// persistedConfig is the JSON shape written to the config file.
type persistedConfig struct {
	MemoriesURL   string `json:"memories_url,omitempty"`
	MemoriesKey   string `json:"memories_key,omitempty"`
	AnthropicKey  string `json:"anthropic_key,omitempty"`
	FastModel     string `json:"fast_model,omitempty"`
	DeepModel     string `json:"deep_model,omitempty"`
	MaxConcurrent int    `json:"max_concurrent,omitempty"`
	LLMProvider   string `json:"llm_provider,omitempty"`
	LLMApiKey     string `json:"llm_api_key,omitempty"`
	LLMBaseURL    string `json:"llm_base_url,omitempty"`
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
		LLMProvider:   envOr("LLM_PROVIDER", "anthropic"),
		LLMApiKey:     os.Getenv("LLM_API_KEY"),
		LLMBaseURL:    os.Getenv("LLM_BASE_URL"),
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
		LLMProvider:   cfg.LLMProvider,
		LLMApiKey:     cfg.LLMApiKey,
		LLMBaseURL:    cfg.LLMBaseURL,
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
	if p.LLMProvider != "" {
		cfg.LLMProvider = p.LLMProvider
	}
	if p.LLMApiKey != "" {
		cfg.LLMApiKey = p.LLMApiKey
	}
	if p.LLMBaseURL != "" {
		cfg.LLMBaseURL = p.LLMBaseURL
	}
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
