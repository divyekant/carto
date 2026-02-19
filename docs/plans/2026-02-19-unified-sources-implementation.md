# Unified Sources Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Replace separate `SignalSource` and `KnowledgeSource` interfaces with a single unified `Source` interface supporting 8 source types (git, github, jira, linear, local-pdf, notion, slack, web) with auto-detection, yaml config, and proper tiered retrieval.

**Architecture:** New `internal/sources/` package with `Source` interface → `Artifact` struct with `Category` (Signal/Knowledge/Context). `Registry` handles auto-detect + yaml config. Pipeline Phase 3 splits into 3a (module-scoped: git) and 3b (project-scoped: all others, concurrent). Storage routes artifacts by category into separate key namespaces. Old `signals/` and `knowledge/` packages deleted after migration.

**Tech Stack:** Go 1.25, net/http for REST APIs, `github.com/ledongthuc/pdf` for PDFs, `go-readability` (new dep) for web scraping, tree-sitter (existing CGO dep), React/TypeScript frontend (shadcn/ui).

**Design Doc:** `docs/plans/2026-02-19-unified-sources-design.md`

---

## Task 1: Create `internal/sources/` — Core Types

**Files:**
- Create: `go/internal/sources/source.go`
- Create: `go/internal/sources/source_test.go`

**Step 1: Write the test file**

```go
// go/internal/sources/source_test.go
package sources

import (
	"context"
	"testing"
	"time"
)

func TestArtifact_CategoryConstants(t *testing.T) {
	// Verify the three categories exist and are distinct.
	cats := []Category{Signal, Knowledge, Context}
	seen := map[Category]bool{}
	for _, c := range cats {
		if seen[c] {
			t.Errorf("duplicate category: %s", c)
		}
		seen[c] = true
	}
	if len(seen) != 3 {
		t.Errorf("expected 3 categories, got %d", len(seen))
	}
}

func TestScope_Constants(t *testing.T) {
	if ProjectScope == ModuleScope {
		t.Error("ProjectScope and ModuleScope should be different")
	}
}

func TestArtifact_Fields(t *testing.T) {
	a := Artifact{
		Source:   "github",
		Category: Signal,
		ID:       "#42",
		Title:    "Fix login",
		Body:     "Details here",
		URL:      "https://github.com/user/repo/issues/42",
		Files:    []string{"auth/login.go"},
		Module:   "root",
		Date:     time.Now(),
		Author:   "alice",
		Tags:     map[string]string{"state": "closed"},
	}
	if a.Source != "github" {
		t.Errorf("Source = %q, want %q", a.Source, "github")
	}
	if a.Category != Signal {
		t.Errorf("Category = %q, want %q", a.Category, Signal)
	}
	if len(a.Files) != 1 || a.Files[0] != "auth/login.go" {
		t.Errorf("Files = %v, want [auth/login.go]", a.Files)
	}
	if a.Tags["state"] != "closed" {
		t.Errorf("Tags[state] = %q, want %q", a.Tags["state"], "closed")
	}
}

// mockSource is a test double implementing Source.
type mockSource struct {
	name      string
	scope     Scope
	configErr error
	artifacts []Artifact
	fetchErr  error
}

func (m *mockSource) Name() string                                                { return m.name }
func (m *mockSource) Scope() Scope                                                { return m.scope }
func (m *mockSource) Configure(cfg SourceConfig) error                            { return m.configErr }
func (m *mockSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	return m.artifacts, m.fetchErr
}

func TestSourceInterface_Compliance(t *testing.T) {
	// Verify mockSource satisfies Source at compile time.
	var _ Source = (*mockSource)(nil)
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -run TestArtifact`
Expected: FAIL — package doesn't exist yet.

**Step 3: Write the source.go file**

```go
// go/internal/sources/source.go
package sources

import (
	"context"
	"time"
)

// Category classifies what an artifact represents.
type Category string

const (
	Signal    Category = "signal"    // file-linked: commits, PRs, issues, tickets
	Knowledge Category = "knowledge" // project-level: PDFs, docs, RFCs, web pages
	Context   Category = "context"   // hybrid: Slack threads, GitHub discussions
)

// Scope controls when a source is fetched during the pipeline.
type Scope int

const (
	ProjectScope Scope = iota // fetch once per project (most sources)
	ModuleScope               // fetch per module (git commits only)
)

// Artifact is the universal output of every source.
type Artifact struct {
	Source   string            // source name: "github", "jira", "notion", etc.
	Category Category
	ID       string            // unique within source: "#42", "PROJ-123", "page-id"
	Title    string
	Body     string
	URL      string
	Files    []string          // optional: linked file paths relative to repo root
	Module   string            // optional: linked module name
	Date     time.Time
	Author   string
	Tags     map[string]string // structured metadata: "state", "priority", etc.
}

// FetchRequest provides context to a source's Fetch method.
type FetchRequest struct {
	Project    string   // project name
	Module     string   // set only for ModuleScope sources
	ModulePath string   // filesystem path, ModuleScope only
	RepoRoot   string   // root of the codebase
}

// SourceConfig holds credentials and settings for a source.
type SourceConfig struct {
	Credentials map[string]string // from Settings UI / env vars
	Settings    map[string]string // from .carto/sources.yaml or auto-detect
}

// Source is the unified interface for all external integrations.
type Source interface {
	Name() string
	Scope() Scope
	Configure(cfg SourceConfig) error
	Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -run "TestArtifact|TestScope|TestSourceInterface"`
