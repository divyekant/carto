# CLI/API Parity + Dense UI Redesign — Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add full CRUD CLI commands with `--json` output, three new API endpoints, a thin SDK package, and redesign every UI page for maximum information density.

**Architecture:** Two independent streams. Stream 1 (CLI/API) adds Cobra subcommands for `projects`, `sources`, and `config`, wires `--json`/`--quiet` global flags, adds three new server handlers, and exposes a `pkg/carto` Go package. Stream 2 (UI) installs three new shadcn components (Table, Switch, Tooltip), then rewrites Layout, Dashboard, Index, ProjectDetail, Query, and Settings pages for dense, multi-column layouts with no width constraints.

**Tech Stack:** Go + Cobra (CLI), net/http (API), React + shadcn/ui + Tailwind CSS (UI)

---

## Stream 1: CLI/API

### Task 1: Add `--json` and `--quiet` global flags

**Files:**
- Modify: `go/cmd/carto/main.go:41-58` (root command setup)

**Step 1: Add persistent flags to root command**

In `main()`, after creating `root`, add:

```go
root.PersistentFlags().Bool("json", false, "Output machine-readable JSON")
root.PersistentFlags().BoolP("quiet", "q", false, "Suppress progress spinners, only output result")
```

**Step 2: Add a `writeOutput` helper function**

Below the existing `formatBytes` helper, add:

```go
// writeOutput writes the result either as JSON (if --json) or by calling the
// human-friendly printer function. Reads the --json flag from cmd.
func writeOutput(cmd *cobra.Command, data any, humanFn func()) {
	jsonMode, _ := cmd.Flags().GetBool("json")
	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(data)
		return
	}
	humanFn()
}
```

**Step 3: Run `go build -o carto ./cmd/carto` to verify it compiles**

Expected: binary builds with no errors.

**Step 4: Commit**

```bash
git add go/cmd/carto/main.go
git commit -m "feat(cli): add --json and --quiet global persistent flags"
```

---

### Task 2: Add `carto projects list` command

**Files:**
- Modify: `go/cmd/carto/main.go` (add `projectsCmd` + `projectsListCmd`)
- Test: `go/cmd/carto/main_test.go` (new file)

**Step 1: Write the test**

Create `go/cmd/carto/main_test.go`:

```go
package main

import (
	"testing"
)

func TestProjectsListCmd(t *testing.T) {
	cmd := projectsCmd()
	if cmd.Use != "projects" {
		t.Fatalf("expected Use=projects, got %s", cmd.Use)
	}
	// Verify list subcommand exists.
	listCmd, _, err := cmd.Find([]string{"list"})
	if err != nil {
		t.Fatalf("list subcommand not found: %v", err)
	}
	if listCmd.Use != "list" {
		t.Fatalf("expected Use=list, got %s", listCmd.Use)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/dk/projects/indexer && go test ./go/cmd/carto/ -run TestProjectsListCmd -v`
Expected: FAIL — `projectsCmd` undefined.

**Step 3: Implement `projectsCmd` and `projectsListCmd`**

In `main.go`, add after the `serveCmd` function and register in `main()` with `root.AddCommand(projectsCmd())`:

```go
func projectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage indexed projects",
	}
	cmd.AddCommand(projectsListCmd())
	cmd.AddCommand(projectsShowCmd())
	cmd.AddCommand(projectsDeleteCmd())
	return cmd
}

func projectsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List indexed projects",
		RunE:  runProjectsList,
	}
}

func runProjectsList(cmd *cobra.Command, args []string) error {
	cfg := config.Load()
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	healthy, _ := memoriesClient.Health()
	_ = healthy

	// Scan the projects directory for manifests.
	projectsDir := cfg.ProjectsDir
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR not set — use --projects-dir or set the environment variable")
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("read projects dir: %w", err)
	}

	type projectRow struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		Files     int    `json:"file_count"`
		IndexedAt string `json:"indexed_at"`
	}

	var projects []projectRow
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(projectsDir, entry.Name())
		mf, err := manifest.Load(root)
		if err != nil || mf.IsEmpty() {
			continue
		}
		projects = append(projects, projectRow{
			Name:      mf.Project,
			Path:      root,
			Files:     len(mf.Files),
			IndexedAt: mf.IndexedAt.Format(time.RFC3339),
		})
	}

	writeOutput(cmd, projects, func() {
		if len(projects) == 0 {
			fmt.Println("No indexed projects found.")
			return
		}
		fmt.Printf("  %-25s %-6s %s\n", "NAME", "FILES", "INDEXED")
		fmt.Printf("  %-25s %-6s %s\n", strings.Repeat("-", 25), strings.Repeat("-", 6), strings.Repeat("-", 20))
		for _, p := range projects {
			fmt.Printf("  %-25s %-6d %s\n", p.Name, p.Files, p.IndexedAt)
		}
	})
	return nil
}
```

Note: Also add `projectsDir` to `config.Config` if not already there. Check `config.Load()` — if it doesn't have `ProjectsDir`, use `os.Getenv("PROJECTS_DIR")` directly in the command.

**Step 4: Run test to verify it passes**

Run: `cd /Users/dk/projects/indexer && go test ./go/cmd/carto/ -run TestProjectsListCmd -v`
Expected: PASS

**Step 5: Commit**

```bash
git add go/cmd/carto/main.go go/cmd/carto/main_test.go
git commit -m "feat(cli): add carto projects list command"
```

---

### Task 3: Add `carto projects show` and `carto projects delete` commands

