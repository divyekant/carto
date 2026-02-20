package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/manifest"
	"github.com/divyekant/carto/internal/patterns"
	"github.com/divyekant/carto/internal/pipeline"
	"github.com/divyekant/carto/internal/scanner"
	"github.com/divyekant/carto/internal/server"
	"github.com/divyekant/carto/internal/sources"
	"github.com/divyekant/carto/internal/storage"
	cartoWeb "github.com/divyekant/carto/web"
)

var version = "0.3.0"

// ANSI escape codes for colored output.
const (
	bold  = "\033[1m"
	green = "\033[32m"
	yellow = "\033[33m"
	cyan  = "\033[36m"
	red   = "\033[31m"
	reset = "\033[0m"
)

// spinner frames for progress display.
var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

func main() {
	root := &cobra.Command{
		Use:     "carto",
		Short:   "Carto -- intent-aware codebase intelligence",
		Version: version,
	}

	root.PersistentFlags().Bool("json", false, "Output machine-readable JSON")
	root.PersistentFlags().BoolP("quiet", "q", false, "Suppress progress spinners, only output result")

	root.AddCommand(indexCmd())
	root.AddCommand(queryCmd())
	root.AddCommand(modulesCmd())
	root.AddCommand(patternsCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(projectsCmd())
	root.AddCommand(sourcesCmd())
	root.AddCommand(configCmdGroup())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// --------------------------------------------------------------------------
// index
// --------------------------------------------------------------------------

func indexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index <path>",
		Short: "Index a codebase",
		Args:  cobra.ExactArgs(1),
		RunE:  runIndex,
	}
	cmd.Flags().Bool("full", false, "Force full re-index")
	cmd.Flags().String("module", "", "Index a single module")
	cmd.Flags().Bool("incremental", false, "Only re-index changed files")
	cmd.Flags().String("project", "", "Project name (defaults to directory name)")
	return cmd
}

func runIndex(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	cfg := config.Load()

	// Determine API key — LLM_API_KEY takes priority, falls back to ANTHROPIC_API_KEY.
	apiKey := cfg.LLMApiKey
	if apiKey == "" {
		apiKey = cfg.AnthropicKey
	}

	if apiKey == "" && cfg.LLMProvider != "ollama" {
		fmt.Fprintf(os.Stderr, "%serror:%s No API key set. Set LLM_API_KEY or ANTHROPIC_API_KEY.\n", red, reset)
		return fmt.Errorf("API key not set")
	}

	full, _ := cmd.Flags().GetBool("full")
	moduleFilter, _ := cmd.Flags().GetString("module")
	incremental, _ := cmd.Flags().GetBool("incremental")
	projectName, _ := cmd.Flags().GetString("project")

	if projectName == "" {
		projectName = filepath.Base(absPath)
	}

	// If --full is set, disable incremental mode.
	if full {
		incremental = false
	}

	// Create LLM client.
	llmClient := llm.NewClient(llm.Options{
		APIKey:        apiKey,
		FastModel:     cfg.FastModel,
		DeepModel:     cfg.DeepModel,
		MaxConcurrent: cfg.MaxConcurrent,
		IsOAuth:       config.IsOAuthToken(apiKey),
		BaseURL:       cfg.LLMBaseURL,
	})

	// Create Memories client.
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// Create unified source registry and register git source.
	registry := sources.NewRegistry()
	registry.Register(sources.NewGitSource(absPath))

	// Progress display state.
	spinIdx := 0
	startTime := time.Now()

	progressFn := func(phase string, done, total int) {
		frame := spinnerFrames[spinIdx%len(spinnerFrames)]
		spinIdx++
		if done >= total {
			fmt.Printf("\r%s%s%s %s [%d/%d]%s\n", green, "✓", reset, phase, done, total, reset)
		} else {
			fmt.Printf("\r%s%s%s %s [%d/%d]", cyan, frame, reset, phase, done, total)
		}
	}

	fmt.Printf("%s%sCarto indexing %s%s\n", bold, cyan, projectName, reset)
	fmt.Printf("  path: %s\n", absPath)
	if moduleFilter != "" {
		fmt.Printf("  module filter: %s\n", moduleFilter)
	}
	if incremental {
		fmt.Printf("  mode: incremental\n")
	} else if full {
		fmt.Printf("  mode: full\n")
	}
	fmt.Println()

	result, err := pipeline.Run(pipeline.Config{
		ProjectName:    projectName,
		RootPath:       absPath,
		LLMClient:      llmClient,
		MemoriesClient: memoriesClient,
		SourceRegistry: registry,
		MaxWorkers:     cfg.MaxConcurrent,
		ProgressFn:     progressFn,
		Incremental:    incremental,
		ModuleFilter:   moduleFilter,
	})
	if err != nil {
		return fmt.Errorf("pipeline failed: %w", err)
	}

	elapsed := time.Since(startTime)

	// Print summary.
	fmt.Println()
	fmt.Printf("%s%s=== Summary ===%s\n", bold, green, reset)
	fmt.Printf("  modules:  %d\n", result.Modules)
	fmt.Printf("  files:    %d\n", result.FilesIndexed)
	fmt.Printf("  atoms:    %d\n", result.AtomsCreated)
	fmt.Printf("  errors:   %d\n", len(result.Errors))
	fmt.Printf("  elapsed:  %s\n", elapsed.Round(time.Millisecond))

	if len(result.Errors) > 0 {
		fmt.Printf("\n%s%sWarnings:%s\n", bold, yellow, reset)
		for i, e := range result.Errors {
			if i >= 10 {
				fmt.Printf("  ... and %d more\n", len(result.Errors)-10)
				break
			}
			fmt.Printf("  - %v\n", e)
		}
	}

	return nil
}

