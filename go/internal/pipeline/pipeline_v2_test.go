package pipeline

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"

	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/sources"
	"github.com/divyekant/carto/internal/storage"
)

// ── TestPipeline_V2_AtomMetadata ───────────────────────────────────────
// Verifies that atoms stored via UpsertBatch carry the required v2 metadata
// fields: name, kind, filepath, module, language. Also checks DocumentAt is set.

func TestPipeline_V2_AtomMetadata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := createTempProject(t)
	llmClient := &mockLLM{}
	mem := &mockMemories{healthy: true}

	result, err := Run(Config{
		ProjectName:    "v2-meta-test",
		RootPath:       dir,
		LLMClient:      llmClient,
		MemoriesClient: mem,
		MaxWorkers:     2,
		SkipSkillFiles: true,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result.AtomsCreated < 1 {
		t.Fatalf("expected at least one atom, got %d", result.AtomsCreated)
	}

	memories := mem.getMemories()

	// Find atom memories (source contains layer:atoms).
	var atomMems []storedMemory
	for _, m := range memories {
		if strings.Contains(m.source, "layer:atoms") {
			atomMems = append(atomMems, m)
		}
	}
	if len(atomMems) == 0 {
		t.Fatal("no atom memories stored")
	}

	requiredFields := []string{"name", "kind", "filepath", "module", "language"}

	var documentAtSet int
	for _, am := range atomMems {
		if am.metadata == nil {
			t.Errorf("atom memory has nil metadata (source=%q)", am.source)
			continue
		}
		// All required fields must be present.
		for _, field := range requiredFields {
			if _, ok := am.metadata[field]; !ok {
				t.Errorf("atom metadata missing field %q (source=%q)", field, am.source)
			}
		}
		// name, kind, module must be non-empty strings for every atom.
		for _, field := range []string{"name", "kind", "module"} {
			val, _ := am.metadata[field].(string)
			if val == "" {
				t.Errorf("atom metadata field %q is empty (source=%q)", field, am.source)
			}
		}
		if am.documentAt != "" {
			documentAtSet++
		}
	}
	// At least some atoms should have DocumentAt set (files have mod times).
	if documentAtSet == 0 {
		t.Error("no atom memories have DocumentAt set")
	}

	// Verify AtomIDs map is populated.
	if len(result.AtomIDs) == 0 {
		t.Error("AtomIDs map is empty after run")
	}
}

// wiringLLM is a mock LLM that returns wiring edges with fully-qualified module
// names matching what the scanner produces. This ensures CreateLink is called
// rather than skipped due to atom ID lookup failures.
type wiringLLM struct {
	moduleName string // set after first deep call to match scanner output
	mu         sync.Mutex
	calls      int
}

func (m *wiringLLM) CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls++

	switch tier {
	case llm.TierFast:
		return json.RawMessage(`{
			"clarified_code": "func main() {}",
			"summary": "Entry point function.",
			"imports": ["fmt"],
			"exports": ["main"]
		}`), nil
	case llm.TierDeep:
		if strings.Contains(prompt, "Synthesize") {
			return json.RawMessage(`{
				"blueprint": "A test system.",
				"patterns": ["single-binary"]
			}`), nil
		}
		// Return wiring with from_module/to_module set to the module name
		// extracted from the prompt so findAtomID can resolve the IDs.
		// The module name appears in the prompt as "Module: <name>".
		mod := m.moduleName
		if mod == "" {
			mod = "example.com/testproject"
		}
		return json.RawMessage(`{
			"module_name": "",
			"wiring": [{"from_atom": "main", "to_atom": "helper", "from_module": "` + mod + `", "to_module": "` + mod + `", "link_type": "calls", "reason": "main calls helper"}],
			"zones": [{"name": "core", "intent": "main logic", "files": ["main.go"]}],
			"module_intent": "Test module."
		}`), nil
	}
	return json.RawMessage(`{}`), nil
}