**Files:**
- Modify: `go/cmd/carto/main.go`

**Step 1: Implement `projectsShowCmd` and `projectsDeleteCmd`**

```go
func projectsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show detailed project info",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectsShow,
	}
}

func runProjectsShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR not set")
	}

	root := filepath.Join(projectsDir, name)
	mf, err := manifest.Load(root)
	if err != nil || mf.IsEmpty() {
		return fmt.Errorf("project %q not found", name)
	}

	yamlCfg, _ := sources.LoadSourcesConfig(root)
	var srcNames []string
	if yamlCfg != nil {
		for k := range yamlCfg.Sources {
			srcNames = append(srcNames, k)
		}
		sort.Strings(srcNames)
	}

	type detail struct {
		Name      string   `json:"name"`
		Path      string   `json:"path"`
		Files     int      `json:"file_count"`
		IndexedAt string   `json:"indexed_at"`
		Sources   []string `json:"sources"`
	}

	d := detail{
		Name:      mf.Project,
		Path:      root,
		Files:     len(mf.Files),
		IndexedAt: mf.IndexedAt.Format(time.RFC3339),
		Sources:   srcNames,
	}

	writeOutput(cmd, d, func() {
		fmt.Printf("%s%sProject: %s%s\n", bold, cyan, d.Name, reset)
		fmt.Printf("  path:       %s\n", d.Path)
		fmt.Printf("  files:      %d\n", d.Files)
		fmt.Printf("  indexed at: %s\n", d.IndexedAt)
		if len(d.Sources) > 0 {
			fmt.Printf("  sources:    %s\n", strings.Join(d.Sources, ", "))
		}
	})
	return nil
}

func projectsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Remove a project's .carto directory",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectsDelete,
	}
}

func runProjectsDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR not set")
	}

	cartoDir := filepath.Join(projectsDir, name, ".carto")
	if _, err := os.Stat(cartoDir); os.IsNotExist(err) {
		return fmt.Errorf("project %q has no .carto directory", name)
	}

	if err := os.RemoveAll(cartoDir); err != nil {
		return fmt.Errorf("remove .carto: %w", err)
	}

	writeOutput(cmd, map[string]string{"status": "deleted", "project": name}, func() {
		fmt.Printf("%s✓%s Deleted index for project %q\n", green, reset, name)
	})
	return nil
}
```

**Step 2: Run `go build -o carto ./cmd/carto` to verify it compiles**

**Step 3: Commit**

```bash
git add go/cmd/carto/main.go
git commit -m "feat(cli): add carto projects show and delete commands"
```

---

### Task 4: Add `carto sources` commands

**Files:**
- Modify: `go/cmd/carto/main.go`

**Step 1: Implement `sourcesCmd` with `list`, `set`, and `rm` subcommands**

Register in `main()` with `root.AddCommand(sourcesCmd())`:

```go
func sourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage project source configuration",
	}
	cmd.AddCommand(sourcesListCmd())
	cmd.AddCommand(sourcesSetCmd())
	cmd.AddCommand(sourcesRmCmd())
	return cmd
}

func sourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <project>",
		Short: "Show configured sources for a project",
		Args:  cobra.ExactArgs(1),
		RunE:  runSourcesList,
	}
}

func runSourcesList(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR not set")
	}
	root := filepath.Join(projectsDir, args[0])
	yamlCfg, err := sources.LoadSourcesConfig(root)
	if err != nil {
		return fmt.Errorf("load sources: %w", err)
	}

	type sourceEntry struct {
		Type     string            `json:"type"`
		Settings map[string]string `json:"settings"`
	}
	var entries []sourceEntry
	if yamlCfg != nil {
		for k, v := range yamlCfg.Sources {
			entries = append(entries, sourceEntry{Type: k, Settings: v.Settings})
		}
	}

	writeOutput(cmd, entries, func() {
		if len(entries) == 0 {
			fmt.Println("No sources configured.")
			return
		}
		for _, e := range entries {
			fmt.Printf("  %s%s%s\n", bold, e.Type, reset)
			for k, v := range e.Settings {
				fmt.Printf("    %s: %s\n", k, v)
			}
		}
	})
	return nil
}

func sourcesSetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set <project> <type> --key=val ...",
		Short: "Add or update a source",
		Args:  cobra.ExactArgs(2),
		RunE:  runSourcesSet,
	}
	// Dynamic key-value flags parsed from remaining args.
	return cmd
}

func runSourcesSet(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR not set")
	}
	projectName := args[0]
	srcType := args[1]
	root := filepath.Join(projectsDir, projectName)

	// Parse remaining key=value pairs from args after --.
	settings := map[string]string{}
	for _, a := range cmd.Flags().Args() {
		if parts := strings.SplitN(a, "=", 2); len(parts) == 2 {
			settings[parts[0]] = parts[1]
		}
	}
	// Also parse from flag args (--key=val).
	cmd.Flags().Visit(func(f *pflag.Flag) {
		if f.Name != "json" && f.Name != "quiet" {
			settings[f.Name] = f.Value.String()
		}
	})

	yamlCfg, _ := sources.LoadSourcesConfig(root)
	if yamlCfg == nil {
		yamlCfg = &sources.SourcesConfig{Sources: map[string]sources.SourceEntry{}}
	}
	existing := yamlCfg.Sources[srcType]
	if existing.Settings == nil {
		existing.Settings = map[string]string{}
	}
	for k, v := range settings {
		existing.Settings[k] = v
	}
	yamlCfg.Sources[srcType] = existing

	if err := sources.SaveSourcesConfig(root, yamlCfg); err != nil {
		return fmt.Errorf("save sources: %w", err)
	}

	writeOutput(cmd, map[string]string{"status": "saved", "source": srcType}, func() {
		fmt.Printf("%s✓%s Source %q updated for %s\n", green, reset, srcType, projectName)
	})
	return nil
}

func sourcesRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <project> <type>",
		Short: "Remove a source",
		Args:  cobra.ExactArgs(2),
		RunE:  runSourcesRm,
	}
}

func runSourcesRm(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR not set")
	}
	projectName := args[0]
	srcType := args[1]
	root := filepath.Join(projectsDir, projectName)

	yamlCfg, _ := sources.LoadSourcesConfig(root)
	if yamlCfg == nil {
		return fmt.Errorf("no sources configured for %q", projectName)
	}
	delete(yamlCfg.Sources, srcType)

	if err := sources.SaveSourcesConfig(root, yamlCfg); err != nil {
		return fmt.Errorf("save sources: %w", err)
	}

	writeOutput(cmd, map[string]string{"status": "removed", "source": srcType}, func() {
		fmt.Printf("%s✓%s Source %q removed from %s\n", green, reset, srcType, projectName)
	})
	return nil
}
```

