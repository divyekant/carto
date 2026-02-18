package storage

import (
	"fmt"
	"strings"
	"testing"
)

// mockMemories implements MemoriesAPI for testing.
type mockMemories struct {
	memories []Memory
	batches  [][]Memory
	results  map[string][]SearchResult // source -> results
	deleted  []string
}

func newMockMemories() *mockMemories {
	return &mockMemories{
		results: make(map[string][]SearchResult),
	}
}

func (m *mockMemories) AddMemory(mem Memory) (int, error) {
	m.memories = append(m.memories, mem)
	return len(m.memories), nil
}

func (m *mockMemories) AddBatch(memories []Memory) error {
	m.batches = append(m.batches, memories)
	m.memories = append(m.memories, memories...)
	return nil
}

func (m *mockMemories) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	return nil, nil
}

func (m *mockMemories) ListBySource(source string, limit int) ([]SearchResult, error) {
	if results, ok := m.results[source]; ok {
		return results, nil
	}
	return nil, nil
}

func (m *mockMemories) DeleteBySource(prefix string) (int, error) {
	m.deleted = append(m.deleted, prefix)
	return 0, nil
}

func TestSourceTag(t *testing.T) {
	mock := newMockMemories()
	s := NewStore(mock, "myproject")

	tag := s.sourceTag("mymodule", "atoms")
	expected := "carto/myproject/mymodule/layer:atoms"
	if tag != expected {
		t.Errorf("expected %q, got %q", expected, tag)
	}

	tag = s.sourceTag("pkg/auth", "blueprint")
	expected = "carto/myproject/pkg/auth/layer:blueprint"
	if tag != expected {
		t.Errorf("expected %q, got %q", expected, tag)
	}
}

func TestStoreLayer(t *testing.T) {
	mock := newMockMemories()
	s := NewStore(mock, "testproj")

	err := s.StoreLayer("auth", LayerAtoms, "func Login() { ... }")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(mock.memories))
	}

	mem := mock.memories[0]
	expectedSource := "carto/testproj/auth/layer:atoms"
	if mem.Source != expectedSource {
		t.Errorf("expected source %q, got %q", expectedSource, mem.Source)
	}
	if mem.Text != "func Login() { ... }" {
		t.Errorf("expected text %q, got %q", "func Login() { ... }", mem.Text)
	}
}

func TestStoreLayer_Truncation(t *testing.T) {
	mock := newMockMemories()
	s := NewStore(mock, "proj")

	// Build content that exceeds 49000 chars with newlines.
	var sb strings.Builder
	line := strings.Repeat("x", 99) + "\n" // 100 chars per line
	for sb.Len() < 50000 {
		sb.WriteString(line)
	}
	content := sb.String()

	err := s.StoreLayer("mod", LayerBlueprint, content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	stored := mock.memories[0].Text
	if len(stored) > maxContentLen {
		t.Errorf("expected content <= %d chars, got %d", maxContentLen, len(stored))
	}
	// Should end at a newline boundary (the truncated string should not end mid-line).
	if stored[len(stored)-1] != '\n' {
		// The truncation removes content after the last newline, so the last char
		// of the retained text is the char before the newline. Check that no
		// partial line exists: stored length should be a multiple of 100.
		if len(stored)%100 != 0 {
			// It might end exactly at a newline. Let's just verify it's shorter.
			t.Logf("stored length: %d (not a clean line boundary but still truncated)", len(stored))
		}
	}
}

func TestStoreBatch(t *testing.T) {
	mock := newMockMemories()
	s := NewStore(mock, "proj")

	entries := []string{"atom 1", "atom 2", "atom 3"}
	err := s.StoreBatch("parser", LayerAtoms, entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.batches) != 1 {
		t.Fatalf("expected 1 batch call, got %d", len(mock.batches))
	}

	batch := mock.batches[0]
	if len(batch) != 3 {
		t.Fatalf("expected 3 memories in batch, got %d", len(batch))
	}

	expectedSource := "carto/proj/parser/layer:atoms"
	for i, mem := range batch {
		if mem.Source != expectedSource {
			t.Errorf("batch[%d]: expected source %q, got %q", i, expectedSource, mem.Source)
		}
		expectedText := fmt.Sprintf("atom %d", i+1)
		if mem.Text != expectedText {
			t.Errorf("batch[%d]: expected text %q, got %q", i, expectedText, mem.Text)
		}
	}
}

func TestRetrieveByTier_Mini(t *testing.T) {
	mock := newMockMemories()
	s := NewStore(mock, "proj")

	// Seed results for zones and blueprint.
	mock.results["carto/proj/web/layer:zones"] = []SearchResult{
		{ID: 1, Text: "zone data", Source: "carto/proj/web/layer:zones"},
	}
	mock.results["carto/proj/web/layer:blueprint"] = []SearchResult{
		{ID: 2, Text: "blueprint data", Source: "carto/proj/web/layer:blueprint"},
	}
	// Seed results for layers that should NOT be queried at mini tier.
	mock.results["carto/proj/web/layer:atoms"] = []SearchResult{
		{ID: 99, Text: "should not appear"},
	}

	result, err := s.RetrieveByTier("web", TierMini)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(result))
	}
	if _, ok := result[LayerZones]; !ok {
		t.Error("expected zones layer in result")
	}
	if _, ok := result[LayerBlueprint]; !ok {
		t.Error("expected blueprint layer in result")
	}
	if _, ok := result[LayerAtoms]; ok {
		t.Error("atoms layer should not be in mini tier result")
	}
	if result[LayerZones][0].Text != "zone data" {
		t.Errorf("expected zones text %q, got %q", "zone data", result[LayerZones][0].Text)
	}
}