// --------------------------------------------------------------------------
// query
// --------------------------------------------------------------------------

func queryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <question>",
		Short: "Query the indexed codebase",
		Args:  cobra.ExactArgs(1),
		RunE:  runQuery,
	}
	cmd.Flags().String("project", "", "Project name to search within")
	cmd.Flags().String("tier", "standard", "Context tier: mini, standard, full")
	cmd.Flags().IntP("count", "k", 10, "Number of results")
	return cmd
}

func runQuery(cmd *cobra.Command, args []string) error {
	query := args[0]

	project, _ := cmd.Flags().GetString("project")
	tier, _ := cmd.Flags().GetString("tier")
	count, _ := cmd.Flags().GetInt("count")

	cfg := config.Load()
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// If a project is provided, try tier-based retrieval.
	if project != "" {
		store := storage.NewStore(memoriesClient, project)

		storageTier := storage.Tier(tier)
		results, err := store.RetrieveByTier(query, storageTier)
		if err != nil {
			return fmt.Errorf("retrieve by tier: %w", err)
		}

		fmt.Printf("%s%sResults for project %q (tier: %s)%s\n\n", bold, cyan, project, tier, reset)

		for layer, entries := range results {
			if len(entries) == 0 {
				continue
			}
			fmt.Printf("%s%s[%s]%s\n", bold, yellow, layer, reset)
			for _, entry := range entries {
				snippet := truncateText(entry.Text, 200)
				fmt.Printf("  %ssource:%s %s\n", cyan, reset, entry.Source)
				fmt.Printf("  %sscore:%s  %.4f\n", cyan, reset, entry.Score)
				fmt.Printf("  %s\n\n", snippet)
			}
		}
		return nil
	}

	// Free-form search across all projects.
	results, err := memoriesClient.Search(query, storage.SearchOptions{
		K:      count,
		Hybrid: true,
	})
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	fmt.Printf("%s%sSearch results for: %q%s (k=%d)\n\n", bold, cyan, query, reset, count)

	if len(results) == 0 {
		fmt.Println("  No results found.")
		return nil
	}

	for i, r := range results {
		snippet := truncateText(r.Text, 200)
		fmt.Printf("%s%d.%s %ssource:%s %s  %sscore:%s %.4f\n", bold, i+1, reset, cyan, reset, r.Source, cyan, reset, r.Score)
		fmt.Printf("   %s\n\n", snippet)
	}

	return nil
}

// --------------------------------------------------------------------------
// modules
// --------------------------------------------------------------------------

func modulesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "modules <path>",
		Short: "List detected modules",
		Args:  cobra.ExactArgs(1),
		RunE:  runModules,
	}
}

