package carto

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIndexOptionsDefaults(t *testing.T) {
	opts := IndexOptions{}
	if opts.Incremental {
		t.Fatal("expected incremental=false by default")
	}
	if opts.Module != "" {
		t.Fatal("expected empty module by default")
	}
	if opts.Project != "" {
		t.Fatal("expected empty project by default")
	}
}

func TestQueryOptionsDefaults(t *testing.T) {
	opts := QueryOptions{}
	if opts.K != 0 {
		t.Fatal("expected K=0 by default (caller sets)")
	}
	if opts.Tier != "" {
		t.Fatal("expected empty tier by default")
	}
	if opts.Project != "" {
		t.Fatal("expected empty project by default")
	}
}

func TestIndexResultFields(t *testing.T) {
	r := IndexResult{
		Modules: 3,
		Files:   42,
		Atoms:   100,
		Errors:  2,
	}
	if r.Modules != 3 || r.Files != 42 || r.Atoms != 100 || r.Errors != 2 {
		t.Fatal("IndexResult fields not set correctly")
	}
}

func TestQueryResultFields(t *testing.T) {
	r := QueryResult{
		Text:   "hello world",
		Source: "carto/test/atoms/main",
		Score:  0.95,
	}
	if r.Text != "hello world" || r.Source != "carto/test/atoms/main" || r.Score != 0.95 {
		t.Fatal("QueryResult fields not set correctly")
	}
}

func TestIndexNoAPIKey(t *testing.T) {
	// Clear all API key env vars to ensure the SDK returns the proper error.
	origLLM := os.Getenv("LLM_API_KEY")
	origAnthropic := os.Getenv("ANTHROPIC_API_KEY")
	origProvider := os.Getenv("LLM_PROVIDER")
	os.Unsetenv("LLM_API_KEY")
	os.Unsetenv("ANTHROPIC_API_KEY")
	os.Unsetenv("LLM_PROVIDER")
	defer func() {
		if origLLM != "" {
			os.Setenv("LLM_API_KEY", origLLM)
		}
		if origAnthropic != "" {
			os.Setenv("ANTHROPIC_API_KEY", origAnthropic)
		}
		if origProvider != "" {
			os.Setenv("LLM_PROVIDER", origProvider)
		}
	}()

	_, err := Index("/nonexistent", IndexOptions{})
	if err == nil {
		t.Fatal("expected error when no API key is set")
	}
	if !contains(err.Error(), "carto:") {
		t.Errorf("expected error wrapped with 'carto:' prefix, got: %v", err)
	}
}

func TestSourcesNoProjectsDir(t *testing.T) {
	orig := os.Getenv("PROJECTS_DIR")
	os.Unsetenv("PROJECTS_DIR")
	defer func() {
		if orig != "" {
			os.Setenv("PROJECTS_DIR", orig)
		}
	}()

	_, err := Sources("test-project")
	if err == nil {
		t.Fatal("expected error when PROJECTS_DIR not set")
	}
	if !contains(err.Error(), "PROJECTS_DIR") {
		t.Errorf("expected error mentioning PROJECTS_DIR, got: %v", err)
	}
}

func TestSourcesMissingProject(t *testing.T) {
	dir := t.TempDir()
	os.Setenv("PROJECTS_DIR", dir)
	defer os.Unsetenv("PROJECTS_DIR")

	result, err := Sources("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected empty sources, got %d", len(result))
	}
}

func TestSourcesWithConfig(t *testing.T) {
	dir := t.TempDir()
	projDir := filepath.Join(dir, "myproject")
	cartoDir := filepath.Join(projDir, ".carto")
	os.MkdirAll(cartoDir, 0o755)
	os.WriteFile(filepath.Join(cartoDir, "sources.yaml"), []byte(`
sources:
  github:
    owner: test
    repo: app
`), 0o644)

	os.Setenv("PROJECTS_DIR", dir)
	defer os.Unsetenv("PROJECTS_DIR")

	result, err := Sources("myproject")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["github"]["owner"] != "test" {
		t.Fatalf("expected owner=test, got %s", result["github"]["owner"])
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsStr(s, substr))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
