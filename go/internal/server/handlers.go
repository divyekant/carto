package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/gitclone"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/manifest"
	"github.com/divyekant/carto/internal/pipeline"
	"github.com/divyekant/carto/internal/sources"
	"github.com/divyekant/carto/internal/storage"
)

// serverStartTime records when the server process started, used for the
// uptime_seconds metric in the /api/metrics response.
var serverStartTime = time.Now()

// requestCounter tracks the total number of HTTP requests served since
// startup, incremented by the logging middleware.
var requestCounter atomic.Int64

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

	// Search with optional project scoping via source prefix.
	sourcePrefix := ""
	opts := storage.SearchOptions{
		K:      req.K,
		Hybrid: true,
	}
	if req.Project != "" {
		sourcePrefix = fmt.Sprintf("carto/%s/", req.Project)
		opts.SourcePrefix = sourcePrefix
		// Request extra results so we have enough after filtering.
		opts.K = req.K * 3
	}

	results, err := s.memoriesClient.SearchAdvanced(req.Text, opts)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	var items []queryResultItem
	for _, sr := range results {
		if sourcePrefix != "" && !strings.HasPrefix(sr.Source, sourcePrefix) {
			continue
		}
		items = append(items, queryResultItem{
			Text:   sr.Text,
			Source: sr.Source,
			Score:  sr.Score,
		})
		if len(items) >= req.K {
			break
		}
	}

	// Fallback: if search returned no project-matching results, use ListBySource
	// to retrieve all memories for the project. This works around search APIs
	// that don't support source-prefix filtering.
	if len(items) == 0 && sourcePrefix != "" {
		listed, listErr := s.memoriesClient.ListBySource(sourcePrefix, req.K*5, 0)
		if listErr == nil {
			for _, sr := range listed {
				items = append(items, queryResultItem{
					Text:   sr.Text,
					Source: sr.Source,
					Score:  sr.Score,
				})
				if len(items) >= req.K {
					break
				}
			}
		}
	}

	if items == nil {
		items = []queryResultItem{}
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
	FastMaxTokens int    `json:"fast_max_tokens"`
	DeepMaxTokens int    `json:"deep_max_tokens"`
	LLMProvider   string `json:"llm_provider"`
	LLMApiKey     string `json:"llm_api_key"`
	LLMBaseURL    string `json:"llm_base_url"`
	GitHubToken   string `json:"github_token"`
	JiraToken     string `json:"jira_token"`
	JiraEmail     string `json:"jira_email"`
	JiraBaseURL   string `json:"jira_base_url"`
	LinearToken   string `json:"linear_token"`
	NotionToken   string `json:"notion_token"`
	SlackToken    string `json:"slack_token"`
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
		FastMaxTokens: cfg.FastMaxTokens,
		DeepMaxTokens: cfg.DeepMaxTokens,
		LLMProvider:   cfg.LLMProvider,
		LLMApiKey:     redactKey(cfg.LLMApiKey),
		LLMBaseURL:    cfg.LLMBaseURL,
		GitHubToken:   redactKey(cfg.GitHubToken),
		JiraToken:     redactKey(cfg.JiraToken),
		JiraEmail:     cfg.JiraEmail,
		JiraBaseURL:   cfg.JiraBaseURL,
		LinearToken:   redactKey(cfg.LinearToken),
		NotionToken:   redactKey(cfg.NotionToken),
		SlackToken:    redactKey(cfg.SlackToken),
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
			if v, ok := val.(float64); ok && v >= 1 {
				s.cfg.MaxConcurrent = int(v)
			}
		case "fast_max_tokens":
			if v, ok := val.(float64); ok && v >= 1 {
				s.cfg.FastMaxTokens = int(v)
			}
		case "deep_max_tokens":
			if v, ok := val.(float64); ok && v >= 1 {
				s.cfg.DeepMaxTokens = int(v)
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
		case "github_token":
			if v, ok := val.(string); ok {
				s.cfg.GitHubToken = v
			}
		case "jira_token":
			if v, ok := val.(string); ok {
				s.cfg.JiraToken = v
			}
		case "jira_email":
			if v, ok := val.(string); ok {
				s.cfg.JiraEmail = v
			}
		case "jira_base_url":
			if v, ok := val.(string); ok {
				s.cfg.JiraBaseURL = v
			}
		case "linear_token":
			if v, ok := val.(string); ok {
				s.cfg.LinearToken = v
			}
		case "notion_token":
			if v, ok := val.(string); ok {
				s.cfg.NotionToken = v
			}
		case "slack_token":
			if v, ok := val.(string); ok {
				s.cfg.SlackToken = v
			}
		}
	}
	// Rebuild the Memories client so queries use the updated credentials.
	s.memoriesClient = storage.NewMemoriesClient(config.ResolveURL(s.cfg.MemoriesURL), s.cfg.MemoriesKey)

	// Persist config so settings survive container restarts.
	cfgSnapshot := s.cfg
	s.cfgMu.Unlock()

	if err := config.Save(cfgSnapshot); err != nil {
		writeError(w, http.StatusInternalServerError, "settings updated but failed to persist: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// indexRequest is the JSON body for POST /api/projects/index.
type indexRequest struct {
	Path        string `json:"path"`
	URL         string `json:"url"`    // Git repo URL (takes precedence over path)
	Branch      string `json:"branch"` // Optional branch
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

	// If a Git URL is provided, it takes precedence over path.
	if req.URL != "" {
		if !gitclone.IsGitURL(req.URL) {
			writeError(w, http.StatusBadRequest, "invalid git URL")
			return
		}
		projectName := req.Project
		if projectName == "" {
			projectName = gitclone.ParseRepoName(req.URL)
		}

		run := s.runs.Start(projectName)
		if run == nil {
			writeError(w, http.StatusConflict, "index already running for project "+projectName)
			return
		}

		s.cfgMu.RLock()
		cfg := s.cfg
		s.cfgMu.RUnlock()

		go s.runIndexFromURL(run, projectName, req, cfg)

		writeJSON(w, http.StatusAccepted, map[string]string{
			"project": projectName,
			"status":  "started",
		})
		return
	}

	// Existing path-based logic continues below.
	if req.Path == "" {
		writeError(w, http.StatusBadRequest, "path or url is required")
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

	// Build unified source registry from .carto/sources.yaml (if present)
	// and auto-detected sources (git, GitHub, PDFs).
	yamlCfg, _ := sources.LoadSourcesConfig(absPath)
	owner, repo := gitclone.ParseOwnerRepo(req.URL)
	srcRegistry := sources.BuildRegistry(absPath, yamlCfg, sources.Credentials{
		GitHubToken: cfg.GitHubToken,
		GitHubOwner: owner,
		GitHubRepo:  repo,
		JiraToken:   cfg.JiraToken,
		JiraEmail:   cfg.JiraEmail,
		JiraBaseURL: cfg.JiraBaseURL,
		LinearToken: cfg.LinearToken,
		NotionToken: cfg.NotionToken,
		SlackToken:  cfg.SlackToken,
	})

	// Create a fresh Memories client from the current config so Settings
	// changes take effect without server restart.
	memoriesClient := storage.NewMemoriesClient(config.ResolveURL(cfg.MemoriesURL), cfg.MemoriesKey)

	result, err := pipeline.Run(pipeline.Config{
		Ctx:               run.Ctx,
		ProjectName:       projectName,
		RootPath:          absPath,
		LLMClient:         llmClient,
		MemoriesClient:    memoriesClient,
		SourceRegistry:    srcRegistry,
		MaxWorkers:        cfg.MaxConcurrent,
		ProgressFn: func(phase string, done, total int) {
			run.SendProgress(phase, done, total)
		},
		LogFn: func(level, msg string) {
			run.SendLog(level, msg)
		},
		Incremental:   req.Incremental,
		ModuleFilter:  req.Module,
		FastMaxTokens: cfg.FastMaxTokens,
		DeepMaxTokens: cfg.DeepMaxTokens,
	})
	if err != nil {
		if err == context.Canceled {
			run.SendStopped()
			return
		}
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

// runIndexFromURL clones a Git repo, runs the pipeline, then cleans up.
func (s *Server) runIndexFromURL(run *IndexRun, projectName string, req indexRequest, cfg config.Config) {
	run.SendLog("info", fmt.Sprintf("Cloning %s...", req.URL))

	token := cfg.GitHubToken
	cloneResult, err := gitclone.Clone(gitclone.CloneOptions{
		URL:    req.URL,
		Branch: req.Branch,
		Token:  token,
		Depth:  1,
	})
	if err != nil {
		run.SendError(err.Error())
		s.runs.Finish(projectName)
		return
	}
	defer cloneResult.Cleanup()

	run.SendLog("info", "Clone complete. Starting pipeline...")

	localReq := indexRequest{
		Path:        cloneResult.Dir,
		Incremental: req.Incremental,
		Module:      req.Module,
		Project:     projectName,
		URL:         req.URL,
	}
	// runIndex handles Finish internally via defer.
	s.runIndex(run, projectName, cloneResult.Dir, localReq, cfg)
}

// handleStopIndex cancels an active indexing run.
func (s *Server) handleStopIndex(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		writeError(w, http.StatusBadRequest, "project name is required")
		return
	}

	if !s.runs.Stop(name) {
		writeError(w, http.StatusNotFound, "no active index run for project "+name)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"project": name,
		"status":  "stopping",
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

// browseResponse is the JSON shape for GET /api/browse.
type browseResponse struct {
	Current     string       `json:"current"`
	Parent      string       `json:"parent"`
	Directories []browseItem `json:"directories"`
}

type browseItem struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// handleBrowse returns subdirectories at a given path for the folder picker.
//
// Security: when projectsDir is configured, browsing is restricted to
// projectsDir and the user's home directory. This prevents arbitrary filesystem
// traversal in production deployments where CARTO_SERVER_TOKEN is set.
// When projectsDir is empty (dev mode), any accessible path is allowed.
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	requestedPath := r.URL.Query().Get("path")

	// Determine the default root to browse from.
	browseRoot := s.projectsDir
	if browseRoot == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			writeError(w, http.StatusInternalServerError, "cannot determine home directory")
			return
		}
		browseRoot = home
	}

	if requestedPath == "" {
		requestedPath = browseRoot
	}

	absPath, err := filepath.Abs(requestedPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// ── Path restriction (security hardening) ──────────────────────────────
	// When projectsDir is configured, restrict browsing to projectsDir and the
	// home directory so the server cannot be used to traverse arbitrary paths.
	if s.projectsDir != "" {
		absProjects, projErr := filepath.Abs(s.projectsDir)
		home, homeErr := os.UserHomeDir()
		absHome := ""
		if homeErr == nil {
			absHome, _ = filepath.Abs(home)
		}

		allowed := false
		if projErr == nil && (absPath == absProjects || strings.HasPrefix(absPath, absProjects+string(filepath.Separator))) {
			allowed = true
		}
		if absHome != "" && (absPath == absHome || strings.HasPrefix(absPath, absHome+string(filepath.Separator))) {
			allowed = true
		}

		if !allowed {
			writeError(w, http.StatusForbidden, "path is outside the allowed browse scope")
			return
		}
	}

	// Verify the path is readable.
	if _, err := os.Stat(absPath); err != nil {
		writeError(w, http.StatusBadRequest, "path not accessible: "+err.Error())
		return
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "cannot read directory: "+err.Error())
		return
	}

	var dirs []browseItem
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dirs = append(dirs, browseItem{
			Name: entry.Name(),
			Path: filepath.Join(absPath, entry.Name()),
		})
	}
	if dirs == nil {
		dirs = []browseItem{}
	}

	writeJSON(w, http.StatusOK, browseResponse{
		Current:     absPath,
		Parent:      filepath.Dir(absPath),
		Directories: dirs,
	})
}

// putSourcesRequest is the JSON body for PUT /api/projects/{name}/sources.
type putSourcesRequest struct {
	Sources map[string]map[string]string `json:"sources"`
}

// handlePutSources writes .carto/sources.yaml for a project.
// An empty sources map deletes the file.
func (s *Server) handlePutSources(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	projPath := filepath.Join(s.projectsDir, name)

	if info, err := os.Stat(projPath); err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	var body putSourcesRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	cartoDir := filepath.Join(projPath, ".carto")
	yamlPath := filepath.Join(cartoDir, "sources.yaml")

	// Empty sources → delete the file.
	if len(body.Sources) == 0 {
		os.Remove(yamlPath)
		writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
		return
	}

	// Build YAML content.
	var buf bytes.Buffer
	buf.WriteString("sources:\n")
	// Sort source names for deterministic output.
	srcNames := make([]string, 0, len(body.Sources))
	for k := range body.Sources {
		srcNames = append(srcNames, k)
	}
	sort.Strings(srcNames)
	for _, srcName := range srcNames {
		settings := body.Sources[srcName]
		buf.WriteString("  " + srcName + ":\n")
		// Sort setting keys too.
		keys := make([]string, 0, len(settings))
		for k := range settings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			buf.WriteString("    " + k + ": " + settings[k] + "\n")
		}
	}

	os.MkdirAll(cartoDir, 0o755)
	if err := os.WriteFile(yamlPath, buf.Bytes(), 0o644); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to write sources config: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// projectDetailResponse is the JSON shape returned by GET /api/projects/{name}.
type projectDetailResponse struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	FileCount int      `json:"file_count"`
	IndexedAt string   `json:"indexed_at"`
	Sources   []string `json:"sources"`
}

// handleGetProject returns detailed info for a single project by reading its
// manifest and sources config.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	projPath := filepath.Join(s.projectsDir, name)

	if info, err := os.Stat(projPath); err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	mf, err := manifest.Load(projPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load manifest: "+err.Error())
		return
	}
	if mf.IsEmpty() && mf.Project == "" {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Collect source names from sources.yaml.
	var sourceNames []string
	yamlCfg, _ := sources.LoadSourcesConfig(projPath)
	if yamlCfg != nil {
		for srcName := range yamlCfg.Sources {
			sourceNames = append(sourceNames, srcName)
		}
		sort.Strings(sourceNames)
	}
	if sourceNames == nil {
		sourceNames = []string{}
	}

	indexedAt := ""
	if !mf.IndexedAt.IsZero() {
		indexedAt = mf.IndexedAt.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, projectDetailResponse{
		Name:      mf.Project,
		Path:      projPath,
		FileCount: len(mf.Files),
		IndexedAt: indexedAt,
		Sources:   sourceNames,
	})
}

// handleDeleteProject removes the .carto/ directory for a project.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	cartoDir := filepath.Join(s.projectsDir, name, ".carto")

	if _, err := os.Stat(cartoDir); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	if err := os.RemoveAll(cartoDir); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete project: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleIndexAll accepts a POST to re-index all projects. It starts indexing
// each project up to maxIndexAllConcurrency at a time and returns 202 immediately.
func (s *Server) handleIndexAll(w http.ResponseWriter, r *http.Request) {
	changedOnly := r.URL.Query().Get("changed") == "true"

	if s.projectsDir == "" {
		writeError(w, http.StatusBadRequest, "projects directory not configured")
		return
	}

	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read projects directory")
		return
	}

	// Collect indexable projects.
	var projectPaths []struct{ name, path string }
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectRoot := filepath.Join(s.projectsDir, entry.Name())
		mf, err := manifest.Load(projectRoot)
		if err != nil || (mf.IsEmpty() && mf.Project == "") {
			continue
		}
		name := mf.Project
		if name == "" {
			name = entry.Name()
		}
		projectPaths = append(projectPaths, struct{ name, path string }{name, projectRoot})
	}

	if len(projectPaths) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"status": "no_projects", "started": 0})
		return
	}

	// Launch indexing with a concurrency limiter (max 3 concurrent).
	const maxConcurrency = 3
	sem := make(chan struct{}, maxConcurrency)

	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()

	started := 0
	for _, p := range projectPaths {
		run := s.runs.Start(p.name)
		if run == nil {
			continue // already running
		}
		started++
		go func(run *IndexRun, name, path string) {
			sem <- struct{}{} // acquire
			defer func() { <-sem }() // release
			req := indexRequest{Path: path, Project: name}
			s.runIndex(run, name, path, req, cfg)
		}(run, p.name, p.path)
	}

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":       "started",
		"started":      started,
		"total":        len(projectPaths),
		"changed_only": changedOnly,
	})
}

