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
