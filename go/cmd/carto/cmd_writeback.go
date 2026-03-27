package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/atoms"
	"github.com/divyekant/carto/internal/chunker"
	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/manifest"
	"github.com/divyekant/carto/internal/scanner"
	"github.com/divyekant/carto/internal/storage"
)

func writebackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "writeback <path>",
		Short: "Re-index individual files without a full pipeline run",
		Long: `Writeback re-chunks and re-analyzes individual files via the fast-tier LLM,
then supersedes their old atoms in Memories. This keeps the index fresh
after small edits without running the full indexing pipeline.

When no --file or --module flags are given, writeback uses the manifest to
auto-detect changed files.`,
		Args: cobra.ExactArgs(1),
		RunE: runWriteback,
	}
	cmd.Flags().StringSlice("file", nil, "File(s) to re-index (repeatable)")
	cmd.Flags().String("module", "", "Module to re-index")
	cmd.Flags().String("project", "", "Project name (defaults to directory name)")
	return cmd
}

// writebackStats tracks per-run counters for the writeback operation.
type writebackStats struct {
	FilesProcessed int `json:"files_processed"`
	Superseded     int `json:"superseded"`
	Added          int `json:"added"`
	Removed        int `json:"removed"`
	Skipped        int `json:"skipped"`
	Errors         int `json:"errors"`
}