func runModules(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	result, err := scanner.Scan(absPath)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	fmt.Printf("%s%sDetected modules in %s%s\n\n", bold, cyan, absPath, reset)

	if len(result.Modules) == 0 {
		fmt.Println("  No modules detected.")
		return nil
	}

	fmt.Printf("  %-30s %-15s %-40s %s\n", "NAME", "TYPE", "PATH", "FILES")
	fmt.Printf("  %-30s %-15s %-40s %s\n",
		strings.Repeat("-", 30),
		strings.Repeat("-", 15),
		strings.Repeat("-", 40),
		strings.Repeat("-", 6))

	for _, mod := range result.Modules {
		relPath := mod.RelPath
		if relPath == "" {
			relPath = "."
		}
		fmt.Printf("  %-30s %-15s %-40s %d\n", mod.Name, mod.Type, relPath, len(mod.Files))
	}

	fmt.Printf("\n  %sTotal:%s %d module(s), %d file(s)\n", bold, reset, len(result.Modules), len(result.Files))

	return nil
}

// --------------------------------------------------------------------------
// patterns
// --------------------------------------------------------------------------

func patternsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patterns <path>",
		Short: "Generate CLAUDE.md and .cursorrules",
		Args:  cobra.ExactArgs(1),
		RunE:  runPatterns,
	}
	cmd.Flags().String("format", "all", "Output format: claude, cursor, all")
	return cmd
}

func runPatterns(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	format, _ := cmd.Flags().GetString("format")

	cfg := config.Load()
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// Scan to discover modules.
	result, err := scanner.Scan(absPath)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	projectName := filepath.Base(absPath)

	// Try to load existing analysis from Memories.
	store := storage.NewStore(memoriesClient, projectName)

	// Build module summaries from scan.
	var moduleSummaries []patterns.ModuleSummary
	for _, mod := range result.Modules {
		moduleSummaries = append(moduleSummaries, patterns.ModuleSummary{
			Name:   mod.Name,
			Type:   mod.Type,
			Intent: "",
		})
	}

	// Attempt to retrieve stored blueprint and patterns.
	var blueprint string
	var pats []string
	var zones []patterns.Zone

	if blueprintResults, err := store.RetrieveLayer("_system", "blueprint"); err == nil && len(blueprintResults) > 0 {
		blueprint = blueprintResults[0].Text
	}

	if patResults, err := store.RetrieveLayer("_system", "patterns"); err == nil && len(patResults) > 0 {
		var parsed []string
		if jsonErr := json.Unmarshal([]byte(patResults[0].Text), &parsed); jsonErr == nil {
			pats = parsed
		}
	}

	// Retrieve zones from each module.
	for _, mod := range result.Modules {
		if zoneResults, err := store.RetrieveLayer(mod.Name, "zones"); err == nil && len(zoneResults) > 0 {
			var modZones []patterns.Zone
			if jsonErr := json.Unmarshal([]byte(zoneResults[0].Text), &modZones); jsonErr == nil {
				zones = append(zones, modZones...)
			}
		}
	}

	input := patterns.Input{
		ProjectName: projectName,
		Blueprint:   blueprint,
		Patterns:    pats,
		Zones:       zones,
		Modules:     moduleSummaries,
	}

	fmt.Printf("%s%sGenerating patterns for %s%s\n", bold, cyan, absPath, reset)
	fmt.Printf("  modules: %d, format: %s\n\n", len(result.Modules), format)

	if err := patterns.WriteFiles(absPath, input, format); err != nil {
		return fmt.Errorf("write patterns: %w", err)
	}

	switch format {
	case "claude":
		fmt.Printf("  %s✓%s CLAUDE.md\n", green, reset)
	case "cursor":
		fmt.Printf("  %s✓%s .cursorrules\n", green, reset)
	default:
		fmt.Printf("  %s✓%s CLAUDE.md\n", green, reset)
		fmt.Printf("  %s✓%s .cursorrules\n", green, reset)
	}

	return nil
}

// --------------------------------------------------------------------------
// status
// --------------------------------------------------------------------------

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <path>",
		Short: "Show index status",
		Args:  cobra.ExactArgs(1),
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	mf, err := manifest.Load(absPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	fmt.Printf("%s%sIndex status for %s%s\n\n", bold, cyan, absPath, reset)

	if mf.IsEmpty() {
		fmt.Printf("  %sNo index found.%s Run %scarto index %s%s to create one.\n", yellow, reset, bold, absPath, reset)
		return nil
	}

	projectName := mf.Project
	if projectName == "" {
		projectName = filepath.Base(absPath)
	}

	// Calculate total size across indexed files.
	var totalSize int64
	for _, entry := range mf.Files {
		totalSize += entry.Size
	}

	fmt.Printf("  %sProject:%s     %s\n", cyan, reset, projectName)
	fmt.Printf("  %sLast indexed:%s %s\n", cyan, reset, mf.IndexedAt.Format(time.RFC3339))
	fmt.Printf("  %sFiles:%s       %d\n", cyan, reset, len(mf.Files))
	fmt.Printf("  %sTotal size:%s  %s\n", cyan, reset, formatBytes(totalSize))

	return nil
}

