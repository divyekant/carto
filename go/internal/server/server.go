package server

import (
	"log"
	"net/http"

	"github.com/anthropic/indexer/internal/config"
	"github.com/anthropic/indexer/internal/storage"
)

// Server holds the dependencies for the Carto web UI.
type Server struct {
	cfg            config.Config
	memoriesClient *storage.MemoriesClient
	projectsDir    string
	mux            *http.ServeMux
}

// New creates a new Server with the given config.
func New(cfg config.Config, memoriesClient *storage.MemoriesClient, projectsDir string) *Server {
	s := &Server{
		cfg:            cfg,
		memoriesClient: memoriesClient,
		projectsDir:    projectsDir,
		mux:            http.NewServeMux(),
	}
	s.routes()
	return s
}

// ServeHTTP implements http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Start runs the HTTP server on the given address.
func (s *Server) Start(addr string) error {
	log.Printf("Carto server starting on %s", addr)
	return http.ListenAndServe(addr, s)
}