// sourcesResponse is the JSON shape returned by GET /api/projects/{name}/sources.
type sourcesResponse struct {
	Sources     map[string]map[string]string `json:"sources"`
	Credentials map[string]bool             `json:"credentials"`
}

// handleGetSources returns the parsed .carto/sources.yaml for a project
// plus boolean availability of global credentials.
func (s *Server) handleGetSources(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	projPath := filepath.Join(s.projectsDir, name)

	if info, err := os.Stat(projPath); err != nil || !info.IsDir() {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// Parse .carto/sources.yaml (nil if not present).
	yamlCfg, err := sources.LoadSourcesConfig(projPath)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read sources config: "+err.Error())
		return
	}

	// Build sources map from YAML.
	srcMap := make(map[string]map[string]string)
	if yamlCfg != nil {
		for srcName, entry := range yamlCfg.Sources {
			srcMap[srcName] = entry.Settings
		}
	}

	// Build credential availability from current config.
	s.cfgMu.RLock()
	cfg := s.cfg
	s.cfgMu.RUnlock()

	creds := map[string]bool{
		"github_token": cfg.GitHubToken != "",
		"jira_token":   cfg.JiraToken != "",
		"jira_email":   cfg.JiraEmail != "",
		"linear_token": cfg.LinearToken != "",
		"notion_token": cfg.NotionToken != "",
		"slack_token":  cfg.SlackToken != "",
	}

	writeJSON(w, http.StatusOK, sourcesResponse{
		Sources:     srcMap,
		Credentials: creds,
	})
}

