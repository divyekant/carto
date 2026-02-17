package atoms

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/anthropic/indexer/internal/llm"
)

// mockLLM implements LLMClient for testing.
type mockLLM struct {
	mu       sync.Mutex
	response string
	calls    int
	prompts  []string
}

func (m *mockLLM) CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++
	m.prompts = append(m.prompts, prompt)
	return json.RawMessage(m.response), nil
}

// errorLLM returns an error for specific call indices (0-based).
type errorLLM struct {
	mu        sync.Mutex
	calls     int
	errorOn   map[int]bool
	validResp string
}

func (m *errorLLM) CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error) {
	m.mu.Lock()
	idx := m.calls
	m.calls++
	shouldError := m.errorOn[idx]
	m.mu.Unlock()

	if shouldError {
		return nil, fmt.Errorf("simulated LLM error")
	}
	return json.RawMessage(m.validResp), nil
}

func sampleChunk() Chunk {
	return Chunk{
		Name:      "processData",
		Kind:      "function",
		Language:  "go",
		FilePath:  "pkg/data/process.go",
		StartLine: 10,
		EndLine:   25,
		Code:      "func processData(d []byte) error {\n\tx := len(d)\n\treturn nil\n}",
	}
}

const validResponse = `{
	"clarified_code": "func processData(data []byte) error {\n\tdataLength := len(data)\n\treturn nil\n}",
	"summary": "Processes raw byte data. Currently a stub that accepts data but performs no transformation.",
	"imports": ["fmt"],
	"exports": ["processData"]
}`

func TestAnalyzeChunk_Basic(t *testing.T) {
	mock := &mockLLM{response: validResponse}
	analyzer := NewAnalyzer(mock)

	chunk := sampleChunk()
	atom, err := analyzer.AnalyzeChunk(chunk)
	if err != nil {
		t.Fatalf("AnalyzeChunk returned error: %v", err)
	}

	// Verify fields carried from chunk.
	if atom.Name != chunk.Name {
		t.Errorf("Name: got %q, want %q", atom.Name, chunk.Name)
	}
	if atom.Kind != chunk.Kind {
		t.Errorf("Kind: got %q, want %q", atom.Kind, chunk.Kind)
	}
	if atom.FilePath != chunk.FilePath {
		t.Errorf("FilePath: got %q, want %q", atom.FilePath, chunk.FilePath)
	}
	if atom.StartLine != chunk.StartLine {
		t.Errorf("StartLine: got %d, want %d", atom.StartLine, chunk.StartLine)
	}
	if atom.EndLine != chunk.EndLine {
		t.Errorf("EndLine: got %d, want %d", atom.EndLine, chunk.EndLine)
	}

	// Verify fields from LLM response.
	if atom.Summary == "" {
		t.Error("Summary should not be empty")
	}
	if atom.ClarifiedCode == "" {
		t.Error("ClarifiedCode should not be empty")
	}
	if len(atom.Imports) == 0 {
		t.Error("Imports should not be empty")
	}
	if atom.Imports[0] != "fmt" {
		t.Errorf("Imports[0]: got %q, want %q", atom.Imports[0], "fmt")
	}
	if len(atom.Exports) == 0 {
		t.Error("Exports should not be empty")
	}
	if atom.Exports[0] != "processData" {
		t.Errorf("Exports[0]: got %q, want %q", atom.Exports[0], "processData")
	}

	// Verify the mock was called exactly once.
	if mock.calls != 1 {
		t.Errorf("LLM calls: got %d, want 1", mock.calls)
	}
}

