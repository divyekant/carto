package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	cfg := Load()
	if cfg.MemoriesURL != "http://localhost:8900" {
		t.Errorf("expected default Memories URL, got %s", cfg.MemoriesURL)
	}
	if cfg.HaikuModel != "claude-haiku-4-5-20251001" {
		t.Errorf("expected default haiku model, got %s", cfg.HaikuModel)
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