Expected: PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/sources/source.go internal/sources/source_test.go
git commit -m "feat(sources): add unified Source interface, Artifact, and Category types"
```

---

## Task 2: Create `internal/sources/` — Registry

**Files:**
- Create: `go/internal/sources/registry.go`
- Modify: `go/internal/sources/source_test.go` (add registry tests)

**Step 1: Add registry tests to source_test.go**

Append to `go/internal/sources/source_test.go`:

```go
func TestRegistry_FetchAll_ProjectScope(t *testing.T) {
	reg := NewRegistry()

	reg.Register(&mockSource{
		name:  "github",
		scope: ProjectScope,
		artifacts: []Artifact{
			{Source: "github", Category: Signal, ID: "#1", Title: "Issue 1"},
			{Source: "github", Category: Signal, ID: "#2", Title: "PR 2"},
		},
	})
	reg.Register(&mockSource{
		name:  "jira",
		scope: ProjectScope,
		artifacts: []Artifact{
			{Source: "jira", Category: Signal, ID: "PROJ-10", Title: "Ticket"},
		},
	})

	req := FetchRequest{Project: "test", RepoRoot: "/tmp/repo"}
	all, err := reg.FetchAllProject(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchAllProject: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 artifacts, got %d", len(all))
	}
}

func TestRegistry_FetchAll_SkipsErrors(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockSource{
		name:     "good",
		scope:    ProjectScope,
		artifacts: []Artifact{{Source: "good", ID: "1", Title: "OK"}},
	})
	reg.Register(&mockSource{
		name:     "bad",
		scope:    ProjectScope,
		fetchErr: fmt.Errorf("connection refused"),
	})
	reg.Register(&mockSource{
		name:     "also-good",
		scope:    ProjectScope,
		artifacts: []Artifact{{Source: "also-good", ID: "2", Title: "OK too"}},
	})

	req := FetchRequest{Project: "test", RepoRoot: "/tmp/repo"}
	all, err := reg.FetchAllProject(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchAllProject: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 artifacts (skipping failed source), got %d", len(all))
	}
}

func TestRegistry_FetchModule(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockSource{
		name:  "git",
		scope: ModuleScope,
		artifacts: []Artifact{
			{Source: "git", Category: Signal, ID: "abc123", Title: "commit"},
		},
	})
	// Project-scoped sources should be ignored by FetchModule.
	reg.Register(&mockSource{
		name:  "jira",
		scope: ProjectScope,
		artifacts: []Artifact{
			{Source: "jira", Category: Signal, ID: "J-1", Title: "ticket"},
		},
	})

	req := FetchRequest{Project: "test", Module: "mymod", ModulePath: "/tmp/repo/mymod", RepoRoot: "/tmp/repo"}
	all, err := reg.FetchModule(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchModule: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 artifact (module-scoped only), got %d", len(all))
	}
	if all[0].Source != "git" {
		t.Errorf("expected git artifact, got %s", all[0].Source)
	}
}

func TestRegistry_Empty(t *testing.T) {
	reg := NewRegistry()
	req := FetchRequest{Project: "test", RepoRoot: "/tmp/repo"}

	project, err := reg.FetchAllProject(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchAllProject on empty: %v", err)
	}
	if len(project) != 0 {
		t.Errorf("expected 0 from empty registry, got %d", len(project))
	}

	module, err := reg.FetchModule(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchModule on empty: %v", err)
	}
	if len(module) != 0 {
		t.Errorf("expected 0 from empty registry, got %d", len(module))
	}
}

func TestRegistry_Sources(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockSource{name: "git", scope: ModuleScope})
	reg.Register(&mockSource{name: "github", scope: ProjectScope})

	names := reg.SourceNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(names))
	}
}
```

Also add `"fmt"` to the imports in `source_test.go` (needed for `fmt.Errorf`).

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -run "TestRegistry"`
Expected: FAIL — `NewRegistry`, `FetchAllProject`, `FetchModule`, `SourceNames` undefined.

**Step 3: Write registry.go**

```go
// go/internal/sources/registry.go
package sources

import (
	"context"
	"log"
	"sync"
)

// Registry holds all configured sources and dispatches fetch calls.
type Registry struct {
	sources []Source
}

// NewRegistry creates an empty source registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a source to the registry.
func (r *Registry) Register(s Source) {
	r.sources = append(r.sources, s)
}

// SourceNames returns the names of all registered sources.
func (r *Registry) SourceNames() []string {
	names := make([]string, len(r.sources))
	for i, s := range r.sources {
		names[i] = s.Name()
	}
	return names
}

// FetchAllProject fetches artifacts from all ProjectScope sources concurrently.
// Individual source errors are logged but do not prevent other sources from running.
func (r *Registry) FetchAllProject(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	var projectSources []Source
	for _, s := range r.sources {
		if s.Scope() == ProjectScope {
			projectSources = append(projectSources, s)
		}
	}

	if len(projectSources) == 0 {
		return nil, nil
	}

	type result struct {
		artifacts []Artifact
		err       error
		name      string
	}

	results := make(chan result, len(projectSources))
	var wg sync.WaitGroup

	for _, s := range projectSources {
		wg.Add(1)
		go func(src Source) {
			defer wg.Done()
			arts, err := src.Fetch(ctx, req)
			results <- result{artifacts: arts, err: err, name: src.Name()}
		}(s)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var all []Artifact
	for res := range results {
		if res.err != nil {
			log.Printf("sources: warning: %s failed: %v", res.name, res.err)
			continue
		}
		all = append(all, res.artifacts...)
	}

	return all, nil
}

// FetchModule fetches artifacts from all ModuleScope sources.
// Only module-scoped sources (e.g. git) are invoked.
func (r *Registry) FetchModule(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	var all []Artifact
	for _, s := range r.sources {
		if s.Scope() != ModuleScope {
			continue
		}
		arts, err := s.Fetch(ctx, req)
		if err != nil {
			log.Printf("sources: warning: %s failed for module %s: %v", s.Name(), req.Module, err)
			continue
		}
		all = append(all, arts...)
	}
	return all, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -race`
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/sources/registry.go internal/sources/source_test.go
git commit -m "feat(sources): add Registry with concurrent FetchAllProject and FetchModule"
```

---

## Task 3: Port Git Source

**Files:**
- Create: `go/internal/sources/git.go`
- Create: `go/internal/sources/git_test.go`

**Step 1: Write the test**

```go
// go/internal/sources/git_test.go
package sources

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// gitCmd runs a git command in the given directory with test user config.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	fullArgs := append([]string{
		"-C", dir,
		"-c", "user.name=test",
		"-c", "user.email=test@test.com",
	}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func setupGitTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init")

	modDir := filepath.Join(dir, "mymodule")
	os.MkdirAll(modDir, 0o755)

	os.WriteFile(filepath.Join(modDir, "main.go"), []byte("package main\n"), 0o644)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "Initial commit")

	os.WriteFile(filepath.Join(modDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "Add main function")

	os.WriteFile(filepath.Join(modDir, "util.go"), []byte("package main\n\nfunc helper() {}\n"), 0o644)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "Fix bug from PR #42")

	return dir
}

