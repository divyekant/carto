package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Compile-time interface check.
var _ Source = (*LinearSource)(nil)

func TestLinearSource_Name(t *testing.T) {
	src := NewLinearSource()
	if src.Name() != "linear" {
		t.Errorf("Name() = %q, want %q", src.Name(), "linear")
	}
}

func TestLinearSource_Scope(t *testing.T) {
	src := NewLinearSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestLinearSource_Configure(t *testing.T) {
	src := NewLinearSource()
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{"team_key": "ENG"},
		Credentials: map[string]string{"linear_token": "lin_test_token"},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if src.teamKey != "ENG" {
		t.Errorf("teamKey = %q, want %q", src.teamKey, "ENG")
	}
	if src.token != "lin_test_token" {
		t.Errorf("token = %q, want %q", src.token, "lin_test_token")
	}
}

func TestLinearSource_Configure_MissingTeamKey(t *testing.T) {
	src := NewLinearSource()
	err := src.Configure(SourceConfig{
		Settings: map[string]string{},
	})
	if err == nil {
		t.Error("expected error when team_key missing")
	}
}

func TestLinearSource_Fetch(t *testing.T) {
	gqlResponse := map[string]any{
		"data": map[string]any{
			"issues": map[string]any{
				"nodes": []map[string]any{
					{
						"identifier":  "ENG-123",
						"title":       "Fix auth flow",
						"description": "Users cannot log in with SSO",
						"url":         "https://linear.app/team/issue/ENG-123",
						"updatedAt":   "2025-06-15T10:30:00Z",
						"creator":     map[string]any{"name": "Alice"},
						"state":       map[string]any{"name": "In Progress"},
						"priority":    2,
						"labels": map[string]any{
							"nodes": []map[string]any{
								{"name": "bug"},
								{"name": "auth"},
							},
						},
					},
					{
						"identifier":  "ENG-456",
						"title":       "Add dark mode",
						"description": "",
						"url":         "https://linear.app/team/issue/ENG-456",
						"updatedAt":   "2025-06-14T08:00:00Z",
						"creator":     map[string]any{"name": "Bob"},
						"state":       map[string]any{"name": "Todo"},
						"priority":    3,
						"labels": map[string]any{
							"nodes": []map[string]any{},
						},
					},
				},
			},
		},
	}

	var receivedAuth string
	var receivedContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedContentType = r.Header.Get("Content-Type")

		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["query"] == "" {
			t.Error("expected non-empty query in request body")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(gqlResponse)
	}))
	defer srv.Close()

	src := NewLinearSource()
	src.apiURL = srv.URL
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{"team_key": "ENG"},
		Credentials: map[string]string{"linear_token": "lin_test_token"},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	// Verify auth header was sent.
	if receivedAuth != "Bearer lin_test_token" {
		t.Errorf("Authorization = %q, want %q", receivedAuth, "Bearer lin_test_token")
	}
	if receivedContentType != "application/json" {
		t.Errorf("Content-Type = %q, want %q", receivedContentType, "application/json")
	}

	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}

	// Verify first issue.
	a := artifacts[0]
	if a.Source != "linear" {
		t.Errorf("Source = %q, want %q", a.Source, "linear")
	}
	if a.Category != Signal {
		t.Errorf("Category = %q, want Signal", a.Category)
	}
	if a.ID != "ENG-123" {
		t.Errorf("ID = %q, want %q", a.ID, "ENG-123")
	}
	if a.Title != "Fix auth flow" {
		t.Errorf("Title = %q, want %q", a.Title, "Fix auth flow")
	}
	if a.Body != "Users cannot log in with SSO" {
		t.Errorf("Body = %q, want %q", a.Body, "Users cannot log in with SSO")
	}
	if a.URL != "https://linear.app/team/issue/ENG-123" {
		t.Errorf("URL = %q, want %q", a.URL, "https://linear.app/team/issue/ENG-123")
	}
	if a.Author != "Alice" {
		t.Errorf("Author = %q, want %q", a.Author, "Alice")
	}
	if a.Tags["type"] != "issue" {
		t.Errorf("Tags[type] = %q, want %q", a.Tags["type"], "issue")
	}
	if a.Tags["status"] != "In Progress" {
		t.Errorf("Tags[status] = %q, want %q", a.Tags["status"], "In Progress")
	}
	if a.Tags["priority"] != "2" {
		t.Errorf("Tags[priority] = %q, want %q", a.Tags["priority"], "2")
	}
	if a.Tags["labels"] != "bug,auth" {
		t.Errorf("Tags[labels] = %q, want %q", a.Tags["labels"], "bug,auth")
	}
	if a.Date.IsZero() {
		t.Error("Date should not be zero")
	}

	// Verify second issue (no labels).
	b := artifacts[1]
	if b.ID != "ENG-456" {
		t.Errorf("ID = %q, want %q", b.ID, "ENG-456")
	}
	if b.Author != "Bob" {
		t.Errorf("Author = %q, want %q", b.Author, "Bob")
	}
	if _, hasLabels := b.Tags["labels"]; hasLabels {
		t.Errorf("expected no labels tag for second issue, got %q", b.Tags["labels"])
	}
	if b.Tags["status"] != "Todo" {
		t.Errorf("Tags[status] = %q, want %q", b.Tags["status"], "Todo")
	}
}