func runWriteback(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	// Validate the path exists and is a directory.
	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("path %q: %w", absPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", absPath)
	}

	cfg := config.Load()

	apiKey := cfg.LLMApiKey
	if apiKey == "" {
		apiKey = cfg.AnthropicKey
	}
	if apiKey == "" && cfg.LLMProvider != "ollama" {
		printError("No API key set. Set LLM_API_KEY or ANTHROPIC_API_KEY.")
		return fmt.Errorf("API key not set")
	}

	projectName, _ := cmd.Flags().GetString("project")
	if projectName == "" {
		projectName = filepath.Base(absPath)
	}

	fileFlags, _ := cmd.Flags().GetStringSlice("file")
	moduleFilter, _ := cmd.Flags().GetString("module")

	// Load manifest.
	mf, err := manifest.Load(absPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}
	if mf.Project == "" {
		mf.Project = projectName
	}

	// Determine which files to process.
	filesToProcess, err := resolveWritebackFiles(absPath, fileFlags, moduleFilter, mf)
	if err != nil {
		return fmt.Errorf("resolve files: %w", err)
	}

	if len(filesToProcess) == 0 {
		writeEnvelopeHuman(cmd, map[string]any{
			"message": "no files to process",
			"stats":   writebackStats{},
		}, nil, func() {
			fmt.Printf("%s%sNo changed files detected.%s\n", bold, gold, reset)
		})
		return nil
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

	// Scan for modules so we can determine which module each file belongs to.
	scanResult, err := scanner.Scan(absPath)
	if err != nil {
		return fmt.Errorf("scan project: %w", err)
	}

	startTime := time.Now()

	quiet, _ := cmd.Root().PersistentFlags().GetBool("quiet")
	if !quiet {
		fmt.Printf("%s%sCarto writeback %s%s\n", bold, gold, projectName, reset)
		fmt.Printf("  path:  %s\n", absPath)
		fmt.Printf("  files: %d\n\n", len(filesToProcess))
	}

	stats := writebackStats{}
	var warnings []string

	for _, relPath := range filesToProcess {
		s, warns, err := writebackFile(cmd, absPath, relPath, projectName, scanResult.Modules, llmClient, memoriesClient, mf)
		warnings = append(warnings, warns...)
		if err != nil {
			stats.Errors++
			warnings = append(warnings, fmt.Sprintf("%s: %v", relPath, err))
			printWarn("%s: %v", relPath, err)
			continue
		}
		stats.FilesProcessed++
		stats.Superseded += s.Superseded
		stats.Added += s.Added
		stats.Removed += s.Removed
		stats.Skipped += s.Skipped

		if !quiet {
			fmt.Printf("  %s%s%s %s (%d superseded, %d added, %d removed)\n",
				green, "✓", reset, relPath, s.Superseded, s.Added, s.Removed)
		}
	}

	// Save updated manifest.
	if err := mf.Save(); err != nil {
		printWarn("failed to save manifest: %v", err)
	}

	elapsed := time.Since(startTime)

	data := map[string]any{
		"project": projectName,
		"path":    absPath,
		"stats":   stats,
		"elapsed": elapsed.Round(time.Millisecond).String(),
	}
	if len(warnings) > 0 {
		data["warnings"] = warnings
	}

	writeEnvelopeHuman(cmd, data, nil, func() {
		fmt.Println()
		fmt.Printf("%s%s=== Writeback Summary ===%s\n", bold, green, reset)
		fmt.Printf("  files:      %d\n", stats.FilesProcessed)
		fmt.Printf("  superseded: %d\n", stats.Superseded)
		fmt.Printf("  added:      %d\n", stats.Added)
		fmt.Printf("  removed:    %d\n", stats.Removed)
		fmt.Printf("  skipped:    %d\n", stats.Skipped)
		fmt.Printf("  errors:     %d\n", stats.Errors)
		fmt.Printf("  elapsed:    %s\n", elapsed.Round(time.Millisecond))

		if len(warnings) > 0 {
			fmt.Printf("\n%s%sWarnings:%s\n", bold, amber, reset)
			for i, w := range warnings {
				if i >= 10 {
					fmt.Printf("  ... and %d more\n", len(warnings)-10)
					break
				}
				fmt.Printf("  - %s\n", w)
			}
		}
	})

	logAuditEvent(cmd, "ok", "", map[string]any{
		"files_processed": stats.FilesProcessed,
		"superseded":      stats.Superseded,
		"added":           stats.Added,
		"removed":         stats.Removed,
	})

	return nil
}

// resolveWritebackFiles determines which files to process based on flags
// and manifest state.
func resolveWritebackFiles(absPath string, fileFlags []string, moduleFilter string, mf *manifest.Manifest) ([]string, error) {
	// Explicit --file flags: convert to relative paths.
	if len(fileFlags) > 0 {
		var relPaths []string
		for _, f := range fileFlags {
			absFile, err := filepath.Abs(f)
			if err != nil {
				return nil, fmt.Errorf("resolve %q: %w", f, err)
			}
			rel, err := filepath.Rel(absPath, absFile)
			if err != nil {
				return nil, fmt.Errorf("make relative %q: %w", f, err)
			}
			if strings.HasPrefix(rel, "..") {
				return nil, fmt.Errorf("file %q is outside project root %q", f, absPath)
			}
			relPaths = append(relPaths, rel)
		}
		return relPaths, nil
	}

	// --module flag: scan and filter files by module.
	if moduleFilter != "" {
		scanResult, err := scanner.Scan(absPath)
		if err != nil {
			return nil, fmt.Errorf("scan for module filter: %w", err)
		}
		for _, mod := range scanResult.Modules {
			if mod.Name == moduleFilter {
				return mod.Files, nil
			}
		}
		return nil, fmt.Errorf("module %q not found", moduleFilter)
	}

	// Auto-detect: scan current files and use manifest.DetectChanges.
	scanResult, err := scanner.Scan(absPath)
	if err != nil {
		return nil, fmt.Errorf("scan for change detection: %w", err)
	}

	var currentFiles []string
	for _, f := range scanResult.Files {
		currentFiles = append(currentFiles, f.RelPath)
	}

	changes, err := mf.DetectChanges(currentFiles, absPath)
	if err != nil {
		return nil, fmt.Errorf("detect changes: %w", err)
	}

	// Process added and modified files (removed files just need cleanup).
	var files []string
	files = append(files, changes.Added...)
	files = append(files, changes.Modified...)
	return files, nil
}

// writebackFile processes a single file: chunks it, analyzes atoms, and
// supersedes/adds/removes atoms in Memories. Returns per-file stats.
func writebackFile(
	cmd *cobra.Command,
	projectRoot, relPath, projectName string,
	modules []scanner.Module,
	llmClient interface {
		CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error)
	},
	memoriesClient *storage.MemoriesClient,
	mf *manifest.Manifest,
) (writebackStats, []string, error) {
	stats := writebackStats{}
	var warnings []string

	absFile := filepath.Join(projectRoot, relPath)

	// Compute hash and skip if unchanged.
	hash, err := mf.ComputeHash(absFile)
	if err != nil {
		return stats, warnings, fmt.Errorf("compute hash: %w", err)
	}

	// Detect which module this file belongs to.
	moduleName := findModule(relPath, modules)

	// Read the file.
	code, err := os.ReadFile(absFile)
	if err != nil {
		return stats, warnings, fmt.Errorf("read file: %w", err)
	}

	// Detect language.
	language := scanner.DetectLanguage(filepath.Base(absFile))

	// Chunk the file.
	chunks, err := chunker.ChunkFile(relPath, code, language, nil)
	if err != nil {
		return stats, warnings, fmt.Errorf("chunk file: %w", err)
	}

	if len(chunks) == 0 {
		// File produced no chunks (empty or unrecognised). Update manifest and skip.
		fi, _ := os.Stat(absFile)
		size := int64(0)
		if fi != nil {
			size = fi.Size()
		}
		mf.UpdateFile(relPath, hash, size)
		stats.Skipped++
		return stats, warnings, nil
	}

	// Convert chunker.Chunk -> atoms.Chunk.
	atomChunks := make([]atoms.Chunk, len(chunks))
	for i, c := range chunks {
		atomChunks[i] = atoms.Chunk{
			Name:      c.Name,
			Kind:      c.Kind,
			Language:  c.Language,
			FilePath:  c.FilePath,
			StartLine: c.StartLine,
			EndLine:   c.EndLine,
			Code:      c.Code,
		}
	}

	// Analyze atoms via fast-tier LLM.
	analyzer := atoms.NewAnalyzer(llmClient)
	newAtoms, err := analyzer.AnalyzeBatch(atomChunks, 5, nil)
	if err != nil {
		return stats, warnings, fmt.Errorf("analyze atoms: %w", err)
	}

	// Set module on each atom.
	for _, a := range newAtoms {
		a.Module = moduleName
	}

	// Build source prefix scoped to this module's atoms layer.
	sourcePrefix := fmt.Sprintf("carto/%s/%s/layer:atoms", projectName, moduleName)

	// Fetch existing atoms for this file from Memories.
	existingAtoms, err := memoriesClient.ListBySource(sourcePrefix, 500, 0)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("list existing atoms: %v (treating as empty)", err))
		existingAtoms = nil
	}

	// Filter to atoms belonging to this file path.
	var oldAtoms []storage.SearchResult
	for _, mem := range existingAtoms {
		if fp, ok := mem.Metadata["filepath"]; ok {
			if fpStr, ok := fp.(string); ok && fpStr == relPath {
				oldAtoms = append(oldAtoms, mem)
			}
		}
	}

	// Match new atoms to old by name+kind.
	type atomKey struct {
		Name string
		Kind string
	}

	oldByKey := make(map[atomKey]storage.SearchResult)
	for _, old := range oldAtoms {
		name, _ := old.Metadata["name"].(string)
		kind, _ := old.Metadata["kind"].(string)
		if name != "" {
			oldByKey[atomKey{Name: name, Kind: kind}] = old
		}
	}

	matchedOldIDs := make(map[int]bool)

	for _, atom := range newAtoms {
		key := atomKey{Name: atom.Name, Kind: atom.Kind}
		text := atom.Summary + "\n\n" + atom.ClarifiedCode
		meta := buildAtomMeta(atom, relPath)

		if old, found := oldByKey[key]; found {
			// Supersede the existing atom.
			_, err := memoriesClient.Supersede(old.ID, text, meta)
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("supersede %s/%s (id=%d): %v", atom.Kind, atom.Name, old.ID, err))
			} else {
				stats.Superseded++
			}
			matchedOldIDs[old.ID] = true
		} else {
			// New atom — upsert it.
			source := fmt.Sprintf("carto/%s/%s/layer:atoms", projectName, moduleName)
			mem := storage.Memory{
				Text:     text,
				Source:   source,
				Key:      fmt.Sprintf("%s:%s:%s:%s", source, relPath, atom.Name, atom.Kind),
				Metadata: meta,
			}
			_, err := memoriesClient.UpsertBatch([]storage.Memory{mem})
			if err != nil {
				warnings = append(warnings, fmt.Sprintf("add %s/%s: %v", atom.Kind, atom.Name, err))
			} else {
				stats.Added++
			}
		}
	}

	// Delete old atoms that no longer exist in the file.
	for _, old := range oldAtoms {
		if !matchedOldIDs[old.ID] {
			if err := memoriesClient.DeleteMemory(old.ID); err != nil {
				warnings = append(warnings, fmt.Sprintf("delete old atom (id=%d): %v", old.ID, err))
			} else {
				stats.Removed++
			}
		}
	}

	// Update manifest entry.
	fi, _ := os.Stat(absFile)
	size := int64(0)
	if fi != nil {
		size = fi.Size()
	}
	mf.UpdateFile(relPath, hash, size)

	verboseLog(cmd, "%s: %d superseded, %d added, %d removed", relPath, stats.Superseded, stats.Added, stats.Removed)

	return stats, warnings, nil
}