func TestGitSource_Name(t *testing.T) {
	src := NewGitSource("")
	if src.Name() != "git" {
		t.Errorf("Name() = %q, want %q", src.Name(), "git")
	}
}

func TestGitSource_Scope(t *testing.T) {
	src := NewGitSource("")
	if src.Scope() != ModuleScope {
		t.Errorf("Scope() = %d, want ModuleScope", src.Scope())
	}
}

func TestGitSource_Fetch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	repoDir := setupGitTestRepo(t)
	src := NewGitSource(repoDir)

	req := FetchRequest{
		Project:    "test",
		Module:     "mymodule",
		ModulePath: filepath.Join(repoDir, "mymodule"),
		RepoRoot:   repoDir,
	}

	artifacts, err := src.Fetch(context.Background(), req)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	var commits int
	for _, a := range artifacts {
		if a.Category != Signal {
			t.Errorf("expected Signal category, got %s", a.Category)
		}
		if a.Source != "git" {
			t.Errorf("expected source=git, got %s", a.Source)
		}
		if a.Tags["type"] == "commit" {
			commits++
		}
	}

	if commits < 3 {
		t.Errorf("expected at least 3 commits, got %d", commits)
	}
}

func TestGitSource_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	src := NewGitSource(dir)

	artifacts, err := src.Fetch(context.Background(), FetchRequest{
		Module:     "test",
		ModulePath: dir,
		RepoRoot:   dir,
	})
	if err != nil {
		t.Fatalf("expected nil error for non-git dir, got: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(artifacts))
	}
}

var _ Source = (*GitSource)(nil)
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -run TestGitSource`
Expected: FAIL — `NewGitSource`, `GitSource` undefined.

**Step 3: Write git.go**

Port from `internal/signals/git.go` adapted to the new interface:

```go
// go/internal/sources/git.go
package sources

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

var prRefPattern = regexp.MustCompile(`(?i)(?:PR\s*#|pull\s*(?:request)?\s*#|#)(\d+)`)

// GitSource extracts commit-based artifacts from git history.
type GitSource struct {
	repoRoot   string
	maxCommits int
}

// NewGitSource creates a git source rooted at the given directory.
func NewGitSource(repoRoot string) *GitSource {
	return &GitSource{repoRoot: repoRoot, maxCommits: 20}
}

func (g *GitSource) Name() string   { return "git" }
func (g *GitSource) Scope() Scope   { return ModuleScope }

func (g *GitSource) Configure(cfg SourceConfig) error {
	if root, ok := cfg.Settings["repo_root"]; ok {
		g.repoRoot = root
	}
	if max, ok := cfg.Settings["max_commits"]; ok {
		var n int
		if _, err := fmt.Sscanf(max, "%d", &n); err == nil && n > 0 {
			g.maxCommits = n
		}
	}
	return nil
}

func (g *GitSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	root := g.repoRoot
	if root == "" {
		root = req.RepoRoot
	}

	args := []string{
		"-C", root,
		"log",
		fmt.Sprintf("--pretty=format:%%H|%%an|%%aI|%%s"),
		fmt.Sprintf("-n%d", g.maxCommits),
	}

	// Scope to module's relative path within the repo.
	if req.ModulePath != "" && req.ModulePath != root {
		relPath := strings.TrimPrefix(req.ModulePath, root+"/")
		if relPath != "" {
			args = append(args, "--", relPath)
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil // not a git repo — return empty
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	var artifacts []Artifact
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		hash, author, dateStr, subject := parts[0], parts[1], parts[2], parts[3]
		date, _ := time.Parse(time.RFC3339, dateStr)

		artifacts = append(artifacts, Artifact{
			Source:   "git",
			Category: Signal,
			ID:       hash,
			Title:    subject,
			Date:     date,
			Author:   author,
			Module:   req.Module,
			Tags:     map[string]string{"type": "commit"},
		})

		// Extract PR references.
		matches := prRefPattern.FindAllStringSubmatch(subject, -1)
		for _, m := range matches {
			artifacts = append(artifacts, Artifact{
				Source:   "git",
				Category: Signal,
				ID:       "#" + m[1],
				Title:    subject,
				Date:     date,
				Author:   author,
				Module:   req.Module,
				Tags:     map[string]string{"type": "pr"},
			})
		}
	}

	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].Date.After(artifacts[j].Date)
	})

	return artifacts, nil
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -race -run TestGitSource`
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/sources/git.go internal/sources/git_test.go
git commit -m "feat(sources): port git source to unified Source interface"
```

---

## Task 4: Port GitHub Source

**Files:**
- Create: `go/internal/sources/github.go`
- Create: `go/internal/sources/github_test.go`

**Step 1: Write the test**

