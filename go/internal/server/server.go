package server

import (
	"io/fs"
	"net/http"
	"strings"
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
	// handler is the fully-composed middleware chain wrapping mux.
	// ServeHTTP delegates to handler instead of mux directly so all
	// middleware (auth, logging, rate-limiting, CORS) runs on every request.
	handler http.Handler
}

// New creates a new Server with the given config. If webFS is non-nil the
// server will serve the embedded SPA and fall back to index.html for
// client-side routes.
//
// Middleware applied (outermost → innermost):
//  1. Request-ID — generate / propagate X-Request-ID for log correlation
//  2. CORS       — set Access-Control-* headers per CARTO_CORS_ORIGINS
//  3. Logging    — structured JSON request/response logging to stdout
//  4. Audit      — extra JSON log line for every mutating operation
//  5. Rate limit — 60 req/min per IP, burst 10
//  6. Bearer auth — token gate when CARTO_SERVER_TOKEN is set
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

	// Build CORS allowed-origins list from config.
	var corsOrigins []string
	if cfg.CORSOrigins != "" {
		for _, o := range strings.Split(cfg.CORSOrigins, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				corsOrigins = append(corsOrigins, trimmed)
			}
		}
	}

	// Compose middleware chain (first listed = outermost = executed first).
	rl := newRateLimiter()
	s.handler = chain(
		s.mux,
		requestIDMiddleware,     // assign X-Request-ID early so all downstream logs include it
		corsMiddleware(corsOrigins),
		loggingMiddleware,
		auditMiddleware,         // extra audit log for POST/PATCH/PUT/DELETE
		rateLimitMiddleware(rl),
		bearerAuth(cfg.ServerToken),
	)

	return s
}

// ServeHTTP implements http.Handler — delegates to the full middleware chain.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.handler.ServeHTTP(w, r)
}

// Start runs the HTTP server on the given address.
func (s *Server) Start(addr string) error {
	serverLog.Info("server_start", "addr", addr)
	return http.ListenAndServe(addr, s)
}
