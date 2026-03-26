// Package pipeline orchestrates the full Carto indexing flow: scan, chunk,
// analyze atoms, extract history and signals, run deep analysis, and store
// results in Memories. It supports incremental indexing via manifest hashes and
// optional module filtering.
package pipeline

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"context"

	"github.com/divyekant/carto/internal/analyzer"
	"github.com/divyekant/carto/internal/atoms"
	"github.com/divyekant/carto/internal/chunker"
	"github.com/divyekant/carto/internal/history"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/manifest"
	"github.com/divyekant/carto/internal/patterns"
	"github.com/divyekant/carto/internal/scanner"
	"github.com/divyekant/carto/internal/sources"
	"github.com/divyekant/carto/internal/storage"
)

// LLMClient is the interface shared by atoms.LLMClient and analyzer.LLMClient.
// Both require the same CompleteJSON signature.
type LLMClient interface {
	CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error)
}

// Config holds all the dependencies the pipeline needs.
type Config struct {
	Ctx            context.Context // optional: cancel to stop the pipeline mid-run
	ProjectName    string
	RootPath       string
	LLMClient      LLMClient
	MemoriesClient storage.MemoriesAPI
	SourceRegistry *sources.Registry // unified source registry (replaces SignalRegistry + KnowledgeRegistry)
	MaxWorkers     int
	ProgressFn     func(phase string, done, total int) // optional progress callback
	LogFn          func(level, msg string)              // optional log callback
	Incremental    bool                                 // use manifest for incremental indexing
	ModuleFilter   string                               // optional: index only this module
	FastMaxTokens  int                                  // optional: override fast-tier max tokens (default 4096)
	DeepMaxTokens  int                                  // optional: override deep-tier max tokens (default 8192)
	SkipSkillFiles bool                                 // if true, skip generating CLAUDE.md and .cursorrules
}

// AtomIDMap maps "module:filepath:name:kind" to the Memories ID assigned during upsert.
type AtomIDMap map[string]int

// atomKey builds the lookup key for an atom ID.
func atomKey(module, filepath, name, kind string) string {
	return module + ":" + filepath + ":" + name + ":" + kind
}

// Result holds the output of a full pipeline run.
type Result struct {
	Modules        int
	FilesIndexed   int
	AtomsCreated   int
	AtomIDs        AtomIDMap
	ModuleAnalyses []analyzer.ModuleAnalysis
	Synthesis      *analyzer.SystemSynthesis
	Errors         []error
}

