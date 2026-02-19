package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/manifest"
	"github.com/divyekant/carto/internal/pipeline"
	"github.com/divyekant/carto/internal/signals"
	"github.com/divyekant/carto/internal/storage"
)

// ProjectInfo describes an indexed project discovered in the projects directory.
type ProjectInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IndexedAt time.Time `json:"indexed_at"`
	FileCount int       `json:"file_count"`
}

// writeJSON marshals v as JSON and writes it to the response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleListProjects scans projectsDir for subdirectories that contain a
// .carto/manifest.json and returns their metadata as a JSON array.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if s.projectsDir == "" {
		writeJSON(w, http.StatusOK, []ProjectInfo{})
		return
	}

	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read projects directory")
		return
	}

	var projects []ProjectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectRoot := filepath.Join(s.projectsDir, entry.Name())
		mf, err := manifest.Load(projectRoot)
		if err != nil {
			continue
		}
		// Skip directories without a manifest file (empty manifest = no project).
		if mf.IsEmpty() && mf.Project == "" {
			continue
		}

		projects = append(projects, ProjectInfo{
			Name:      mf.Project,
			Path:      projectRoot,
			IndexedAt: mf.IndexedAt,
			FileCount: len(mf.Files),
		})
	}

	if projects == nil {
		projects = []ProjectInfo{}
	}

	writeJSON(w, http.StatusOK, projects)
}

// queryRequest is the JSON body for POST /api/query.
type queryRequest struct {
	Text    string `json:"text"`
	Project string `json:"project"`
	Tier    string `json:"tier"`
	K       int    `json:"k"`
}

// queryResultItem is a single result in the query response.
type queryResultItem struct {
	Text   string  `json:"text"`
	Source string  `json:"source"`
	Score  float64 `json:"score"`
	Layer  string  `json:"layer,omitempty"`
}

// handleQuery searches the memories index. If a project is specified, it uses
// tier-based retrieval and flattens the results. Otherwise it performs a
// free-form hybrid search across all projects.
func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	var req queryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Text == "" {
		writeError(w, http.StatusBadRequest, "text is required")
		return
	}

	if req.Tier == "" {
		req.Tier = "standard"
	}
	if req.K == 0 {
		req.K = 10
	}

	if req.Project != "" {
		// Tier-based retrieval for a specific project.
		store := storage.NewStore(s.memoriesClient, req.Project)
		tierResults, err := store.RetrieveByTier(req.Text, storage.Tier(req.Tier))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// Flatten the map of layer results into a single list.
		var items []queryResultItem
		for layer, results := range tierResults {
			for _, sr := range results {
				items = append(items, queryResultItem{
					Text:   sr.Text,
					Source: sr.Source,
					Score:  sr.Score,
					Layer:  layer,
				})
			}
		}
		if items == nil {
			items = []queryResultItem{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": items})
		return
	}

	// Free-form search across all projects.
	results, err := s.memoriesClient.Search(req.Text, storage.SearchOptions{
		K:      req.K,
		Hybrid: true,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]queryResultItem, len(results))
	for i, sr := range results {
		items[i] = queryResultItem{
			Text:   sr.Text,
			Source: sr.Source,
			Score:  sr.Score,
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"results": items})
}

// redactKey masks the middle of an API key, showing the first 8 and last 4
// characters with **** in between. Keys shorter than 16 characters are fully
// redacted to avoid leaking too much of short keys.
func redactKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) < 16 {
		return "****"
	}
	return key[:8] + "****" + key[len(key)-4:]
}

// configResponse is the JSON shape returned by GET /api/config.
type configResponse struct {
	MemoriesURL   string `json:"memories_url"`
	MemoriesKey   string `json:"memories_key"`
	AnthropicKey  string `json:"anthropic_key"`
	FastModel     string `json:"fast_model"`
	DeepModel     string `json:"deep_model"`
	MaxConcurrent int    `json:"max_concurrent"`
	LLMProvider   string `json:"llm_provider"`
	LLMApiKey     string `json:"llm_api_key"`
	LLMBaseURL    string `json:"llm_base_url"`
}

// handleGetConfig returns the current server config with API keys redacted.
func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()

	writeJSON(w, http.StatusOK, configResponse{
		MemoriesURL:   cfg.MemoriesURL,
		MemoriesKey:   redactKey(cfg.MemoriesKey),
		AnthropicKey:  redactKey(cfg.AnthropicKey),
		FastModel:     cfg.FastModel,
		DeepModel:     cfg.DeepModel,
		MaxConcurrent: cfg.MaxConcurrent,
		LLMProvider:   cfg.LLMProvider,
		LLMApiKey:     redactKey(cfg.LLMApiKey),
		LLMBaseURL:    cfg.LLMBaseURL,
	})
}