Note: This requires a `sources.SaveSourcesConfig(root, cfg)` function. If it doesn't exist, implement it in `go/internal/sources/config.go` — it should write YAML to `.carto/sources.yaml` using the same sorted-key pattern as `handlePutSources`.

**Step 2: Run `go build -o carto ./cmd/carto` to verify it compiles**

**Step 3: Commit**

```bash
git add go/cmd/carto/main.go go/internal/sources/config.go
git commit -m "feat(cli): add carto sources list/set/rm commands"
```

---

### Task 5: Add `carto config get/set` commands

**Files:**
- Modify: `go/cmd/carto/main.go`

**Step 1: Implement `configCmd` with `get` and `set` subcommands**

Register in `main()` with `root.AddCommand(configCmdGroup())` (rename to avoid collision with `config` package):

```go
func configCmdGroup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage server configuration",
	}
	cmd.AddCommand(configGetCmd())
	cmd.AddCommand(configSetCmd())
	return cmd
}

func configGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Show config (all or one key)",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runConfigGet,
	}
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	cfg := config.Load()
	// Build a map of all config values.
	all := map[string]any{
		"llm_provider":   cfg.LLMProvider,
		"fast_model":     cfg.FastModel,
		"deep_model":     cfg.DeepModel,
		"max_concurrent": cfg.MaxConcurrent,
		"memories_url":   cfg.MemoriesURL,
		"llm_base_url":   cfg.LLMBaseURL,
	}

	if len(args) == 1 {
		key := args[0]
		val, ok := all[key]
		if !ok {
			return fmt.Errorf("unknown config key: %s", key)
		}
		writeOutput(cmd, map[string]any{key: val}, func() {
			fmt.Printf("%s: %v\n", key, val)
		})
		return nil
	}

	writeOutput(cmd, all, func() {
		for k, v := range all {
			fmt.Printf("  %-18s %v\n", k, v)
		}
	})
	return nil
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a config value",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key, value := args[0], args[1]
	cfg := config.Load()

	switch key {
	case "llm_provider":
		cfg.LLMProvider = value
	case "fast_model":
		cfg.FastModel = value
	case "deep_model":
		cfg.DeepModel = value
	case "memories_url":
		cfg.MemoriesURL = value
	case "llm_base_url":
		cfg.LLMBaseURL = value
	case "llm_api_key":
		cfg.LLMApiKey = value
	case "anthropic_key":
		cfg.AnthropicKey = value
	case "memories_key":
		cfg.MemoriesKey = value
	case "github_token":
		cfg.GitHubToken = value
	case "jira_token":
		cfg.JiraToken = value
	case "jira_email":
		cfg.JiraEmail = value
	case "jira_base_url":
		cfg.JiraBaseURL = value
	case "linear_token":
		cfg.LinearToken = value
	case "notion_token":
		cfg.NotionToken = value
	case "slack_token":
		cfg.SlackToken = value
	default:
		return fmt.Errorf("unknown config key: %s", key)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	writeOutput(cmd, map[string]string{"status": "updated", "key": key}, func() {
		fmt.Printf("%s✓%s %s updated\n", green, reset, key)
	})
	return nil
}
```

**Step 2: Run `go build -o carto ./cmd/carto`**

**Step 3: Commit**

```bash
git add go/cmd/carto/main.go
git commit -m "feat(cli): add carto config get/set commands"
```

---

### Task 6: Add `carto index --all` and `--changed` flags

**Files:**
- Modify: `go/cmd/carto/main.go` (extend `indexCmd`)

**Step 1: Add `--all` and `--changed` flags to `indexCmd`**

```go
// In indexCmd():
cmd.Flags().Bool("all", false, "Re-index all projects")
cmd.Flags().Bool("changed", false, "Re-index only modified projects")
cmd.Args = cobra.MaximumNArgs(1) // path is optional when --all is used
```

**Step 2: Extend `runIndex` to handle batch mode**

At the top of `runIndex`, before the single-project logic:

```go
allFlag, _ := cmd.Flags().GetBool("all")
changedFlag, _ := cmd.Flags().GetBool("changed")

if allFlag || changedFlag {
	return runIndexAll(cmd, changedFlag)
}
if len(args) == 0 {
	return fmt.Errorf("path is required (or use --all/--changed)")
}
```