// metricsResponse is the JSON shape returned by GET /api/metrics.
// Fields align with common B2B SaaS observability schemas (Datadog, Prometheus).
type metricsResponse struct {
	Version        string  `json:"version"`
	UptimeSeconds  float64 `json:"uptime_seconds"`
	GoRoutines     int     `json:"go_routines"`
	MemAllocMB     float64 `json:"mem_alloc_mb"`
	MemSysMB       float64 `json:"mem_sys_mb"`
	GCCycles       uint32  `json:"gc_cycles"`
	ActiveRuns     int     `json:"active_index_runs"`
	TotalRequests  int64   `json:"total_requests"`
	ProjectsDir    string  `json:"projects_dir,omitempty"`
	AuthEnabled    bool    `json:"auth_enabled"`
}

// aboutResponse is the JSON shape returned by GET /api/about.
// It exposes product identity information without leaking any config secrets.
type aboutResponse struct {
	Name        string   `json:"name"`
	Version     string   `json:"version"`
	Tagline     string   `json:"tagline"`
	Description string   `json:"description"`
	ForWhom     string   `json:"for_whom"`
	HowItWorks  string   `json:"how_it_works"`
	Features    []string `json:"features"`
	ProjectURL  string   `json:"project_url"`
	Colors      []struct {
		Name string `json:"name"`
		Hex  string `json:"hex"`
		Role string `json:"role"`
	} `json:"brand_colors"`
}

