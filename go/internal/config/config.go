package config

import (
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

func Load() Config {
	return Config{
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
