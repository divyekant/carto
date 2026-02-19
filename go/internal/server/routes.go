package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/divyekant/carto/internal/storage"
)

func (s *Server) routes() {
	// API routes.
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
	s.mux.HandleFunc("POST /api/query", s.handleQuery)
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PATCH /api/config", s.handlePatchConfig)
	s.mux.HandleFunc("POST /api/projects/index", s.handleStartIndex)
	s.mux.HandleFunc("GET /api/projects/{name}/progress", s.handleProgress)
	s.mux.HandleFunc("POST /api/test-memories", s.handleTestMemories)

	// SPA static files + fallback (only when embedded assets are provided).
	if s.webFS != nil {
		s.mux.HandleFunc("GET /", s.handleSPA)
	}
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	healthy, _ := s.memoriesClient.Health()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"memories_healthy": healthy,
	})
}

// handleTestMemories tests connectivity to a Memories server using the
// URL and API key provided in the request body (not the server's saved config).
func (s *Server) handleTestMemories(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL    string `json:"url"`
		APIKey string `json:"api_key"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if req.URL == "" {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false, "error": "URL is required"})
		return
	}

	// When running inside Docker, rewrite localhost to host.docker.internal
	// so the server can reach services on the host machine.
	testURL := req.URL
	if isDocker() {
		testURL = strings.Replace(testURL, "localhost", "host.docker.internal", 1)
		testURL = strings.Replace(testURL, "127.0.0.1", "host.docker.internal", 1)
	}

	client := storage.NewMemoriesClient(testURL, req.APIKey)
	healthy, err := client.Health()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false, "error": err.Error()})
		return
	}
	if !healthy {
		writeJSON(w, http.StatusOK, map[string]any{"connected": false, "error": "Server returned unhealthy status"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"connected": true})
}

// isDocker returns true when running inside a Docker container.
func isDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// handleSPA serves static files from the embedded web FS and falls back to
// index.html for any path that does not match a real file (SPA client-side
// routing).
func (s *Server) handleSPA(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if path == "/" {
		path = "/index.html"
	}

	// Try to serve static file.
	fsPath := path[1:] // strip leading /
	f, err := s.webFS.Open(fsPath)
	if err == nil {
		f.Close()
		http.FileServerFS(s.webFS).ServeHTTP(w, r)
		return
	}

	// SPA fallback: serve index.html for client-side routing.
	data, err := fs.ReadFile(s.webFS, "index.html")
	if err != nil {
		http.Error(w, "index.html not found", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(data)
}
