package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/anthropic/indexer/internal/config"
	"github.com/anthropic/indexer/internal/llm"
	"github.com/anthropic/indexer/internal/manifest"
	"github.com/anthropic/indexer/internal/patterns"
	"github.com/anthropic/indexer/internal/pipeline"
	"github.com/anthropic/indexer/internal/scanner"
	"github.com/anthropic/indexer/internal/signals"
	"github.com/anthropic/indexer/internal/storage"
)

var version = "0.2.0"

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

	root.AddCommand(indexCmd())
	root.AddCommand(queryCmd())
	root.AddCommand(modulesCmd())
	root.AddCommand(patternsCmd())
	root.AddCommand(statusCmd())

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
		HaikuModel:    cfg.HaikuModel,
		OpusModel:     cfg.OpusModel,
		MaxConcurrent: cfg.MaxConcurrent,
		IsOAuth:       config.IsOAuthToken(apiKey),
		BaseURL:       cfg.LLMBaseURL,
	})

	// Create Memories client.
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// Create signal registry and register git signals.
	registry := signals.NewRegistry()
	registry.Register(signals.NewGitSignalSource(absPath))

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
		SignalRegistry: registry,
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
