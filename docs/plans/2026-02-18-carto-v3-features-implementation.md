# Carto v3 Features Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add universal deployment support, Git repo URL indexing, folder picker UI, and external source integrations to Carto.

**Architecture:** Four features built incrementally. Feature 1 centralizes Docker URL resolution into `config/`. Feature 2 adds a `gitclone` package and extends the index API for Git URLs. Feature 3 adds a server-side directory browser API and a React `FolderPicker` component. Feature 4 adds a `knowledge` package and GitHub signal source for external integrations. Each feature is independently deployable.

**Tech Stack:** Go 1.25, React 18, TypeScript, shadcn/ui, tree-sitter (CGO), Memories REST API.

---

## Feature 1: Universal Deployment (Centralize URL Resolution)

### Task 1.1: Add `ResolveURL` and move `isDocker` to config package

**Files:**
- Modify: `go/internal/config/config.go`

**Step 1: Write the test**

Create `go/internal/config/config_test.go`:

```go
package config

import (
	"os"
	"testing"
)

func TestResolveURL_NonDocker(t *testing.T) {
	// Outside Docker, URLs pass through unchanged.
	url := ResolveURL("http://localhost:8900")
	if url != "http://localhost:8900" {
		t.Errorf("expected localhost unchanged, got %s", url)
	}
}

func TestResolveURL_Docker(t *testing.T) {
	// Simulate Docker by creating /.dockerenv — skip if not possible.
	// Instead, test the internal resolve function directly.
	tests := []struct {
		input    string
		inDocker bool
		expected string
	}{
		{"http://localhost:8900", false, "http://localhost:8900"},
		{"http://127.0.0.1:8900", false, "http://127.0.0.1:8900"},
		{"http://localhost:8900", true, "http://host.docker.internal:8900"},
		{"http://127.0.0.1:8900", true, "http://host.docker.internal:8900"},
		{"https://memories.example.com", true, "https://memories.example.com"},
		{"https://memories.example.com", false, "https://memories.example.com"},
	}
	for _, tt := range tests {
		got := resolveURLForDocker(tt.input, tt.inDocker)
		if got != tt.expected {
			t.Errorf("resolveURLForDocker(%q, %v) = %q, want %q", tt.input, tt.inDocker, got, tt.expected)
		}
	}
}

func TestIsDocker(t *testing.T) {
	// We can't reliably create /.dockerenv in tests, so just verify it
	// returns a bool and doesn't panic.
	result := IsDocker()
	_ = result
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/config/ -run TestResolveURL -v`
Expected: FAIL — `resolveURLForDocker` and `IsDocker` not defined.

**Step 3: Implement `ResolveURL`, `resolveURLForDocker`, `IsDocker`**

Add to `go/internal/config/config.go`:

```go
// IsDocker returns true when running inside a Docker container.
func IsDocker() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// ResolveURL rewrites localhost/127.0.0.1 URLs to host.docker.internal
// when running inside Docker. Remote URLs pass through unchanged.
func ResolveURL(rawURL string) string {
	return resolveURLForDocker(rawURL, IsDocker())
}

func resolveURLForDocker(rawURL string, inDocker bool) string {
	if !inDocker {
		return rawURL
	}
	u := strings.Replace(rawURL, "localhost", "host.docker.internal", 1)
	u = strings.Replace(u, "127.0.0.1", "host.docker.internal", 1)
	return u
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/config/ -run TestResolveURL -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/internal/config/config.go go/internal/config/config_test.go && git commit -m "feat: add ResolveURL and IsDocker to config package"
```

---

### Task 1.2: Replace inline Docker rewrites with `config.ResolveURL`

**Files:**
- Modify: `go/cmd/carto/main.go:504-508` — replace inline `os.Stat("/.dockerenv")` block
- Modify: `go/internal/server/handlers.go:257-261` — replace inline rewrite in `handlePatchConfig`
- Modify: `go/internal/server/handlers.go:353-357` — replace inline rewrite in `runIndex`
- Modify: `go/internal/server/routes.go:56-62` — replace inline rewrite in `handleTestMemories`
- Modify: `go/internal/server/routes.go:77-81` — remove `isDocker()` function (moved to config)

**Step 1: Update `main.go:runServe`**

Replace lines 504-508 in `runServe`:
```go
// Before:
memoriesURL := cfg.MemoriesURL
if _, err := os.Stat("/.dockerenv"); err == nil {
    memoriesURL = strings.Replace(memoriesURL, "localhost", "host.docker.internal", 1)
    memoriesURL = strings.Replace(memoriesURL, "127.0.0.1", "host.docker.internal", 1)
}
memoriesClient := storage.NewMemoriesClient(memoriesURL, cfg.MemoriesKey)

// After:
memoriesClient := storage.NewMemoriesClient(config.ResolveURL(cfg.MemoriesURL), cfg.MemoriesKey)
```

Remove now-unused `"os"` import if no other usage remains. Keep `strings` if still used elsewhere.

**Step 2: Update `handlers.go:handlePatchConfig`**

Replace lines 257-262:
```go
// Before:
memoriesURL := s.cfg.MemoriesURL
if isDocker() {
    memoriesURL = strings.Replace(memoriesURL, "localhost", "host.docker.internal", 1)
    memoriesURL = strings.Replace(memoriesURL, "127.0.0.1", "host.docker.internal", 1)
}
s.memoriesClient = storage.NewMemoriesClient(memoriesURL, s.cfg.MemoriesKey)

// After:
s.memoriesClient = storage.NewMemoriesClient(config.ResolveURL(s.cfg.MemoriesURL), s.cfg.MemoriesKey)
```