// ── TestPipeline_V2_WiringCreatesLinks ─────────────────────────────────
// Verifies that when the pipeline processes a project with cross-module
// dependencies, it calls CreateLink at least once (Phase 5 graph links).
//
// Uses a custom mock LLM that returns wiring edges with fully-qualified module
// names, ensuring findAtomID resolves the atom IDs and CreateLink is invoked.

func TestPipeline_V2_WiringCreatesLinks(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	dir := createTempProject(t)

	// wiringLLM returns wiring with the actual module name so CreateLink fires.
	llmClient := &wiringLLM{moduleName: "example.com/testproject"}

	// Use mockMemories (from pipeline_test.go) which tracks CreateLink calls.
	mem := &mockMemories{healthy: true}

	registry := sources.NewRegistry()

	result, err := Run(Config{
		ProjectName:    "v2-links-test",
		RootPath:       dir,
		LLMClient:      llmClient,
		MemoriesClient: mem,
		SourceRegistry: registry,
		MaxWorkers:     2,
		SkipSkillFiles: true,
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	// Verify wiring edges were produced by the deep analyzer.
	hasWiring := false
	for _, ma := range result.ModuleAnalyses {
		if len(ma.Wiring) > 0 {
			hasWiring = true
			break
		}
	}
	if !hasWiring {
		t.Fatal("no wiring edges produced — expected at least one from mock LLM")
	}

	// Verify AtomIDs were populated so link resolution can work.
	if len(result.AtomIDs) == 0 {
		t.Fatal("AtomIDs map is empty — cannot resolve wiring to graph links")
	}

	links := mem.getLinks()
	if len(links) == 0 {
		t.Error("CreateLink was never called — expected at least one wiring link when atom IDs are resolvable")
	}

	// Each link should have non-zero IDs and a non-empty type.
	for i, lnk := range links {
		if lnk.fromID == 0 || lnk.toID == 0 {
			t.Errorf("link[%d]: fromID=%d toID=%d — both must be non-zero", i, lnk.fromID, lnk.toID)
		}
		if lnk.linkType == "" {
			t.Errorf("link[%d]: linkType is empty", i)
		}
	}
}

// ── TestWriteback_MatchAndSupersede ────────────────────────────────────
// Unit-level test of the atom matching logic used by the writeback command.
// Given a set of "old" atoms (existing in Memories) and "new" atoms (from
// re-chunking a file), verifies the correct supersede/add/delete decisions.
//
// This mirrors the logic in cmd/carto/cmd_writeback.go:writebackFile without
// importing that package. The decisions are:
//   - new atom matches old by name+kind → supersede
//   - new atom has no match            → add
//   - old atom has no matching new     → delete

func TestWriteback_MatchAndSupersede(t *testing.T) {
	type atomKey struct {
		Name string
		Kind string
	}

	// oldAtoms simulates what ListBySource returns for a file currently in Memories.
	oldAtoms := []storage.SearchResult{
		{ID: 10, Metadata: map[string]any{"name": "main", "kind": "function", "filepath": "main.go"}},
		{ID: 11, Metadata: map[string]any{"name": "helper", "kind": "function", "filepath": "main.go"}},
		{ID: 12, Metadata: map[string]any{"name": "OldType", "kind": "type", "filepath": "main.go"}},
	}

	// newAtoms simulates what the analyzer produces after re-chunking the file.
	// "main" and "helper" still exist; "OldType" was deleted; "NewFunc" is new.
	type newAtom struct {
		Name string
		Kind string
	}
	newAtoms := []newAtom{
		{Name: "main", Kind: "function"},
		{Name: "helper", Kind: "function"},
		{Name: "NewFunc", Kind: "function"},
	}

	// Build old lookup map.
	oldByKey := make(map[atomKey]storage.SearchResult)
	for _, old := range oldAtoms {
		name, _ := old.Metadata["name"].(string)
		kind, _ := old.Metadata["kind"].(string)
		if name != "" {
			oldByKey[atomKey{Name: name, Kind: kind}] = old
		}
	}

	matchedOldIDs := make(map[int]bool)
	var superseded, added int

	for _, a := range newAtoms {
		key := atomKey{Name: a.Name, Kind: a.Kind}
		if old, found := oldByKey[key]; found {
			superseded++
			matchedOldIDs[old.ID] = true
		} else {
			added++
		}
	}

	// Count deletions: old atoms not matched by any new atom.
	var removed int
	for _, old := range oldAtoms {
		if !matchedOldIDs[old.ID] {
			removed++
		}
	}

	// Verify decisions.
	if superseded != 2 {
		t.Errorf("superseded: got %d, want 2 (main + helper)", superseded)
	}
	if added != 1 {
		t.Errorf("added: got %d, want 1 (NewFunc)", added)
	}
	if removed != 1 {
		t.Errorf("removed: got %d, want 1 (OldType)", removed)
	}

	// Verify the correct IDs were matched.
	if !matchedOldIDs[10] {
		t.Error("ID 10 (main) should be in matchedOldIDs")
	}
	if !matchedOldIDs[11] {
		t.Error("ID 11 (helper) should be in matchedOldIDs")
	}
	if matchedOldIDs[12] {
		t.Error("ID 12 (OldType) should NOT be in matchedOldIDs (it was deleted)")
	}
}

// TestWriteback_MatchAndSupersede_EmptyOld verifies that when there are no
// existing atoms (first writeback of a file), all new atoms are added.
func TestWriteback_MatchAndSupersede_EmptyOld(t *testing.T) {
	type atomKey struct {
		Name string
		Kind string
	}

	oldAtoms := []storage.SearchResult{} // nothing in Memories yet

	type newAtom struct {
		Name string
		Kind string
	}
	newAtoms := []newAtom{
		{Name: "main", Kind: "function"},
		{Name: "Config", Kind: "type"},
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
	var superseded, added int

	for _, a := range newAtoms {
		key := atomKey{Name: a.Name, Kind: a.Kind}
		if old, found := oldByKey[key]; found {
			superseded++
			matchedOldIDs[old.ID] = true
		} else {
			added++
		}
	}

	var removed int
	for _, old := range oldAtoms {
		if !matchedOldIDs[old.ID] {
			removed++
		}
	}

	if superseded != 0 {
		t.Errorf("superseded: got %d, want 0", superseded)
	}
	if added != 2 {
		t.Errorf("added: got %d, want 2", added)
	}
	if removed != 0 {
		t.Errorf("removed: got %d, want 0", removed)
	}
}

// TestWriteback_MatchAndSupersede_AllDeleted verifies that when the new file
// produces no atoms (e.g. all functions removed), all old atoms are deleted.
func TestWriteback_MatchAndSupersede_AllDeleted(t *testing.T) {
	type atomKey struct {
		Name string
		Kind string
	}

	oldAtoms := []storage.SearchResult{
		{ID: 20, Metadata: map[string]any{"name": "foo", "kind": "function", "filepath": "foo.go"}},
		{ID: 21, Metadata: map[string]any{"name": "bar", "kind": "function", "filepath": "foo.go"}},
	}

	type newAtom struct {
		Name string
		Kind string
	}
	newAtoms := []newAtom{} // file is now empty / all atoms removed

	oldByKey := make(map[atomKey]storage.SearchResult)
	for _, old := range oldAtoms {
		name, _ := old.Metadata["name"].(string)
		kind, _ := old.Metadata["kind"].(string)
		if name != "" {
			oldByKey[atomKey{Name: name, Kind: kind}] = old
		}
	}

	matchedOldIDs := make(map[int]bool)
	var superseded, added int

	for _, a := range newAtoms {
		key := atomKey{Name: a.Name, Kind: a.Kind}
		if old, found := oldByKey[key]; found {
			superseded++
			matchedOldIDs[old.ID] = true
		} else {
			added++
		}
	}

	var removed int
	for _, old := range oldAtoms {
		if !matchedOldIDs[old.ID] {
			removed++
		}
	}

	if superseded != 0 {
		t.Errorf("superseded: got %d, want 0", superseded)
	}
	if added != 0 {
		t.Errorf("added: got %d, want 0", added)
	}
	if removed != 2 {
		t.Errorf("removed: got %d, want 2", removed)
	}
}
