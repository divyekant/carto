package server

import (
	"encoding/json"
	"io/fs"
	"net/http"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/storage"
)

func (s *Server) routes() {
	// ── Health probes ──────────────────────────────────────────────────────
	// /healthz           Kubernetes-standard root-level liveness probe.
	//                    Useful for Docker HEALTHCHECK and load-balancer
	//                    probes that cannot reach /api/* paths.
	// /api/health        legacy combined endpoint (backward compat)
	// /api/health/live   liveness:  is the process alive? (no deps checked)
	// /api/health/ready  readiness: are all dependencies reachable?
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /api/health", s.handleHealth)
	s.mux.HandleFunc("GET /api/health/live", s.handleLiveness)
	s.mux.HandleFunc("GET /api/health/ready", s.handleReadiness)

	// ── Project management ─────────────────────────────────────────────────
	s.mux.HandleFunc("GET /api/projects", s.handleListProjects)
	s.mux.HandleFunc("GET /api/projects/runs", s.handleListRuns)
	s.mux.HandleFunc("POST /api/projects/index", s.handleStartIndex)
	s.mux.HandleFunc("POST /api/projects/index-all", s.handleIndexAll)
	s.mux.HandleFunc("GET /api/projects/{name}", s.handleGetProject)
	s.mux.HandleFunc("DELETE /api/projects/{name}", s.handleDeleteProject)
	s.mux.HandleFunc("GET /api/projects/{name}/progress", s.handleProgress)
	s.mux.HandleFunc("POST /api/projects/{name}/stop", s.handleStopIndex)
	s.mux.HandleFunc("GET /api/projects/{name}/sources", s.handleGetSources)
	s.mux.HandleFunc("PUT /api/projects/{name}/sources", s.handlePutSources)

	// ── Query & search ─────────────────────────────────────────────────────
	s.mux.HandleFunc("POST /api/query", s.handleQuery)

	// ── Config & diagnostics ───────────────────────────────────────────────
	s.mux.HandleFunc("GET /api/config", s.handleGetConfig)
	s.mux.HandleFunc("PATCH /api/config", s.handlePatchConfig)
	s.mux.HandleFunc("POST /api/test-memories", s.handleTestMemories)
	s.mux.HandleFunc("GET /api/browse", s.handleBrowse)

	// ── Observability ──────────────────────────────────────────────────────
	// /api/metrics returns lightweight runtime metrics (uptime, goroutines,
	// memory, active runs, request count). Bearer-auth protected when enabled.
	s.mux.HandleFunc("GET /api/metrics", s.handleMetrics)

	// ── Product identity ───────────────────────────────────────────────────
	// /api/about returns the product identity card: tagline, description,
	// audience, how-it-works, features, and brand color palette.
	// Intentionally unauthenticated so external dashboards can embed it.
	s.mux.HandleFunc("GET /api/about", s.handleAbout)

	// ── SPA static assets + client-side routing fallback ──────────────────
	if s.webFS != nil {
		s.mux.HandleFunc("GET /", s.handleSPA)
	}
}

// handleHealth is the legacy combined health endpoint kept for backward
// compatibility with existing monitoring integrations.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	healthy, _ := s.memoriesClient.Health()
	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "ok",
		"memories_healthy": healthy,
		"docker":           config.IsDocker(),
		"version":          config.Version,
	})
}

// handleHealthz is the Kubernetes-standard root-level liveness endpoint.
// Unlike /api/health/live it lives outside the /api/ prefix so it works
// out-of-the-box with Docker HEALTHCHECK and load balancers whose probe
// paths cannot be configured (e.g. GCP Cloud Run, Fly.io default checks).
//
// It intentionally performs zero external-dependency checks — if the HTTP
// server can respond at all, the process is considered live. Use
// /api/health/ready for readiness probing with Memories dependency checks.
//
// Authentication is not required: the bearerAuth middleware already passes
// all paths that do not start with /api/ without checking the token.
func (s *Server) handleHealthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "ok",
		"version": config.Version,
	})
}

// handleLiveness responds 200 OK as long as the HTTP server is running.
// Container orchestrators (Kubernetes, ECS) use this to decide whether to
// restart the pod — it intentionally never checks external dependencies.
func (s *Server) handleLiveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "alive",
		"version": config.Version,
	})
}

// handleReadiness probes all required dependencies and returns 503 when any
// are unavailable. Orchestrators stop routing traffic to an unready pod
// without restarting it — appropriate for transient outages.
func (s *Server) handleReadiness(w http.ResponseWriter, _ *http.Request) {
	healthy, err := s.memoriesClient.Health()

	deps := map[string]any{
		"memories": map[string]any{
			"healthy": healthy,
			"url":     s.cfg.MemoriesURL,
		},
	}

	if err != nil {
		deps["memories"].(map[string]any)["error"] = err.Error()
	}

	status := http.StatusOK
	overall := "ready"
	if !healthy {
		status = http.StatusServiceUnavailable
		overall = "not_ready"
	}

	writeJSON(w, status, map[string]any{
		"status":       overall,
		"version":      config.Version,
		"dependencies": deps,
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

	testURL := config.ResolveURL(req.URL)
	apiKey := req.APIKey
	if apiKey == "" {
		s.cfgMu.RLock()
		apiKey = s.cfg.MemoriesKey
		s.cfgMu.RUnlock()
	}

	client := storage.NewMemoriesClient(testURL, apiKey)
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
