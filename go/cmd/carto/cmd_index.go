package main

import (
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
	"github.com/divyekant/carto/internal/pipeline"
	"github.com/divyekant/carto/internal/sources"
	"github.com/divyekant/carto/internal/storage"
)

func indexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index [path]",
		Short: "Index a codebase",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runIndex,
	}
	cmd.Flags().Bool("full", false, "Force full re-index")
	cmd.Flags().String("module", "", "Index a single module")
	cmd.Flags().Bool("incremental", false, "Only re-index changed files")
	cmd.Flags().String("project", "", "Project name (defaults to directory name)")
	cmd.Flags().Bool("all", false, "Re-index all projects")
	cmd.Flags().Bool("changed", false, "Re-index only modified projects")
	return cmd
}

func runIndex(cmd *cobra.Command, args []string) error {
	allFlag, _ := cmd.Flags().GetBool("all")
	changedFlag, _ := cmd.Flags().GetBool("changed")

	if allFlag || changedFlag {
		return runIndexAll(cmd, changedFlag)
	}

	if len(args) == 0 {
		return fmt.Errorf("path argument is required (or use --all / --changed)")
	}

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

	// Create LLM client using the configured provider.
	llmClient, llmErr := llm.NewPipelineClient(cfg.LLMProvider, llm.Options{
		APIKey:        apiKey,
		FastModel:     cfg.FastModel,
		DeepModel:     cfg.DeepModel,
		MaxConcurrent: cfg.MaxConcurrent,
		IsOAuth:       config.IsOAuthToken(apiKey),
		BaseURL:       cfg.LLMBaseURL,
	})
	if llmErr != nil {
		return fmt.Errorf("create LLM provider %q: %w", cfg.LLMProvider, llmErr)
	}

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
			fmt.Printf("\r%s%s%s %s [%d/%d]", gold, frame, reset, phase, done, total)
		}
	}

	fmt.Printf("%s%sCarto indexing %s%s\n", bold, gold, projectName, reset)
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
		fmt.Printf("\n%s%sWarnings:%s\n", bold, amber, reset)
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

// runIndexAll lists projects that would be indexed when --all or --changed is used.
// It does NOT run the pipeline (that requires LLM keys); it only enumerates projects.
//
// When changedOnly is true, a project is included only if any of its tracked
// source files (excluding .carto/) has been modified after the manifest's
// IndexedAt timestamp. This avoids unnecessary LLM calls for unmodified codebases.
func runIndexAll(cmd *cobra.Command, changedOnly bool) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("read projects dir: %w", err)
	}

	quiet, _ := cmd.Flags().GetBool("quiet")

	type projectEntry struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		IndexedAt string `json:"indexed_at"`
		Changed   bool   `json:"changed"`
	}

	var projects []projectEntry
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

		// For --changed, skip projects with no modifications since last index.
		hasChanges := true
		if changedOnly && !mf.IndexedAt.IsZero() {
			hasChanges = projectHasChanges(projectPath, mf.IndexedAt)
			if !hasChanges {
				verboseLog(cmd, "skipping %q — no changes since %s", name, mf.IndexedAt.Format(time.RFC3339))
				continue
			}
		}

		projects = append(projects, projectEntry{
			Name:      name,
			Path:      projectPath,
			IndexedAt: mf.IndexedAt.Format(time.RFC3339),
			Changed:   hasChanges,
		})

		if !quiet {
			marker := ""
			if changedOnly {
				marker = " " + amber + "(changed)" + reset
			}
			fmt.Printf("  %s%s%s%s\n", bold, name, reset, marker)
		}
	}

	mode := "all"
	if changedOnly {
		mode = "changed"
	}

	writeEnvelopeHuman(cmd, map[string]interface{}{
		"mode":     mode,
		"projects": projects,
	}, nil, func() {
		if len(projects) == 0 {
			if changedOnly {
				fmt.Println("No projects with changes found.")
			} else {
				fmt.Println("No indexed projects found.")
			}
			return
		}
		fmt.Printf("\n%s%sWould re-index %d project(s) (mode: %s)%s\n", bold, gold, len(projects), mode, reset)
	})
	return nil
}

// projectHasChanges reports whether any non-.carto/ file in projectPath has
// been modified after indexedAt. It skips the .carto directory to avoid
// treating manifest updates as source changes.
//
// The comparison uses file mtime as a fast proxy. For large repositories a
// full hash comparison would be more accurate but significantly slower.
func projectHasChanges(projectPath string, indexedAt time.Time) bool {
	changed := false
	_ = filepath.WalkDir(projectPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip unreadable entries
		}
		// Exclude the .carto/ metadata directory from change detection.
		if d.IsDir() && strings.HasSuffix(d.Name(), ".carto") {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.ModTime().After(indexedAt) {
			changed = true
			return fs.SkipAll // short-circuit once one changed file found
		}
		return nil
	})
	return changed
}