```go
// go/internal/sources/github_test.go
package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGitHubSource_Name(t *testing.T) {
	src := NewGitHubSource()
	if src.Name() != "github" {
		t.Errorf("Name() = %q, want %q", src.Name(), "github")
	}
}

func TestGitHubSource_Scope(t *testing.T) {
	src := NewGitHubSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestGitHubSource_Configure(t *testing.T) {
	src := NewGitHubSource()
	err := src.Configure(SourceConfig{
		Settings:    map[string]string{"owner": "octocat", "repo": "Hello-World"},
		Credentials: map[string]string{"github_token": "ghp_test"},
	})
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if src.owner != "octocat" || src.repo != "Hello-World" {
		t.Error("owner/repo not set")
	}
}

func TestGitHubSource_Configure_Missing(t *testing.T) {
	src := NewGitHubSource()
	err := src.Configure(SourceConfig{Settings: map[string]string{}})
	if err == nil {
		t.Error("expected error when owner/repo missing")
	}
}

func TestGitHubSource_Fetch(t *testing.T) {
	issues := []map[string]any{
		{
			"number":       42,
			"title":        "Fix login bug",
			"body":         "Login fails on mobile",
			"html_url":     "https://github.com/user/repo/issues/42",
			"created_at":   "2025-01-01T00:00:00Z",
			"user":         map[string]any{"login": "alice"},
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

	src := NewGitHubSource()
	src.baseURL = srv.URL
	src.Configure(SourceConfig{
		Settings: map[string]string{"owner": "user", "repo": "repo"},
	})

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(artifacts) != 2 {
		t.Fatalf("expected 2 artifacts, got %d", len(artifacts))
	}

	// Verify issue.
	if artifacts[0].Category != Signal || artifacts[0].ID != "#42" {
		t.Errorf("unexpected issue: %+v", artifacts[0])
	}
	if artifacts[0].Source != "github" {
		t.Errorf("expected source=github, got %s", artifacts[0].Source)
	}
	// Verify PR.
	if artifacts[1].Tags["type"] != "pr" || artifacts[1].ID != "#43" {
		t.Errorf("unexpected PR: %+v", artifacts[1])
	}
}

var _ Source = (*GitHubSource)(nil)
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -run TestGitHubSource`
Expected: FAIL — `NewGitHubSource`, `GitHubSource` undefined.

**Step 3: Write github.go**

Port from `internal/signals/github.go` with improvements (pagination support, proper Source interface):

```go
// go/internal/sources/github.go
package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GitHubSource fetches issues and PRs from the GitHub API.
type GitHubSource struct {
	owner    string
	repo     string
	token    string
	baseURL  string
	maxPages int
	http     http.Client
}

// NewGitHubSource creates an unconfigured GitHub source.
func NewGitHubSource() *GitHubSource {
	return &GitHubSource{
		baseURL:  "https://api.github.com",
		maxPages: 3,
		http:     http.Client{Timeout: 15 * time.Second},
	}
}

func (g *GitHubSource) Name() string { return "github" }
func (g *GitHubSource) Scope() Scope { return ProjectScope }

func (g *GitHubSource) Configure(cfg SourceConfig) error {
	g.owner = cfg.Settings["owner"]
	g.repo = cfg.Settings["repo"]
	if t, ok := cfg.Credentials["github_token"]; ok {
		g.token = t
	}
	if g.owner == "" || g.repo == "" {
		return fmt.Errorf("github: owner and repo are required")
	}
	return nil
}

func (g *GitHubSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	var artifacts []Artifact

	issues, err := g.fetchIssues(ctx)
	if err != nil {
		return nil, fmt.Errorf("github: fetch issues: %w", err)
	}
	artifacts = append(artifacts, issues...)

	prs, err := g.fetchPRs(ctx)
	if err != nil {
		return nil, fmt.Errorf("github: fetch PRs: %w", err)
	}
	artifacts = append(artifacts, prs...)

	return artifacts, nil
}

type ghIssue struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	CreatedAt   time.Time     `json:"created_at"`
	User        ghUser        `json:"user"`
	PullRequest *ghPullReqRef `json:"pull_request"`
	State       string        `json:"state"`
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
	State     string    `json:"state"`
}

type ghUser struct {
	Login string `json:"login"`
}

func (g *GitHubSource) apiGet(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL+path, nil)
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

func (g *GitHubSource) fetchIssues(ctx context.Context) ([]Artifact, error) {
	var ghIssues []ghIssue
	path := fmt.Sprintf("/repos/%s/%s/issues?state=all&per_page=30&sort=updated", g.owner, g.repo)
	if err := g.apiGet(ctx, path, &ghIssues); err != nil {
		return nil, err
	}

	var artifacts []Artifact
	for _, issue := range ghIssues {
		if issue.PullRequest != nil {
			continue
		}
		artifacts = append(artifacts, Artifact{
			Source:   "github",
			Category: Signal,
			ID:       fmt.Sprintf("#%d", issue.Number),
			Title:    issue.Title,
			Body:     truncateBody(issue.Body, 500),
			URL:      issue.HTMLURL,
			Date:     issue.CreatedAt,
			Author:   issue.User.Login,
			Tags:     map[string]string{"type": "issue", "state": issue.State},
		})
	}
	return artifacts, nil
}

func (g *GitHubSource) fetchPRs(ctx context.Context) ([]Artifact, error) {
	var ghPRs []ghPR
	path := fmt.Sprintf("/repos/%s/%s/pulls?state=all&per_page=30&sort=updated", g.owner, g.repo)
	if err := g.apiGet(ctx, path, &ghPRs); err != nil {
		return nil, err
	}

	var artifacts []Artifact
	for _, pr := range ghPRs {
		artifacts = append(artifacts, Artifact{
			Source:   "github",
			Category: Signal,
			ID:       fmt.Sprintf("#%d", pr.Number),
			Title:    pr.Title,
			Body:     truncateBody(pr.Body, 500),
			URL:      pr.HTMLURL,
			Date:     pr.CreatedAt,
			Author:   pr.User.Login,
			Tags:     map[string]string{"type": "pr", "state": pr.State},
		})
	}
	return artifacts, nil
}

func truncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -race -run TestGitHubSource`
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/sources/github.go internal/sources/github_test.go
git commit -m "feat(sources): port github source to unified Source interface"
```

---

## Task 5: Port Local PDF Source

**Files:**
- Create: `go/internal/sources/pdf.go`
- Create: `go/internal/sources/pdf_test.go`

**Step 1: Write the test**

```go
// go/internal/sources/pdf_test.go
package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPDFSource_Name(t *testing.T) {
	src := NewPDFSource()
	if src.Name() != "local-pdf" {
		t.Errorf("Name() = %q, want %q", src.Name(), "local-pdf")
	}
}

