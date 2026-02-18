package server

import (
	"io/fs"
	"net/http"
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
