package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
	srv := New(config.Config{}, memoriesClient)

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
	srv := New(config.Config{}, memoriesClient)

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