func TestPDFSource_Scope(t *testing.T) {
	src := NewPDFSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestPDFSource_Configure_MissingDir(t *testing.T) {
	src := NewPDFSource()
	err := src.Configure(SourceConfig{Settings: map[string]string{}})
	if err == nil {
		t.Error("expected error when dir missing")
	}
}

func TestPDFSource_Fetch_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	src := NewPDFSource()
	src.Configure(SourceConfig{Settings: map[string]string{"dir": dir}})

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts from empty dir, got %d", len(artifacts))
	}
}

func TestPDFSource_Fetch_SkipsNonPDF(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# hello"), 0o644)
	os.WriteFile(filepath.Join(dir, "data.csv"), []byte("a,b\n1,2"), 0o644)

	src := NewPDFSource()
	src.Configure(SourceConfig{Settings: map[string]string{"dir": dir}})

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts (no PDFs), got %d", len(artifacts))
	}
}

func TestPDFSource_ArtifactCategory(t *testing.T) {
	// Verify artifacts produced by PDFSource would have Knowledge category.
	// We can't easily produce a real PDF in a test, so verify via the struct.
	a := Artifact{
		Source:   "local-pdf",
		Category: Knowledge,
		Title:    "Test Doc",
	}
	if a.Category != Knowledge {
		t.Errorf("expected Knowledge category, got %s", a.Category)
	}
}

var _ Source = (*PDFSource)(nil)
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -run TestPDFSource`
Expected: FAIL — `NewPDFSource`, `PDFSource` undefined.

**Step 3: Write pdf.go**

```go
// go/internal/sources/pdf.go
package sources

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
)

// PDFSource reads PDF files from a configured directory.
type PDFSource struct {
	dir string
}

// NewPDFSource creates a PDF knowledge source.
func NewPDFSource() *PDFSource {
	return &PDFSource{}
}

func (p *PDFSource) Name() string { return "local-pdf" }
func (p *PDFSource) Scope() Scope { return ProjectScope }

func (p *PDFSource) Configure(cfg SourceConfig) error {
	dir := cfg.Settings["dir"]
	if dir == "" {
		return fmt.Errorf("local-pdf: 'dir' setting is required")
	}
	p.dir = dir
	return nil
}

func (p *PDFSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, fmt.Errorf("local-pdf: read dir: %w", err)
	}

	var artifacts []Artifact
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

		title := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		artifacts = append(artifacts, Artifact{
			Source:   "local-pdf",
			Category: Knowledge,
			ID:       entry.Name(),
			Title:    title,
			Body:     text,
			URL:      "file://" + absPath,
			Tags:     map[string]string{"format": "pdf"},
		})
	}
	return artifacts, nil
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

**Step 4: Run tests to verify they pass**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -race -run TestPDFSource`
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/sources/pdf.go internal/sources/pdf_test.go
git commit -m "feat(sources): port local-pdf source to unified Source interface"
```

---

## Task 6: Update Pipeline to Use `sources.Registry`

**Files:**
- Modify: `go/internal/pipeline/pipeline.go`
- Modify: `go/internal/pipeline/pipeline_test.go`

**Step 1: Update pipeline_test.go**

Replace the `mockSignalSource` with a `mockSource` implementing the new interface, and update `Config` usage:

- Remove import of `"github.com/divyekant/carto/internal/signals"`
- Add import of `"github.com/divyekant/carto/internal/sources"`
- Replace `mockSignalSource` struct:

```go
type mockPipelineSource struct{}

func (s *mockPipelineSource) Name() string      { return "mock" }
func (s *mockPipelineSource) Scope() sources.Scope { return sources.ProjectScope }
func (s *mockPipelineSource) Configure(cfg sources.SourceConfig) error { return nil }
func (s *mockPipelineSource) Fetch(ctx context.Context, req sources.FetchRequest) ([]sources.Artifact, error) {
	return []sources.Artifact{
		{Source: "mock", Category: sources.Signal, ID: "TEST-1", Title: "Test ticket", Author: "tester"},
	}, nil
}
```

- Update `TestRun_FullPipeline` to use `sources.NewRegistry()` and `sources.Register(&mockPipelineSource{})` instead of `signals.NewRegistry()`.
- Replace `SignalRegistry: registry,` with `SourceRegistry: registry,`
- Update layer check: verify `"signals"` layer is still stored.

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/pipeline/ -v -run TestRun_FullPipeline`
Expected: FAIL — `SourceRegistry` field doesn't exist on `Config` yet.

**Step 3: Update pipeline.go**

Key changes to `go/internal/pipeline/pipeline.go`:

1. Replace imports: remove `signals` and `knowledge`, add `sources`.
2. In `Config` struct: replace `SignalRegistry *signals.Registry` and `KnowledgeRegistry *knowledge.Registry` with `SourceRegistry *sources.Registry`.
3. Phase 3: Replace signal-fetching loop with two steps:
   - `cfg.SourceRegistry.FetchModule(ctx, req)` for module-scoped sources (git)
   - After the per-module loop, `cfg.SourceRegistry.FetchAllProject(ctx, req)` once for project-scoped sources
4. Remove Phase 3b knowledge section (now handled by project-scoped sources).
5. Phase 5: Store module artifacts under `layer:signals` (same as before). Store project artifacts by category:
   - Signal → `_project/layer:signals/{source}`
   - Knowledge → `_knowledge/{source}/{artifact.ID}`
   - Context → `_context/{source}/{artifact.ID}`

The `moduleContext` struct changes from holding `signals []signals.Signal` to `artifacts []sources.Artifact`.

**Step 4: Run full test suite**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/pipeline/ -v -race`
Expected: ALL PASS

**Step 5: Run full project tests**

Run: `cd /Users/dk/projects/indexer/go && go test ./... -short`
Expected: PASS (old signal/knowledge packages still compile, just not used by pipeline anymore)