**Step 3: Update `handlers.go:runIndex`**

Replace lines 353-358:
```go
// Before:
memoriesURL := cfg.MemoriesURL
if isDocker() {
    memoriesURL = strings.Replace(memoriesURL, "localhost", "host.docker.internal", 1)
    memoriesURL = strings.Replace(memoriesURL, "127.0.0.1", "host.docker.internal", 1)
}
memoriesClient := storage.NewMemoriesClient(memoriesURL, cfg.MemoriesKey)

// After:
memoriesClient := storage.NewMemoriesClient(config.ResolveURL(cfg.MemoriesURL), cfg.MemoriesKey)
```

**Step 4: Update `routes.go:handleTestMemories`**

Replace lines 56-62:
```go
// Before:
testURL := req.URL
if isDocker() {
    testURL = strings.Replace(testURL, "localhost", "host.docker.internal", 1)
    testURL = strings.Replace(testURL, "127.0.0.1", "host.docker.internal", 1)
}

// After:
testURL := config.ResolveURL(req.URL)
```

Add `"github.com/divyekant/carto/internal/config"` to routes.go imports.

**Step 5: Remove `isDocker()` from routes.go**

Delete lines 77-81 (the `isDocker` function). It now lives in `config.IsDocker()`.

Update `handleHealth` (line 36) to use `config.IsDocker()` instead of `isDocker()`.

**Step 6: Run all tests**

Run: `cd /Users/dk/projects/indexer/go && go build ./... && go test ./... -short`
Expected: PASS — all code compiles, no test regressions.

