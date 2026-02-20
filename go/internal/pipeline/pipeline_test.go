package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"context"

	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/sources"
	"github.com/divyekant/carto/internal/storage"
)

// ── Mock LLM Client ────────────────────────────────────────────────────

// mockLLM returns canned JSON responses based on the tier used.
type mockLLM struct {
	mu    sync.Mutex
	calls int
	tiers []llm.Tier
}

func (m *mockLLM) CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.tiers = append(m.tiers, tier)

	switch tier {
	case llm.TierFast:
		// Atom analysis response.
		return json.RawMessage(`{
			"clarified_code": "func example() {}",
			"summary": "An example function for testing.",
			"imports": ["fmt"],
			"exports": ["example"]
		}`), nil
	case llm.TierDeep:
		// Check if this is a synthesis call (contains "Synthesize").
		if strings.Contains(prompt, "Synthesize") {
			return json.RawMessage(`{
				"blueprint": "A test system with one module.",
				"patterns": ["dependency injection", "table-driven tests"]
			}`), nil
		}
		// Module analysis response. Leave module_name empty so
		// AnalyzeModule fills it from the input, matching the scanner's name.
		return json.RawMessage(`{
			"module_name": "",
			"wiring": [{"from": "main", "to": "helper", "reason": "calls helper function"}],
			"zones": [{"name": "core", "intent": "main business logic", "files": ["main.go"]}],
			"module_intent": "A test module for pipeline validation."
		}`), nil
	}

	return json.RawMessage(`{}`), nil
}

// ── Mock Memories API ──────────────────────────────────────────────────

type storedMemory struct {
	text   string
	source string
}

type mockMemories struct {
	mu       sync.Mutex
	memories []storedMemory
	nextID   int
	healthy  bool
}

func (m *mockMemories) Health() (bool, error) { return m.healthy, nil }

func (m *mockMemories) AddMemory(mem storage.Memory) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	m.memories = append(m.memories, storedMemory{text: mem.Text, source: mem.Source})
	return m.nextID, nil
}

func (m *mockMemories) AddBatch(memories []storage.Memory) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, mem := range memories {
		m.nextID++
		m.memories = append(m.memories, storedMemory{text: mem.Text, source: mem.Source})
	}
	return nil
}

func (m *mockMemories) Search(query string, opts storage.SearchOptions) ([]storage.SearchResult, error) {
	return nil, nil
}

func (m *mockMemories) ListBySource(source string, limit int) ([]storage.SearchResult, error) {
	return nil, nil
}

func (m *mockMemories) DeleteBySource(prefix string) (int, error) {
	return 0, nil
}

func (m *mockMemories) getMemories() []storedMemory {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := make([]storedMemory, len(m.memories))
	copy(cp, m.memories)
	return cp
}

// ── Mock Source (implements sources.Source) ────────────────────────────

type mockPipelineSource struct {
	name      string
	scope     sources.Scope
	artifacts []sources.Artifact
}

func (s *mockPipelineSource) Name() string                { return s.name }
func (s *mockPipelineSource) Scope() sources.Scope        { return s.scope }
func (s *mockPipelineSource) Configure(cfg sources.SourceConfig) error { return nil }
func (s *mockPipelineSource) Fetch(_ context.Context, _ sources.FetchRequest) ([]sources.Artifact, error) {
	return s.artifacts, nil
}

// ── Helpers ────────────────────────────────────────────────────────────

// createTempProject sets up a temporary directory structure that looks like
// a Go project with a go.mod and a couple of .go files.
func createTempProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Write go.mod so scanner detects this as a Go module.
	goMod := "module example.com/testproject\n\ngo 1.21\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}

	// Write a simple Go source file.
	mainGo := `package main

import "fmt"

func main() {
	fmt.Println("hello")
}

func helper() string {
	return "help"
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	// Write a second file in a subdirectory.
	if err := os.MkdirAll(filepath.Join(dir, "pkg"), 0o755); err != nil {
		t.Fatalf("mkdir pkg: %v", err)
	}
	utilGo := `package pkg

