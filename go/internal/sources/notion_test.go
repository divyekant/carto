package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// Compile-time interface check.
var _ Source = (*NotionSource)(nil)

func TestNotionSource_Name(t *testing.T) {
	src := NewNotionSource()
	if src.Name() != "notion" {
		t.Errorf("Name() = %q, want %q", src.Name(), "notion")
	}
}

func TestNotionSource_Scope(t *testing.T) {
	src := NewNotionSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestNotionSource_Configure(t *testing.T) {
	src := NewNotionSource()
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{"database_id": "db-123"},
		Credentials: map[string]string{"notion_token": "ntn_test"},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if src.databaseID != "db-123" {
		t.Errorf("databaseID = %q, want %q", src.databaseID, "db-123")
	}
	if src.token != "ntn_test" {
		t.Errorf("token = %q, want %q", src.token, "ntn_test")
	}
}

func TestNotionSource_Configure_MissingDatabaseID(t *testing.T) {
	src := NewNotionSource()
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{},
		Credentials: map[string]string{"notion_token": "ntn_test"},
	})
	if err == nil {
		t.Error("expected error when database_id is missing")
	}
}

func TestNotionSource_Fetch(t *testing.T) {
	// Mock Notion database query response.
	queryResp := map[string]any{
		"results": []map[string]any{
			{
				"id":               "page-abc-123",
				"url":              "https://www.notion.so/page-abc-123",
				"last_edited_time": "2025-06-15T10:30:00Z",
				"properties": map[string]any{
					"Name": map[string]any{
						"type": "title",
						"title": []map[string]any{
							{"plain_text": "Architecture Decision Record"},
						},
					},
				},
			},
			{
				"id":               "page-def-456",
				"url":              "https://www.notion.so/page-def-456",
				"last_edited_time": "2025-06-14T08:00:00Z",
				"properties": map[string]any{
					"Title": map[string]any{
						"type": "title",
						"title": []map[string]any{
							{"plain_text": "Onboarding Guide"},
						},
					},
				},
			},
		},
	}

	// Mock block content for page-abc-123.
	blocksABC := map[string]any{
		"results": []map[string]any{
			{
				"type": "heading_1",
				"heading_1": map[string]any{
					"rich_text": []map[string]any{
						{"plain_text": "Overview"},
					},
				},
			},
			{
				"type": "paragraph",
				"paragraph": map[string]any{
					"rich_text": []map[string]any{
						{"plain_text": "We chose a microservices architecture for scalability."},
					},
				},
			},
		},
	}

	// Mock block content for page-def-456.
	blocksDEF := map[string]any{
		"results": []map[string]any{
			{
				"type": "paragraph",
				"paragraph": map[string]any{
					"rich_text": []map[string]any{
						{"plain_text": "Welcome to the team! Here is how to get started."},
					},
				},
			},
		},
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/databases/db-test-789/query", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer ntn_secret" {
			t.Errorf("missing or wrong Authorization header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Notion-Version") != "2022-06-28" {
			t.Errorf("missing or wrong Notion-Version header: %s", r.Header.Get("Notion-Version"))
		}

		// Verify request body structure.
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}
		if _, ok := body["page_size"]; !ok {
			t.Error("request body missing page_size")
		}

		json.NewEncoder(w).Encode(queryResp)
	})

	mux.HandleFunc("/blocks/page-abc-123/children", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("expected GET for blocks, got %s", r.Method)
		}
		json.NewEncoder(w).Encode(blocksABC)
	})

	mux.HandleFunc("/blocks/page-def-456/children", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(blocksDEF)
	})

	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := NewNotionSource()
	src.baseURL = srv.URL
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{"database_id": "db-test-789"},
		Credentials: map[string]string{"notion_token": "ntn_secret"},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test-project"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}

	// Verify first artifact (page-abc-123).
	a0 := artifacts[0]
	if a0.Source != "notion" {
		t.Errorf("Source = %q, want %q", a0.Source, "notion")
	}
	if a0.Category != Knowledge {
		t.Errorf("Category = %q, want %q", a0.Category, Knowledge)
	}
	if a0.ID != "page-abc-123" {
		t.Errorf("ID = %q, want %q", a0.ID, "page-abc-123")
	}
	if a0.Title != "Architecture Decision Record" {
		t.Errorf("Title = %q, want %q", a0.Title, "Architecture Decision Record")
	}
	if a0.URL != "https://www.notion.so/page-abc-123" {
		t.Errorf("URL = %q, want %q", a0.URL, "https://www.notion.so/page-abc-123")
	}
	if a0.Tags["type"] != "page" {
		t.Errorf("Tags[type] = %q, want %q", a0.Tags["type"], "page")
	}
	if a0.Author != "" {
		t.Errorf("Author = %q, want empty string", a0.Author)
	}
	if a0.Date.IsZero() {
		t.Error("Date should not be zero")
	}
	// Body should contain block text.
	if len(a0.Body) == 0 {
		t.Error("Body should not be empty")
	}

	// Verify second artifact (page-def-456).
	a1 := artifacts[1]
	if a1.ID != "page-def-456" {
		t.Errorf("ID = %q, want %q", a1.ID, "page-def-456")
	}
	if a1.Title != "Onboarding Guide" {
		t.Errorf("Title = %q, want %q", a1.Title, "Onboarding Guide")
	}
}