**Step 6: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/pipeline/pipeline.go internal/pipeline/pipeline_test.go
git commit -m "refactor(pipeline): replace SignalRegistry+KnowledgeRegistry with unified SourceRegistry"
```

---

## Task 7: Update Storage — Retrieval Tier for Knowledge & Context

**Files:**
- Modify: `go/internal/storage/store.go`
- Modify: `go/internal/storage/store_test.go` (or create if not exists)

**Step 1: Write the test**

Add tests to verify that Full tier retrieves `_knowledge` and `_context` entries:

```go
func TestStore_StoreAndRetrieveKnowledge(t *testing.T) {
	mem := &mockMemories{healthy: true}
	store := NewStore(mem, "test-project")

	// Store a knowledge artifact.
	err := store.StoreLayer("_knowledge/notion", "Design-RFC", "# Design RFC\n\nContent here")
	if err != nil {
		t.Fatalf("StoreLayer: %v", err)
	}

	// Verify it was stored with the correct source tag.
	memories := mem.getMemories()
	found := false
	for _, m := range memories {
		if strings.Contains(m.source, "_knowledge") && strings.Contains(m.source, "notion") {
			found = true
		}
	}
	if !found {
		t.Error("knowledge artifact not stored with expected source tag")
	}
}

func TestStore_StoreAndRetrieveContext(t *testing.T) {
	mem := &mockMemories{healthy: true}
	store := NewStore(mem, "test-project")

	err := store.StoreLayer("_context/slack", "thread-123", "Discussion about auth")
	if err != nil {
		t.Fatalf("StoreLayer: %v", err)
	}

	memories := mem.getMemories()
	found := false
	for _, m := range memories {
		if strings.Contains(m.source, "_context") && strings.Contains(m.source, "slack") {
			found = true
		}
	}
	if !found {
		t.Error("context artifact not stored with expected source tag")
	}
}
```

**Step 2: Add `RetrieveKnowledge` and `RetrieveContext` methods**

Add to `store.go`:

```go
// RetrieveKnowledge retrieves all knowledge artifacts for the project.
func (s *Store) RetrieveKnowledge() ([]SearchResult, error) {
	prefix := fmt.Sprintf("carto/%s/_knowledge/", s.project)
	return s.memories.ListBySource(prefix, 0)
}

// RetrieveContext retrieves all context artifacts for the project.
func (s *Store) RetrieveContext() ([]SearchResult, error) {
	prefix := fmt.Sprintf("carto/%s/_context/", s.project)
	return s.memories.ListBySource(prefix, 0)
}

// RetrieveProjectSignals retrieves project-level (non-module) signals.
func (s *Store) RetrieveProjectSignals() ([]SearchResult, error) {
	prefix := fmt.Sprintf("carto/%s/_project/layer:signals/", s.project)
	return s.memories.ListBySource(prefix, 0)
}
```

Update `RetrieveByTier` for Full tier to also call `RetrieveKnowledge` and `RetrieveContext`.

**Step 3: Run tests**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/storage/ -v -race`
Expected: ALL PASS

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/storage/store.go internal/storage/store_test.go
git commit -m "feat(storage): add knowledge and context retrieval methods to Store"
```

---

## Task 8: Update Server Handlers — Wire New Registry

**Files:**
- Modify: `go/internal/server/handlers.go`

**Step 1: Update `runIndex` to build sources.Registry**

In `go/internal/server/handlers.go` function `runIndex` (line ~384):

1. Replace imports: `signals` and `knowledge` → `sources`.
2. Replace the signal registry block (lines 384-396) and knowledge block (lines 398-406) with:

```go
	registry := sources.NewRegistry()
	registry.Register(sources.NewGitSource(absPath))

	// Auto-detect GitHub from URL or git remote.
	owner, repo := "", ""
	if req.URL != "" {
		owner, repo = gitclone.ParseOwnerRepo(req.URL)
	}
	if owner == "" {
		owner, repo = detectGitHubRemote(absPath)
	}
	if owner != "" {
		ghSrc := sources.NewGitHubSource()
		ghSrc.Configure(sources.SourceConfig{
			Settings:    map[string]string{"owner": owner, "repo": repo},
			Credentials: map[string]string{"github_token": cfg.GitHubToken},
		})
		registry.Register(ghSrc)
	}

	// Auto-detect local PDFs.
	docsDir := filepath.Join(absPath, "docs")
	if info, err := os.Stat(docsDir); err == nil && info.IsDir() {
		pdfSrc := sources.NewPDFSource()
		pdfSrc.Configure(sources.SourceConfig{
			Settings: map[string]string{"dir": docsDir},
		})
		registry.Register(pdfSrc)
	}
```

3. In the pipeline config, replace `SignalRegistry` and `KnowledgeRegistry` with `SourceRegistry: registry,`.

4. Add `detectGitHubRemote` helper:

```go
func detectGitHubRemote(repoRoot string) (owner, repo string) {
	cmd := exec.Command("git", "-C", repoRoot, "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		return "", ""
	}
	url := strings.TrimSpace(string(out))
	return gitclone.ParseOwnerRepo(url)
}
```

**Step 2: Run build**

Run: `cd /Users/dk/projects/indexer/go && go build ./...`
Expected: Compiles successfully.

**Step 3: Run full tests**

Run: `cd /Users/dk/projects/indexer/go && go test ./... -short`
Expected: ALL PASS

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/server/handlers.go
git commit -m "refactor(server): wire unified sources.Registry into runIndex"
```

---

## Task 9: Add Config Fields for New Source Credentials

**Files:**
- Modify: `go/internal/config/config.go`

**Step 1: Add new fields to Config struct**

Add after `GitHubToken` (line 20):

```go
	JiraToken     string
	JiraEmail     string
	LinearToken   string
	NotionToken   string
	SlackToken    string
```

**Step 2: Add matching fields to `persistedConfig`**