**Step 7: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/cmd/carto/main.go go/internal/server/handlers.go go/internal/server/routes.go && git commit -m "refactor: centralize Docker URL rewriting into config.ResolveURL"
```

---

## Feature 2: Git Repo URL Indexing

### Task 2.1: Create `gitclone` package with `Clone` function

**Files:**
- Create: `go/internal/gitclone/clone.go`
- Create: `go/internal/gitclone/clone_test.go`

**Step 1: Write the test**

```go
// go/internal/gitclone/clone_test.go
package gitclone

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://github.com/user/repo", true},
		{"https://github.com/user/repo.git", true},
		{"git@github.com:user/repo.git", true},
		{"/Users/dk/projects/my-project", false},
		{"./relative/path", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsGitURL(tt.input)
		if got != tt.expected {
			t.Errorf("IsGitURL(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestParseRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/user/my-repo", "my-repo"},
		{"https://github.com/user/my-repo.git", "my-repo"},
		{"git@github.com:user/my-repo.git", "my-repo"},
	}
	for _, tt := range tests {
		got := ParseRepoName(tt.input)
		if got != tt.expected {
			t.Errorf("ParseRepoName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestClone_PublicRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping clone test in short mode")
	}

	result, err := Clone(CloneOptions{
		URL:   "https://github.com/octocat/Hello-World",
		Depth: 1,
	})
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	defer result.Cleanup()

	// Verify cloned directory exists and contains files.
	if _, err := os.Stat(filepath.Join(result.Dir, ".git")); err != nil {
		t.Error("expected .git directory in clone")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/gitclone/ -run TestIsGitURL -v`
Expected: FAIL — package does not exist.

**Step 3: Implement the package**

```go
// go/internal/gitclone/clone.go
package gitclone

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneOptions configures a git clone operation.
type CloneOptions struct {
	URL    string // Git repo URL (HTTPS or SSH)
	Branch string // Optional branch, defaults to HEAD
	Token  string // GitHub PAT for private repos
	Depth  int    // Clone depth, default 1 (shallow)
}

// CloneResult holds the result of a successful clone.
type CloneResult struct {
	Dir     string // Temp directory containing the cloned repo
	Cleanup func() // Call to remove the temp directory
}

// IsGitURL returns true if the input looks like a Git URL rather than a local path.
func IsGitURL(input string) bool {
	if input == "" {
		return false
	}
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		return true
	}
	if strings.HasPrefix(input, "git@") {
		return true
	}
	return false
}

// ParseRepoName extracts the repository name from a Git URL.
func ParseRepoName(gitURL string) string {
	// Handle git@host:user/repo.git
	if strings.HasPrefix(gitURL, "git@") {
		parts := strings.SplitN(gitURL, ":", 2)
		if len(parts) == 2 {
			name := filepath.Base(parts[1])
			return strings.TrimSuffix(name, ".git")
		}
	}
	// Handle https://host/user/repo.git
	u, err := url.Parse(gitURL)
	if err != nil {
		return filepath.Base(gitURL)
	}
	name := filepath.Base(u.Path)
	return strings.TrimSuffix(name, ".git")
}

// Clone performs a shallow git clone to a temporary directory.
func Clone(opts CloneOptions) (*CloneResult, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("gitclone: URL is required")
	}
	if opts.Depth == 0 {
		opts.Depth = 1
	}

	tmpDir, err := os.MkdirTemp("", "carto-clone-*")
	if err != nil {
		return nil, fmt.Errorf("gitclone: create temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	// Build clone URL — inject token for HTTPS private repos.
	cloneURL := opts.URL
	if opts.Token != "" && strings.HasPrefix(cloneURL, "https://") {
		u, err := url.Parse(cloneURL)
		if err == nil {
			u.User = url.UserPassword("x-access-token", opts.Token)
			cloneURL = u.String()
		}
	}

	args := []string{"clone", "--depth", fmt.Sprintf("%d", opts.Depth)}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	args = append(args, cloneURL, tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return nil, fmt.Errorf("gitclone: git clone failed: %w", err)
	}

	return &CloneResult{Dir: tmpDir, Cleanup: cleanup}, nil
}
```

**Step 4: Run tests**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/gitclone/ -run "TestIsGitURL|TestParseRepoName" -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/internal/gitclone/ && git commit -m "feat: add gitclone package for shallow repo cloning"
```

---

### Task 2.2: Add `github_token` to config and Settings UI

**Files:**
- Modify: `go/internal/config/config.go` — add `GitHubToken` field
- Modify: `go/internal/server/handlers.go` — add `github_token` to configResponse, handlePatchConfig
- Modify: `go/web/src/pages/Settings.tsx` — add GitHub Token field

**Step 1: Add `GitHubToken` to config**

In `go/internal/config/config.go`:

Add `GitHubToken string` to `Config` struct (after `LLMBaseURL`).
Add `GitHubToken string \`json:"github_token,omitempty"\`` to `persistedConfig`.
Add `GitHubToken: os.Getenv("GITHUB_TOKEN"),` in `Load()`.
Add `GitHubToken: cfg.GitHubToken,` in `Save()`.
Add merge logic in `mergeConfig`:
```go
if p.GitHubToken != "" {
    cfg.GitHubToken = p.GitHubToken
}
```

**Step 2: Add to server config handler**

In `go/internal/server/handlers.go`:

Add `GitHubToken string \`json:"github_token"\`` to `configResponse` struct.
Add `GitHubToken: redactKey(cfg.GitHubToken),` in `handleGetConfig`.
Add case in `handlePatchConfig`:
```go
case "github_token":
    if v, ok := val.(string); ok {
        s.cfg.GitHubToken = v
    }
```

**Step 3: Add GitHub Token field to Settings UI**

In `go/web/src/pages/Settings.tsx`, add to the `Config` interface:
```typescript
github_token: string
```

Add to initial state and the save patch logic. Add a new Card section after the Memories Server card:

```tsx
<Card className="bg-card border-border">
  <CardHeader>
    <CardTitle className="text-base">Integrations</CardTitle>
  </CardHeader>
  <CardContent className="space-y-4">
    <div className="space-y-2">
      <Label htmlFor="github_token">GitHub Token</Label>
      <Input
        id="github_token"
        type="password"
        placeholder="ghp_... (optional, for private repos)"
        value={config.github_token || ''}
        onChange={(e) => updateField('github_token', e.target.value)}
      />
      <p className="text-xs text-muted-foreground">
        Personal access token for cloning private repositories. Leave empty for public repos only.
      </p>
    </div>
  </CardContent>
</Card>
```

**Step 4: Build and verify**

Run: `cd /Users/dk/projects/indexer/go && go build ./... && go test ./... -short`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/internal/config/config.go go/internal/server/handlers.go go/web/src/pages/Settings.tsx && git commit -m "feat: add GitHub token to config and Settings UI"
```

---

### Task 2.3: Extend index API and `runIndex` to support Git URLs

**Files:**
- Modify: `go/internal/server/handlers.go:277-282` — extend `indexRequest` struct
- Modify: `go/internal/server/handlers.go:286-326` — update `handleStartIndex`
- Modify: `go/internal/server/handlers.go:329-396` — update `runIndex`

**Step 1: Extend `indexRequest`**

```go
type indexRequest struct {
	Path        string `json:"path"`
	URL         string `json:"url"`          // Git repo URL (takes precedence over path)
	Branch      string `json:"branch"`       // Optional branch
	Incremental bool   `json:"incremental"`
	Module      string `json:"module"`
	Project     string `json:"project"`
}
```

**Step 2: Update `handleStartIndex`**

After decoding the request, add URL handling before the path validation:

```go
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

// Existing path-based logic continues below...
if req.Path == "" {
    writeError(w, http.StatusBadRequest, "path or url is required")
    return
}
```

**Step 3: Add `runIndexFromURL` method**

```go
// runIndexFromURL clones a Git repo, runs the pipeline, then cleans up.
func (s *Server) runIndexFromURL(run *IndexRun, projectName string, req indexRequest, cfg config.Config) {
	defer s.runs.Finish(projectName)

	run.SendLog("info", fmt.Sprintf("Cloning %s...", req.URL))

	token := cfg.GitHubToken
	result, err := gitclone.Clone(gitclone.CloneOptions{
		URL:    req.URL,
		Branch: req.Branch,
		Token:  token,
		Depth:  1,
	})
	if err != nil {
		run.SendError(err.Error())
		return
	}
	defer result.Cleanup()

	run.SendLog("info", "Clone complete. Starting pipeline...")

	// Reuse the existing runIndex with the cloned path.
	localReq := indexRequest{
		Path:        result.Dir,
		Incremental: req.Incremental,
		Module:      req.Module,
		Project:     projectName,
	}
	s.runIndex(run, projectName, result.Dir, localReq, cfg)
}
```

Add import: `"github.com/divyekant/carto/internal/gitclone"`

**Step 4: Build and test**

Run: `cd /Users/dk/projects/indexer/go && go build ./... && go test ./... -short`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/internal/server/handlers.go && git commit -m "feat: support Git repo URL indexing via POST /api/projects/index"
```

---

### Task 2.4: Add Git URL tab to IndexRun UI

**Files:**
- Modify: `go/web/src/pages/IndexRun.tsx`

**Step 1: Add tab toggle and Git URL inputs**

Add tab state:
```typescript
const [inputMode, setInputMode] = useState<'local' | 'git'>('local')
const [gitUrl, setGitUrl] = useState('')
const [branch, setBranch] = useState('')
```

Replace the idle state card content with a tabbed layout:

```tsx
{state === 'idle' && (
  <Card className="bg-card border-border max-w-lg">
    <CardHeader>
      <CardTitle className="text-base">Start Indexing</CardTitle>
    </CardHeader>
    <CardContent className="space-y-4">
      {/* Tab toggle */}
      <div className="flex gap-1 p-1 bg-muted rounded-lg">
        <button
          className={`flex-1 px-3 py-1.5 text-sm rounded-md transition-colors ${
            inputMode === 'local' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
          }`}
          onClick={() => setInputMode('local')}
        >
          Local Path
        </button>
        <button
          className={`flex-1 px-3 py-1.5 text-sm rounded-md transition-colors ${
            inputMode === 'git' ? 'bg-background text-foreground shadow-sm' : 'text-muted-foreground hover:text-foreground'
          }`}
          onClick={() => setInputMode('git')}
        >
          Git URL
        </button>
      </div>

      {inputMode === 'local' && (
        <div className="space-y-2">
          <Label htmlFor="path">Project Path</Label>
          <Input
            id="path"
            placeholder="/projects/my-project"
            value={path}
            onChange={(e) => setPath(e.target.value)}
          />
        </div>
      )}

      {inputMode === 'git' && (
        <>
          <div className="space-y-2">
            <Label htmlFor="gitUrl">Repository URL</Label>
            <Input
              id="gitUrl"
              placeholder="https://github.com/user/repo"
              value={gitUrl}
              onChange={(e) => setGitUrl(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="branch">Branch (optional)</Label>
            <Input
              id="branch"
              placeholder="main"
              value={branch}
              onChange={(e) => setBranch(e.target.value)}
            />
          </div>
        </>
      )}

      {/* Existing module filter and incremental checkbox remain */}
      <div className="space-y-2">
        <Label htmlFor="module">Module Filter (optional)</Label>
        <Input ... />
      </div>
      <div className="flex items-center gap-2">
        <input type="checkbox" ... />
        <Label htmlFor="incremental">Incremental</Label>
      </div>

      <Button onClick={startIndexing} disabled={inputMode === 'local' ? !path.trim() : !gitUrl.trim()}>
        Start Indexing
      </Button>
    </CardContent>
  </Card>
)}
```

**Step 2: Update `startIndexing` function**

```typescript
async function startIndexing() {
  if (inputMode === 'local' && !path.trim()) return
  if (inputMode === 'git' && !gitUrl.trim()) return
  // ...existing reset code...

  try {
    const body: Record<string, unknown> = { incremental }
    if (inputMode === 'local') {
      body.path = path.trim()
    } else {
      body.url = gitUrl.trim()
      if (branch.trim()) body.branch = branch.trim()
    }
    if (module.trim()) body.module = module.trim()
    // ...rest of existing fetch logic...
  }
}
```

**Step 3: Build frontend**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`
Expected: Build succeeds.

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/web/src/pages/IndexRun.tsx && git commit -m "feat: add Git URL tab to IndexRun page"
```

---

## Feature 3: Folder Picker

### Task 3.1: Add `handleBrowse` API endpoint

**Files:**
- Modify: `go/internal/server/handlers.go` — add `handleBrowse` handler
- Modify: `go/internal/server/routes.go` — register route

**Step 1: Write the handler test**

Create `go/internal/server/handlers_test.go` (or add to existing):

```go
package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleBrowse(t *testing.T) {
	// Create a temp directory structure.
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "project-a"), 0o755)
	os.MkdirAll(filepath.Join(tmp, "project-b"), 0o755)
	os.WriteFile(filepath.Join(tmp, "file.txt"), []byte("hi"), 0o644)

	srv := &Server{projectsDir: tmp}

	req := httptest.NewRequest("GET", "/api/browse?path="+tmp, nil)
	rec := httptest.NewRecorder()
	srv.handleBrowse(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result struct {
		Current     string `json:"current"`
		Parent      string `json:"parent"`
		Directories []struct {
			Name string `json:"name"`
			Path string `json:"path"`
		} `json:"directories"`
	}
	json.NewDecoder(rec.Body).Decode(&result)

	if result.Current != tmp {
		t.Errorf("expected current=%s, got %s", tmp, result.Current)
	}
	if len(result.Directories) != 2 {
		t.Errorf("expected 2 directories, got %d", len(result.Directories))
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/server/ -run TestHandleBrowse -v`
Expected: FAIL — `handleBrowse` not defined.

**Step 3: Implement `handleBrowse`**

Add to `go/internal/server/handlers.go`:

```go
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

	// Default to projects directory or home directory.
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
		// Skip hidden directories.
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
```

**Step 4: Register route**

In `go/internal/server/routes.go`, add after the existing route registrations:
```go
s.mux.HandleFunc("GET /api/browse", s.handleBrowse)
```

**Step 5: Run test**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/server/ -run TestHandleBrowse -v`
Expected: PASS

**Step 6: Run all tests**

Run: `cd /Users/dk/projects/indexer/go && go build ./... && go test ./... -short`
Expected: PASS

**Step 7: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/internal/server/handlers.go go/internal/server/routes.go && git commit -m "feat: add GET /api/browse endpoint for folder picker"
```

---

### Task 3.2: Create `FolderPicker` React component

**Files:**
- Create: `go/web/src/components/FolderPicker.tsx`

**Step 1: Implement the component**

```tsx
// go/web/src/components/FolderPicker.tsx
import { useEffect, useState } from 'react'
import { Button } from '@/components/ui/button'

interface BrowseResult {
  current: string
  parent: string
  directories: { name: string; path: string }[]
}

interface FolderPickerProps {
  value: string
  onChange: (path: string) => void
}

export function FolderPicker({ value, onChange }: FolderPickerProps) {
  const [open, setOpen] = useState(false)
  const [browsePath, setBrowsePath] = useState('')
  const [data, setData] = useState<BrowseResult | null>(null)
  const [loading, setLoading] = useState(false)

  function browse(path: string) {
    setLoading(true)
    const params = path ? `?path=${encodeURIComponent(path)}` : ''
    fetch(`/api/browse${params}`)
      .then(r => r.json())
      .then((result: BrowseResult) => {
        setData(result)
        setBrowsePath(result.current)
      })
      .catch(() => setData(null))
      .finally(() => setLoading(false))
  }

  useEffect(() => {
    if (open && !data) {
      browse(value || '')
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  function select() {
    onChange(browsePath)
    setOpen(false)
  }

  if (!open) {
    return (
      <div className="flex gap-2">
        <div className="flex-1 px-3 py-2 text-sm border border-border rounded-md bg-background truncate">
          {value || <span className="text-muted-foreground">No folder selected</span>}
        </div>
        <Button variant="secondary" size="sm" onClick={() => setOpen(true)}>
          Browse
        </Button>
      </div>
    )
  }

  return (
    <div className="border border-border rounded-md bg-background">
      {/* Breadcrumb */}
      <div className="px-3 py-2 border-b border-border flex items-center gap-1 text-sm overflow-x-auto">
        <span className="text-muted-foreground truncate">{browsePath}</span>
      </div>

      {/* Directory listing */}
      <div className="max-h-48 overflow-y-auto">
        {loading ? (
          <div className="px-3 py-4 text-sm text-muted-foreground">Loading...</div>
        ) : data ? (
          <>
            {data.parent !== data.current && (
              <button
                className="w-full px-3 py-1.5 text-sm text-left hover:bg-muted flex items-center gap-2"
                onClick={() => browse(data.parent)}
              >
                <span className="text-muted-foreground">..</span>
                <span className="text-muted-foreground text-xs">Parent directory</span>
              </button>
            )}
            {data.directories.length === 0 && (
              <div className="px-3 py-4 text-sm text-muted-foreground">No subdirectories</div>
            )}
            {data.directories.map(dir => (
              <button
                key={dir.path}
                className="w-full px-3 py-1.5 text-sm text-left hover:bg-muted flex items-center gap-2"
                onClick={() => browse(dir.path)}
              >
                <span>📁</span>
                <span>{dir.name}</span>
              </button>
            ))}
          </>
        ) : (
          <div className="px-3 py-4 text-sm text-red-400">Failed to load</div>
        )}
      </div>

      {/* Actions */}
      <div className="px-3 py-2 border-t border-border flex gap-2 justify-end">
        <Button variant="ghost" size="sm" onClick={() => setOpen(false)}>Cancel</Button>
        <Button size="sm" onClick={select}>Select This Folder</Button>
      </div>
    </div>
  )
}
```

**Step 2: Build frontend to verify**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/web/src/components/FolderPicker.tsx && git commit -m "feat: add FolderPicker component for directory browsing"
```

---

### Task 3.3: Integrate FolderPicker into IndexRun page

**Files:**
- Modify: `go/web/src/pages/IndexRun.tsx`

**Step 1: Replace the path text input with FolderPicker**

Import the component:
```typescript
import { FolderPicker } from '@/components/FolderPicker'
```

Replace the Local Path input section (within the `inputMode === 'local'` block):
```tsx
{inputMode === 'local' && (
  <div className="space-y-2">
    <Label>Project Path</Label>
    <FolderPicker value={path} onChange={setPath} />
  </div>
)}
```

**Step 2: Build and verify**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`
Expected: Build succeeds.

**Step 3: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/web/src/pages/IndexRun.tsx && git commit -m "feat: integrate FolderPicker into IndexRun local path tab"
```

---

## Feature 4: External Source Integrations

### Task 4.1: Add GitHub Signal Source

**Files:**
- Create: `go/internal/signals/github.go`
- Create: `go/internal/signals/github_test.go`

**Step 1: Write the test**

```go
// go/internal/signals/github_test.go
package signals

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubSignalSource_Name(t *testing.T) {
	src := NewGitHubSignalSource()
	if src.Name() != "github" {
		t.Errorf("expected name 'github', got %q", src.Name())
	}
}

func TestGitHubSignalSource_Configure(t *testing.T) {
	src := NewGitHubSignalSource()
	err := src.Configure(map[string]string{
		"owner": "octocat",
		"repo":  "Hello-World",
		"token": "ghp_test",
	})
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
	if src.owner != "octocat" || src.repo != "Hello-World" {
		t.Error("owner/repo not set correctly")
	}
}

func TestGitHubSignalSource_FetchSignals(t *testing.T) {
	// Mock GitHub API.
	issues := []map[string]any{
		{
			"number":     42,
			"title":      "Fix login bug",
			"body":       "Login fails on mobile",
			"html_url":   "https://github.com/user/repo/issues/42",
			"created_at": "2025-01-01T00:00:00Z",
			"user":       map[string]any{"login": "alice"},
			"pull_request": nil,
		},
	}
	prs := []map[string]any{
		{
			"number":     43,
			"title":      "Add dark mode",
			"body":       "Implements dark theme",
			"html_url":   "https://github.com/user/repo/pull/43",
			"created_at": "2025-01-02T00:00:00Z",
			"user":       map[string]any{"login": "bob"},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/user/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(issues)
	})
	mux.HandleFunc("/repos/user/repo/pulls", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(prs)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := NewGitHubSignalSource()
	src.baseURL = srv.URL
	src.Configure(map[string]string{
		"owner": "user",
		"repo":  "repo",
	})

	signals, err := src.FetchSignals(Module{Name: "root"})
	if err != nil {
		t.Fatalf("FetchSignals failed: %v", err)
	}
	if len(signals) != 2 {
		t.Fatalf("expected 2 signals, got %d", len(signals))
	}

	// Verify issue signal.
	if signals[0].Type != "issue" || signals[0].ID != "#42" {
		t.Errorf("unexpected issue signal: %+v", signals[0])
	}
	// Verify PR signal.
	if signals[1].Type != "pr" || signals[1].ID != "#43" {
		t.Errorf("unexpected PR signal: %+v", signals[1])
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/signals/ -run TestGitHub -v`
Expected: FAIL — `NewGitHubSignalSource` not defined.

**Step 3: Implement GitHub signal source**

```go
// go/internal/signals/github.go
package signals

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GitHubSignalSource fetches issues and PRs from the GitHub API.
type GitHubSignalSource struct {
	owner   string
	repo    string
	token   string
	baseURL string
	http    http.Client
}

// NewGitHubSignalSource creates an unconfigured GitHub signal source.
func NewGitHubSignalSource() *GitHubSignalSource {
	return &GitHubSignalSource{
		baseURL: "https://api.github.com",
		http:    http.Client{Timeout: 15 * time.Second},
	}
}

func (g *GitHubSignalSource) Name() string { return "github" }

func (g *GitHubSignalSource) Configure(cfg map[string]string) error {
	g.owner = cfg["owner"]
	g.repo = cfg["repo"]
	if t, ok := cfg["token"]; ok {
		g.token = t
	}
	if g.owner == "" || g.repo == "" {
		return fmt.Errorf("github: owner and repo are required")
	}
	return nil
}

func (g *GitHubSignalSource) FetchSignals(module Module) ([]Signal, error) {
	var signals []Signal

	// Fetch issues (excludes PRs by default on the issues endpoint,
	// but the API returns PRs too — we filter by checking pull_request field).
	issues, err := g.fetchIssues()
	if err != nil {
		return nil, fmt.Errorf("github: fetch issues: %w", err)
	}
	signals = append(signals, issues...)

	// Fetch PRs.
	prs, err := g.fetchPRs()
	if err != nil {
		return nil, fmt.Errorf("github: fetch PRs: %w", err)
	}
	signals = append(signals, prs...)

	return signals, nil
}

type ghIssue struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	CreatedAt   time.Time     `json:"created_at"`
	User        ghUser        `json:"user"`
	PullRequest *ghPullReqRef `json:"pull_request"`
}

type ghPullReqRef struct {
	URL string `json:"url"`
}

type ghPR struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	User      ghUser    `json:"user"`
}

type ghUser struct {
	Login string `json:"login"`
}

func (g *GitHubSignalSource) apiGet(path string, v any) error {
	req, err := http.NewRequest("GET", g.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (g *GitHubSignalSource) fetchIssues() ([]Signal, error) {
	var ghIssues []ghIssue
	path := fmt.Sprintf("/repos/%s/%s/issues?state=all&per_page=30&sort=updated", g.owner, g.repo)
	if err := g.apiGet(path, &ghIssues); err != nil {
		return nil, err
	}

	var signals []Signal
	for _, issue := range ghIssues {
		// Skip pull requests (GitHub API returns them on the issues endpoint).
		if issue.PullRequest != nil {
			continue
		}
		signals = append(signals, Signal{
			Type:   "issue",
			ID:     fmt.Sprintf("#%d", issue.Number),
			Title:  issue.Title,
			Body:   truncateBody(issue.Body, 500),
			URL:    issue.HTMLURL,
			Date:   issue.CreatedAt,
			Author: issue.User.Login,
		})
	}
	return signals, nil
}

func (g *GitHubSignalSource) fetchPRs() ([]Signal, error) {
	var ghPRs []ghPR
	path := fmt.Sprintf("/repos/%s/%s/pulls?state=all&per_page=30&sort=updated", g.owner, g.repo)
	if err := g.apiGet(path, &ghPRs); err != nil {
		return nil, err
	}

	var signals []Signal
	for _, pr := range ghPRs {
		signals = append(signals, Signal{
			Type:   "pr",
			ID:     fmt.Sprintf("#%d", pr.Number),
			Title:  pr.Title,
			Body:   truncateBody(pr.Body, 500),
			URL:    pr.HTMLURL,
			Date:   pr.CreatedAt,
			Author: pr.User.Login,
		})
	}
	return signals, nil
}

func truncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

**Step 4: Run tests**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/signals/ -run TestGitHub -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/internal/signals/github.go go/internal/signals/github_test.go && git commit -m "feat: add GitHub issues/PRs signal source"
```

---

### Task 4.2: Wire GitHub signal source into pipeline

**Files:**
- Modify: `go/internal/server/handlers.go:348-349` — register GitHub source in `runIndex`
- Modify: `go/internal/gitclone/clone.go` — add `ParseOwnerRepo` helper

**Step 1: Add `ParseOwnerRepo` to gitclone**

```go
// ParseOwnerRepo extracts owner and repo name from a GitHub URL.
// Returns ("", "") if the URL is not a recognized GitHub URL.
func ParseOwnerRepo(gitURL string) (owner, repo string) {
	// Handle git@github.com:user/repo.git
	if strings.HasPrefix(gitURL, "git@github.com:") {
		path := strings.TrimPrefix(gitURL, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "", ""
	}
	// Handle https://github.com/user/repo
	u, err := url.Parse(gitURL)
	if err != nil || u.Host != "github.com" {
		return "", ""
	}
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}
```

Add test for it in `clone_test.go`:
```go
func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		input         string
		expectOwner   string
		expectRepo    string
	}{
		{"https://github.com/octocat/Hello-World", "octocat", "Hello-World"},
		{"https://github.com/octocat/Hello-World.git", "octocat", "Hello-World"},
		{"git@github.com:octocat/Hello-World.git", "octocat", "Hello-World"},
		{"https://gitlab.com/user/repo", "", ""},
	}
	for _, tt := range tests {
		owner, repo := ParseOwnerRepo(tt.input)
		if owner != tt.expectOwner || repo != tt.expectRepo {
			t.Errorf("ParseOwnerRepo(%q) = (%q, %q), want (%q, %q)",
				tt.input, owner, repo, tt.expectOwner, tt.expectRepo)
		}
	}
}
```

**Step 2: Register GitHub source in `runIndex` when URL is a GitHub repo**

In `handlers.go`, update `runIndex` signal registry section (around line 348):

```go
registry := signals.NewRegistry()
registry.Register(signals.NewGitSignalSource(absPath))

// If we know the GitHub owner/repo, also register the GitHub signal source.
if owner, repo := gitclone.ParseOwnerRepo(req.URL); owner != "" {
    ghSrc := signals.NewGitHubSignalSource()
    ghSrc.Configure(map[string]string{
        "owner": owner,
        "repo":  repo,
        "token": cfg.GitHubToken,
    })
    registry.Register(ghSrc)
}
```

Note: `req.URL` will be empty for local-path indexing, so `ParseOwnerRepo` returns `""` and the source is skipped.

**Step 3: Run tests and build**

Run: `cd /Users/dk/projects/indexer/go && go build ./... && go test ./... -short`
Expected: PASS

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/internal/gitclone/clone.go go/internal/gitclone/clone_test.go go/internal/server/handlers.go && git commit -m "feat: wire GitHub signal source into indexing pipeline"
```

---

### Task 4.3: Create Knowledge Source interface and Local PDF source

**Files:**
- Create: `go/internal/knowledge/knowledge.go`
- Create: `go/internal/knowledge/pdf.go`
- Create: `go/internal/knowledge/knowledge_test.go`

**Step 1: Write the interface and registry**

```go
// go/internal/knowledge/knowledge.go
package knowledge

import "log"

// Document represents a standalone knowledge document not tied to a specific module.
type Document struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	URL     string `json:"url"`
	Type    string `json:"type"` // "pdf", "gdoc", etc.
}

// KnowledgeSource is the plugin interface for project-level documents.
type KnowledgeSource interface {
	Name() string
	Configure(cfg map[string]string) error
	FetchDocuments(project string) ([]Document, error)
}

// Registry holds all configured knowledge sources.
type Registry struct {
	sources []KnowledgeSource
}

// NewRegistry creates an empty knowledge source registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a knowledge source.
func (r *Registry) Register(s KnowledgeSource) {
	r.sources = append(r.sources, s)
}

// FetchAll collects documents from every registered source.
func (r *Registry) FetchAll(project string) ([]Document, error) {
	var all []Document
	for _, s := range r.sources {
		docs, err := s.FetchDocuments(project)
		if err != nil {
			log.Printf("knowledge: warning: source %s failed: %v", s.Name(), err)
			continue
		}
		all = append(all, docs...)
	}
	return all, nil
}
```

**Step 2: Implement Local PDF source**

This uses a pure-Go PDF text extractor. Add dependency first:

Run: `cd /Users/dk/projects/indexer/go && go get github.com/ledongthuc/pdf`

```go
// go/internal/knowledge/pdf.go
package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
)

// LocalPDFSource reads PDF files from a configured directory.
type LocalPDFSource struct {
	dir string
}

// NewLocalPDFSource creates a PDF knowledge source.
func NewLocalPDFSource() *LocalPDFSource {
	return &LocalPDFSource{}
}

func (p *LocalPDFSource) Name() string { return "local-pdf" }

func (p *LocalPDFSource) Configure(cfg map[string]string) error {
	dir, ok := cfg["dir"]
	if !ok || dir == "" {
		return fmt.Errorf("local-pdf: 'dir' is required")
	}
	p.dir = dir
	return nil
}

func (p *LocalPDFSource) FetchDocuments(project string) ([]Document, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, fmt.Errorf("local-pdf: read dir: %w", err)
	}

	var docs []Document
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
			continue
		}

		absPath := filepath.Join(p.dir, entry.Name())
		text, err := extractPDFText(absPath)
		if err != nil {
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}

		docs = append(docs, Document{
			Title:   strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Content: text,
			URL:     "file://" + absPath,
			Type:    "pdf",
		})
	}
	return docs, nil
}

func extractPDFText(path string) (string, error) {
	f, reader, err := pdflib.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}
```

**Step 3: Write test**

```go
// go/internal/knowledge/knowledge_test.go
package knowledge

import (
	"testing"
)

func TestRegistry_Empty(t *testing.T) {
	r := NewRegistry()
	docs, err := r.FetchAll("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}

type mockSource struct {
	docs []Document
}

func (m *mockSource) Name() string                                   { return "mock" }
func (m *mockSource) Configure(cfg map[string]string) error          { return nil }
func (m *mockSource) FetchDocuments(project string) ([]Document, error) { return m.docs, nil }

func TestRegistry_FetchAll(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockSource{docs: []Document{
		{Title: "Doc A", Content: "content a", Type: "mock"},
	}})
	r.Register(&mockSource{docs: []Document{
		{Title: "Doc B", Content: "content b", Type: "mock"},
	}})

	docs, err := r.FetchAll("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestLocalPDFSource_Configure(t *testing.T) {
	src := NewLocalPDFSource()
	err := src.Configure(map[string]string{})
	if err == nil {
		t.Error("expected error when dir not set")
	}

	err = src.Configure(map[string]string{"dir": "/tmp"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
```

**Step 4: Run tests**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/knowledge/ -v`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/internal/knowledge/ && git commit -m "feat: add knowledge source interface with local PDF source"
```

---

### Task 4.4: Wire knowledge sources into pipeline storage

**Files:**
- Modify: `go/internal/pipeline/pipeline.go` — add knowledge phase after signals
- Modify: `go/internal/pipeline/pipeline.go:32-44` — add `KnowledgeRegistry` to `Config`

**Step 1: Add `KnowledgeRegistry` to pipeline Config**

```go
import "github.com/divyekant/carto/internal/knowledge"

type Config struct {
	// ...existing fields...
	KnowledgeRegistry *knowledge.Registry // optional: project-level knowledge sources
}
```

**Step 2: Add knowledge fetch + store phase**

After Phase 3 (History + Signals, around line 296) and before Phase 4 (Deep Analysis), add:

```go
// ── Phase 3b: Knowledge Documents ────────────────────────────────
if cfg.KnowledgeRegistry != nil {
    logFn("info", "Fetching knowledge documents...")
    knowledgeDocs, kErr := cfg.KnowledgeRegistry.FetchAll(cfg.ProjectName)
    if kErr != nil {
        result.Errors = append(result.Errors, kErr)
    }
    if len(knowledgeDocs) > 0 {
        logFn("info", fmt.Sprintf("Found %d knowledge document(s)", len(knowledgeDocs)))
        store := storage.NewStore(cfg.MemoriesClient, cfg.ProjectName)
        for _, doc := range knowledgeDocs {
            source := fmt.Sprintf("carto/%s/_knowledge/%s/%s", cfg.ProjectName, "source:"+doc.Type, doc.Title)
            content := fmt.Sprintf("# %s\n\nSource: %s\nType: %s\n\n%s", doc.Title, doc.URL, doc.Type, doc.Content)
            if err := store.StoreRaw(source, content); err != nil {
                log.Printf("pipeline: warning: failed to store knowledge doc %s: %v", doc.Title, err)
                result.Errors = append(result.Errors, err)
            }
        }
    }
}
```

Note: `StoreRaw` may need to be added to the `Store` type if it doesn't exist. Check `storage/store.go` first. If only `StoreLayer` exists, use that with module `_knowledge` and layer name as the doc type+title.

**Step 3: Build and test**

Run: `cd /Users/dk/projects/indexer/go && go build ./... && go test ./... -short`
Expected: PASS

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer && git add go/internal/pipeline/pipeline.go && git commit -m "feat: integrate knowledge sources into indexing pipeline"
```

---

## Build, Deploy, and Verify

### Task 5.1: Full build and test

**Step 1: Run all Go tests**

Run: `cd /Users/dk/projects/indexer/go && go test -race ./... -short`
Expected: PASS

**Step 2: Build frontend**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`
Expected: Build succeeds.

**Step 3: Build Go binary**

Run: `cd /Users/dk/projects/indexer/go && go build -o carto ./cmd/carto`
Expected: Binary compiles.

**Step 4: Docker build**

Run: `cd /Users/dk/projects/indexer/go && docker compose build`
Expected: Image builds.

**Step 5: Deploy**

Run: `cd /Users/dk/projects/indexer/go && docker compose up -d`
Expected: Container starts on port 8950.

**Step 6: Commit**

```bash
cd /Users/dk/projects/indexer && git add -A && git commit -m "chore: full build verification for v3 features"
```

---

## Summary of Implementation Order

| Task | Feature | Description | Estimated Complexity |
|------|---------|-------------|---------------------|
| 1.1 | Universal Deploy | Add `ResolveURL` + `IsDocker` to config | Low |
| 1.2 | Universal Deploy | Replace all inline Docker rewrites | Low |
| 2.1 | Git URL Indexing | Create `gitclone` package | Medium |
| 2.2 | Git URL Indexing | Add `github_token` to config + Settings | Low |
| 2.3 | Git URL Indexing | Extend index API for URLs | Medium |
| 2.4 | Git URL Indexing | Add Git URL tab to IndexRun UI | Low |
| 3.1 | Folder Picker | Add `handleBrowse` API endpoint | Low |
| 3.2 | Folder Picker | Create `FolderPicker` component | Medium |
| 3.3 | Folder Picker | Integrate into IndexRun page | Low |
| 4.1 | Integrations | GitHub Issues/PRs signal source | Medium |
| 4.2 | Integrations | Wire GitHub source into pipeline | Low |
| 4.3 | Integrations | Knowledge interface + PDF source | Medium |
| 4.4 | Integrations | Wire knowledge into pipeline | Low |
| 5.1 | Verification | Full build + deploy + test | Low |

**Dependencies:** 1.1 → 1.2 (sequential). Features 2, 3, 4 are independent of each other but all depend on Feature 1. Within each feature, tasks are sequential.