func TestAnalyzeChunk_PromptContainsCode(t *testing.T) {
	mock := &mockLLM{response: validResponse}
	analyzer := NewAnalyzer(mock)

	chunk := sampleChunk()
	_, err := analyzer.AnalyzeChunk(chunk)
	if err != nil {
		t.Fatalf("AnalyzeChunk returned error: %v", err)
	}

	if len(mock.prompts) == 0 {
		t.Fatal("no prompts captured")
	}

	prompt := mock.prompts[0]

	// The prompt must contain the chunk's code.
	if !strings.Contains(prompt, chunk.Code) {
		t.Errorf("prompt does not contain chunk code.\nPrompt:\n%s", prompt)
	}

	// The prompt must reference the language, kind, name, and file path.
	if !strings.Contains(prompt, chunk.Language) {
		t.Errorf("prompt does not contain language %q", chunk.Language)
	}
	if !strings.Contains(prompt, chunk.Kind) {
		t.Errorf("prompt does not contain kind %q", chunk.Kind)
	}
	if !strings.Contains(prompt, chunk.Name) {
		t.Errorf("prompt does not contain name %q", chunk.Name)
	}
	if !strings.Contains(prompt, chunk.FilePath) {
		t.Errorf("prompt does not contain file path %q", chunk.FilePath)
	}
}

func TestAnalyzeBatch_Parallel(t *testing.T) {
	mock := &mockLLM{response: validResponse}
	analyzer := NewAnalyzer(mock)

	chunks := make([]Chunk, 5)
	for i := range chunks {
		chunks[i] = Chunk{
			Name:      fmt.Sprintf("func%d", i),
			Kind:      "function",
			Language:  "go",
			FilePath:  fmt.Sprintf("pkg/f%d.go", i),
			StartLine: i * 10,
			EndLine:   i*10 + 9,
			Code:      fmt.Sprintf("func func%d() {}", i),
		}
	}

	var progressCalls atomic.Int32
	var lastDone, lastTotal atomic.Int32

	atoms, err := analyzer.AnalyzeBatch(chunks, 2, func(done, total int) {
		progressCalls.Add(1)
		lastDone.Store(int32(done))
		lastTotal.Store(int32(total))
	})
	if err != nil {
		t.Fatalf("AnalyzeBatch returned error: %v", err)
	}

	// All 5 chunks should produce atoms.
	if len(atoms) != 5 {
		t.Errorf("got %d atoms, want 5", len(atoms))
	}

	// Progress should have been called 5 times.
	if pc := progressCalls.Load(); pc != 5 {
		t.Errorf("progress called %d times, want 5", pc)
	}

	// Final progress should report 5/5.
	if ld := lastDone.Load(); ld != 5 {
		t.Errorf("last done: got %d, want 5", ld)
	}
	if lt := lastTotal.Load(); lt != 5 {
		t.Errorf("last total: got %d, want 5", lt)
	}

	// LLM should have been called 5 times.
	mock.mu.Lock()
	calls := mock.calls
	mock.mu.Unlock()
	if calls != 5 {
		t.Errorf("LLM calls: got %d, want 5", calls)
	}
}

func TestAnalyzeBatch_SkipsErrors(t *testing.T) {
	// Errors on calls 1 and 3 (0-indexed).
	mock := &errorLLM{
		errorOn:   map[int]bool{1: true, 3: true},
		validResp: validResponse,
	}
	analyzer := NewAnalyzer(mock)

	chunks := make([]Chunk, 5)
	for i := range chunks {
		chunks[i] = Chunk{
			Name:      fmt.Sprintf("func%d", i),
			Kind:      "function",
			Language:  "go",
			FilePath:  fmt.Sprintf("pkg/f%d.go", i),
			StartLine: i * 10,
			EndLine:   i*10 + 9,
			Code:      fmt.Sprintf("func func%d() {}", i),
		}
	}

	var progressCalls atomic.Int32

	// Use maxWorkers=1 so call order is deterministic.
	atoms, err := analyzer.AnalyzeBatch(chunks, 1, func(done, total int) {
		progressCalls.Add(1)
	})
	if err != nil {
		t.Fatalf("AnalyzeBatch returned error: %v", err)
	}

	// 2 of 5 chunks errored, so we should get 3 atoms.
	if len(atoms) != 3 {
		t.Errorf("got %d atoms, want 3 (2 errors skipped)", len(atoms))
	}

	// Progress should still be called for all 5 chunks.
	if pc := progressCalls.Load(); pc != 5 {
		t.Errorf("progress called %d times, want 5", pc)
	}
}
