package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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

func TestQueryEndpoint(t *testing.T) {
	// Mock memories server that returns search results for POST /search.
	memSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/search" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"results": []map[string]any{
					{"id": 1, "text": "function handleAuth() {...}", "score": 0.95, "source": "carto/myproj/auth/layer:atoms"},
					{"id": 2, "text": "JWT token validation", "score": 0.88, "source": "carto/myproj/auth/layer:zones"},
				},
			})
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer memSrv.Close()

	memoriesClient := storage.NewMemoriesClient(memSrv.URL, "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	body := strings.NewReader(`{"text": "how does auth work?", "k": 5}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	results, ok := resp["results"].([]any)
	if !ok {
		t.Fatalf("expected results array, got %T", resp["results"])
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestQueryEndpoint_MissingText(t *testing.T) {
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	srv := New(config.Config{}, memoriesClient, "")

	body := strings.NewReader(`{"project": "myproj"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/query", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing text, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["error"] != "text is required" {
		t.Errorf("expected 'text is required' error, got '%v'", resp["error"])
	}
}

func TestGetConfig(t *testing.T) {
	cfg := config.Config{
		MemoriesURL:   "http://localhost:8900",
		MemoriesKey:   "god-is-an-astronaut-key",
		AnthropicKey:  "sk-ant-api03-very-long-secret-key-value",
		HaikuModel:    "claude-haiku-4-5-20251001",
		OpusModel:     "claude-opus-4-6",
		MaxConcurrent: 10,
		LLMProvider:   "anthropic",
		LLMApiKey:     "sk-llm-0123456789abcdef-secret",
		LLMBaseURL:    "",
	}
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	srv := New(cfg, memoriesClient, "")

	req := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp configResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Non-secret fields should be returned as-is.
	if resp.MemoriesURL != "http://localhost:8900" {
		t.Errorf("unexpected memories_url: %s", resp.MemoriesURL)
	}
	if resp.HaikuModel != "claude-haiku-4-5-20251001" {
		t.Errorf("unexpected haiku_model: %s", resp.HaikuModel)
	}

	// Secret fields should be redacted: first 8 + **** + last 4.
	if resp.AnthropicKey == cfg.AnthropicKey {
		t.Error("anthropic_key should be redacted, but was returned in full")
	}
	if !strings.Contains(resp.AnthropicKey, "****") {
		t.Errorf("anthropic_key should contain ****, got %q", resp.AnthropicKey)
	}
	if !strings.HasPrefix(resp.AnthropicKey, "sk-ant-a") {
		t.Errorf("anthropic_key should start with first 8 chars, got %q", resp.AnthropicKey)
	}

	if resp.LLMApiKey == cfg.LLMApiKey {
		t.Error("llm_api_key should be redacted")
	}
	if !strings.Contains(resp.LLMApiKey, "****") {
		t.Errorf("llm_api_key should contain ****, got %q", resp.LLMApiKey)
	}
}

func TestPatchConfig(t *testing.T) {
	cfg := config.Config{
		MemoriesURL:   "http://localhost:8900",
		HaikuModel:    "claude-haiku-4-5-20251001",
		MaxConcurrent: 10,
	}
	memoriesClient := storage.NewMemoriesClient("http://127.0.0.1:1", "test-key")
	srv := New(cfg, memoriesClient, "")

	// PATCH to update haiku_model and max_concurrent.
	patchBody := strings.NewReader(`{"haiku_model": "claude-haiku-4-5-20260101", "max_concurrent": 20}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/config", patchBody)
	patchReq.Header.Set("Content-Type", "application/json")
	pw := httptest.NewRecorder()
	srv.ServeHTTP(pw, patchReq)

	if pw.Code != http.StatusOK {
		t.Fatalf("PATCH expected 200, got %d: %s", pw.Code, pw.Body.String())
	}

	// GET to verify the mutation persisted.
	getReq := httptest.NewRequest(http.MethodGet, "/api/config", nil)
	gw := httptest.NewRecorder()
	srv.ServeHTTP(gw, getReq)

	if gw.Code != http.StatusOK {
		t.Fatalf("GET expected 200, got %d", gw.Code)
	}

	var resp configResponse
	if err := json.NewDecoder(gw.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if resp.HaikuModel != "claude-haiku-4-5-20260101" {
		t.Errorf("expected patched haiku_model, got %q", resp.HaikuModel)
	}
	if resp.MaxConcurrent != 20 {
		t.Errorf("expected patched max_concurrent=20, got %d", resp.MaxConcurrent)
	}
	// Unchanged field should remain the same.
	if resp.MemoriesURL != "http://localhost:8900" {
		t.Errorf("memories_url should be unchanged, got %q", resp.MemoriesURL)
	}
}