Implement `runIndexAll`:

```go
func runIndexAll(cmd *cobra.Command, changedOnly bool) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR not set")
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("read projects dir: %w", err)
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	var indexed []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		root := filepath.Join(projectsDir, entry.Name())
		mf, err := manifest.Load(root)
		if err != nil || mf.IsEmpty() {
			continue
		}

		if changedOnly {
			changed, _ := manifest.HasChanges(root, mf)
			if !changed {
				continue
			}
		}

		if !quiet {
			fmt.Printf("%s→%s Indexing %s...\n", cyan, reset, mf.Project)
		}

		// Run the index pipeline for this project.
		// (Reuse the single-project logic from runIndex, extract as needed.)
		indexed = append(indexed, mf.Project)
	}

	writeOutput(cmd, map[string]any{"indexed": indexed, "count": len(indexed)}, func() {
		fmt.Printf("\n%s✓%s Indexed %d project(s)\n", green, reset, len(indexed))
	})
	return nil
}
```

Note: If `manifest.HasChanges` doesn't exist, you'll need to implement it. It should compare the manifest's file hashes against current file hashes on disk.

**Step 3: Run `go build -o carto ./cmd/carto`**

**Step 4: Commit**

```bash
git add go/cmd/carto/main.go
git commit -m "feat(cli): add --all and --changed flags to carto index"
```

---

### Task 7: Wire `--json` into existing commands

**Files:**
- Modify: `go/cmd/carto/main.go` (update `runQuery`, `runModules`, `runStatus`)

**Step 1: Update `runQuery` to respect `--json`**

Replace the final output section of `runQuery` with a `writeOutput` call:

```go
writeOutput(cmd, results, func() {
	// Existing human-readable output logic...
})
```

Do the same for `runModules` and `runStatus`.

**Step 2: Run tests**

Run: `cd /Users/dk/projects/indexer && go test ./go/cmd/carto/ -v`
Expected: PASS

**Step 3: Commit**

```bash
git add go/cmd/carto/main.go
git commit -m "feat(cli): wire --json output into query, modules, status commands"
```

---

### Task 8: Add new API endpoints

**Files:**
- Modify: `go/internal/server/routes.go` (add 3 routes)
- Modify: `go/internal/server/handlers.go` (add 3 handlers)
- Test: `go/internal/server/server_test.go` (add tests)

**Step 1: Write failing tests**

Add to `server_test.go`:

```go
func TestGetProjectDetail(t *testing.T) {
	// Create temp dir with a project that has a manifest.
	// GET /api/projects/{name}
	// Expect 200 with JSON containing name, path, file_count, indexed_at, sources.
}

func TestDeleteProject(t *testing.T) {
	// Create temp dir with .carto/manifest.json.
	// DELETE /api/projects/{name}
	// Expect 200 and .carto directory removed.
}

func TestIndexAll(t *testing.T) {
	// POST /api/projects/index-all
	// Expect 202 Accepted.
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/dk/projects/indexer && go test ./go/internal/server/ -run "TestGetProjectDetail|TestDeleteProject|TestIndexAll" -v`

**Step 3: Add routes in `routes.go`**

```go
s.mux.HandleFunc("GET /api/projects/{name}", s.handleGetProject)
s.mux.HandleFunc("DELETE /api/projects/{name}", s.handleDeleteProject)
s.mux.HandleFunc("POST /api/projects/index-all", s.handleIndexAll)
```

Note: The `GET /api/projects/{name}` route must be registered AFTER more specific routes like `GET /api/projects/{name}/sources` and `GET /api/projects/{name}/progress` to avoid conflicts with Go 1.22+ ServeMux pattern matching.

**Step 4: Implement handlers in `handlers.go`**

```go
// handleGetProject returns detailed info about a single project.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	projPath := filepath.Join(s.projectsDir, name)

	mf, err := manifest.Load(projPath)
	if err != nil || mf.IsEmpty() {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	yamlCfg, _ := sources.LoadSourcesConfig(projPath)
	var srcNames []string
	if yamlCfg != nil {
		for k := range yamlCfg.Sources {
			srcNames = append(srcNames, k)
		}
		sort.Strings(srcNames)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"name":       mf.Project,
		"path":       projPath,
		"file_count": len(mf.Files),
		"indexed_at": mf.IndexedAt,
		"sources":    srcNames,
	})
}

// handleDeleteProject removes the .carto directory for a project.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	cartoDir := filepath.Join(s.projectsDir, name, ".carto")

	if _, err := os.Stat(cartoDir); os.IsNotExist(err) {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	if err := os.RemoveAll(cartoDir); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// handleIndexAll triggers re-indexing of all (or changed) projects.
func (s *Server) handleIndexAll(w http.ResponseWriter, r *http.Request) {
	changedOnly := r.URL.Query().Get("changed") == "true"
	// TODO: Implement batch indexing logic.
	writeJSON(w, http.StatusAccepted, map[string]any{
		"status":       "started",
		"changed_only": changedOnly,
	})
}
```

**Step 5: Run tests to verify they pass**

Run: `cd /Users/dk/projects/indexer && go test ./go/internal/server/ -v`

**Step 6: Commit**

```bash
git add go/internal/server/routes.go go/internal/server/handlers.go go/internal/server/server_test.go
git commit -m "feat(api): add GET/DELETE /api/projects/{name} and POST /api/projects/index-all"
```

---

### Task 9: Add `sources.SaveSourcesConfig` helper