```go
	JiraToken     string `json:"jira_token,omitempty"`
	JiraEmail     string `json:"jira_email,omitempty"`
	LinearToken   string `json:"linear_token,omitempty"`
	NotionToken   string `json:"notion_token,omitempty"`
	SlackToken    string `json:"slack_token,omitempty"`
```

**Step 3: Update `Load()` to read env vars**

Add after `GitHubToken` loading:

```go
	JiraToken:     os.Getenv("JIRA_API_TOKEN"),
	JiraEmail:     os.Getenv("JIRA_EMAIL"),
	LinearToken:   os.Getenv("LINEAR_API_KEY"),
	NotionToken:   os.Getenv("NOTION_API_KEY"),
	SlackToken:    os.Getenv("SLACK_BOT_TOKEN"),
```

**Step 4: Update `Save()` and `mergeConfig()` for the new fields**

Add corresponding lines to both functions for each new field.

**Step 5: Run tests**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/config/ -v`
Expected: ALL PASS

**Step 6: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/config/config.go
git commit -m "feat(config): add credential fields for jira, linear, notion, slack"
```

---

## Task 10: Build Jira Source

**Files:**
- Create: `go/internal/sources/jira.go`
- Create: `go/internal/sources/jira_test.go`

**Step 1: Write the test**

```go
// go/internal/sources/jira_test.go
package sources

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestJiraSource_Name(t *testing.T) {
	src := NewJiraSource()
	if src.Name() != "jira" {
		t.Errorf("Name() = %q, want %q", src.Name(), "jira")
	}
}

func TestJiraSource_Scope(t *testing.T) {
	src := NewJiraSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestJiraSource_Configure_Missing(t *testing.T) {
	src := NewJiraSource()
	err := src.Configure(SourceConfig{Settings: map[string]string{}})
	if err == nil {
		t.Error("expected error when url/project missing")
	}
}

func TestJiraSource_Fetch(t *testing.T) {
	response := map[string]any{
		"issues": []map[string]any{
			{
				"key": "PROJ-123",
				"fields": map[string]any{
					"summary": "Fix auth bug",
					"description": map[string]any{
						"content": []map[string]any{
							{"type": "paragraph", "content": []map[string]any{
								{"type": "text", "text": "Auth is broken"},
							}},
						},
					},
					"status":  map[string]any{"name": "Done"},
					"creator": map[string]any{"displayName": "Alice"},
					"updated": "2025-01-01T00:00:00.000+0000",
				},
			},
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/rest/api/3/search", func(w http.ResponseWriter, r *http.Request) {
		// Verify basic auth.
		user, _, ok := r.BasicAuth()
		if !ok || user != "alice@test.com" {
			w.WriteHeader(401)
			return
		}
		json.NewEncoder(w).Encode(response)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	src := NewJiraSource()
	src.baseURL = srv.URL
	src.Configure(SourceConfig{
		Settings:    map[string]string{"url": srv.URL, "project": "PROJ"},
		Credentials: map[string]string{"jira_token": "test-token", "jira_email": "alice@test.com"},
	})

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(artifacts) != 1 {
		t.Fatalf("expected 1 artifact, got %d", len(artifacts))
	}
	if artifacts[0].ID != "PROJ-123" {
		t.Errorf("ID = %q, want %q", artifacts[0].ID, "PROJ-123")
	}
	if artifacts[0].Category != Signal {
		t.Errorf("Category = %q, want Signal", artifacts[0].Category)
	}
	if artifacts[0].Tags["state"] != "Done" {
		t.Errorf("Tags[state] = %q, want %q", artifacts[0].Tags["state"], "Done")
	}
}

var _ Source = (*JiraSource)(nil)
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -run TestJiraSource`
Expected: FAIL

**Step 3: Write jira.go**

Implement `JiraSource` using Jira REST API v3 with basic auth (email:token), JQL search, ADF (Atlassian Document Format) body extraction.

**Step 4: Run tests**

Run: `cd /Users/dk/projects/indexer/go && go test ./internal/sources/ -v -race -run TestJiraSource`
Expected: ALL PASS

