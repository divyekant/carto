package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleBrowse(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "project-a"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "project-b"), 0o755)
	os.MkdirAll(filepath.Join(tmp, ".hidden"), 0o755)
	os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hi"), 0o644)

	srv := &Server{projectsDir: tmp}

	req := httptest.NewRequest("GET", "/api/browse?path="+tmp, nil)
	rec := httptest.NewRecorder()
	srv.handleBrowse(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result browseResponse
	json.NewDecoder(rec.Body).Decode(&result)

	if result.Current != tmp {
		t.Errorf("expected current=%s, got %s", tmp, result.Current)
	}
	// Should have 2 directories (project-a, project-b), NOT .hidden or file.txt
	if len(result.Directories) != 2 {
		t.Errorf("expected 2 directories, got %d", len(result.Directories))
	}
}

func TestHandleBrowse_DefaultPath(t *testing.T) {
	tmp := t.TempDir()
	srv := &Server{projectsDir: tmp}

	req := httptest.NewRequest("GET", "/api/browse", nil)
	rec := httptest.NewRecorder()
	srv.handleBrowse(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var result browseResponse
	json.NewDecoder(rec.Body).Decode(&result)

	if result.Current != tmp {
		t.Errorf("expected default to projects dir %s, got %s", tmp, result.Current)
	}
}