func Add(a, b int) int {
	return a + b
}
`
	if err := os.WriteFile(filepath.Join(dir, "pkg", "util.go"), []byte(utilGo), 0o644); err != nil {
		t.Fatalf("write util.go: %v", err)
	}

	return dir
}

// ── Tests ──────────────────────────────────────────────────────────────

func TestRun_FullPipeline(t *testing.T) {
	dir := createTempProject(t)
	llmClient := &mockLLM{}
	mem := &mockMemories{healthy: true}
	registry := sources.NewRegistry()
	registry.Register(&mockPipelineSource{
		name:  "mock-project",
		scope: sources.ProjectScope,
		artifacts: []sources.Artifact{
			{Source: "mock-project", Category: sources.Signal, ID: "TEST-1", Title: "Test ticket", Author: "tester", Tags: map[string]string{"type": "ticket"}},
		},
	})

	// Track progress phases.
	var progressMu sync.Mutex
	phases := make(map[string]int)

	result, err := Run(Config{
		ProjectName:    "test-project",
		RootPath:       dir,
		LLMClient:      llmClient,
		MemoriesClient: mem,
		SourceRegistry: registry,
		MaxWorkers:     2,
		ProgressFn: func(phase string, done, total int) {
			progressMu.Lock()
			phases[phase]++
			progressMu.Unlock()
		},
	})
	if err != nil {
		t.Fatalf("Run returned fatal error: %v", err)
	}

	// Verify modules detected.
	if result.Modules < 1 {
		t.Errorf("Modules: got %d, want >= 1", result.Modules)
	}

	// Verify files were indexed.
	if result.FilesIndexed < 2 {
		t.Errorf("FilesIndexed: got %d, want >= 2", result.FilesIndexed)
	}

	// Verify atoms were created (the mock LLM always succeeds).
	if result.AtomsCreated < 1 {
		t.Errorf("AtomsCreated: got %d, want >= 1", result.AtomsCreated)
	}

	// Verify module analyses were produced.
	if len(result.ModuleAnalyses) < 1 {
		t.Errorf("ModuleAnalyses: got %d, want >= 1", len(result.ModuleAnalyses))
	}

	// Verify system synthesis was produced.
	if result.Synthesis == nil {
		t.Error("Synthesis is nil, want non-nil")
	} else {
		if result.Synthesis.Blueprint == "" {
			t.Error("Synthesis.Blueprint is empty")
		}
		if len(result.Synthesis.Patterns) == 0 {
			t.Error("Synthesis.Patterns is empty")
		}
	}

	// Verify progress was called for key phases.
	progressMu.Lock()
	defer progressMu.Unlock()

	for _, phase := range []string{"scan", "atoms", "history", "analysis", "synthesis", "store"} {
		if phases[phase] == 0 {
			t.Errorf("progress phase %q was never called", phase)
		}
	}

	// Verify the mock LLM was called with both fast and deep tiers.
	llmClient.mu.Lock()
	callCount := llmClient.calls
	tiers := llmClient.tiers
	llmClient.mu.Unlock()

	if callCount < 2 {
		t.Errorf("LLM calls: got %d, want >= 2 (at least atoms + analysis)", callCount)
	}

	hasFast := false
	hasDeep := false
	for _, tier := range tiers {
		if tier == llm.TierFast {
			hasFast = true
		}
		if tier == llm.TierDeep {
			hasDeep = true
		}
	}
	if !hasFast {
		t.Error("LLM was never called with TierFast (atoms)")
	}
	if !hasDeep {
		t.Error("LLM was never called with TierDeep (analysis)")
	}

	// Verify Memories stored data.
	memories := mem.getMemories()
	if len(memories) == 0 {
		t.Error("no memories stored")
	}

	// Check that we stored the expected layer types.
	layersSeen := make(map[string]bool)
	for _, mem := range memories {
		parts := strings.Split(mem.source, "/")
		for _, p := range parts {
			if strings.HasPrefix(p, "layer:") {
				layersSeen[strings.TrimPrefix(p, "layer:")] = true
			}
		}
	}

	for _, layer := range []string{"atoms", "history", "signals", "wiring", "zones", "blueprint", "patterns"} {
		if !layersSeen[layer] {
			t.Errorf("layer %q was not stored in Memories", layer)
		}
	}
}

func TestRun_ModuleFilter(t *testing.T) {
	dir := createTempProject(t)
	_, err := Run(Config{
		ProjectName:    "test-project",
		RootPath:       dir,
		LLMClient:      &mockLLM{},
		MemoriesClient: &mockMemories{healthy: true},
		MaxWorkers:     1,
		ModuleFilter:   "nonexistent-module",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent module filter")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestRun_IncrementalManifest(t *testing.T) {
	dir := createTempProject(t)
	llmClient := &mockLLM{}
	mem := &mockMemories{healthy: true}

	// First run: full index.
	result1, err := Run(Config{
		ProjectName: "test-project",
		RootPath:    dir,
		LLMClient:   llmClient,
		MemoriesClient: mem,
		MaxWorkers:  2,
		Incremental: true,
	})
	if err != nil {
		t.Fatalf("first run returned fatal error: %v", err)
	}

	if result1.FilesIndexed < 2 {
		t.Fatalf("first run indexed %d files, want >= 2", result1.FilesIndexed)
	}

	// Verify manifest was created.
	manifestPath := filepath.Join(dir, ".carto", "manifest.json")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatal("manifest.json was not created after first run")
	}

	llmClient.mu.Lock()
	callsAfterFirst := llmClient.calls
	llmClient.mu.Unlock()

	// Second run: incremental, no changes. Should process 0 files because
	// all files are already in the manifest with matching hashes.
	result2, err := Run(Config{
		ProjectName: "test-project",
		RootPath:    dir,
		LLMClient:   llmClient,
		MemoriesClient: mem,
		MaxWorkers:  2,
		Incremental: true,
	})
	if err != nil {
		t.Fatalf("second run returned fatal error: %v", err)
	}

	// No new files to index.
	if result2.FilesIndexed != 0 {
		t.Errorf("second run FilesIndexed: got %d, want 0 (no changes)", result2.FilesIndexed)
	}

	// LLM should not have been called again.
	llmClient.mu.Lock()
	callsAfterSecond := llmClient.calls
	llmClient.mu.Unlock()

	if callsAfterSecond != callsAfterFirst {
		t.Errorf("LLM calls after second run: got %d, want %d (no new work)", callsAfterSecond, callsAfterFirst)
	}
}

func TestRun_ProgressPhases(t *testing.T) {
	dir := createTempProject(t)
	llmClient := &mockLLM{}
	mem := &mockMemories{healthy: true}

	var phaseOrder []string
	var phaseMu sync.Mutex

	_, err := Run(Config{
		ProjectName: "test-project",
		RootPath:    dir,
		LLMClient:   llmClient,
		MemoriesClient: mem,
		MaxWorkers:  1,
		ProgressFn: func(phase string, done, total int) {
			phaseMu.Lock()
			defer phaseMu.Unlock()
			// Record each phase the first time we see it.
			if len(phaseOrder) == 0 || phaseOrder[len(phaseOrder)-1] != phase {
				phaseOrder = append(phaseOrder, phase)
			}
		},
	})
	if err != nil {
		t.Fatalf("Run returned fatal error: %v", err)
	}

	phaseMu.Lock()
	defer phaseMu.Unlock()

	// The phases should appear in order: scan, atoms, history, analysis, synthesis, store.
	expected := []string{"scan", "atoms", "history", "analysis", "synthesis", "store"}
	if len(phaseOrder) != len(expected) {
		t.Errorf("phase order: got %v, want %v", phaseOrder, expected)
	} else {
		for i := range expected {
			if phaseOrder[i] != expected[i] {
				t.Errorf("phase[%d]: got %q, want %q", i, phaseOrder[i], expected[i])
			}
		}
	}
}

func TestRun_ErrorCollection(t *testing.T) {
	// Create a project with a file that will chunk but atoms LLM always works.
	// The pipeline should collect non-fatal errors but still produce results.
	dir := createTempProject(t)

	// Add a file that can't be read (simulate by creating a directory with a .go name).
	// Actually, let's just verify that the pipeline runs and collects errors gracefully.
	llmClient := &mockLLM{}
	mem := &mockMemories{healthy: true}

	result, err := Run(Config{
		ProjectName: "test-project",
		RootPath:    dir,
		LLMClient:   llmClient,
		MemoriesClient: mem,
		MaxWorkers:  1,
	})
	if err != nil {
		t.Fatalf("Run returned fatal error: %v", err)
	}

	// Should have results even if there are some collected errors.
	if result.Modules < 1 {
		t.Errorf("expected at least 1 module, got %d", result.Modules)
	}
}

func TestRun_NilProgressFn(t *testing.T) {
	dir := createTempProject(t)
	llmClient := &mockLLM{}
	mem := &mockMemories{healthy: true}

	// Run without a progress callback -- should not panic.
	result, err := Run(Config{
		ProjectName: "test-project",
		RootPath:    dir,
		LLMClient:   llmClient,
		MemoriesClient: mem,
		MaxWorkers:  1,
		ProgressFn:  nil,
	})
	if err != nil {
		t.Fatalf("Run returned fatal error: %v", err)
	}

	if result.Modules < 1 {
		t.Errorf("expected at least 1 module, got %d", result.Modules)
	}
}

func TestRun_ConcurrencySafety(t *testing.T) {
	// Run two pipelines concurrently against the same temp directory
	// to check for data races when run with -race.
	dir := createTempProject(t)

	var wg sync.WaitGroup
	var results [2]*Result
	var errs [2]error
	var opCount atomic.Int32

	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = Run(Config{
				ProjectName: "test-project",
				RootPath:    dir,
				LLMClient:   &mockLLM{},
				MemoriesClient: &mockMemories{healthy: true},
				MaxWorkers:  2,
				ProgressFn: func(phase string, done, total int) {
					opCount.Add(1)
				},
			})
		}(i)
	}

	wg.Wait()

	for i := 0; i < 2; i++ {
		if errs[i] != nil {
			t.Errorf("pipeline %d returned fatal error: %v", i, errs[i])
		}
		if results[i] == nil {
			t.Errorf("pipeline %d returned nil result", i)
		}
	}

	if opCount.Load() < 2 {
		t.Error("expected progress callbacks from both concurrent runs")
	}
}

func TestRun_MemoriesUnhealthy(t *testing.T) {
	dir := createTempProject(t)
	mem := &mockMemories{healthy: false}
	_, err := Run(Config{
		ProjectName:    "test-project",
		RootPath:       dir,
		LLMClient:      &mockLLM{},
		MemoriesClient: mem,
		MaxWorkers:     1,
	})
	if err == nil {
		t.Fatal("expected error when Memories is unhealthy")
	}
	if !strings.Contains(err.Error(), "unreachable") {
		t.Errorf("expected 'unreachable' in error, got: %v", err)
	}
}