**Files:**
- Modify: `go/internal/sources/config.go`
- Test: `go/internal/sources/config_test.go`

**Step 1: Write failing test**

```go
func TestSaveSourcesConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &SourcesConfig{
		Sources: map[string]SourceEntry{
			"github": {Settings: map[string]string{"owner": "test", "repo": "app"}},
			"jira":   {Settings: map[string]string{"project": "PROJ"}},
		},
	}
	if err := SaveSourcesConfig(dir, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	// Reload and verify round-trip.
	loaded, err := LoadSourcesConfig(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Sources["github"].Settings["owner"] != "test" {
		t.Fatalf("expected owner=test")
	}
}
```

**Step 2: Run test to verify it fails**

**Step 3: Implement `SaveSourcesConfig`**

```go
// SaveSourcesConfig writes the sources config to .carto/sources.yaml.
func SaveSourcesConfig(projectRoot string, cfg *SourcesConfig) error {
	cartoDir := filepath.Join(projectRoot, ".carto")
	if err := os.MkdirAll(cartoDir, 0o755); err != nil {
		return fmt.Errorf("create .carto dir: %w", err)
	}

	var buf bytes.Buffer
	buf.WriteString("sources:\n")
	names := make([]string, 0, len(cfg.Sources))
	for k := range cfg.Sources {
		names = append(names, k)
	}
	sort.Strings(names)
	for _, name := range names {
		entry := cfg.Sources[name]
		buf.WriteString("  " + name + ":\n")
		keys := make([]string, 0, len(entry.Settings))
		for k := range entry.Settings {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			buf.WriteString("    " + k + ": " + entry.Settings[k] + "\n")
		}
	}

	return os.WriteFile(filepath.Join(cartoDir, "sources.yaml"), buf.Bytes(), 0o644)
}
```

**Step 4: Run tests**

Run: `cd /Users/dk/projects/indexer && go test ./go/internal/sources/ -run TestSaveSourcesConfig -v`

**Step 5: Commit**

```bash
git add go/internal/sources/config.go go/internal/sources/config_test.go
git commit -m "feat(sources): add SaveSourcesConfig for CLI write support"
```

---

### Task 10: Create `pkg/carto` SDK package

**Files:**
- Create: `go/pkg/carto/carto.go`
- Test: `go/pkg/carto/carto_test.go`

**Step 1: Write the test**

```go
package carto

import (
	"testing"
)

func TestIndexOptionsDefaults(t *testing.T) {
	opts := IndexOptions{}
	if opts.Incremental {
		t.Fatal("expected incremental=false by default")
	}
}

func TestQueryOptionsDefaults(t *testing.T) {
	opts := QueryOptions{}
	if opts.K != 0 {
		t.Fatal("expected K=0 by default (caller sets)")
	}
}
```

**Step 2: Run test to verify it fails**

**Step 3: Implement the package**

```go
// Package carto provides a thin Go SDK for programmatic access to Carto
// indexing and querying. It wraps the internal packages with a stable API.
package carto

import (
	"fmt"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/pipeline"
	"github.com/divyekant/carto/internal/sources"
	"github.com/divyekant/carto/internal/storage"
)

// IndexOptions configures an indexing run.
type IndexOptions struct {
	Incremental bool
	Module      string
	Project     string // defaults to directory name
}

// IndexResult contains the output of an indexing run.
type IndexResult struct {
	Modules int
	Files   int
	Atoms   int
	Errors  int
}

// Index runs the Carto indexing pipeline on the given path.
func Index(path string, opts IndexOptions) (*IndexResult, error) {
	cfg := config.Load()
	apiKey := cfg.LLMApiKey
	if apiKey == "" {
		apiKey = cfg.AnthropicKey
	}
	if apiKey == "" && cfg.LLMProvider != "ollama" {
		return nil, fmt.Errorf("no API key set")
	}

	llmClient := llm.NewClient(llm.Options{
		APIKey:        apiKey,
		FastModel:     cfg.FastModel,
		DeepModel:     cfg.DeepModel,
		MaxConcurrent: cfg.MaxConcurrent,
		BaseURL:       cfg.LLMBaseURL,
	})

	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	registry := sources.NewRegistry()
	registry.Register(sources.NewGitSource(path))

	projectName := opts.Project
	if projectName == "" {
		projectName = filepath.Base(path)
	}

	result, err := pipeline.Run(pipeline.Config{
		ProjectName:    projectName,
		RootPath:       path,
		LLMClient:      llmClient,
		MemoriesClient: memoriesClient,
		SourceRegistry: registry,
		MaxWorkers:     cfg.MaxConcurrent,
		Incremental:    opts.Incremental,
		ModuleFilter:   opts.Module,
	})
	if err != nil {
		return nil, err
	}

	return &IndexResult{
		Modules: result.Modules,
		Files:   result.FilesIndexed,
		Atoms:   result.AtomsCreated,
		Errors:  len(result.Errors),
	}, nil
}

// QueryOptions configures a query.
type QueryOptions struct {
	Project string
	Tier    string // mini, standard, full
	K       int
}

// QueryResult is a single search result.
type QueryResult struct {
	Text   string
	Source string
	Score  float64
}

// Query searches the Carto index.
func Query(text string, opts QueryOptions) ([]QueryResult, error) {
	cfg := config.Load()
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	if opts.K == 0 {
		opts.K = 10
	}

	searchOpts := storage.SearchOptions{K: opts.K, Hybrid: true}
	if opts.Project != "" {
		searchOpts.Source = fmt.Sprintf("carto/%s/", opts.Project)
		searchOpts.K = opts.K * 3
	}

	results, err := memoriesClient.Search(text, searchOpts)
	if err != nil {
		return nil, err
	}

	var out []QueryResult
	for _, r := range results {
		out = append(out, QueryResult{
			Text:   r.Text,
			Source: r.Source,
			Score:  r.Score,
		})
		if len(out) >= opts.K {
			break
		}
	}
	return out, nil
}

// Sources returns the configured sources for a project.
func Sources(projectName string) (map[string]map[string]string, error) {
	projectsDir := config.Load().ProjectsDir
	if projectsDir == "" {
		return nil, fmt.Errorf("PROJECTS_DIR not set")
	}
	root := filepath.Join(projectsDir, projectName)
	yamlCfg, err := sources.LoadSourcesConfig(root)
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]string{}
	if yamlCfg != nil {
		for k, v := range yamlCfg.Sources {
			result[k] = v.Settings
		}
	}
	return result, nil
}
```

