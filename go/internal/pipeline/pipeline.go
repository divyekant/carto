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
	"sync"

	"github.com/anthropic/indexer/internal/analyzer"
	"github.com/anthropic/indexer/internal/atoms"
	"github.com/anthropic/indexer/internal/chunker"
	"github.com/anthropic/indexer/internal/history"
	"github.com/anthropic/indexer/internal/llm"
	"github.com/anthropic/indexer/internal/manifest"
	"github.com/anthropic/indexer/internal/scanner"
	"github.com/anthropic/indexer/internal/signals"
	"github.com/anthropic/indexer/internal/storage"
)

// LLMClient is the interface shared by atoms.LLMClient and analyzer.LLMClient.
// Both require the same CompleteJSON signature.
type LLMClient interface {
	CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error)
}

// Config holds all the dependencies the pipeline needs.
type Config struct {
	ProjectName    string
	RootPath       string
	LLMClient      LLMClient
	MemoriesClient storage.MemoriesAPI
	SignalRegistry *signals.Registry
	MaxWorkers     int
	ProgressFn     func(phase string, done, total int) // optional progress callback
	Incremental    bool                                 // use manifest for incremental indexing
	ModuleFilter   string                               // optional: index only this module
}

// Result holds the output of a full pipeline run.
type Result struct {
	Modules        int
	FilesIndexed   int
	AtomsCreated   int
	ModuleAnalyses []analyzer.ModuleAnalysis
	Synthesis      *analyzer.SystemSynthesis
	Errors         []error
}

