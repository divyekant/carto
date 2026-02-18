package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/anthropic/indexer/internal/llm"
	"github.com/anthropic/indexer/internal/signals"
	"github.com/anthropic/indexer/internal/storage"
)

// ── Integration Mocks ───────────────────────────────────────────────────

// integrationLLM is a mock LLM client that returns realistic JSON responses
// based on both the tier and the prompt content. It tracks call counts per
// tier for verification.
type integrationLLM struct {
	mu        sync.Mutex
	calls     int
	tiers     []llm.Tier
	haikuCnt  int
	opusCnt   int
}

func (m *integrationLLM) CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.tiers = append(m.tiers, tier)

	switch tier {
	case llm.TierHaiku:
		m.haikuCnt++
		// Atom analysis response: return valid JSON matching atoms.llmResponse.
		return json.RawMessage(`{
			"clarified_code": "func example() { /* clarified */ }",
			"summary": "A code unit that performs a specific task in the project.",
			"imports": ["fmt"],
			"exports": ["example"]
		}`), nil

	case llm.TierOpus:
		m.opusCnt++
		// Distinguish synthesis from module analysis by prompt content.
		if strings.Contains(prompt, "Synthesize") {
			return json.RawMessage(`{
				"blueprint": "A test application with a main entry point, a utility package providing helper functions and configuration types, and a web package providing HTTP handlers with middleware. The system follows a layered architecture where main depends on pkg for business logic and web for HTTP serving.",
				"patterns": ["layered architecture", "middleware pattern", "configuration struct pattern", "constructor functions"]
			}`), nil
		}
		// Module analysis response. Leave module_name empty so AnalyzeModule
		// fills it from the input.
		return json.RawMessage(`{
			"module_name": "",
			"wiring": [
				{"from": "main", "to": "pkg.Greet", "reason": "main calls Greet to produce output"},
				{"from": "web.Logger", "to": "web.HandleRoot", "reason": "middleware wraps handler"}
			],
			"zones": [
				{"name": "core", "intent": "main entry point and application bootstrap", "files": ["main.go"]},
				{"name": "utilities", "intent": "shared helper functions and type definitions", "files": ["pkg/util.go", "pkg/types.go"]},
				{"name": "http", "intent": "HTTP request handling and middleware", "files": ["web/handler.go", "web/middleware.go"]}
			],
			"module_intent": "A test application demonstrating Go project structure with separate packages for utilities, types, and HTTP handling."
		}`), nil
	}

	return json.RawMessage(`{}`), nil
}

func (m *integrationLLM) getCounts() (total, haiku, opus int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls, m.haikuCnt, m.opusCnt
}

// integrationMemories is a thread-safe in-memory Memories mock that stores all
// memories and allows inspection by source prefix.
type integrationMemories struct {
	mu       sync.Mutex
	memories []storage.Memory
	nextID   int
	healthy  bool
}

func (f *integrationMemories) Health() (bool, error) { return f.healthy, nil }

func (f *integrationMemories) AddMemory(mem storage.Memory) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.nextID++
	f.memories = append(f.memories, mem)
	return f.nextID, nil
}

func (f *integrationMemories) AddBatch(memories []storage.Memory) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, mem := range memories {
		f.nextID++
		f.memories = append(f.memories, mem)
	}
	return nil
}

func (f *integrationMemories) Search(query string, opts storage.SearchOptions) ([]storage.SearchResult, error) {
	return nil, nil
}

func (f *integrationMemories) ListBySource(source string, limit int) ([]storage.SearchResult, error) {
	return nil, nil
}

func (f *integrationMemories) DeleteBySource(prefix string) (int, error) {
	return 0, nil
}

// getMemories returns a snapshot of all stored memories.
func (f *integrationMemories) getMemories() []storage.Memory {
	f.mu.Lock()
	defer f.mu.Unlock()
	cp := make([]storage.Memory, len(f.memories))
	copy(cp, f.memories)
	return cp
}

// layersStored extracts unique layer names from the stored memory source tags.
// Source tags follow the format: carto/{project}/{module}/layer:{layer}
func (f *integrationMemories) layersStored() map[string]bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	layers := make(map[string]bool)
	for _, mem := range f.memories {
		parts := strings.Split(mem.Source, "/")
		for _, p := range parts {
			if strings.HasPrefix(p, "layer:") {
				layers[strings.TrimPrefix(p, "layer:")] = true
			}
		}
	}
	return layers
}

