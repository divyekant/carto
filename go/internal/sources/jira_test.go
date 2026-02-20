package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Compile-time interface check.
var _ Source = (*JiraSource)(nil)

func TestJiraSource_Name(t *testing.T) {
	src := NewJiraSource()
	if src.Name() != "jira" {
		t.Errorf("Name() = %q, want %q", src.Name(), "jira")
	}
}

func TestJiraSource_Scope(t *testing.T) {
	src := NewJiraSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestJiraSource_Configure(t *testing.T) {
	src := NewJiraSource()
	err := src.Configure(SourceConfig{
		Settings: map[string]string{
			"base_url":    "https://mycompany.atlassian.net",
			"project_key": "PROJ",
		},
		Credentials: map[string]string{
			"jira_email": "alice@example.com",
			"jira_token": "secret-token",
		},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if src.baseURL != "https://mycompany.atlassian.net" {
		t.Errorf("baseURL = %q, want %q", src.baseURL, "https://mycompany.atlassian.net")
	}
	if src.projectKey != "PROJ" {
		t.Errorf("projectKey = %q, want %q", src.projectKey, "PROJ")
	}
	if src.email != "alice@example.com" {
		t.Errorf("email = %q, want %q", src.email, "alice@example.com")
	}
	if src.token != "secret-token" {
		t.Errorf("token = %q, want %q", src.token, "secret-token")
	}
}

func TestJiraSource_Configure_MissingBaseURL(t *testing.T) {
	src := NewJiraSource()
	err := src.Configure(SourceConfig{
		Settings: map[string]string{
			"project_key": "PROJ",
		},
	})
	if err == nil {
		t.Error("expected error when base_url is missing")
	}
}

func TestJiraSource_Configure_MissingProjectKey(t *testing.T) {
	src := NewJiraSource()
	err := src.Configure(SourceConfig{
		Settings: map[string]string{
			"base_url": "https://mycompany.atlassian.net",
		},
	})
	if err == nil {
		t.Error("expected error when project_key is missing")
	}
}

func TestJiraSource_Fetch(t *testing.T) {
	searchResp := map[string]any{
		"issues": []map[string]any{
			{
				"key": "PROJ-123",
				"fields": map[string]any{
					"summary":     "Fix authentication timeout",
					"description": "Users are getting logged out after 5 minutes of inactivity.",
					"updated":     "2025-06-15T10:30:00.000+0000",
					"creator":     map[string]any{"displayName": "Alice Smith"},
					"issuetype":   map[string]any{"name": "Bug"},
					"status":      map[string]any{"name": "In Progress"},
					"priority":    map[string]any{"name": "High"},
				},
			},
			{
				"key": "PROJ-124",
				"fields": map[string]any{
					"summary":     "Add dark mode support",
					"description": "Implement dark mode for the dashboard.",
					"updated":     "2025-06-14T08:00:00.000+0000",
					"creator":     map[string]any{"displayName": "Bob Jones"},
					"issuetype":   map[string]any{"name": "Story"},
					"status":      map[string]any{"name": "To Do"},
					"priority":    map[string]any{"name": "Medium"},
				},
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/search", func(w http.ResponseWriter, r *http.Request) {
		// Verify the request has the expected query parameters.
		jql := r.URL.Query().Get("jql")
		if jql == "" {
			t.Error("expected jql parameter in search request")
		}
		// Verify Basic Auth header is present.
		auth := r.Header.Get("Authorization")
		if auth == "" {
			t.Error("expected Authorization header")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResp)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := NewJiraSource()
	err := src.Configure(SourceConfig{
		Settings: map[string]string{
			"base_url":    srv.URL,
			"project_key": "PROJ",
		},
		Credentials: map[string]string{
			"jira_email": "alice@example.com",
			"jira_token": "test-token",
		},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}

	// Verify first issue.
	a := artifacts[0]
	if a.Source != "jira" {
		t.Errorf("Source = %q, want %q", a.Source, "jira")
	}
	if a.Category != Signal {
		t.Errorf("Category = %q, want Signal", a.Category)
	}
	if a.ID != "PROJ-123" {
		t.Errorf("ID = %q, want %q", a.ID, "PROJ-123")
	}
	if a.Title != "Fix authentication timeout" {
		t.Errorf("Title = %q, want %q", a.Title, "Fix authentication timeout")
	}
	if a.Body != "Users are getting logged out after 5 minutes of inactivity." {
		t.Errorf("Body = %q", a.Body)
	}
	if a.URL != srv.URL+"/browse/PROJ-123" {
		t.Errorf("URL = %q, want %q", a.URL, srv.URL+"/browse/PROJ-123")
	}
	if a.Author != "Alice Smith" {
		t.Errorf("Author = %q, want %q", a.Author, "Alice Smith")
	}
	if a.Date.IsZero() {
		t.Error("Date should not be zero")
	}
	if a.Tags["type"] != "Bug" {
		t.Errorf("Tags[type] = %q, want %q", a.Tags["type"], "Bug")
	}
	if a.Tags["status"] != "In Progress" {
		t.Errorf("Tags[status] = %q, want %q", a.Tags["status"], "In Progress")
	}
	if a.Tags["priority"] != "High" {
		t.Errorf("Tags[priority] = %q, want %q", a.Tags["priority"], "High")
	}

	// Verify second issue.
	b := artifacts[1]
	if b.ID != "PROJ-124" {
		t.Errorf("second issue ID = %q, want %q", b.ID, "PROJ-124")
	}
	if b.Tags["type"] != "Story" {
		t.Errorf("second issue Tags[type] = %q, want %q", b.Tags["type"], "Story")
	}
}
