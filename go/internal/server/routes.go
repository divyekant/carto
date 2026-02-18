package server

import (
	"encoding/json"
	"net/http"
)

func (s *Server) routes() {
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	healthy, _ := s.memoriesClient.Health()
	resp := map[string]any{
		"status":           "ok",
		"memories_healthy": healthy,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