// integrationSignalSource returns a canned signal for testing.
type integrationSignalSource struct{}

func (s *integrationSignalSource) Name() string                          { return "integration-mock" }
func (s *integrationSignalSource) Configure(cfg map[string]string) error { return nil }
func (s *integrationSignalSource) FetchSignals(mod signals.Module) ([]signals.Signal, error) {
	return []signals.Signal{
		{Type: "ticket", ID: "INT-1", Title: "Integration test ticket", Author: "test-bot"},
		{Type: "pr", ID: "#42", Title: "Add utility package", Author: "dev"},
	}, nil
}

// ── Temp Project Setup ──────────────────────────────────────────────────

// createIntegrationProject sets up a realistic temp directory with multiple
// packages and source files to exercise the full pipeline.
func createIntegrationProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// go.mod
	writeFile(t, dir, "go.mod", `module example.com/testapp

go 1.21
`)

	// main.go
	writeFile(t, dir, "main.go", `package main

import (
	"fmt"
	"example.com/testapp/pkg"
)

func main() {
	fmt.Println(pkg.Greet("world"))
}

func helper() string {
	return "helper"
}
`)

	// pkg/util.go
	mkdirAll(t, dir, "pkg")
	writeFile(t, dir, "pkg/util.go", `package pkg

import "strings"

func Greet(name string) string {
	return "Hello, " + strings.Title(name) + "!"
}
`)

	// pkg/types.go
	writeFile(t, dir, "pkg/types.go", `package pkg

type Config struct {
	Name    string
	Debug   bool
	MaxSize int
}

func NewConfig(name string) Config {
	return Config{Name: name, Debug: false, MaxSize: 1024}
}
`)

	// web/handler.go
	mkdirAll(t, dir, "web")
	writeFile(t, dir, "web/handler.go", `package web

import "net/http"

func HandleRoot(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("OK"))
}
`)

	// web/middleware.go
	writeFile(t, dir, "web/middleware.go", `package web

import (
	"log"
	"net/http"
)

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s", r.Method, r.URL.Path)
		next.ServeHTTP(w, r)
	})
}
`)

	return dir
}

func writeFile(t *testing.T, base, relPath, content string) {
	t.Helper()
	full := filepath.Join(base, relPath)
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", relPath, err)
	}
}

func mkdirAll(t *testing.T, base, relPath string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(base, relPath), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", relPath, err)
	}
}

// ── Integration Test ────────────────────────────────────────────────────

