package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	FaissURL      string
	FaissAPIKey   string
	AnthropicKey  string
	HaikuModel    string
	OpusModel     string
	MaxConcurrent int
}

func Load() Config {
	return Config{
		FaissURL:      envOr("FAISS_URL", "http://localhost:8900"),
		FaissAPIKey:   envOr("FAISS_API_KEY", "god-is-an-astronaut"),
		AnthropicKey:  os.Getenv("ANTHROPIC_API_KEY"),
		HaikuModel:    envOr("CARTO_HAIKU_MODEL", "claude-haiku-4-5-20251001"),
		OpusModel:     envOr("CARTO_OPUS_MODEL", "claude-opus-4-6"),
		MaxConcurrent: envOrInt("CARTO_MAX_CONCURRENT", 10),
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