// Run executes the full indexing pipeline across five phases:
//  1. Scan — discover files and modules
//  2. Chunk + Atoms — split files into chunks and analyze with fast-tier LLM
//  3. History + Signals — extract git history and external signals
//  4. Deep Analysis — per-module wiring/zones analysis and system synthesis
//  5. Store — persist all layers to Memories and update manifest
func Run(cfg Config) (*Result, error) {
	ctx := cfg.Ctx
	if ctx == nil {
		ctx = context.Background()
	}

	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 4
	}

	// Pre-flight: verify Memories server is reachable.
	if healthy, err := cfg.MemoriesClient.Health(); err != nil || !healthy {
		return nil, fmt.Errorf("pipeline: memories server unreachable at startup — verify MEMORIES_URL and ensure the server is running")
	}

	result := &Result{}
	atomIDs := AtomIDMap{}
	progress := cfg.ProgressFn
	if progress == nil {
		progress = func(string, int, int) {}
	}
	logFn := cfg.LogFn
	if logFn == nil {
		logFn = func(string, string) {}
	}

	// cancelled is a helper to check for context cancellation.
	cancelled := func() bool {
		select {
		case <-ctx.Done():
			return true
		default:
			return false
		}
	}

	// ── Phase 1: Scan ──────────────────────────────────────────────────
	logFn("info", fmt.Sprintf("Scanning %s...", cfg.RootPath))
	progress("scan", 0, 1)

	scanResult, err := scanner.Scan(cfg.RootPath)
	if err != nil {
		return nil, fmt.Errorf("pipeline: scan failed: %w", err)
	}

	progress("scan", 1, 1)

	// Apply module filter.
	modules := scanResult.Modules
	if cfg.ModuleFilter != "" {
		modules = filterModules(modules, cfg.ModuleFilter)
	}

	result.Modules = len(modules)
	if cfg.ModuleFilter != "" && len(modules) == 0 {
		available := make([]string, len(scanResult.Modules))
		for i, m := range scanResult.Modules {
			available[i] = m.Name
		}
		return nil, fmt.Errorf("pipeline: module %q not found. available: %v", cfg.ModuleFilter, available)
	}
	if len(modules) == 0 {
		logFn("info", "No modules found, nothing to index")
		return result, nil
	}
	logFn("info", fmt.Sprintf("Found %d module(s) with %d total files", len(modules), countModuleFiles(modules)))

	// Load/create manifest — always track indexed files so subsequent runs
	// can use --incremental. In non-incremental mode we still save at the end.
	if cfg.Incremental {
		logFn("info", "Incremental mode: checking for changed files...")
	}
	mf, err := manifest.Load(cfg.RootPath)
	if err != nil {
		log.Printf("pipeline: warning: failed to load manifest, starting fresh: %v", err)
		logFn("warn", "Failed to load manifest, starting fresh")
		mf = manifest.NewManifest(cfg.RootPath, cfg.ProjectName)
	}

	// Build a set of files that need indexing (respecting incremental mode).
	type moduleWork struct {
		module       scanner.Module
		filesToIndex []string // relative paths of files to process
	}

	var work []moduleWork
	totalFiles := 0

	for _, mod := range modules {
		files := mod.Files
		if cfg.Incremental && !mf.IsEmpty() {
			changed, detectErr := mf.DetectChanges(files, scanResult.Root)
			if detectErr != nil {
				log.Printf("pipeline: warning: change detection failed for %s: %v", mod.Name, detectErr)
				// Fall through to full index for this module.
			} else {
				// Only process added and modified files.
				files = append(changed.Added, changed.Modified...)

				// Clean removed files from Memories.
				// Delete only atoms belonging to the removed files, not the
				// entire module's atoms layer. This preserves atoms for
				// unchanged files.
				if len(changed.Removed) > 0 {
					atomsPrefix := fmt.Sprintf("carto/%s/%s/layer:atoms", cfg.ProjectName, mod.Name)
					existing, listErr := cfg.MemoriesClient.ListBySource(atomsPrefix, 500, 0)
					if listErr == nil {
						removedSet := make(map[string]bool)
						for _, rp := range changed.Removed {
							removedSet[rp] = true
						}
						for _, mem := range existing {
							if fp, ok := mem.Metadata["filepath"].(string); ok && removedSet[fp] {
								cfg.MemoriesClient.DeleteMemory(mem.ID)
							}
						}
					}
					// Remove from manifest.
					for _, rp := range changed.Removed {
						mf.RemoveFile(rp)
					}
				}
			}
		}

		if len(files) == 0 {
			continue
		}

		work = append(work, moduleWork{module: mod, filesToIndex: files})
		totalFiles += len(files)
	}

	result.FilesIndexed = totalFiles

	if cancelled() {
		return result, context.Canceled
	}

	// ── Phase 2: Chunk + Atoms (parallel per module) ───────────────────
	logFn("info", fmt.Sprintf("Chunking and analyzing %d files across %d module(s)...", totalFiles, len(work)))

	type moduleAtoms struct {
		module scanner.Module
		atoms  []*atoms.Atom
	}

	atomAnalyzer := atoms.NewAnalyzer(cfg.LLMClient, cfg.FastMaxTokens)
	moduleAtomsList := make([]moduleAtoms, len(work))
	var atomErrors []error

	atomsDone := 0
	var atomsMu sync.Mutex

	sem := make(chan struct{}, cfg.MaxWorkers)
	var wg sync.WaitGroup

	for i, w := range work {
		if cancelled() {
			break
		}
		wg.Add(1)

		// Acquire semaphore with context awareness.
		acquired := false
		select {
		case sem <- struct{}{}:
			acquired = true
		case <-ctx.Done():
		}
		if !acquired {
			wg.Done()
			break
		}

		go func(idx int, mw moduleWork) {
			defer wg.Done()
			defer func() { <-sem }()

			if cancelled() {
				return
			}

			allChunks, chunkErrs := chunkModuleFiles(mw.module, mw.filesToIndex, scanResult.Root)

			if cancelled() {
				return
			}

			// Convert chunker.Chunk to atoms.Chunk.
			atomChunks := make([]atoms.Chunk, len(allChunks))
			for j, c := range allChunks {
				atomChunks[j] = atoms.Chunk{
					Name:      c.Name,
					Kind:      c.Kind,
					Language:  c.Language,
					FilePath:  c.FilePath,
					StartLine: c.StartLine,
					EndLine:   c.EndLine,
					Code:      c.Code,
				}
			}

			// Analyze atoms.
			analyzed, analyzeErr := atomAnalyzer.AnalyzeBatchCtx(ctx, atomChunks, cfg.MaxWorkers, nil)

			// Set module on each atom and store with metadata via UpsertBatch.
			if len(analyzed) > 0 {
				for _, a := range analyzed {
					a.Module = mw.module.Name
				}
				memories := make([]storage.Memory, len(analyzed))
				for j, a := range analyzed {
					var docAt string
					// a.FilePath is the absolute path set by the chunker.
					if info, statErr := os.Stat(a.FilePath); statErr == nil {
						docAt = info.ModTime().Format(time.RFC3339)
					}
					memories[j] = storage.Memory{
						Text:       a.Summary + "\n\n" + a.ClarifiedCode,
						Source:     fmt.Sprintf("carto/%s/%s/layer:atoms", cfg.ProjectName, mw.module.Name),
						DocumentAt: docAt,
						Metadata: map[string]any{
							"name":     a.Name,
							"kind":     a.Kind,
							"filepath": a.FilePath,
							"module":   mw.module.Name,
							"language": a.Language,
						},
					}
				}
				results, upsertErr := cfg.MemoriesClient.UpsertBatch(memories)
				if upsertErr != nil {
					atomsMu.Lock()
					atomErrors = append(atomErrors, fmt.Errorf("upsert atoms for %s: %w", mw.module.Name, upsertErr))
					atomsMu.Unlock()
				} else {
					atomsMu.Lock()
					for j, r := range results {
						key := atomKey(mw.module.Name, analyzed[j].FilePath, analyzed[j].Name, analyzed[j].Kind)
						atomIDs[key] = r.ID
					}
					atomsMu.Unlock()
				}
			}

			atomsMu.Lock()
			moduleAtomsList[idx] = moduleAtoms{module: mw.module, atoms: analyzed}
			if analyzeErr != nil {
				atomErrors = append(atomErrors, analyzeErr)
			}
			atomErrors = append(atomErrors, chunkErrs...)
			atomsDone++
			d := atomsDone
			atomsMu.Unlock()
			progress("atoms", d, len(work))
		}(i, w)
	}

	wg.Wait()
	result.Errors = append(result.Errors, atomErrors...)

	// Count total atoms.
	for _, ma := range moduleAtomsList {
		result.AtomsCreated += len(ma.atoms)
	}

	if cancelled() {
		return result, context.Canceled
	}

	// ── Phase 3: History + Signals (parallel per module) ───────────────
	logFn("info", fmt.Sprintf("Extracted %d atoms. Fetching git history and signals...", result.AtomsCreated))

	type moduleContext struct {
		history   []*history.FileHistory
		artifacts []sources.Artifact // module-scoped source artifacts (e.g., git commits)
	}

	moduleContexts := make([]moduleContext, len(work))
	var contextErrors []error
	var contextMu sync.Mutex
	contextDone := 0

	for i, w := range work {
		if cancelled() {
			break
		}
		wg.Add(1)

		// Acquire semaphore with context awareness.
		acquired := false
		select {
		case sem <- struct{}{}:
			acquired = true
		case <-ctx.Done():
		}
		if !acquired {
			wg.Done()
			break
		}

		go func(idx int, mw moduleWork) {
			defer wg.Done()
			defer func() { <-sem }()

			if cancelled() {
				return
			}

			// Extract git history.
			histories, histErr := history.ExtractBulkHistory(
				scanResult.Root,
				mw.filesToIndex,
				&history.ExtractOptions{MaxCommits: 50, Since: "6 months ago"},
				cfg.MaxWorkers,
			)

			if cancelled() {
				return
			}

			// Fetch module-scoped source artifacts (e.g., git commits).
			var arts []sources.Artifact
			if cfg.SourceRegistry != nil {
				req := sources.FetchRequest{
					Project:    cfg.ProjectName,
					Module:     mw.module.Name,
					ModulePath: mw.module.Path,
					RepoRoot:   scanResult.Root,
				}
				var srcErr error
				arts, srcErr = cfg.SourceRegistry.FetchModule(ctx, req)
				if srcErr != nil {
					contextMu.Lock()
					contextErrors = append(contextErrors, srcErr)
					contextMu.Unlock()
				}
			}

			contextMu.Lock()
			moduleContexts[idx] = moduleContext{history: histories, artifacts: arts}
			if histErr != nil {
				contextErrors = append(contextErrors, histErr)
			}
			contextDone++
			d := contextDone
			contextMu.Unlock()
			progress("history", d, len(work))
		}(i, w)
	}

	wg.Wait()
	result.Errors = append(result.Errors, contextErrors...)

	// ── Phase 3b: Project-Scope Sources ──────────────────────────────
	var projectArtifacts []sources.Artifact
	if cfg.SourceRegistry != nil {
		logFn("info", "Fetching project-scope sources...")
		req := sources.FetchRequest{
			Project:  cfg.ProjectName,
			RepoRoot: scanResult.Root,
		}
		pArts, pErr := cfg.SourceRegistry.FetchAllProject(ctx, req)
		if pErr != nil {
			result.Errors = append(result.Errors, pErr)
		}
		projectArtifacts = pArts
		if len(projectArtifacts) > 0 {
			logFn("info", fmt.Sprintf("Fetched %d project-scope artifact(s)", len(projectArtifacts)))
		}
	}

	if cancelled() {
		return result, context.Canceled
	}

	// ── Phase 4: Deep Analysis ─────────────────────────────────────────
	logFn("info", fmt.Sprintf("Running deep analysis on %d module(s)...", len(work)))
	deepAnalyzer := analyzer.NewDeepAnalyzer(cfg.LLMClient, cfg.DeepMaxTokens)

	// Build ModuleInput for each module.
	inputs := make([]analyzer.ModuleInput, len(work))
	for i, w := range work {
		inputs[i] = analyzer.ModuleInput{
			Name:    w.module.Name,
			Path:    w.module.Path,
			Atoms:   moduleAtomsList[i].atoms,
			History: moduleContexts[i].history,
			Signals: moduleContexts[i].artifacts,
		}
	}

	moduleAnalyses, deepErr := deepAnalyzer.AnalyzeModulesCtx(ctx, inputs, cfg.MaxWorkers, func(done, total int) {
		progress("analysis", done, total)
	})
	if deepErr != nil {
		result.Errors = append(result.Errors, deepErr)
	}
	result.ModuleAnalyses = moduleAnalyses

	// System synthesis.
	if len(moduleAnalyses) > 0 {
		progress("synthesis", 0, 1)
		synthesis, synthErr := deepAnalyzer.SynthesizeSystem(moduleAnalyses)
		if synthErr != nil {
			result.Errors = append(result.Errors, synthErr)
		} else {
			result.Synthesis = synthesis
		}
		progress("synthesis", 1, 1)
	}

	if cancelled() {
		return result, context.Canceled
	}

	// ── Phase 5: Store ─────────────────────────────────────────────────
	logFn("info", "Storing results in Memories...")
	store := storage.NewStore(cfg.MemoriesClient, cfg.ProjectName)
	storeDone := 0
	// Total store ops: per-module layers (4 each: history, signals, wiring, zones)
	// + system-wide (2: blueprint, patterns). Atoms are stored in Phase 2.
	storeTotal := len(work)*4 + 2

	for i, w := range work {
		if cancelled() {
			return result, context.Canceled
		}

		modName := w.module.Name

		// For non-incremental runs, clear existing module data before storing
		// to prevent duplicate entries accumulating in Memories.
		if !cfg.Incremental {
			if err := store.ClearModule(modName); err != nil {
				log.Printf("pipeline: warning: failed to clear module %s before re-storing: %v", modName, err)
			}
		}

		// Atoms are stored in Phase 2 via UpsertBatch with metadata.

		// Store history.
		if histJSON, err := json.Marshal(moduleContexts[i].history); err == nil {
			if err := store.StoreLayer(modName, "history", string(histJSON)); err != nil {
				log.Printf("pipeline: warning: failed to store history for %s: %v", modName, err)
				result.Errors = append(result.Errors, err)
			}
		}
		storeDone++
		progress("store", storeDone, storeTotal)

		// Store module-scoped artifacts (signals).
		if sigsJSON, err := json.Marshal(moduleContexts[i].artifacts); err == nil {
			if err := store.StoreLayer(modName, "signals", string(sigsJSON)); err != nil {
				log.Printf("pipeline: warning: failed to store signals for %s: %v", modName, err)
				result.Errors = append(result.Errors, err)
			}
		}
		storeDone++
		progress("store", storeDone, storeTotal)

		// Store zones from module analysis (if available).
		// Wiring is now represented as graph links (created below).
		if ma := findModuleAnalysis(moduleAnalyses, modName); ma != nil {
			if zonesJSON, err := json.Marshal(ma.Zones); err == nil {
				if err := store.StoreLayer(modName, "zones", string(zonesJSON)); err != nil {
					log.Printf("pipeline: warning: failed to store zones for %s: %v", modName, err)
					result.Errors = append(result.Errors, err)
				}
			}
			storeDone++
			progress("store", storeDone, storeTotal)
		} else {
			storeDone++
			progress("store", storeDone, storeTotal)
		}

		// Update manifest for each file in this module.
		if mf != nil {
			for _, relPath := range w.filesToIndex {
				absPath := filepath.Join(scanResult.Root, relPath)
				hash, hashErr := mf.ComputeHash(absPath)
				if hashErr != nil {
					log.Printf("pipeline: warning: hash failed for %s: %v", relPath, hashErr)
					result.Errors = append(result.Errors, fmt.Errorf("hash failed for %s: %w", relPath, hashErr))
					continue
				}
				info, statErr := os.Stat(absPath)
				if statErr != nil {
					continue
				}
				mf.UpdateFile(relPath, hash, info.Size())
			}
		}
	}

	// Create graph links from wiring edges.
	for _, ma := range result.ModuleAnalyses {
		for _, edge := range ma.Wiring {
			fromID := findAtomID(atomIDs, edge.FromModule, edge.FromAtom)
			toID := findAtomID(atomIDs, edge.ToModule, edge.ToAtom)
			if fromID == 0 || toID == 0 {
				logFn("warn", fmt.Sprintf("skip link: %s/%s -> %s/%s (atom not found)",
					edge.FromModule, edge.FromAtom, edge.ToModule, edge.ToAtom))
				continue
			}
			if err := cfg.MemoriesClient.CreateLink(fromID, toID, edge.LinkType); err != nil {
				logFn("warn", fmt.Sprintf("create link %d->%d: %v", fromID, toID, err))
			}
		}
	}

	// Store system-wide blueprint and patterns.
	if result.Synthesis != nil {
		if err := store.StoreLayer("_system", "blueprint", result.Synthesis.Blueprint); err != nil {
			log.Printf("pipeline: warning: failed to store blueprint: %v", err)
			result.Errors = append(result.Errors, err)
		}
		storeDone++
		progress("store", storeDone, storeTotal)

		if patternsJSON, err := json.Marshal(result.Synthesis.Patterns); err == nil {
			if err := store.StoreLayer("_system", "patterns", string(patternsJSON)); err != nil {
				log.Printf("pipeline: warning: failed to store patterns: %v", err)
				result.Errors = append(result.Errors, err)
			}
		}
		storeDone++
		progress("store", storeDone, storeTotal)
	} else {
		storeDone += 2
		progress("store", storeDone, storeTotal)
	}

	// Store project-scope artifacts by category.
	for _, art := range projectArtifacts {
		layer := "_signals" // default for Signal category
		switch art.Category {
		case sources.Knowledge:
			layer = "_knowledge"
		case sources.Context:
			layer = "_context"
		}
		content := fmt.Sprintf("# %s\n\nSource: %s\nURL: %s\n\n%s", art.Title, art.Source, art.URL, art.Body)
		key := art.Source + "/" + art.ID
		if err := store.StoreLayer(layer, key, content); err != nil {
			log.Printf("pipeline: warning: failed to store %s artifact %s: %v", layer, art.ID, err)
			result.Errors = append(result.Errors, err)
		}
	}

	// Save manifest.
	if mf != nil {
		mf.Project = cfg.ProjectName
		if err := mf.Save(); err != nil {
			log.Printf("pipeline: warning: failed to save manifest: %v", err)
			result.Errors = append(result.Errors, err)
		}
	}

	// ── Phase 6: Generate Skill Files ─────────────────────────────────
	if !cfg.SkipSkillFiles && result.Synthesis != nil {
		logFn("info", "Generating skill files (CLAUDE.md, .cursorrules)...")
		progress("skillfiles", 0, 1)

		input := buildPatternsInput(cfg.ProjectName, result.Synthesis, result.ModuleAnalyses)
		if err := patterns.WriteFiles(cfg.RootPath, input, "all"); err != nil {
			log.Printf("pipeline: warning: failed to write skill files: %v", err)
			result.Errors = append(result.Errors, err)
		}
		progress("skillfiles", 1, 1)
	}

	result.AtomIDs = atomIDs
	return result, nil
}