// --------------------------------------------------------------------------
// serve
// --------------------------------------------------------------------------

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Carto web UI",
		RunE:  runServe,
	}
	cmd.Flags().String("port", "8950", "Port to listen on")
	cmd.Flags().String("projects-dir", "", "Directory containing indexed projects")
	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetString("port")
	projectsDir, _ := cmd.Flags().GetString("projects-dir")

	// Set config persistence path inside the projects directory so it
	// survives container restarts (the projects dir is a mounted volume).
	if projectsDir != "" {
		config.ConfigPath = filepath.Join(projectsDir, ".carto-server.json")
	}

	cfg := config.Load()

	memoriesClient := storage.NewMemoriesClient(config.ResolveURL(cfg.MemoriesURL), cfg.MemoriesKey)

	// Extract the dist subdirectory from the embedded FS.
	distFS, err := fs.Sub(cartoWeb.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("embedded web assets: %w", err)
	}

	srv := server.New(cfg, memoriesClient, projectsDir, distFS)
	fmt.Printf("%s%sCarto server%s starting on http://localhost:%s\n", bold, cyan, reset, port)
	return srv.Start(":" + port)
}

// --------------------------------------------------------------------------
// helpers
// --------------------------------------------------------------------------

// truncateText shortens a string to the given max length, appending "..." if
// truncation occurs. It also replaces newlines with spaces for single-line display.
func truncateText(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// formatBytes returns a human-readable byte size string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// writeOutput renders data as JSON (if --json flag is set) or invokes
// the human-readable callback.
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

// --------------------------------------------------------------------------
// projects
// --------------------------------------------------------------------------

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
		Short: "List all indexed projects",
		RunE:  runProjectsList,
	}
}

func runProjectsList(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("read projects dir: %w", err)
	}

	type projectInfo struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		Files     int    `json:"files"`
		IndexedAt string `json:"indexed_at"`
	}

	var projects []projectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectPath := filepath.Join(projectsDir, entry.Name())
		mf, err := manifest.Load(projectPath)
		if err != nil || mf.IsEmpty() {
			continue
		}
		name := mf.Project
		if name == "" {
			name = entry.Name()
		}
		projects = append(projects, projectInfo{
			Name:      name,
			Path:      projectPath,
			Files:     len(mf.Files),
			IndexedAt: mf.IndexedAt.Format(time.RFC3339),
		})
	}

	writeOutput(cmd, projects, func() {
		if len(projects) == 0 {
			fmt.Println("No indexed projects found.")
			return
		}
		fmt.Printf("%s%sIndexed projects%s\n\n", bold, cyan, reset)
		fmt.Printf("  %-25s %-8s %s\n", "NAME", "FILES", "INDEXED AT")
		fmt.Printf("  %-25s %-8s %s\n",
			strings.Repeat("-", 25),
			strings.Repeat("-", 8),
			strings.Repeat("-", 20))
		for _, p := range projects {
			fmt.Printf("  %-25s %-8d %s\n", p.Name, p.Files, p.IndexedAt)
		}
		fmt.Printf("\n  %sTotal:%s %d project(s)\n", bold, reset, len(projects))
	})
	return nil
}

func projectsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show details of an indexed project",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectsShow,
	}
}

func runProjectsShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	projectPath := filepath.Join(projectsDir, name)
	mf, err := manifest.Load(projectPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}
	if mf.IsEmpty() {
		return fmt.Errorf("project %q not found or has no index", name)
	}

	// Calculate total size.
	var totalSize int64
	for _, entry := range mf.Files {
		totalSize += entry.Size
	}

	// Load sources config if present.
	srcCfg, _ := sources.LoadSourcesConfig(projectPath)
	var sourceNames []string
	if srcCfg != nil {
		for k := range srcCfg.Sources {
			sourceNames = append(sourceNames, k)
		}
	}

	type showData struct {
		Name      string   `json:"name"`
		Path      string   `json:"path"`
		Files     int      `json:"files"`
		TotalSize string   `json:"total_size"`
		IndexedAt string   `json:"indexed_at"`
		Sources   []string `json:"sources,omitempty"`
	}

	data := showData{
		Name:      mf.Project,
		Path:      projectPath,
		Files:     len(mf.Files),
		TotalSize: formatBytes(totalSize),
		IndexedAt: mf.IndexedAt.Format(time.RFC3339),
		Sources:   sourceNames,
	}

	writeOutput(cmd, data, func() {
		fmt.Printf("%s%sProject: %s%s\n\n", bold, cyan, data.Name, reset)
		fmt.Printf("  %sPath:%s        %s\n", cyan, reset, data.Path)
		fmt.Printf("  %sFiles:%s       %d\n", cyan, reset, data.Files)
		fmt.Printf("  %sTotal size:%s  %s\n", cyan, reset, data.TotalSize)
		fmt.Printf("  %sIndexed at:%s  %s\n", cyan, reset, data.IndexedAt)
		if len(data.Sources) > 0 {
			fmt.Printf("  %sSources:%s     %s\n", cyan, reset, strings.Join(data.Sources, ", "))
		}
	})
	return nil
}

func projectsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a project's .carto directory",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectsDelete,
	}
}

func runProjectsDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	cartoDir := filepath.Join(projectsDir, name, ".carto")
	info, err := os.Stat(cartoDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project %q has no .carto directory", name)
	}

	if err := os.RemoveAll(cartoDir); err != nil {
		return fmt.Errorf("delete .carto: %w", err)
	}

	type deleteResult struct {
		Name    string `json:"name"`
		Deleted bool   `json:"deleted"`
	}

	writeOutput(cmd, deleteResult{Name: name, Deleted: true}, func() {
		fmt.Printf("%s✓%s Deleted .carto directory for project %q\n", green, reset, name)
	})
	return nil
}

// --------------------------------------------------------------------------
// sources
// --------------------------------------------------------------------------

func sourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage project source configurations",
	}
	cmd.AddCommand(sourcesListCmd())
	cmd.AddCommand(sourcesSetCmd())
	cmd.AddCommand(sourcesRmCmd())
	return cmd
}

func sourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <project>",
		Short: "List configured sources for a project",
		Args:  cobra.ExactArgs(1),
		RunE:  runSourcesList,
	}
}

func runSourcesList(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	projectPath := filepath.Join(projectsDir, args[0])
	srcCfg, err := sources.LoadSourcesConfig(projectPath)
	if err != nil {
		return fmt.Errorf("load sources: %w", err)
	}

	if srcCfg == nil || len(srcCfg.Sources) == 0 {
		writeOutput(cmd, map[string]interface{}{"sources": map[string]interface{}{}}, func() {
			fmt.Println("No sources configured.")
		})
		return nil
	}

	// Build a JSON-friendly representation.
	type sourceDetail struct {
		Type     string            `json:"type"`
		Settings map[string]string `json:"settings,omitempty"`
	}
	var details []sourceDetail
	for name, entry := range srcCfg.Sources {
		details = append(details, sourceDetail{
			Type:     name,
			Settings: entry.Settings,
		})
	}

	writeOutput(cmd, details, func() {
		fmt.Printf("%s%sSources for %s%s\n\n", bold, cyan, args[0], reset)
		for name, entry := range srcCfg.Sources {
			fmt.Printf("  %s%s%s\n", bold, name, reset)
			for k, v := range entry.Settings {
				fmt.Printf("    %s: %s\n", k, v)
			}
			for k, vals := range entry.ListSettings {
				fmt.Printf("    %s: [%s]\n", k, strings.Join(vals, ", "))
			}
		}
	})
	return nil
}

func sourcesSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <project> <type> [key=value ...]",
		Short: "Set or update a source for a project",
		Args:  cobra.MinimumNArgs(2),
		RunE:  runSourcesSet,
	}
}