**Step 4: Run tests**

Run: `cd /Users/dk/projects/indexer && go test ./go/pkg/carto/ -v`

**Step 5: Commit**

```bash
git add go/pkg/carto/carto.go go/pkg/carto/carto_test.go
git commit -m "feat(sdk): add pkg/carto thin SDK with Index, Query, Sources"
```

---

## Stream 2: UI Dense Redesign

### Task 11: Install shadcn Table, Switch, and Tooltip components

**Files:**
- Create: `go/web/src/components/ui/table.tsx`
- Create: `go/web/src/components/ui/switch.tsx`
- Create: `go/web/src/components/ui/tooltip.tsx`

**Step 1: Install the three components**

Run from `go/web/`:

```bash
cd /Users/dk/projects/indexer/go/web && npx shadcn@latest add table switch tooltip
```

**Step 2: Verify files were created**

Check that `src/components/ui/table.tsx`, `switch.tsx`, and `tooltip.tsx` exist.

**Step 3: Run `npm run build` to verify it compiles**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`
Expected: Build succeeds.

**Step 4: Commit**

```bash
git add go/web/src/components/ui/table.tsx go/web/src/components/ui/switch.tsx go/web/src/components/ui/tooltip.tsx go/web/package.json go/web/package-lock.json
git commit -m "feat(ui): install shadcn Table, Switch, Tooltip components"
```

---

### Task 12: Redesign Layout — collapsible icon-only sidebar

**Files:**
- Modify: `go/web/src/components/Layout.tsx`

**Step 1: Rewrite Layout.tsx**

Replace the current sidebar (w-56, always expanded) with an icon-only sidebar (w-12, 48px) that expands to w-48 on hover. Use `group` + `group-hover:` Tailwind utilities:

Key changes:
- Sidebar: `w-12 hover:w-48 transition-all duration-200 group`
- Logo: Show icon only by default, expand to "Carto" on hover
- Nav items: Icon-only by default, show label on `group-hover:opacity-100`
- Mobile: Keep existing drawer behavior unchanged
- Main content: `ml-12` instead of being flex-pushed by sidebar
- Remove the max-width constraints in main: use `p-4 md:p-5` (tighter than `md:p-6`)
- Theme toggle: Show in sidebar footer, icon-only

Specific implementation:

```tsx
// Sidebar: icon-only, expand on hover
<aside
  className={cn(
    'fixed inset-y-0 left-0 z-50 border-r border-border bg-sidebar flex flex-col transition-all duration-200 overflow-hidden',
    'w-12 hover:w-48 group/sidebar',
    // Mobile: full-width drawer
    'md:static',
    mobileOpen ? 'w-48 translate-x-0' : '-translate-x-full md:translate-x-0'
  )}
>
  <div className="p-2 border-b border-border h-12 flex items-center">
    <span className="text-lg font-bold text-primary shrink-0">C</span>
    <span className="ml-1 text-sm font-bold text-primary opacity-0 group-hover/sidebar:opacity-100 transition-opacity whitespace-nowrap">arto</span>
  </div>
  <nav className="flex-1 p-1 space-y-0.5">
    {navItems.map((item) => (
      <NavLink
        key={item.to}
        to={item.to}
        end={item.to === '/'}
        className={({ isActive }) =>
          cn(
            'flex items-center gap-2 px-2.5 py-2 rounded-md text-sm transition-colors',
            isActive
              ? 'bg-primary/10 text-primary'
              : 'text-muted-foreground hover:text-foreground hover:bg-accent'
          )
        }
      >
        <span className="text-base shrink-0">{item.icon}</span>
        <span className="opacity-0 group-hover/sidebar:opacity-100 transition-opacity whitespace-nowrap text-xs">
          {item.label}
        </span>
      </NavLink>
    ))}
  </nav>
  <div className="p-1 border-t border-border">
    <ThemeToggle />
  </div>
</aside>

{/* Main content — leave room for icon sidebar */}
<main className="flex-1 overflow-y-auto p-3 pt-14 md:p-5 md:pt-5 md:ml-12">
  <Outlet />
</main>
```

**Step 2: Build and verify**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`

**Step 3: Commit**

```bash
git add go/web/src/components/Layout.tsx
git commit -m "feat(ui): redesign Layout with icon-only collapsible sidebar"
```

---

### Task 13: Redesign Dashboard — data table replaces card grid

**Files:**
- Modify: `go/web/src/pages/Dashboard.tsx`
- The `ProjectCard` component can be left in place but is no longer imported.

