package server

import (
	"net/http"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
	s.mux.HandleFunc("POST /api/query", s.handleQuery)
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PATCH /api/config", s.handlePatchConfig)
	s.mux.HandleFunc("POST /api/projects/index", s.handleStartIndex)
	s.mux.HandleFunc("GET /api/projects/{name}/progress", s.handleProgress)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	healthy, _ := s.memoriesClient.Health()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"memories_healthy": healthy,
	})
}