// findModule returns the module name for a given relative file path.
// Falls back to the project root directory name if no module matches.
func findModule(relPath string, modules []scanner.Module) string {
	bestMatch := ""
	bestLen := 0
	for _, mod := range modules {
		// A file belongs to the module whose RelPath is the longest prefix match.
		prefix := mod.RelPath
		if prefix == "." {
			prefix = ""
		}
		if prefix == "" || strings.HasPrefix(relPath, prefix+"/") || relPath == prefix {
			if len(prefix) > bestLen {
				bestMatch = mod.Name
				bestLen = len(prefix)
			}
		}
	}
	if bestMatch == "" && len(modules) > 0 {
		// Fall back to the root module.
		for _, mod := range modules {
			if mod.RelPath == "." || mod.RelPath == "" {
				return mod.Name
			}
		}
		return modules[0].Name
	}
	return bestMatch
}

// buildAtomMeta builds the metadata map for an atom stored in Memories.
// Matches the 5 fields used by the pipeline's Phase 2 (no start_line/end_line).
func buildAtomMeta(a *atoms.Atom, relPath string) map[string]any {
	return map[string]any{
		"name":     a.Name,
		"kind":     a.Kind,
		"language": a.Language,
		"module":   a.Module,
		"filepath": relPath,
	}
}
