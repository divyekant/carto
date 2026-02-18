package storage

import "fmt"

// Layer constants for tagging in Memories.
const (
	LayerAtoms     = "atoms"     // Layer 1a
	LayerHistory   = "history"   // Layer 1b
	LayerSignals   = "signals"   // Layer 1c
	LayerWiring    = "wiring"    // Layer 2
	LayerZones     = "zones"     // Layer 3
	LayerBlueprint = "blueprint" // Layer 4
	LayerPatterns  = "patterns"  // Layer 5
)

// allLayers is the complete ordered list of layers.
var allLayers = []string{
	LayerAtoms,
	LayerHistory,
	LayerSignals,
	LayerWiring,
	LayerZones,
	LayerBlueprint,
	LayerPatterns,
}

// tierLayers maps each tier to its required layers.
var tierLayers = map[Tier][]string{
	TierMini:     {LayerZones, LayerBlueprint},
	TierStandard: {LayerZones, LayerBlueprint, LayerAtoms, LayerWiring},
	TierFull:     {LayerZones, LayerBlueprint, LayerAtoms, LayerWiring, LayerHistory, LayerSignals},
}

// maxContentLen is the Memories content limit (50k) with a safety margin.
const maxContentLen = 49000

// Tier controls how much context to retrieve.
type Tier string

const (
	TierMini     Tier = "mini"     // zones + blueprint only (~5KB)
	TierStandard Tier = "standard" // + atom summaries + wiring (~50KB)
	TierFull     Tier = "full"     // + clarified code + history + signals (~500KB)
)

// MemoriesAPI is the interface Store uses from MemoriesClient.
// This enables testing with mocks instead of requiring a real HTTP server.
type MemoriesAPI interface {
	Health() (bool, error)
	AddMemory(m Memory) (int, error)
	AddBatch(memories []Memory) error
	Search(query string, opts SearchOptions) ([]SearchResult, error)
	ListBySource(source string, limit int) ([]SearchResult, error)
	DeleteBySource(prefix string) (int, error)
}

// Store provides domain-specific Memories storage for carto layers.
type Store struct {
	memories MemoriesAPI
	project  string
}

// NewStore creates a Store scoped to a project name.
func NewStore(memories MemoriesAPI, project string) *Store {
	return &Store{memories: memories, project: project}
}

// sourceTag returns the Memories source tag for a module and layer.
// Format: carto/{project}/{module}/layer:{layer}
func (s *Store) sourceTag(module, layer string) string {
	return fmt.Sprintf("carto/%s/%s/layer:%s", s.project, module, layer)
}

// StoreLayer stores content in Memories with the appropriate source tag.
// Content exceeding 49000 chars is truncated at the last newline boundary.
func (s *Store) StoreLayer(module, layer, content string) error {
	content = truncate(content, maxContentLen)
	_, err := s.memories.AddMemory(Memory{
		Text:   content,
		Source: s.sourceTag(module, layer),
	})
	return err
}

// StoreBatch stores multiple entries for a layer. Each entry gets the same
// source tag. Useful for storing individual atoms or other granular data.
func (s *Store) StoreBatch(module, layer string, entries []string) error {
	tag := s.sourceTag(module, layer)
	memories := make([]Memory, len(entries))
	for i, entry := range entries {
		memories[i] = Memory{
			Text:   truncate(entry, maxContentLen),
			Source: tag,
		}
	}
	return s.memories.AddBatch(memories)
}

// RetrieveByTier retrieves context at the requested tier level.
// Returns a map keyed by layer name containing the search results for each layer.
//
//   - mini: zones + blueprint
//   - standard: mini + atoms + wiring
//   - full: standard + history + signals
func (s *Store) RetrieveByTier(module string, tier Tier) (map[string][]SearchResult, error) {
	layers, ok := tierLayers[tier]
	if !ok {
		return nil, fmt.Errorf("unknown tier: %s", tier)
	}

	result := make(map[string][]SearchResult, len(layers))
	for _, layer := range layers {
		results, err := s.RetrieveLayer(module, layer)
		if err != nil {
			return nil, fmt.Errorf("retrieve layer %s: %w", layer, err)
		}
		result[layer] = results
	}
	return result, nil
}

// RetrieveLayer retrieves all entries for a specific layer using ListBySource.
func (s *Store) RetrieveLayer(module, layer string) ([]SearchResult, error) {
	return s.memories.ListBySource(s.sourceTag(module, layer), 0)
}

// ClearModule deletes all entries for a module across all layers.
func (s *Store) ClearModule(module string) error {
	for _, layer := range allLayers {
		tag := s.sourceTag(module, layer)
		if _, err := s.memories.DeleteBySource(tag); err != nil {
			return fmt.Errorf("clear layer %s: %w", layer, err)
		}
	}
	return nil
}

// ClearProject deletes all entries for the entire project.
func (s *Store) ClearProject() error {
	prefix := fmt.Sprintf("carto/%s/", s.project)
	_, err := s.memories.DeleteBySource(prefix)
	return err
}

// truncate shortens content to at most maxLen characters. It cuts at the last
// newline before maxLen to avoid splitting mid-line. If no newline is found,
// it truncates at maxLen exactly.
func truncate(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}

	// Look for the last newline within the allowed range.
	cut := content[:maxLen]
	lastNL := -1
	for i := len(cut) - 1; i >= 0; i-- {
		if cut[i] == '\n' {
			lastNL = i
			break
		}
	}

	if lastNL > 0 {
		return content[:lastNL]
	}
	// No newline found; hard truncate.
	return cut
}
