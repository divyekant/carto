package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubSource_Name(t *testing.T) {
	src := NewGitHubSource()
	if src.Name() != "github" {
		t.Errorf("Name() = %q, want %q", src.Name(), "github")
	}
}

func TestGitHubSource_Scope(t *testing.T) {
	src := NewGitHubSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestGitHubSource_Configure(t *testing.T) {
	src := NewGitHubSource()
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{"owner": "octocat", "repo": "Hello-World"},
		Credentials: map[string]string{"github_token": "ghp_test"},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if src.owner != "octocat" || src.repo != "Hello-World" {
		t.Error("owner/repo not set")
	}
}

func TestGitHubSource_Configure_Missing(t *testing.T) {
	src := NewGitHubSource()
	err := src.Configure(SourceConfig{Settings: map[string]string{}})
	if err == nil {
		t.Error("expected error when owner/repo missing")
	}
}

func TestGitHubSource_Fetch(t *testing.T) {
	issues := []map[string]any{
		{
			"number":       42,
			"title":        "Fix login bug",
			"body":         "Login fails on mobile",
			"html_url":     "https://github.com/user/repo/issues/42",
			"created_at":   "2025-01-01T00:00:00Z",
			"state":        "open",
			"user":         map[string]any{"login": "alice"},
			"pull_request": nil,
		},
	}
	prs := []map[string]any{
		{
			"number":     43,
			"title":      "Add dark mode",
			"body":       "Implements dark theme",
			"html_url":   "https://github.com/user/repo/pull/43",
			"created_at": "2025-01-02T00:00:00Z",
			"state":      "merged",
			"user":       map[string]any{"login": "bob"},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/user/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(issues)
	})
	mux.HandleFunc("/repos/user/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(prs)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := NewGitHubSource()
	src.baseURL = srv.URL
	src.Configure(SourceConfig{
		Settings: map[string]string{"owner": "user", "repo": "repo"},
	})

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}

	// Verify issue.
	if artifacts[0].Category != Signal || artifacts[0].ID != "#42" {
		t.Errorf("unexpected issue: %+v", artifacts[0])
	}
	if artifacts[0].Source != "github" {
		t.Errorf("expected source=github, got %s", artifacts[0].Source)
	}
	if artifacts[0].Tags["type"] != "issue" {
		t.Errorf("expected type=issue, got %s", artifacts[0].Tags["type"])
	}
	// Verify PR.
	if artifacts[1].Tags["type"] != "pr" || artifacts[1].ID != "#43" {
		t.Errorf("unexpected PR: %+v", artifacts[1])
	}
}

var _ Source = (*GitHubSource)(nil)