**Step 5: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add internal/sources/jira.go internal/sources/jira_test.go
git commit -m "feat(sources): add Jira source with REST API v3 integration"
```

---

## Task 11: Build Linear Source

**Files:**
- Create: `go/internal/sources/linear.go`
- Create: `go/internal/sources/linear_test.go`

Follow the same TDD pattern as Task 10. Linear uses GraphQL API:
- Query: `issues(filter: { team: { key: { eq: $team } } }, orderBy: updatedAt, first: 50)`
- Auth: Bearer token in `Authorization` header
- Produces Signal artifacts with `Tags: {"state": issue.state.name, "priority": issue.priority}`

**Step 1:** Write test with mock GraphQL server.
**Step 2:** Run test, verify fail.
**Step 3:** Implement `linear.go`.
**Step 4:** Run tests, verify pass.
**Step 5:** Commit: `feat(sources): add Linear source with GraphQL integration`

---

## Task 12: Build Notion Source

**Files:**
- Create: `go/internal/sources/notion.go`
- Create: `go/internal/sources/notion_test.go`

Follow TDD pattern. Notion uses REST API:
- `POST /v1/databases/{id}/query` for page list
- `GET /v1/blocks/{id}/children` for content
- Auth: Bearer token with `Notion-Version: 2022-06-28` header
- Produces Knowledge artifacts

**Step 1:** Write test with mock Notion server.
**Step 2:** Run test, verify fail.
**Step 3:** Implement `notion.go` with block-to-text conversion.
**Step 4:** Run tests, verify pass.
**Step 5:** Commit: `feat(sources): add Notion source with database query integration`

---

## Task 13: Build Slack Source

**Files:**
- Create: `go/internal/sources/slack.go`
- Create: `go/internal/sources/slack_test.go`

Follow TDD pattern. Slack uses Web API:
- `conversations.history` for channel messages
- `conversations.replies` for thread replies
- Auth: Bearer token
- Group messages into threads, 3+ message threads → Context artifact
- Filter: last 30 days, skip bots, skip short threads

**Step 1:** Write test with mock Slack API server.
**Step 2:** Run test, verify fail.
**Step 3:** Implement `slack.go`.
**Step 4:** Run tests, verify pass.
**Step 5:** Commit: `feat(sources): add Slack source with thread-based context extraction`

---

## Task 14: Build Web Source

**Files:**
- Create: `go/internal/sources/web.go`
- Create: `go/internal/sources/web_test.go`

Follow TDD pattern. Web source:
- HTTP GET each configured URL
- Extract readable content (add `go-readability` dependency)
- Truncate at 50KB per page
- Produces Knowledge artifacts

**Step 1:** Write test with mock HTTP server serving HTML.
**Step 2:** Run test, verify fail.
**Step 3:** Add `go-readability` dep: `go get github.com/go-shiori/go-readability`
**Step 4:** Implement `web.go`.
**Step 5:** Run tests, verify pass.
**Step 6:** Commit: `feat(sources): add web source with readability-based content extraction`

---

## Task 15: Add `.carto/sources.yaml` Parsing + Auto-Detect

**Files:**
- Create: `go/internal/sources/config.go`
- Create: `go/internal/sources/config_test.go`
- Modify: `go/internal/server/handlers.go` (wire yaml loading)

**Step 1: Write test for yaml parsing**

```go
func TestParseSourcesYAML(t *testing.T) {
	yaml := `
sources:
  jira:
    url: https://mycompany.atlassian.net
    project: PROJ
  slack:
    channels:
      - "#engineering"
      - "#architecture"
  web:
    urls:
      - https://docs.example.com/api
  github:
    discussions: true
`
	cfg, err := ParseSourcesConfig([]byte(yaml))
	if err != nil {
		t.Fatalf("ParseSourcesConfig: %v", err)
	}
	if cfg.Sources["jira"].Settings["project"] != "PROJ" {
		t.Error("jira project not parsed")
	}
	if len(cfg.Sources["slack"].ListSettings["channels"]) != 2 {
		t.Error("slack channels not parsed")
	}
}
```

**Step 2:** Implement yaml parser with `gopkg.in/yaml.v3`.
**Step 3:** Add `BuildRegistry` function that combines auto-detect + yaml config + credentials.
**Step 4:** Wire into `runIndex` in handlers.go.
**Step 5:** Run tests.
**Step 6:** Commit: `feat(sources): add .carto/sources.yaml parsing and auto-detect registry builder`

---

## Task 16: Update Settings UI — Integration Credentials

**Files:**
- Modify: `go/web/src/pages/Settings.tsx` (lines 556-575)
- Modify: `go/internal/server/handlers.go` (settings API to accept new fields)

**Step 1: Expand the Integrations card**

Replace the single GitHub token field with fields for all credentials:

| Field | Label | Placeholder | Help Text |
|---|---|---|---|
| `github_token` | GitHub Token | `ghp_...` | Private repos, issues, PRs |
| `jira_token` | Jira API Token | `...` | Jira Cloud REST API |
| `jira_email` | Jira Email | `user@company.com` | Used with API token for auth |
| `linear_token` | Linear API Key | `lin_api_...` | Linear GraphQL API |
| `notion_token` | Notion Integration Token | `ntn_...` | Notion database access |
| `slack_token` | Slack Bot Token | `xoxb-...` | Channel history access |

Each field follows the same pattern as the existing GitHub token field (password type, optional placeholder, help text).

**Step 2: Update settings API handler**

Ensure the settings GET/PUT handlers in `handlers.go` include the new config fields.

**Step 3: Build frontend**

Run: `cd /Users/dk/projects/indexer/go/web && npm install && npx tsc -b && npx vite build`
Expected: Build succeeds.

**Step 4: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add web/src/pages/Settings.tsx internal/server/handlers.go
git commit -m "feat(ui): expand Settings integrations with all source credential fields"
```

---

## Task 17: Delete Old Packages + Final Integration

**Files:**
- Delete: `go/internal/signals/` (entire directory)
- Delete: `go/internal/knowledge/` (entire directory)
- Modify: Any remaining imports of old packages

**Step 1: Search for remaining imports**

Run: `cd /Users/dk/projects/indexer/go && grep -r "internal/signals\|internal/knowledge" --include="*.go" .`

Fix any remaining references. The pipeline and server should already be updated (Tasks 6, 8).

**Step 2: Delete old packages**

```bash
rm -rf /Users/dk/projects/indexer/go/internal/signals
rm -rf /Users/dk/projects/indexer/go/internal/knowledge
```

**Step 3: Run full test suite**

Run: `cd /Users/dk/projects/indexer/go && go test -race ./...`
Expected: ALL PASS

**Step 4: Run go vet**

Run: `cd /Users/dk/projects/indexer/go && go vet ./...`
Expected: Clean

**Step 5: Build**

Run: `cd /Users/dk/projects/indexer/go && go build -o carto ./cmd/carto`
Expected: Build succeeds

**Step 6: Build frontend**

Run: `cd /Users/dk/projects/indexer/go/web && npx tsc -b && npx vite build`
Expected: Build succeeds

**Step 7: Commit**

```bash
cd /Users/dk/projects/indexer/go
git add -A
git commit -m "refactor: delete deprecated signals/ and knowledge/ packages, complete unified sources migration"
```

---

## Task 18: Docker Build + Deploy Verification

**Step 1: Docker build**

Run: `cd /Users/dk/projects/indexer/go && docker compose build`
Expected: Build succeeds

**Step 2: Deploy**

Run: `cd /Users/dk/projects/indexer/go && docker compose down && docker compose up -d`

**Step 3: Health check**

Run: `curl -s http://localhost:8950/api/health | python3 -m json.tool`
Expected: `{"status": "ok", "docker": true, "memories_healthy": true}`

**Step 4: Commit tag**

```bash
cd /Users/dk/projects/indexer/go
git tag -a v0.4.0 -m "feat: unified sources architecture with 8 source types"
```