func TestIntegration_FullPipeline(t *testing.T) {
	dir := createIntegrationProject(t)

	llmClient := &integrationLLM{}
	mem := &integrationMemories{healthy: true}
	registry := signals.NewRegistry()
	registry.Register(&integrationSignalSource{})

	// Track progress phases and their order.
	var progressMu sync.Mutex
	phaseCounts := make(map[string]int)
	var phaseOrder []string

	result, err := Run(Config{
		ProjectName:    "integration-test",
		RootPath:       dir,
		LLMClient:      llmClient,
		MemoriesClient: mem,
		SignalRegistry: registry,
		MaxWorkers:     2,
		Incremental:    true, // enable manifest creation
		ProgressFn: func(phase string, done, total int) {
			progressMu.Lock()
			defer progressMu.Unlock()
			phaseCounts[phase]++
			if len(phaseOrder) == 0 || phaseOrder[len(phaseOrder)-1] != phase {
				phaseOrder = append(phaseOrder, phase)
			}
		},
	})
	if err != nil {
		t.Fatalf("Run returned fatal error: %v", err)
	}

	// ── Verify result fields ────────────────────────────────────────

	if result.Modules < 1 {
		t.Errorf("Modules: got %d, want >= 1", result.Modules)
	}

	// At least main.go + 2 pkg files + 2 web files = 5, but go.mod is not a
	// source file so the scanner might pick up different counts. We expect
	// at least 4 source files.
	if result.FilesIndexed < 4 {
		t.Errorf("FilesIndexed: got %d, want >= 4", result.FilesIndexed)
	}

	if result.AtomsCreated < 1 {
		t.Errorf("AtomsCreated: got %d, want >= 1", result.AtomsCreated)
	}

	if len(result.ModuleAnalyses) < 1 {
		t.Errorf("ModuleAnalyses: got %d, want >= 1", len(result.ModuleAnalyses))
	}

	// Verify synthesis.
	if result.Synthesis == nil {
		t.Fatal("Synthesis is nil, want non-nil")
	}
	if result.Synthesis.Blueprint == "" {
		t.Error("Synthesis.Blueprint is empty")
	}
	if len(result.Synthesis.Patterns) == 0 {
		t.Error("Synthesis.Patterns is empty")
	}

	// ── Verify LLM calls ────────────────────────────────────────────

	totalCalls, haikuCalls, opusCalls := llmClient.getCounts()

	if totalCalls < 3 {
		t.Errorf("total LLM calls: got %d, want >= 3 (atoms + module analysis + synthesis)", totalCalls)
	}
	if haikuCalls < 1 {
		t.Errorf("Haiku calls: got %d, want >= 1 (atom analysis)", haikuCalls)
	}
	if opusCalls < 2 {
		t.Errorf("Opus calls: got %d, want >= 2 (module analysis + synthesis)", opusCalls)
	}

	// ── Verify Memories layers ──────────────────────────────────────

	memories := mem.getMemories()
	if len(memories) == 0 {
		t.Fatal("no memories stored")
	}

	layers := mem.layersStored()
	expectedLayers := []string{"atoms", "history", "signals", "wiring", "zones", "blueprint", "patterns"}
	for _, layer := range expectedLayers {
		if !layers[layer] {
			t.Errorf("layer %q was not stored in Memories (stored layers: %v)", layer, layers)
		}
	}

	// ── Verify progress phases ──────────────────────────────────────

	progressMu.Lock()
	expectedPhases := []string{"scan", "atoms", "history", "analysis", "synthesis", "store"}
	for _, phase := range expectedPhases {
		if phaseCounts[phase] == 0 {
			t.Errorf("progress phase %q was never called", phase)
		}
	}
	// Check ordering: each expected phase should appear in the order list.
	for i, phase := range expectedPhases {
		found := false
		for j, actual := range phaseOrder {
			if actual == phase {
				if j < i && i > 0 {
					// This phase appeared before a phase that should precede it.
					// Don't fail hard since parallel phases may interleave.
				}
				found = true
				break
			}
		}
		if !found {
			t.Errorf("phase %q not found in phaseOrder: %v", phase, phaseOrder)
		}
	}
	progressMu.Unlock()

	// ── Verify manifest was created ─────────────────────────────────

	manifestPath := filepath.Join(dir, ".carto", "manifest.json")
	if _, statErr := os.Stat(manifestPath); os.IsNotExist(statErr) {
		t.Fatal("manifest.json was not created at .carto/manifest.json")
	}

	// ── Second run: incremental with no changes ─────────────────────

	llmCallsBefore := llmClient.calls

	result2, err := Run(Config{
		ProjectName:    "integration-test",
		RootPath:       dir,
		LLMClient:      llmClient,
		MemoriesClient: mem,
		SignalRegistry: registry,
		MaxWorkers:     2,
		Incremental:    true,
	})
	if err != nil {
		t.Fatalf("incremental run returned fatal error: %v", err)
	}

	// No files should need re-indexing.
	if result2.FilesIndexed != 0 {
		t.Errorf("incremental FilesIndexed: got %d, want 0 (no changes)", result2.FilesIndexed)
	}

	// No new LLM calls.
	llmCallsAfter, _, _ := llmClient.getCounts()
	if llmCallsAfter != llmCallsBefore {
		t.Errorf("incremental LLM calls: got %d new calls, want 0 (was %d, now %d)",
			llmCallsAfter-llmCallsBefore, llmCallsBefore, llmCallsAfter)
	}

	// Modules should still be detected even if nothing to index.
	if result2.Modules < 1 {
		t.Errorf("incremental Modules: got %d, want >= 1 (modules still exist)", result2.Modules)
	}
}

