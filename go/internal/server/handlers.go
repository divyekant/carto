package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/gitclone"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/manifest"
	"github.com/divyekant/carto/internal/pipeline"
	"github.com/divyekant/carto/internal/sources"
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

	// Search with optional project scoping via source prefix.
	sourcePrefix := ""
	opts := storage.SearchOptions{
		K:      req.K,
		Hybrid: true,
	}
	if req.Project != "" {
		sourcePrefix = fmt.Sprintf("carto/%s/", req.Project)
		opts.Source = sourcePrefix
		// Request extra results so we have enough after filtering.
		opts.K = req.K * 3
	}

	results, err := s.memoriesClient.Search(req.Text, opts)
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
func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	requestedPath := r.URL.Query().Get("path")

	if requestedPath == "" {
		if s.projectsDir != "" {
			requestedPath = s.projectsDir
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				writeError(w, http.StatusInternalServerError, "cannot determine home directory")
				return
			}
			requestedPath = home
		}
	}

	absPath, err := filepath.Abs(requestedPath)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	// Security: in Docker, restrict to projects directory.
	if config.IsDocker() && s.projectsDir != "" {
		if !strings.HasPrefix(absPath, s.projectsDir) {
			writeError(w, http.StatusForbidden, "path outside allowed directory")
			return
		}
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

// handleIndexAll accepts a POST to re-index all projects. For now it returns
// 202 Accepted with the changed_only flag parsed from the query string.
func (s *Server) handleIndexAll(w http.ResponseWriter, r *http.Request) {
	changedOnly := r.URL.Query().Get("changed") == "true"

	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":       "started",
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
