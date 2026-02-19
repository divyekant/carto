package signals

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubSignalSource_Name(t *testing.T) {
	src := NewGitHubSignalSource()
	if src.Name() != "github" {
		t.Errorf("expected name 'github', got %q", src.Name())
	}
}

func TestGitHubSignalSource_Configure(t *testing.T) {
	src := NewGitHubSignalSource()
	err := src.Configure(map[string]string{
		"owner": "octocat",
		"repo":  "Hello-World",
		"token": "ghp_test",
	})
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
	if src.owner != "octocat" || src.repo != "Hello-World" {
		t.Error("owner/repo not set correctly")
	}
}

func TestGitHubSignalSource_Configure_MissingFields(t *testing.T) {
	src := NewGitHubSignalSource()
	err := src.Configure(map[string]string{})
	if err == nil {
		t.Error("expected error when owner/repo missing")
	}
}

func TestGitHubSignalSource_FetchSignals(t *testing.T) {
	// Mock GitHub API.
	issues := []map[string]any{
		{
			"number":       42,
			"title":        "Fix login bug",
			"body":         "Login fails on mobile",
			"html_url":     "https://github.com/user/repo/issues/42",
			"created_at":   "2025-01-01T00:00:00Z",
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

	src := NewGitHubSignalSource()
	src.baseURL = srv.URL
	src.Configure(map[string]string{
		"owner": "user",
		"repo":  "repo",
	})

	signals, err := src.FetchSignals(Module{Name: "root"})
	if err != nil {
		t.Fatalf("FetchSignals failed: %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}

	// Verify issue signal.
	if signals[0].Type != "issue" || signals[0].ID != "#42" {
		t.Errorf("unexpected issue signal: %+v", signals[0])
	}
	if signals[0].Title != "Fix login bug" {
		t.Errorf("unexpected issue title: %q", signals[0].Title)
	}
	// Verify PR signal.
	if signals[1].Type != "pr" || signals[1].ID != "#43" {
		t.Errorf("unexpected PR signal: %+v", signals[1])
	}
}