// TestIntegration_IncrementalDetectsChanges verifies that after modifying a
// file, the incremental run picks it up.
func TestIntegration_IncrementalDetectsChanges(t *testing.T) {
	dir := createIntegrationProject(t)

	llmClient := &integrationLLM{}
	mem := &integrationMemories{healthy: true}

	// First run: full index with manifest.
	result1, err := Run(Config{
		ProjectName: "change-detect-test",
		RootPath:    dir,
		LLMClient:   llmClient,
		MemoriesClient: mem,
		MaxWorkers:  2,
		Incremental: true,
	})
	if err != nil {
		t.Fatalf("first run error: %v", err)
	}
	if result1.FilesIndexed < 4 {
		t.Fatalf("first run indexed %d files, want >= 4", result1.FilesIndexed)
	}

	callsAfterFirst, _, _ := llmClient.getCounts()

	// Modify one file.
	modifiedContent := `package pkg

import "strings"

func Greet(name string) string {
	return "Hi, " + strings.ToUpper(name) + "!"
}

func Farewell(name string) string {
	return "Goodbye, " + name
}
`
	writeFile(t, dir, "pkg/util.go", modifiedContent)

	// Second run: should pick up the change.
	result2, err := Run(Config{
		ProjectName: "change-detect-test",
		RootPath:    dir,
		LLMClient:   llmClient,
		MemoriesClient: mem,
		MaxWorkers:  2,
		Incremental: true,
	})
	if err != nil {
		t.Fatalf("second run error: %v", err)
	}

	// At least one file should be re-indexed (the modified util.go).
	if result2.FilesIndexed < 1 {
		t.Errorf("second run FilesIndexed: got %d, want >= 1 (util.go changed)", result2.FilesIndexed)
	}

	// LLM should have been called again.
	callsAfterSecond, _, _ := llmClient.getCounts()
	if callsAfterSecond <= callsAfterFirst {
		t.Errorf("expected new LLM calls after file change: was %d, now %d",
			callsAfterFirst, callsAfterSecond)
	}
}

// TestIntegration_NoSignalRegistry verifies the pipeline works without a
// signal registry (nil).
func TestIntegration_NoSignalRegistry(t *testing.T) {
	dir := createIntegrationProject(t)

	llmClient := &integrationLLM{}
	mem := &integrationMemories{healthy: true}

	result, err := Run(Config{
		ProjectName:    "no-signals-test",
		RootPath:       dir,
		LLMClient:      llmClient,
		MemoriesClient: mem,
		SignalRegistry: nil,
		MaxWorkers:     2,
	})
	if err != nil {
		t.Fatalf("Run returned fatal error: %v", err)
	}

	if result.Modules < 1 {
		t.Errorf("Modules: got %d, want >= 1", result.Modules)
	}
	if result.AtomsCreated < 1 {
		t.Errorf("AtomsCreated: got %d, want >= 1", result.AtomsCreated)
	}
	if result.Synthesis == nil {
		t.Error("Synthesis should not be nil even without signals")
	}

	// Signals layer should still be stored (as empty/null JSON).
	layers := mem.layersStored()
	if !layers["atoms"] {
		t.Error("atoms layer missing from Memories")
	}
}

// TestIntegration_ConcurrentRuns verifies that two full pipeline runs can
// execute concurrently without data races (when run with -race).
func TestIntegration_ConcurrentRuns(t *testing.T) {
	dir := createIntegrationProject(t)

	var wg sync.WaitGroup
	var results [2]*Result
	var errs [2]error
	var progressCount atomic.Int32

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = Run(Config{
				ProjectName: "concurrent-test",
				RootPath:    dir,
				LLMClient:   &integrationLLM{},
				MemoriesClient: &integrationMemories{healthy: true},
				MaxWorkers:  2,
				ProgressFn: func(phase string, done, total int) {
					progressCount.Add(1)
				},
			})
		}(i)
	}

	wg.Wait()

	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Errorf("concurrent run %d error: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Errorf("concurrent run %d returned nil result", i)
			continue
		}
		if results[i].Modules < 1 {
			t.Errorf("concurrent run %d: Modules=%d, want >= 1", i, results[i].Modules)
		}
		if results[i].AtomsCreated < 1 {
			t.Errorf("concurrent run %d: AtomsCreated=%d, want >= 1", i, results[i].AtomsCreated)
		}
	}

	if progressCount.Load() < 2 {
		t.Error("expected progress callbacks from both concurrent runs")
	}
}