// findAtomID searches the AtomIDMap for an atom by module and name.
// It returns 0 if no match is found.
func findAtomID(ids AtomIDMap, module, name string) int {
	for key, id := range ids {
		parts := strings.SplitN(key, ":", 4)
		if len(parts) >= 3 && parts[0] == module && parts[2] == name {
			return id
		}
	}
	return 0
}

// filterModules returns only the module matching the given name.
func filterModules(modules []scanner.Module, name string) []scanner.Module {
	for _, m := range modules {
		if m.Name == name {
			return []scanner.Module{m}
		}
	}
	return nil
}

// chunkModuleFiles reads and chunks all files for a module.
// It returns the concatenated chunks and any non-fatal errors encountered.
func chunkModuleFiles(mod scanner.Module, filesToIndex []string, scanRoot string) ([]chunker.Chunk, []error) {
	var allChunks []chunker.Chunk
	var errs []error

	for _, relPath := range filesToIndex {
		absPath := filepath.Join(scanRoot, relPath)

		code, err := os.ReadFile(absPath)
		if err != nil {
			log.Printf("pipeline: warning: cannot read %s: %v", relPath, err)
			errs = append(errs, err)
			continue
		}

		lang := scanner.DetectLanguage(filepath.Base(relPath))

		chunks, err := chunker.ChunkFile(absPath, code, lang, nil)
		if err != nil {
			log.Printf("pipeline: warning: chunking failed for %s: %v", relPath, err)
			errs = append(errs, err)
			continue
		}

		allChunks = append(allChunks, chunks...)
	}

	return allChunks, errs
}

