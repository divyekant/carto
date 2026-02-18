package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropic/indexer/internal/config"
	"github.com/anthropic/indexer/internal/storage"
)

func TestHealthEndpoint(t *testing.T) {
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["status"] != "ok" {
		t.Errorf("expected status 'ok', got '%v'", resp["status"])
	}
	if resp["memories_healthy"] != true {
		t.Errorf("expected memories_healthy true, got '%v'", resp["memories_healthy"])
	}
}

func TestHealthEndpoint_MemoriesDown(t *testing.T) {
	// Point to unreachable server
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 even when memories is down, got %d", w.Code)
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["memories_healthy"] != false {
		t.Errorf("expected memories_healthy false when server is down, got '%v'", resp["memories_healthy"])
	}
}

func TestListProjects(t *testing.T) {
	// Create a temp directory with 3 subdirectories:
	// - projA and projB have .carto/manifest.json
	// - noindex has no manifest
	tmpDir := t.TempDir()

	// Project A: valid manifest with files
	projADir := filepath.Join(tmpDir, "projA")
	os.MkdirAll(filepath.Join(projADir, ".carto"), 0o755)
	mfA := map[string]any{
		"version":    "1.0",
		"project":    "projA",
		"indexed_at": time.Now().Add(-1 * time.Hour).Format(time.RFC3339),
		"files": map[string]any{
			"main.go": map[string]any{"hash": "abc", "size": 100, "indexed_at": time.Now().Format(time.RFC3339)},
			"util.go": map[string]any{"hash": "def", "size": 200, "indexed_at": time.Now().Format(time.RFC3339)},
		},
	}
	mfAData, _ := json.Marshal(mfA)
	os.WriteFile(filepath.Join(projADir, ".carto", "manifest.json"), mfAData, 0o644)

	// Project B: valid manifest with 1 file
	projBDir := filepath.Join(tmpDir, "projB")
	os.MkdirAll(filepath.Join(projBDir, ".carto"), 0o755)
	mfB := map[string]any{
		"version":    "1.0",
		"project":    "projB",
		"indexed_at": time.Now().Format(time.RFC3339),
		"files": map[string]any{
			"index.ts": map[string]any{"hash": "ghi", "size": 300, "indexed_at": time.Now().Format(time.RFC3339)},
		},
	}
	mfBData, _ := json.Marshal(mfB)
	os.WriteFile(filepath.Join(projBDir, ".carto", "manifest.json"), mfBData, 0o644)

	// No-index directory: just a plain directory, no manifest
	os.MkdirAll(filepath.Join(tmpDir, "noindex"), 0o755)

	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	srv := New(config.Config{}, memoriesClient, tmpDir)

	req := httptest.NewRequest(http.MethodGet, "/api/projects", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var projects []ProjectInfo
	if err := json.NewDecoder(w.Body).Decode(&projects); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(projects) != 2 {
		t.Fatalf("expected 2 projects, got %d: %+v", len(projects), projects)
	}

	// Build a map for easier assertions.
	byName := map[string]ProjectInfo{}
	for _, p := range projects {
		byName[p.Name] = p
	}

	if pa, ok := byName["projA"]; !ok {
		t.Error("expected projA in results")
	} else if pa.FileCount != 2 {
		t.Errorf("projA: expected 2 files, got %d", pa.FileCount)
	}

	if pb, ok := byName["projB"]; !ok {
		t.Error("expected projB in results")
	} else if pb.FileCount != 1 {
		t.Errorf("projB: expected 1 file, got %d", pb.FileCount)
	}
}