// handlePatchConfig applies partial updates to the server config.
func (s *Server) handlePatchConfig(w http.ResponseWriter, r *http.Request) {
	var patch map[string]any
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	s.cfgMu.Lock()
	for key, val := range patch {
		switch key {
		case "memories_url":
			if v, ok := val.(string); ok {
				s.cfg.MemoriesURL = v
			}
		case "memories_key":
			if v, ok := val.(string); ok {
				s.cfg.MemoriesKey = v
			}
		case "anthropic_key":
			if v, ok := val.(string); ok {
				s.cfg.AnthropicKey = v
			}
		case "fast_model":
			if v, ok := val.(string); ok {
				s.cfg.FastModel = v
			}
		case "deep_model":
			if v, ok := val.(string); ok {
				s.cfg.DeepModel = v
			}
		case "max_concurrent":
			if v, ok := val.(float64); ok {
				s.cfg.MaxConcurrent = int(v)
			}
		case "llm_provider":
			if v, ok := val.(string); ok {
				s.cfg.LLMProvider = v
			}
		case "llm_api_key":
			if v, ok := val.(string); ok {
				s.cfg.LLMApiKey = v
			}
		case "llm_base_url":
			if v, ok := val.(string); ok {
				s.cfg.LLMBaseURL = v
			}
		}
	}
	s.cfgMu.Unlock()

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// indexRequest is the JSON body for POST /api/projects/index.
type indexRequest struct {
	Path        string `json:"path"`
	Incremental bool   `json:"incremental"`
	Module      string `json:"module"`
	Project     string `json:"project"`
}

// handleStartIndex launches an asynchronous pipeline.Run for the given path.
// Returns 202 Accepted with the project name, or 409 if already running.
func (s *Server) handleStartIndex(w http.ResponseWriter, r *http.Request) {
	var req indexRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path is required")
		return
	}

	absPath, err := filepath.Abs(req.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	projectName := req.Project
	if projectName == "" {
		projectName = filepath.Base(absPath)
	}

	run := s.runs.Start(projectName)
	if run == nil {
		writeError(w, http.StatusConflict, "index already running for project "+projectName)
		return
	}

	// Read current config under read lock.
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()

	go s.runIndex(run, projectName, absPath, req, cfg)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"project": projectName,
		"status":  "started",
	})
}

// runIndex executes the pipeline in a goroutine and sends progress/result via the IndexRun.
func (s *Server) runIndex(run *IndexRun, projectName, absPath string, req indexRequest, cfg config.Config) {
	defer s.runs.Finish(projectName)

	start := time.Now()

	apiKey := cfg.LLMApiKey
	if apiKey == "" {
		apiKey = cfg.AnthropicKey
	}

	llmClient := llm.NewClient(llm.Options{
		APIKey:        apiKey,
		FastModel:     cfg.FastModel,
		DeepModel:     cfg.DeepModel,
		MaxConcurrent: cfg.MaxConcurrent,
		IsOAuth:       config.IsOAuthToken(apiKey),
		BaseURL:       cfg.LLMBaseURL,
	})

	registry := signals.NewRegistry()
	registry.Register(signals.NewGitSignalSource(absPath))

	result, err := pipeline.Run(pipeline.Config{
		ProjectName:    projectName,
		RootPath:       absPath,
		LLMClient:      llmClient,
		MemoriesClient: s.memoriesClient,
		SignalRegistry: registry,
		MaxWorkers:     cfg.MaxConcurrent,
		ProgressFn: func(phase string, done, total int) {
			run.SendProgress(phase, done, total)
		},
		LogFn: func(level, msg string) {
			run.SendLog(level, msg)
		},
		Incremental:  req.Incremental,
		ModuleFilter: req.Module,
	})
	if err != nil {
		run.SendError(err.Error())
		return
	}

	elapsed := time.Since(start)

	errMsgs := make([]string, len(result.Errors))
	for i, e := range result.Errors {
		errMsgs[i] = e.Error()
	}

	run.SendResult(IndexResult{
		Modules: result.Modules,
		Files:   result.FilesIndexed,
		Atoms:   result.AtomsCreated,
		Errors:  len(result.Errors),
		Elapsed: elapsed,
		ErrMsgs: errMsgs,
	})
}

// handleProgress streams SSE events for an active indexing run.
func (s *Server) handleProgress(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}

	run := s.runs.Get(name)
	if run == nil {
		writeError(w, http.StatusNotFound, "no active index run for project "+name)
		return
	}

	run.WriteSSE(w, r)
}

// handleListRuns returns the status of all active/recent indexing runs.
func (s *Server) handleListRuns(w http.ResponseWriter, r *http.Request) {
	runs := s.runs.ListRuns()
	if runs == nil {
		runs = []RunStatus{}
	}
	writeJSON(w, http.StatusOK, runs)
}