// Run executes the full indexing pipeline across five phases:
//  1. Scan — discover files and modules
//  2. Chunk + Atoms — split files into chunks and analyze with Haiku
//  3. History + Signals — extract git history and external signals
//  4. Deep Analysis — per-module wiring/zones analysis and system synthesis
//  5. Store — persist all layers to Memories and update manifest
func Run(cfg Config) (*Result, error) {
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 4
	}

	// Pre-flight: verify Memories server is reachable.
	if healthy, err := cfg.MemoriesClient.Health(); err != nil || !healthy {
		return nil, fmt.Errorf("pipeline: memories server unreachable at startup — verify MEMORIES_URL and ensure the server is running")
	}

	result := &Result{}
	progress := cfg.ProgressFn
	if progress == nil {
		progress = func(string, int, int) {}
	}

	// ── Phase 1: Scan ──────────────────────────────────────────────────
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
		return result, nil
	}

	// Load/create manifest — always track indexed files so subsequent runs
	// can use --incremental. In non-incremental mode we still save at the end.
	mf, err := manifest.Load(cfg.RootPath)
	if err != nil {
		log.Printf("pipeline: warning: failed to load manifest, starting fresh: %v", err)
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
				if len(changed.Removed) > 0 {
					store := storage.NewStore(cfg.MemoriesClient, cfg.ProjectName)
					if clearErr := store.ClearModule(mod.Name); clearErr != nil {
						log.Printf("pipeline: warning: failed to clear module %s: %v", mod.Name, clearErr)
						result.Errors = append(result.Errors, clearErr)
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

	// ── Phase 2: Chunk + Atoms (parallel per module) ───────────────────
	type moduleAtoms struct {
		module scanner.Module
		atoms  []*atoms.Atom
	}

	atomAnalyzer := atoms.NewAnalyzer(cfg.LLMClient)
	moduleAtomsList := make([]moduleAtoms, len(work))
	var atomErrors []error

	atomsDone := 0
	var atomsMu sync.Mutex

	sem := make(chan struct{}, cfg.MaxWorkers)
	var wg sync.WaitGroup

	for i, w := range work {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, mw moduleWork) {
			defer wg.Done()
			defer func() { <-sem }()

			allChunks, chunkErrs := chunkModuleFiles(mw.module, mw.filesToIndex, scanResult.Root)

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
			analyzed, analyzeErr := atomAnalyzer.AnalyzeBatch(atomChunks, cfg.MaxWorkers, nil)

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

	// ── Phase 3: History + Signals (parallel per module) ───────────────
	type moduleContext struct {
		history []*history.FileHistory
		signals []signals.Signal
	}

	moduleContexts := make([]moduleContext, len(work))
	var contextErrors []error
	var contextMu sync.Mutex
	contextDone := 0

	for i, w := range work {
		wg.Add(1)
		sem <- struct{}{}

		go func(idx int, mw moduleWork) {
			defer wg.Done()
			defer func() { <-sem }()

			// Extract git history.
			histories, histErr := history.ExtractBulkHistory(
				scanResult.Root,
				mw.filesToIndex,
				&history.ExtractOptions{MaxCommits: 50, Since: "6 months ago"},
				cfg.MaxWorkers,
			)

			// Fetch signals.
			var sigs []signals.Signal
			if cfg.SignalRegistry != nil {
				sigMod := signals.Module{
					Name:    mw.module.Name,
					Path:    mw.module.Path,
					RelPath: mw.module.RelPath,
					Files:   mw.filesToIndex,
				}
				var sigErr error
				sigs, sigErr = cfg.SignalRegistry.FetchAll(sigMod)
				if sigErr != nil {
					contextMu.Lock()
					contextErrors = append(contextErrors, sigErr)
					contextMu.Unlock()
				}
			}

			contextMu.Lock()
			moduleContexts[idx] = moduleContext{history: histories, signals: sigs}
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

	// ── Phase 4: Deep Analysis ─────────────────────────────────────────
	deepAnalyzer := analyzer.NewDeepAnalyzer(cfg.LLMClient)

	// Build ModuleInput for each module.
	inputs := make([]analyzer.ModuleInput, len(work))
	for i, w := range work {
		inputs[i] = analyzer.ModuleInput{
			Name:    w.module.Name,
			Path:    w.module.Path,
			Atoms:   moduleAtomsList[i].atoms,
			History: moduleContexts[i].history,
			Signals: moduleContexts[i].signals,
		}
	}

	moduleAnalyses, deepErr := deepAnalyzer.AnalyzeModules(inputs, cfg.MaxWorkers, func(done, total int) {
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

	// ── Phase 5: Store ─────────────────────────────────────────────────
	store := storage.NewStore(cfg.MemoriesClient, cfg.ProjectName)
	storeDone := 0
	// Total store ops: per-module layers (5 each) + system-wide (2).
	storeTotal := len(work)*5 + 2

	for i, w := range work {
		modName := w.module.Name

		// Store atoms.
		if atomsJSON, err := json.Marshal(moduleAtomsList[i].atoms); err == nil {
			if err := store.StoreLayer(modName, "atoms", string(atomsJSON)); err != nil {
				log.Printf("pipeline: warning: failed to store atoms for %s: %v", modName, err)
				result.Errors = append(result.Errors, err)
			}
		}
		storeDone++
		progress("store", storeDone, storeTotal)

		// Store history.
		if histJSON, err := json.Marshal(moduleContexts[i].history); err == nil {
			if err := store.StoreLayer(modName, "history", string(histJSON)); err != nil {
				log.Printf("pipeline: warning: failed to store history for %s: %v", modName, err)
				result.Errors = append(result.Errors, err)
			}
		}
		storeDone++
		progress("store", storeDone, storeTotal)

		// Store signals.
		if sigsJSON, err := json.Marshal(moduleContexts[i].signals); err == nil {
			if err := store.StoreLayer(modName, "signals", string(sigsJSON)); err != nil {
				log.Printf("pipeline: warning: failed to store signals for %s: %v", modName, err)
				result.Errors = append(result.Errors, err)
			}
		}
		storeDone++
		progress("store", storeDone, storeTotal)

		// Store wiring and zones from module analysis (if available).
		if ma := findModuleAnalysis(moduleAnalyses, modName); ma != nil {
			if wiringJSON, err := json.Marshal(ma.Wiring); err == nil {
				if err := store.StoreLayer(modName, "wiring", string(wiringJSON)); err != nil {
					log.Printf("pipeline: warning: failed to store wiring for %s: %v", modName, err)
					result.Errors = append(result.Errors, err)
				}
			}
			storeDone++
			progress("store", storeDone, storeTotal)

			if zonesJSON, err := json.Marshal(ma.Zones); err == nil {
				if err := store.StoreLayer(modName, "zones", string(zonesJSON)); err != nil {
					log.Printf("pipeline: warning: failed to store zones for %s: %v", modName, err)
					result.Errors = append(result.Errors, err)
				}
			}
			storeDone++
			progress("store", storeDone, storeTotal)
		} else {
			storeDone += 2
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

	// Save manifest.
	if mf != nil {
		mf.Project = cfg.ProjectName
		if err := mf.Save(); err != nil {
			log.Printf("pipeline: warning: failed to save manifest: %v", err)
			result.Errors = append(result.Errors, err)
		}
	}

	return result, nil
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
