package server

import (
	"io/fs"
	"log"
	"net/http"
	"sync"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/storage"
)

// Server holds the dependencies for the Carto web UI.
type Server struct {
	cfg            config.Config
	cfgMu          sync.RWMutex
	memoriesClient *storage.MemoriesClient
	projectsDir    string
	runs           *RunManager
	webFS          fs.FS
	mux            *http.ServeMux
}

// New creates a new Server with the given config. If webFS is non-nil the
// server will serve the embedded SPA and fall back to index.html for
// client-side routes.
func New(cfg config.Config, memoriesClient *storage.MemoriesClient, projectsDir string, webFS fs.FS) *Server {
	s := &Server{
		cfg:            cfg,
		memoriesClient: memoriesClient,
		projectsDir:    projectsDir,
		runs:           NewRunManager(),
		webFS:          webFS,
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
