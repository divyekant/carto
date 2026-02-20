package sources

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Compile-time interface check.
var _ Source = (*WebSource)(nil)

func TestWebSource_Name(t *testing.T) {
	src := NewWebSource()
	if src.Name() != "web" {
		t.Errorf("Name() = %q, want %q", src.Name(), "web")
	}
}

func TestWebSource_Scope(t *testing.T) {
	src := NewWebSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestWebSource_Configure(t *testing.T) {
	src := NewWebSource()
	err := src.Configure(SourceConfig{
		Settings: map[string]string{
			"urls": "https://example.com, https://example.org",
		},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if len(src.urls) != 2 {
		t.Fatalf("expected 2 urls, got %d", len(src.urls))
	}
	if src.urls[0] != "https://example.com" {
		t.Errorf("urls[0] = %q, want %q", src.urls[0], "https://example.com")
	}
	if src.urls[1] != "https://example.org" {
		t.Errorf("urls[1] = %q, want %q", src.urls[1], "https://example.org")
	}
}

func TestWebSource_Configure_MissingURLs(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]string
	}{
		{"empty settings", map[string]string{}},
		{"empty urls value", map[string]string{"urls": ""}},
		{"only whitespace", map[string]string{"urls": " , , "}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := NewWebSource()
			err := src.Configure(SourceConfig{Settings: tt.settings})
			if err == nil {
				t.Error("expected error when urls missing or empty")
			}
		})
	}
}

func TestWebSource_Fetch(t *testing.T) {
	const testHTML = `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<h1>Hello World</h1>
<p>This is a test page with some content.</p>
<script>var x = 1;</script>
<style>body { color: red; }</style>
</body>
</html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(testHTML))
	}))
	defer srv.Close()

	src := NewWebSource()
	err := src.Configure(SourceConfig{
		Settings: map[string]string{"urls": srv.URL},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}

	a := artifacts[0]
	if a.Source != "web" {
		t.Errorf("Source = %q, want %q", a.Source, "web")
	}
	if a.Category != Knowledge {
		t.Errorf("Category = %q, want Knowledge", a.Category)
	}
	if a.ID != srv.URL {
		t.Errorf("ID = %q, want %q", a.ID, srv.URL)
	}
	if a.Title != "Test Page" {
		t.Errorf("Title = %q, want %q", a.Title, "Test Page")
	}
	if a.URL != srv.URL {
		t.Errorf("URL = %q, want %q", a.URL, srv.URL)
	}
	if a.Date.IsZero() {
		t.Error("Date should not be zero")
	}
	if a.Tags["type"] != "webpage" {
		t.Errorf("Tags[type] = %q, want %q", a.Tags["type"], "webpage")
	}
	if a.Tags["content_type"] != "text/html; charset=utf-8" {
		t.Errorf("Tags[content_type] = %q, want %q", a.Tags["content_type"], "text/html; charset=utf-8")
	}
	// Body should contain visible text but not script/style content.
	if !strings.Contains(a.Body, "Hello World") {
		t.Errorf("Body should contain 'Hello World', got %q", a.Body)
	}
	if !strings.Contains(a.Body, "test page with some content") {
		t.Errorf("Body should contain page text, got %q", a.Body)
	}
	if strings.Contains(a.Body, "var x") {
		t.Error("Body should not contain script content")
	}
	if strings.Contains(a.Body, "color: red") {
		t.Error("Body should not contain style content")
	}
}

func TestWebSource_Fetch_SkipsBadURLs(t *testing.T) {
	const goodHTML = `<html><head><title>Good Page</title></head><body>Content</body></html>`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/good" {
			w.Header().Set("Content-Type", "text/html")
			w.Write([]byte(goodHTML))
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	src := NewWebSource()
	err := src.Configure(SourceConfig{
		Settings: map[string]string{
			"urls": srv.URL + "/bad, " + srv.URL + "/good",
		},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch returned error: %v (should succeed with partial failures)", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact (bad URL skipped), got %d", len(artifacts))
	}
	if artifacts[0].Title != "Good Page" {
		t.Errorf("Title = %q, want %q", artifacts[0].Title, "Good Page")
	}
}