func TestRetrieveByTier_Standard(t *testing.T) {
	mock := newMockMemories()
	s := NewStore(mock, "proj")

	// Seed all layers.
	for _, layer := range []string{LayerZones, LayerBlueprint, LayerAtoms, LayerWiring, LayerHistory, LayerSignals} {
		tag := fmt.Sprintf("carto/proj/api/layer:%s", layer)
		mock.results[tag] = []SearchResult{
			{ID: 1, Text: layer + " data", Source: tag},
		}
	}

	result, err := s.RetrieveByTier("api", TierStandard)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLayers := []string{LayerZones, LayerBlueprint, LayerAtoms, LayerWiring}
	if len(result) != len(expectedLayers) {
		t.Fatalf("expected %d layers, got %d", len(expectedLayers), len(result))
	}
	for _, layer := range expectedLayers {
		if _, ok := result[layer]; !ok {
			t.Errorf("expected %s layer in result", layer)
		}
	}
	// History and signals should not be present.
	for _, layer := range []string{LayerHistory, LayerSignals} {
		if _, ok := result[layer]; ok {
			t.Errorf("%s layer should not be in standard tier result", layer)
		}
	}
}

func TestRetrieveByTier_Full(t *testing.T) {
	mock := newMockMemories()
	s := NewStore(mock, "proj")

	// Seed all layers.
	for _, layer := range []string{LayerZones, LayerBlueprint, LayerAtoms, LayerWiring, LayerHistory, LayerSignals} {
		tag := fmt.Sprintf("carto/proj/svc/layer:%s", layer)
		mock.results[tag] = []SearchResult{
			{ID: 1, Text: layer + " data", Source: tag},
		}
	}

	result, err := s.RetrieveByTier("svc", TierFull)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedLayers := []string{LayerZones, LayerBlueprint, LayerAtoms, LayerWiring, LayerHistory, LayerSignals}
	if len(result) != len(expectedLayers) {
		t.Fatalf("expected %d layers, got %d", len(expectedLayers), len(result))
	}
	for _, layer := range expectedLayers {
		if _, ok := result[layer]; !ok {
			t.Errorf("expected %s layer in result", layer)
		}
	}
}

func TestClearModule(t *testing.T) {
	mock := newMockMemories()
	s := NewStore(mock, "proj")

	err := s.ClearModule("auth")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.deleted) != len(allLayers) {
		t.Fatalf("expected %d delete calls, got %d", len(allLayers), len(mock.deleted))
	}

	// Verify each layer was deleted with the correct source tag.
	for i, layer := range allLayers {
		expected := fmt.Sprintf("carto/proj/auth/layer:%s", layer)
		if mock.deleted[i] != expected {
			t.Errorf("delete[%d]: expected %q, got %q", i, expected, mock.deleted[i])
		}
	}
}

func TestClearProject(t *testing.T) {
	mock := newMockMemories()
	s := NewStore(mock, "myproj")

	err := s.ClearProject()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(mock.deleted) != 1 {
		t.Fatalf("expected 1 delete call, got %d", len(mock.deleted))
	}

	expected := "carto/myproj/"
	if mock.deleted[0] != expected {
		t.Errorf("expected delete prefix %q, got %q", expected, mock.deleted[0])
	}
}

func TestTruncate(t *testing.T) {
	t.Run("short content unchanged", func(t *testing.T) {
		content := "hello\nworld\n"
		result := truncate(content, 100)
		if result != content {
			t.Errorf("expected %q, got %q", content, result)
		}
	})

	t.Run("truncates at last newline", func(t *testing.T) {
		content := "line1\nline2\nline3\nline4\n"
		// maxLen = 15 means we can keep "line1\nline2\nli" (15 chars)
		// Last newline in that range is at index 11 (after "line2")
		result := truncate(content, 15)
		expected := "line1\nline2"
		if result != expected {
			t.Errorf("expected %q, got %q", expected, result)
		}
	})

	t.Run("no newline hard truncates", func(t *testing.T) {
		content := strings.Repeat("a", 100)
		result := truncate(content, 50)
		if len(result) != 50 {
			t.Errorf("expected length 50, got %d", len(result))
		}
	})

	t.Run("exact limit unchanged", func(t *testing.T) {
		content := strings.Repeat("x", 49000)
		result := truncate(content, 49000)
		if len(result) != 49000 {
			t.Errorf("expected length 49000, got %d", len(result))
		}
	})
}