func runSourcesSet(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	projectName := args[0]
	sourceType := args[1]

	projectPath := filepath.Join(projectsDir, projectName)
	srcCfg, err := sources.LoadSourcesConfig(projectPath)
	if err != nil {
		return fmt.Errorf("load sources: %w", err)
	}
	if srcCfg == nil {
		srcCfg = &sources.SourcesYAML{
			Sources: make(map[string]sources.SourceEntry),
		}
	}

	// Get or create the entry.
	entry, exists := srcCfg.Sources[sourceType]
	if !exists {
		entry = sources.SourceEntry{
			Settings:     make(map[string]string),
			ListSettings: make(map[string][]string),
			Raw:          make(map[string]interface{}),
		}
	}
	if entry.Settings == nil {
		entry.Settings = make(map[string]string)
	}

	// Parse key=value pairs from remaining args.
	for _, kv := range args[2:] {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid key=value pair: %q", kv)
		}
		entry.Settings[parts[0]] = parts[1]
	}

	srcCfg.Sources[sourceType] = entry
	if err := sources.SaveSourcesConfig(projectPath, srcCfg); err != nil {
		return fmt.Errorf("save sources: %w", err)
	}

	writeOutput(cmd, map[string]string{"project": projectName, "source": sourceType, "status": "updated"}, func() {
		fmt.Printf("%s✓%s Source %q updated for project %q\n", green, reset, sourceType, projectName)
	})
	return nil
}

func sourcesRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <project> <type>",
		Short: "Remove a source from a project",
		Args:  cobra.ExactArgs(2),
		RunE:  runSourcesRm,
	}
}

func runSourcesRm(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	projectName := args[0]
	sourceType := args[1]

	projectPath := filepath.Join(projectsDir, projectName)
	srcCfg, err := sources.LoadSourcesConfig(projectPath)
	if err != nil {
		return fmt.Errorf("load sources: %w", err)
	}
	if srcCfg == nil || len(srcCfg.Sources) == 0 {
		return fmt.Errorf("no sources configured for project %q", projectName)
	}

	if _, exists := srcCfg.Sources[sourceType]; !exists {
		return fmt.Errorf("source %q not found for project %q", sourceType, projectName)
	}

	delete(srcCfg.Sources, sourceType)
	if err := sources.SaveSourcesConfig(projectPath, srcCfg); err != nil {
		return fmt.Errorf("save sources: %w", err)
	}

	writeOutput(cmd, map[string]string{"project": projectName, "source": sourceType, "status": "removed"}, func() {
		fmt.Printf("%s✓%s Source %q removed from project %q\n", green, reset, sourceType, projectName)
	})
	return nil
}

// --------------------------------------------------------------------------
// config
// --------------------------------------------------------------------------

func configCmdGroup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and update Carto configuration",
	}
	cmd.AddCommand(configGetCmd())
	cmd.AddCommand(configSetCmd())
	return cmd
}

func configGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Show configuration values",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runConfigGet,
	}
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	cfg := config.Load()

	// Non-sensitive config fields for display.
	configMap := map[string]string{
		"memories_url":   cfg.MemoriesURL,
		"fast_model":     cfg.FastModel,
		"deep_model":     cfg.DeepModel,
		"max_concurrent": fmt.Sprintf("%d", cfg.MaxConcurrent),
		"llm_provider":   cfg.LLMProvider,
		"llm_base_url":   cfg.LLMBaseURL,
	}

	if len(args) == 1 {
		key := args[0]
		val, ok := configMap[key]
		if !ok {
			return fmt.Errorf("unknown config key: %q", key)
		}
		writeOutput(cmd, map[string]string{key: val}, func() {
			fmt.Printf("%s: %s\n", key, val)
		})
		return nil
	}

	writeOutput(cmd, configMap, func() {
		fmt.Printf("%s%sConfiguration%s\n\n", bold, cyan, reset)
		// Print keys in a stable order.
		orderedKeys := []string{
			"memories_url", "fast_model", "deep_model",
			"max_concurrent", "llm_provider", "llm_base_url",
		}
		for _, k := range orderedKeys {
			v := configMap[k]
			if v == "" {
				v = "(not set)"
			}
			fmt.Printf("  %-18s %s\n", k, v)
		}
	})
	return nil
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
	}
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	cfg := config.Load()

	switch key {
	case "memories_url":
		cfg.MemoriesURL = value
	case "fast_model":
		cfg.FastModel = value
	case "deep_model":
		cfg.DeepModel = value
	case "max_concurrent":
		n, err := fmt.Sscanf(value, "%d", &cfg.MaxConcurrent)
		if n != 1 || err != nil {
			return fmt.Errorf("max_concurrent must be an integer")
		}
	case "llm_provider":
		cfg.LLMProvider = value
	case "llm_base_url":
		cfg.LLMBaseURL = value
	default:
		return fmt.Errorf("unknown or read-only config key: %q", key)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	writeOutput(cmd, map[string]string{key: value, "status": "saved"}, func() {
		fmt.Printf("%s✓%s Set %s = %s\n", green, reset, key, value)
	})
	return nil
}