**Step 1: Rewrite Dashboard.tsx**

Replace the card grid with a shadcn `Table`. Key changes:

- Remove `ProjectCard` import
- Import `Table, TableBody, TableCell, TableHead, TableHeader, TableRow` from `@/components/ui/table`
- Header bar: project count left, Memories badge + "Index New" button right — all on one row, compact
- Data table columns: Name, Path (truncated), Files, Sources, Last Indexed, Status, Actions (re-index button)
- Clickable rows → navigate to project detail
- Sortable columns (optional, can be done later)
- Text sizes: `text-xs` for table cells, `text-sm` for headers
- Remove `text-2xl` heading, use `text-lg font-semibold` instead
- Remove `mb-6` gaps, use `mb-3`

```tsx
// Compact header
<div className="flex items-center justify-between mb-3">
  <div>
    <h2 className="text-lg font-semibold">Dashboard</h2>
    <p className="text-xs text-muted-foreground">{projects.length} projects</p>
  </div>
  <div className="flex items-center gap-2">
    {health && (
      <Badge variant={health.memories_healthy ? 'default' : 'destructive'} className="text-xs">
        {health.memories_healthy ? 'Memories ✓' : 'Memories ✗'}
      </Badge>
    )}
    <Button size="sm" onClick={() => navigate('/index')}>Index New</Button>
  </div>
</div>

// Data table
<Table>
  <TableHeader>
    <TableRow>
      <TableHead className="text-xs">Name</TableHead>
      <TableHead className="text-xs">Path</TableHead>
      <TableHead className="text-xs w-16">Files</TableHead>
      <TableHead className="text-xs w-24">Last Indexed</TableHead>
      <TableHead className="text-xs w-20">Status</TableHead>
    </TableRow>
  </TableHeader>
  <TableBody>
    {projects.map((p) => {
      const run = runStatuses[p.name]
      return (
        <TableRow
          key={p.name}
          className="cursor-pointer hover:bg-muted/50"
          onClick={() => navigate(`/projects/${encodeURIComponent(p.name)}`)}
        >
          <TableCell className="text-sm font-medium">{p.name}</TableCell>
          <TableCell className="text-xs text-muted-foreground truncate max-w-[200px]" title={p.path}>{p.path}</TableCell>
          <TableCell className="text-xs">{p.file_count}</TableCell>
          <TableCell className="text-xs text-muted-foreground">{getTimeAgo(p.indexed_at)}</TableCell>
          <TableCell>
            {run?.status === 'running' && <Badge variant="secondary" className="text-xs">Running</Badge>}
            {run?.status === 'error' && <Badge variant="destructive" className="text-xs">Error</Badge>}
            {(!run || run.status === 'complete') && <Badge variant="default" className="text-xs">Indexed</Badge>}
          </TableCell>
        </TableRow>
      )
    })}
  </TableBody>
</Table>
```

Move the `getTimeAgo` function from `ProjectCard.tsx` into Dashboard (or a shared utils file).

**Step 2: Build and verify**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`

**Step 3: Commit**

```bash
git add go/web/src/pages/Dashboard.tsx
git commit -m "feat(ui): redesign Dashboard with data table, kill card grid"
```

---

### Task 14: Redesign Index page — compact single-row form

**Files:**
- Modify: `go/web/src/pages/IndexRun.tsx`

**Step 1: Rewrite the idle state form**

Replace the card-wrapped form with a compact inline layout:

- Single row: path/URL input (flex-1) + options (incremental toggle, module filter) + Start button — all inline
- Use shadcn `Switch` for incremental toggle instead of raw checkbox
- Below the form (when running): progress bar left (flex-1), scrolling log right (flex-1) — side-by-side using `flex gap-3`
- Remove `max-w-lg` and `max-w-2xl` constraints
- Remove wrapper `Card` around the idle form
- Text sizes: `text-sm` for labels, `text-xs` for sub-labels
- Page heading: `text-lg font-semibold mb-3` (not `text-2xl mb-6`)

Key layout when running:
```tsx
<div className="flex gap-3">
  {/* Left: progress */}
  <div className="flex-1 min-w-0">
    <ProgressBar ... />
    {result && <ResultSummary ... />}
  </div>
  {/* Right: log */}
  <div className="flex-1 min-w-0 bg-muted/50 rounded-md p-2 max-h-80 overflow-y-auto font-mono text-xs">
    {logs.map(...)}
  </div>
</div>
```

**Step 2: Build and verify**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`

**Step 3: Commit**

```bash
git add go/web/src/pages/IndexRun.tsx
git commit -m "feat(ui): redesign Index page with compact inline form and side-by-side layout"
```

---

### Task 15: Redesign ProjectDetail — two-column layout

**Files:**
- Modify: `go/web/src/pages/ProjectDetail.tsx`
- Modify: `go/web/src/components/SourcesEditor.tsx`

**Step 1: Rewrite ProjectDetail.tsx for two-column layout**

- Two-column layout: `grid grid-cols-1 lg:grid-cols-2 gap-3`
- Left column: Sources (compact toggles with inline fields)
- Right column: Index controls + last run summary
- Remove `max-w-2xl` constraint
- Replace the spacious `space-y-6` with `gap-3`
- Heading: `text-lg font-semibold` with back arrow and badge, `mb-3`
- Everything should fit on one screen without scrolling

**Step 2: Rewrite SourcesEditor.tsx to use shadcn Switch**

Replace custom toggle buttons with `<Switch>` from shadcn:

```tsx
import { Switch } from '@/components/ui/switch'

// Replace the custom button toggle with:
<Switch checked={isEnabled} onCheckedChange={() => toggleSource(def.key)} />
```

Also make each source row more compact:
- Remove `p-4` padding, use `p-2`
- Remove `mb-3` gap, use `mb-1`
- Fields grid: remove `gap-3`, use `gap-2`
- Field labels: `text-xs`

**Step 3: Build and verify**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`

**Step 4: Commit**

```bash
git add go/web/src/pages/ProjectDetail.tsx go/web/src/components/SourcesEditor.tsx
git commit -m "feat(ui): redesign ProjectDetail two-column + SourcesEditor with shadcn Switch"
```

---

### Task 16: Redesign Query page — compact search + list results

**Files:**
- Modify: `go/web/src/pages/Query.tsx`
- Modify: `go/web/src/components/QueryResult.tsx`

**Step 1: Rewrite Query.tsx**

- Search bar pinned at top with all filters on same row: input (flex-1) + project select + tier buttons + count input + search button
- Remove `max-w-2xl` and `mb-6` spacing
- Remove `space-y-4` between filter groups — put everything on one row
- Heading: `text-lg font-semibold mb-3`

**Step 2: Rewrite QueryResult.tsx**

Replace cards with compact list rows:
- Remove `Card` wrapper entirely
- Use a `div` with `border-b border-border py-2` for each result
- Layout: source left (mono, truncated), score as thin colored bar (inline), truncated text right
- Click to expand full text
- Text: `text-xs` throughout

```tsx
<div
  className="flex items-start gap-3 py-2 border-b border-border cursor-pointer hover:bg-muted/30"
  onClick={() => setExpanded(!expanded)}
>
  <span className="text-xs font-mono text-muted-foreground shrink-0 w-6">{index}.</span>
  <span className="text-xs font-mono truncate max-w-[200px]" title={source}>{source}</span>
  <div className="w-12 shrink-0">
    <div className="h-1.5 bg-muted rounded-full overflow-hidden">
      <div className="h-full bg-primary rounded-full" style={{ width: `${Math.min(score * 100, 100)}%` }} />
    </div>
  </div>
  <pre className="text-xs text-muted-foreground flex-1 truncate whitespace-pre-wrap">{preview}</pre>
</div>
```

**Step 3: Build and verify**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`

**Step 4: Commit**

```bash
git add go/web/src/pages/Query.tsx go/web/src/components/QueryResult.tsx
git commit -m "feat(ui): redesign Query page with inline filters and compact result rows"
```

---

### Task 17: Redesign Settings — two-column grid, one screen

**Files:**
- Modify: `go/web/src/pages/Settings.tsx`

**Step 1: Rewrite Settings.tsx**

- Two-column grid: `grid grid-cols-1 lg:grid-cols-2 gap-3`
- Left column: LLM config (provider, API key, models, concurrency)
- Right column: Connections (Memories server + all integrations)
- Remove `max-w-2xl` constraint
- Remove individual `Card` wrappers — use lighter section dividers (`border-b border-border pb-3`)
- Compact all inputs: reduce `space-y-4` to `space-y-2`
- Heading: `text-lg font-semibold mb-3`
- Integration fields: group all on right column with minimal spacing
- One screen, no scrolling on desktop

**Step 2: Build and verify**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`

**Step 3: Commit**

```bash
git add go/web/src/pages/Settings.tsx
git commit -m "feat(ui): redesign Settings as two-column grid, remove cards"
```

---

### Task 18: Remove global width constraints and tighten typography

**Files:**
- Modify: Any remaining files with `max-w-2xl`, `max-w-lg`, `text-2xl`
- Modify: `go/web/src/index.css` (if global typography overrides needed)

**Step 1: Search and remove width constraints**

Search all `.tsx` files for `max-w-2xl`, `max-w-lg`, `max-w-md` and remove them. Also replace any remaining `text-2xl` headings with `text-lg font-semibold`.

**Step 2: Verify no remaining constraints**

Run: `grep -r "max-w-2xl\|max-w-lg\|max-w-md" go/web/src/`
Expected: No results.

**Step 3: Full build + visual check**

Run: `cd /Users/dk/projects/indexer/go/web && npm run build`
Then start the server and visually verify all pages in the browser.

**Step 4: Commit**

```bash
git add -A go/web/src/
git commit -m "refactor(ui): remove all width constraints, tighten typography"
```

---

### Task 19: Final integration test — build Go binary + Docker deploy

**Files:**
- No new files — validation only

**Step 1: Run all Go tests**

```bash
cd /Users/dk/projects/indexer && go test ./... -short
```
Expected: All PASS.

**Step 2: Build the Go binary**

```bash
cd /Users/dk/projects/indexer && go build -o carto ./go/cmd/carto
```
Expected: Builds without error.

**Step 3: Test CLI commands**

```bash
./carto --version
./carto projects list --json 2>&1 || true
./carto config get --json 2>&1 || true
```

**Step 4: Build and deploy Docker**

```bash
cd /Users/dk/projects/indexer && docker compose build && docker compose up -d
```

**Step 5: Verify web UI**

Open `http://localhost:8950` and verify:
- Dashboard shows data table
- Sidebar is icon-only, expands on hover
- All pages use dense layout without wasted space
- Query page has inline filters
- Settings fits on one screen

**Step 6: Commit any final fixes**

```bash
git add -A && git commit -m "chore: final integration fixes for CLI/API + dense UI"
```