// findModuleAnalysis looks up a ModuleAnalysis by module name.
func findModuleAnalysis(analyses []analyzer.ModuleAnalysis, name string) *analyzer.ModuleAnalysis {
	for i := range analyses {
		if analyses[i].ModuleName == name {
			return &analyses[i]
		}
	}
	return nil
}

// countModuleFiles counts the total files across all modules.
func countModuleFiles(modules []scanner.Module) int {
	n := 0
	for _, m := range modules {
		n += len(m.Files)
	}
	return n
}

// buildPatternsInput assembles a patterns.Input from pipeline results.
func buildPatternsInput(projectName string, synthesis *analyzer.SystemSynthesis, analyses []analyzer.ModuleAnalysis) patterns.Input {
	input := patterns.Input{
		ProjectName: projectName,
		Blueprint:   synthesis.Blueprint,
		Patterns:    synthesis.Patterns,
	}

	for _, ma := range analyses {
		input.Modules = append(input.Modules, patterns.ModuleSummary{
			Name:   ma.ModuleName,
			Intent: ma.ModuleIntent,
		})
		for _, z := range ma.Zones {
			input.Zones = append(input.Zones, patterns.Zone{
				Name:   z.Name,
				Intent: z.Intent,
				Files:  z.Files,
			})
		}
	}

	return input
}