// handleAbout returns a static product identity card with the brand tagline,
// description, audience, workflow, features, and color palette.
// This endpoint is intentionally unauthenticated so dashboards and landing
// pages can embed it without requiring the server token.
func (s *Server) handleAbout(w http.ResponseWriter, _ *http.Request) {
	type colorEntry struct {
		Name string `json:"name"`
		Hex  string `json:"hex"`
		Role string `json:"role"`
	}

	resp := aboutResponse{
		Name:    "Carto",
		Version: config.Version,
		Tagline: "Map your codebase. Navigate with intent.",
		Description: "Carto indexes your source code, documentation, issues, and knowledge " +
			"bases into a semantic vector store, making every file, pattern, and " +
			"architectural decision retrievable by meaning — not just keyword.",
		ForWhom: "Engineering teams that want AI assistants to understand their whole " +
			"project. Platform engineers building internal developer portals. CTOs " +
			"who need codebase-wide insights, automated documentation, and " +
			"dependency graphs on demand.",
		HowItWorks: "1. Index: Scan your codebase, extract modules, analyse patterns with LLMs, " +
			"and store semantic embeddings in a layered Memories vector store. " +
			"2. Query: Ask natural-language questions. Carto retrieves the right code, " +
			"docs, and context across your entire project history. " +
			"3. Generate: Produce CLAUDE.md and .cursorrules files so AI assistants " +
			"receive a detailed map of your project's intent and architecture. " +
			"4. Integrate: Connect GitHub Issues, Jira, Notion, Slack, and PDFs into " +
			"one unified knowledge graph.",
		Features: []string{
			"Semantic code search across your entire repository",
			"LLM-powered module intent extraction (Anthropic, OpenAI, Ollama)",
			"Layered storage: atoms → modules → blueprints → patterns",
			"CLAUDE.md and .cursorrules generator for AI assistant context",
			"GitHub, Jira, Linear, Notion, Slack, PDF source connectors",
			"Incremental re-indexing — only changed files are re-processed",
			"Docker-native deployment with bearer-auth and audit logging",
		},
		ProjectURL: "https://github.com/divyekant/carto",
	}

	type paletteEntry struct {
		Name string `json:"name"`
		Hex  string `json:"hex"`
		Role string `json:"role"`
	}
	palette := []paletteEntry{
		{"Gold (dark)", "#d4af37", "Primary actions, logo mark, active states (dark theme)"},
		{"Darkened Gold (light)", "#b8960f", "Primary actions, logo mark, active states (light theme)"},
		{"Success Green", "#16A34A", "Success indicators and healthy status"},
		{"Warning Amber", "#CA8A04", "Warnings and advisory messages"},
		{"Error Red", "#DC2626", "Errors and destructive actions"},
		{"Info Blue", "#2563EB", "Info callouts and informational states"},
	}

	// Marshal palette separately so we can embed it as a raw JSON array.
	writeJSON(w, http.StatusOK, map[string]any{
		"name":         resp.Name,
		"version":      resp.Version,
		"tagline":      resp.Tagline,
		"description":  resp.Description,
		"for_whom":     resp.ForWhom,
		"how_it_works": resp.HowItWorks,
		"features":     resp.Features,
		"project_url":  resp.ProjectURL,
		"brand_colors": palette,
	})
}

// handleMetrics returns lightweight runtime and operational metrics.
// This endpoint bypasses bearer auth (it is placed before the auth middleware
// path check) — it intentionally omits sensitive data so it can be consumed
// by internal monitoring probes without requiring the server token.
//
// For Prometheus-style scraping, mount a separate prom exporter; this
// endpoint serves JSON-based dashboards and ops-team health views.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	s.cfgMu.RLock()
	authEnabled := s.cfg.ServerToken != ""
	s.cfgMu.RUnlock()

	resp := metricsResponse{
		Version:       config.Version,
		UptimeSeconds: time.Since(serverStartTime).Seconds(),
		GoRoutines:    runtime.NumGoroutine(),
		MemAllocMB:    float64(memStats.Alloc) / (1024 * 1024),
		MemSysMB:      float64(memStats.Sys) / (1024 * 1024),
		GCCycles:      memStats.NumGC,
		ActiveRuns:    len(s.runs.ListRuns()),
		TotalRequests: requestCounter.Load(),
		ProjectsDir:   s.projectsDir,
		AuthEnabled:   authEnabled,
	}

	writeJSON(w, http.StatusOK, resp)
}
