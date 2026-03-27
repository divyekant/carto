# Carto v2: Memories-Native Storage Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Upgrade Carto to use Memories v5 features — supersede-based write-back, graph-native wiring, structured atom metadata, and 6-signal search.

**Architecture:** Pipeline-Native Graph approach. Storage client gets new methods (upsert, supersede, links, advanced search). Pipeline Phase 2 stores atoms with metadata and returns IDs. Phase 4 outputs typed edges instead of JSON blobs. Phase 5 creates graph links. New `carto writeback` command enables file-level index updates. Breaking change — old indexes require full re-index.

**Tech Stack:** Go 1.25+, CGO (tree-sitter), Cobra CLI, Memories v5 REST API, React/TypeScript (web UI)

**Spec:** `docs/superpowers/specs/2026-03-26-carto-v2-memories-native-design.md`

---

## Task 1: Atom Struct — Add Language and Module Fields

**Files:**
- Modify: `go/internal/atoms/analyzer.go:25-35` (Atom struct)
- Test: `go/internal/atoms/analyzer_test.go`

- [ ] **Step 1: Write test for Language and Module fields on Atom**

```go
func TestAtom_HasLanguageAndModule(t *testing.T) {
	a := &Atom{
		Name:     "handleAuth",
		Kind:     "function",
		FilePath: "src/auth/handler.go",
		Language: "go",
		Module:   "auth",
		Summary:  "Validates JWT tokens",
	}
	if a.Language != "go" {
		t.Errorf("expected Language=go, got %s", a.Language)
	}
	if a.Module != "auth" {
		t.Errorf("expected Module=auth, got %s", a.Module)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/atoms/ -run TestAtom_HasLanguageAndModule -v`
Expected: FAIL — `a.Language undefined`

- [ ] **Step 3: Add Language and Module to Atom struct**

In `go/internal/atoms/analyzer.go`, add two fields to the `Atom` struct (after `EndLine`):

```go
type Atom struct {
	Name         string   `json:"name"`
	Kind         string   `json:"kind"`
	FilePath     string   `json:"file_path"`
	Summary      string   `json:"summary"`
	ClarifiedCode string  `json:"clarified_code"`
	Imports      []string `json:"imports"`
	Exports      []string `json:"exports"`
	StartLine    int      `json:"start_line"`
	EndLine      int      `json:"end_line"`
	Language     string   `json:"language"`
	Module       string   `json:"module"`
}
```

- [ ] **Step 4: Thread Language from Chunk to Atom in AnalyzeChunk**

In `AnalyzeChunk` (~line 88-117), after constructing the Atom from LLM response, set `Language` from the input chunk:

```go
atom := &Atom{
	Name:          chunk.Name,
	Kind:          chunk.Kind,
	FilePath:      chunk.FilePath,
	Summary:       resp.Summary,
	ClarifiedCode: resp.ClarifiedCode,
	Imports:       resp.Imports,
	Exports:       resp.Exports,
	StartLine:     chunk.StartLine,
	EndLine:       chunk.EndLine,
	Language:      chunk.Language,
}
```

Note: `Module` is NOT set here — it's set by the pipeline after atom analysis returns (pipeline knows module context, atom analyzer doesn't).

- [ ] **Step 5: Run tests**

Run: `cd go && go test ./internal/atoms/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add go/internal/atoms/analyzer.go go/internal/atoms/analyzer_test.go
git commit -m "feat(atoms): add Language and Module fields to Atom struct"
```

---

## Task 2: Manifest Version Bump

**Files:**
- Modify: `go/internal/manifest/manifest.go:44` (version constant)
- Modify: `go/internal/manifest/manifest.go:54-87` (Load function)
- Test: `go/internal/manifest/manifest_test.go`

- [ ] **Step 1: Write test for v2 manifest version and v1 detection**

```go
func TestManifest_VersionV2(t *testing.T) {
	dir := t.TempDir()
	m := NewManifest(dir, "test-project")
	if m.Version != "2.0" {
		t.Errorf("expected Version=2.0, got %s", m.Version)
	}
}

func TestManifest_LoadV1ReturnsError(t *testing.T) {
	dir := t.TempDir()
	cartoDir := filepath.Join(dir, ".carto")
	os.MkdirAll(cartoDir, 0o755)
	data := []byte(`{"version":"1.0","project":"old","files":{}}`)
	os.WriteFile(filepath.Join(cartoDir, "manifest.json"), data, 0o644)

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for v1 manifest")
	}
	if !strings.Contains(err.Error(), "upgrade") {
		t.Errorf("error should mention upgrade, got: %s", err)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./internal/manifest/ -run "TestManifest_VersionV2|TestManifest_LoadV1" -v`
Expected: FAIL

- [ ] **Step 3: Update NewManifest to use version 2.0**

In `go/internal/manifest/manifest.go` line 44, change:

```go
Version: "2.0",
```

- [ ] **Step 4: Add v1 detection in Load**

In the `Load` function, after unmarshaling (around line 82), add version check:

```go
if m.Version == "1.0" {
	return nil, fmt.Errorf("manifest: v1.0 format detected — run 'carto index' to upgrade to v2 format")
}
```

- [ ] **Step 5: Run tests**

Run: `cd go && go test ./internal/manifest/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add go/internal/manifest/manifest.go go/internal/manifest/manifest_test.go
git commit -m "feat(manifest): bump version to 2.0 with v1 detection"
```

---

## Task 3: Storage — Struct Updates (Memory, SearchResult, SearchOptions)

**Files:**
- Modify: `go/internal/storage/memories.go:16-38` (structs)
- Test: `go/internal/storage/memories_test.go`

- [ ] **Step 1: Write test for new Memory fields**

```go
func TestMemory_NewFields(t *testing.T) {
	m := Memory{
		Text:       "test summary",
		Source:     "carto/proj/mod/layer:atoms",
		Metadata:   map[string]any{"name": "foo", "kind": "function", "module": "auth"},
		DocumentAt: "2026-03-26T00:00:00Z",
	}
	if m.DocumentAt != "2026-03-26T00:00:00Z" {
		t.Errorf("DocumentAt not set correctly")
	}
	if m.Metadata["name"] != "foo" {
		t.Errorf("Metadata not set correctly")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/storage/ -run TestMemory_NewFields -v`
Expected: FAIL — `m.DocumentAt undefined`

- [ ] **Step 3: Update Memory struct**

```go
type Memory struct {
	Text       string         `json:"text"`
	Source     string         `json:"source"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	DocumentAt string         `json:"document_at,omitempty"`
}
```

Remove the `Deduplicate` field. Update the existing test at `memories_test.go:84,104-106` that sets `Deduplicate: true` — remove that assertion since the field no longer exists.

- [ ] **Step 4: Update SearchResult struct**

```go
type SearchResult struct {
	ID           int            `json:"id"`
	Text         string         `json:"text"`
	Score        float64        `json:"score"`
	Source       string         `json:"source"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	MatchType    string         `json:"match_type,omitempty"`
	Confidence   float64        `json:"confidence,omitempty"`
	GraphSupport float64        `json:"graph_support,omitempty"`
}
```

Note: field rename from `Meta` to `Metadata` with same JSON tag `"metadata"`.

- [ ] **Step 5: Update SearchOptions struct**

```go
type SearchOptions struct {
	K                int     `json:"k,omitempty"`
	Threshold        float64 `json:"threshold,omitempty"`
	Hybrid           bool    `json:"hybrid,omitempty"`
	SourcePrefix     string  `json:"source_prefix,omitempty"`
	ConfidenceWeight float64 `json:"confidence_weight,omitempty"`
	FeedbackWeight   float64 `json:"feedback_weight,omitempty"`
	GraphWeight      float64 `json:"graph_weight,omitempty"`
	Since            string  `json:"since,omitempty"`
	Until            string  `json:"until,omitempty"`
}
```

- [ ] **Step 6: Add UpsertResult and Link types**

```go
type UpsertResult struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

type Link struct {
	ToID      int    `json:"to_id"`
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
}
```

- [ ] **Step 7: Fix all compilation errors from Meta→Metadata rename**

Search for `\.Meta` across the codebase and update to `.Metadata`. Confirmed affected files:
- `go/internal/storage/memories_test.go:162-163` — uses `.Meta["key"]`
- `go/cmd/carto/cmd_export.go:97` — uses `r.Meta`
- `go/cmd/carto/cmd_export.go:42-48` — `exportEntry` struct references `Metadata` (already correct JSON tag)
- Test files referencing `SearchResult.Meta`

Run: `cd go && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 8: Run all storage tests**

Run: `cd go && go test ./internal/storage/ -v`
Expected: ALL PASS

- [ ] **Step 9: Commit**

```bash
git add go/internal/storage/memories.go go/internal/storage/memories_test.go
git commit -m "feat(storage): update structs for Memories v5 (metadata, document_at, search signals)"
```

---

## Task 4: Storage — UpsertBatch and Supersede Methods

**Files:**
- Modify: `go/internal/storage/memories.go`
- Test: `go/internal/storage/memories_test.go`

- [ ] **Step 1: Write test for UpsertBatch**

```go
func TestMemoriesClient_UpsertBatch(t *testing.T) {
	var received []Memory
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/upsert-batch" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}
		var body struct {
			Memories []Memory `json:"memories"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		received = body.Memories
		results := make([]UpsertResult, len(body.Memories))
		for i := range body.Memories {
			results[i] = UpsertResult{ID: 100 + i, Status: "created"}
		}
		json.NewEncoder(w).Encode(map[string]any{"results": results})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	mems := []Memory{
		{Text: "atom1", Source: "carto/proj/mod/layer:atoms", Metadata: map[string]any{"name": "foo"}},
		{Text: "atom2", Source: "carto/proj/mod/layer:atoms", Metadata: map[string]any{"name": "bar"}},
	}
	results, err := client.UpsertBatch(mems)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].ID != 100 {
		t.Errorf("expected ID=100, got %d", results[0].ID)
	}
	if len(received) != 2 {
		t.Errorf("expected 2 memories sent, got %d", len(received))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/storage/ -run TestMemoriesClient_UpsertBatch -v`
Expected: FAIL — method not found

- [ ] **Step 3: Implement UpsertBatch**

Add to `memories.go`:

```go
func (c *MemoriesClient) UpsertBatch(memories []Memory) ([]UpsertResult, error) {
	const batchSize = 500
	var allResults []UpsertResult

	for i := 0; i < len(memories); i += batchSize {
		end := i + batchSize
		if end > len(memories) {
			end = len(memories)
		}
		batch := memories[i:end]

		resp, err := c.request("POST", "/memory/upsert-batch", map[string]any{
			"memories": batch,
		})
		if err != nil {
			return allResults, fmt.Errorf("upsert batch: %w", err)
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return allResults, fmt.Errorf("upsert batch: status %d: %s", resp.StatusCode, body)
		}

		var result struct {
			Results []UpsertResult `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return allResults, fmt.Errorf("upsert batch decode: %w", err)
		}
		resp.Body.Close()
		allResults = append(allResults, result.Results...)
	}
	return allResults, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go && go test ./internal/storage/ -run TestMemoriesClient_UpsertBatch -v`
Expected: PASS

- [ ] **Step 5: Write test for Supersede**

```go
func TestMemoriesClient_Supersede(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/supersede" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		var body struct {
			OldID int            `json:"old_id"`
			Text  string         `json:"text"`
			Meta  map[string]any `json:"metadata"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.OldID != 42 {
			t.Errorf("expected old_id=42, got %d", body.OldID)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"old_id":        42,
			"new_id":        99,
			"previous_text": "old text",
		})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	newID, err := client.Supersede(42, "new text", map[string]any{"name": "updated"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newID != 99 {
		t.Errorf("expected newID=99, got %d", newID)
	}
}
```

- [ ] **Step 6: Implement Supersede**

```go
func (c *MemoriesClient) Supersede(oldID int, newText string, newMeta map[string]any) (int, error) {
	resp, err := c.request("POST", "/memory/supersede", map[string]any{
		"old_id":   oldID,
		"text":     newText,
		"metadata": newMeta,
	})
	if err != nil {
		return 0, fmt.Errorf("supersede: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("supersede: status %d: %s", resp.StatusCode, body)
	}

	var result struct {
		NewID int `json:"new_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("supersede decode: %w", err)
	}
	return result.NewID, nil
}
```

- [ ] **Step 7: Run all storage tests**

Run: `cd go && go test ./internal/storage/ -v`
Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add go/internal/storage/memories.go go/internal/storage/memories_test.go
git commit -m "feat(storage): add UpsertBatch and Supersede methods for Memories v5"
```

---

## Task 5: Storage — Graph Link Methods

**Files:**
- Modify: `go/internal/storage/memories.go`
- Test: `go/internal/storage/memories_test.go`

- [ ] **Step 1: Write tests for CreateLink, GetLinks, DeleteLinks**

```go
func TestMemoriesClient_CreateLink(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/100/link" || r.Method != "POST" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		var body struct {
			ToID int    `json:"to_id"`
			Type string `json:"type"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.ToID != 200 || body.Type != "related_to" {
			t.Errorf("unexpected body: %+v", body)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	err := client.CreateLink(100, 200, "related_to")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoriesClient_GetLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/100/links" || r.Method != "GET" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"links": []Link{
				{ToID: 200, Type: "related_to", CreatedAt: "2026-03-26T00:00:00Z"},
			},
		})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	links, err := client.GetLinks(100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(links) != 1 || links[0].ToID != 200 {
		t.Errorf("unexpected links: %+v", links)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go && go test ./internal/storage/ -run "TestMemoriesClient_CreateLink|TestMemoriesClient_GetLinks" -v`
Expected: FAIL

- [ ] **Step 3: Implement CreateLink, GetLinks, DeleteLinks**

```go
func (c *MemoriesClient) CreateLink(fromID, toID int, linkType string) error {
	path := fmt.Sprintf("/memory/%d/link", fromID)
	resp, err := c.request("POST", path, map[string]any{
		"to_id": toID,
		"type":  linkType,
	})
	if err != nil {
		return fmt.Errorf("create link: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create link: status %d: %s", resp.StatusCode, body)
	}
	return nil
}

func (c *MemoriesClient) GetLinks(id int) ([]Link, error) {
	path := fmt.Sprintf("/memory/%d/links", id)
	resp, err := c.request("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("get links: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("get links: status %d: %s", resp.StatusCode, body)
	}
	var result struct {
		Links []Link `json:"links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("get links decode: %w", err)
	}
	return result.Links, nil
}

func (c *MemoriesClient) DeleteLinks(id int) error {
	links, err := c.GetLinks(id)
	if err != nil {
		return err
	}
	for _, link := range links {
		path := fmt.Sprintf("/memory/%d/link/%d", id, link.ToID)
		resp, err := c.request("DELETE", path, nil)
		if err != nil {
			log.Printf("warn: delete link %d->%d: %v", id, link.ToID, err)
			continue
		}
		resp.Body.Close()
	}
	return nil
}
```

- [ ] **Step 4: Run all storage tests**

Run: `cd go && go test ./internal/storage/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add go/internal/storage/memories.go go/internal/storage/memories_test.go
git commit -m "feat(storage): add graph link methods (CreateLink, GetLinks, DeleteLinks)"
```

---

## Task 6: Storage — SearchAdvanced Method

**Files:**
- Modify: `go/internal/storage/memories.go`
- Test: `go/internal/storage/memories_test.go`

- [ ] **Step 1: Write test for SearchAdvanced with all 6 signal weights**

```go
func TestMemoriesClient_SearchAdvanced(t *testing.T) {
	var receivedBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&receivedBody)
		json.NewEncoder(w).Encode(map[string]any{
			"results": []SearchResult{
				{ID: 1, Text: "result", Score: 0.9, MatchType: "direct", Confidence: 0.95},
			},
		})
	}))
	defer srv.Close()

	client := NewMemoriesClient(srv.URL, "test-key")
	opts := SearchOptions{
		K: 10, Hybrid: true, SourcePrefix: "carto/proj/",
		ConfidenceWeight: 0.1, FeedbackWeight: 0.1, GraphWeight: 0.2,
		Since: "2026-01-01T00:00:00Z", Until: "2026-03-26T00:00:00Z",
	}
	results, err := client.SearchAdvanced("test query", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].MatchType != "direct" {
		t.Errorf("unexpected results: %+v", results)
	}
	// Verify all params were sent
	if receivedBody["confidence_weight"] != 0.1 {
		t.Errorf("confidence_weight not sent: %v", receivedBody)
	}
	if receivedBody["graph_weight"] != 0.2 {
		t.Errorf("graph_weight not sent: %v", receivedBody)
	}
	if receivedBody["since"] != "2026-01-01T00:00:00Z" {
		t.Errorf("since not sent: %v", receivedBody)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/storage/ -run TestMemoriesClient_SearchAdvanced -v`
Expected: FAIL

- [ ] **Step 3: Implement SearchAdvanced**

Replace the existing `Search` method with `SearchAdvanced`:

```go
func (c *MemoriesClient) SearchAdvanced(query string, opts SearchOptions) ([]SearchResult, error) {
	body := map[string]any{
		"query":  query,
		"k":      opts.K,
		"hybrid": opts.Hybrid,
	}
	if opts.SourcePrefix != "" {
		body["source_prefix"] = opts.SourcePrefix
	}
	if opts.Threshold > 0 {
		body["threshold"] = opts.Threshold
	}
	if opts.ConfidenceWeight > 0 {
		body["confidence_weight"] = opts.ConfidenceWeight
	}
	if opts.FeedbackWeight > 0 {
		body["feedback_weight"] = opts.FeedbackWeight
	}
	if opts.GraphWeight > 0 {
		body["graph_weight"] = opts.GraphWeight
	}
	if opts.Since != "" {
		body["since"] = opts.Since
	}
	if opts.Until != "" {
		body["until"] = opts.Until
	}

	resp, err := c.request("POST", "/search", body)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("search: status %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("search decode: %w", err)
	}
	return result.Results, nil
}
```

- [ ] **Step 4: Remove old Search method**

Delete the `Search(query string, opts SearchOptions)` method entirely. It is replaced by `SearchAdvanced`.

- [ ] **Step 5: Run all storage tests**

Run: `cd go && go test ./internal/storage/ -v`
Expected: ALL PASS (some old tests may need updating to use SearchAdvanced)

- [ ] **Step 6: Commit**

```bash
git add go/internal/storage/memories.go go/internal/storage/memories_test.go
git commit -m "feat(storage): add SearchAdvanced with 6-signal support, remove old Search"
```

---

## Task 7: MemoriesAPI Interface Update

**Files:**
- Modify: `go/internal/storage/store.go:51-59` (interface)
- Modify: All files referencing the old interface (pipeline, server, CLI, tests)

- [ ] **Step 1: Update MemoriesAPI interface**

```go
type MemoriesAPI interface {
	Health() (bool, error)
	AddMemory(m Memory) (int, error)  // kept: used by StoreLayer for non-atom layers
	UpsertBatch(memories []Memory) ([]UpsertResult, error)
	Supersede(oldID int, newText string, newMeta map[string]any) (int, error)
	SearchAdvanced(query string, opts SearchOptions) ([]SearchResult, error)
	ListBySource(source string, limit, offset int) ([]SearchResult, error)
	DeleteMemory(id int) error
	DeleteBySource(prefix string) (int, error)
	Count(sourcePrefix string) (int, error)
	CreateLink(fromID, toID int, linkType string) error
	GetLinks(id int) ([]Link, error)
	DeleteLinks(id int) error
}
// Note: AddMemory is kept because StoreLayer (used for zones, blueprint, history,
// signals, patterns) still calls it. Only atom storage moves to UpsertBatch.
```

- [ ] **Step 2: Update Store methods that used old interface**

In `store.go`, update `StoreBatch` to use `UpsertBatch` internally:

```go
func (s *Store) StoreBatch(module, layer string, entries []string) error {
	tag := s.sourceTag(module, layer)
	memories := make([]Memory, len(entries))
	for i, entry := range entries {
		memories[i] = Memory{Text: truncate(entry, maxContentLen), Source: tag}
	}
	_, err := s.memories.UpsertBatch(memories)
	return err
}
```

- [ ] **Step 3: Fix all compilation errors**

Find all mock implementations of `MemoriesAPI` and update them to implement the new 12-method interface. The mock at `go/internal/storage/store_test.go:23-60` currently has 7 methods — add stubs for `UpsertBatch`, `Supersede`, `CreateLink`, `GetLinks`, `DeleteLinks`, `DeleteMemory`, and rename `Search` to `SearchAdvanced`. Also check `go/internal/pipeline/pipeline_test.go` and `go/internal/server/handlers_test.go` for mock implementations.

Update all call sites: `Search(` → `SearchAdvanced(`, `AddBatch(` → `UpsertBatch(` (in `cmd_import.go:122,136`).

Run: `cd go && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 4: Run full test suite**

Run: `cd go && go test ./... -short`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add -A
git commit -m "refactor(storage): update MemoriesAPI interface for v5 methods"
```

---

## Task 8: Deep Analyzer — WiringEdge Output Format

**Files:**
- Modify: `go/internal/analyzer/deep.go:32-36` (Dependency struct → WiringEdge)
- Modify: `go/internal/analyzer/deep.go:80-146` (buildModulePrompt)
- Test: `go/internal/analyzer/deep_test.go`

- [ ] **Step 1: Write test for WiringEdge struct and parsing**

```go
func TestWiringEdge_ParseFromJSON(t *testing.T) {
	raw := `{
		"module_name": "nb",
		"wiring": [
			{
				"from_atom": "TranslationManager",
				"from_module": "nb",
				"to_atom": "EmailDelivery",
				"to_module": "delivery-email",
				"link_type": "related_to",
				"reason": "NB dispatches email via EmailDelivery.send()"
			}
		],
		"zones": [],
		"module_intent": "test"
	}`

	var analysis struct {
		Wiring []WiringEdge `json:"wiring"`
	}
	err := json.Unmarshal([]byte(raw), &analysis)
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(analysis.Wiring) != 1 {
		t.Fatalf("expected 1 edge, got %d", len(analysis.Wiring))
	}
	edge := analysis.Wiring[0]
	if edge.FromAtom != "TranslationManager" || edge.ToModule != "delivery-email" {
		t.Errorf("unexpected edge: %+v", edge)
	}
	if edge.LinkType != "related_to" {
		t.Errorf("expected link_type=related_to, got %s", edge.LinkType)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/analyzer/ -run TestWiringEdge_ParseFromJSON -v`
Expected: FAIL — `WiringEdge` undefined

- [ ] **Step 3: Replace Dependency with WiringEdge**

In `deep.go`, replace the `Dependency` struct:

```go
type WiringEdge struct {
	FromAtom   string `json:"from_atom"`
	ToAtom     string `json:"to_atom"`
	FromModule string `json:"from_module"`
	ToModule   string `json:"to_module"`
	LinkType   string `json:"link_type"`
	Reason     string `json:"reason"`
}
```

Update `ModuleAnalysis`:

```go
type ModuleAnalysis struct {
	ModuleName   string       `json:"module_name"`
	Wiring       []WiringEdge `json:"wiring"`
	Zones        []Zone       `json:"zones"`
	ModuleIntent string       `json:"module_intent"`
}
```

- [ ] **Step 4: Update buildModulePrompt to request edge format**

In the wiring section of `buildModulePrompt`, update the JSON schema description to request the new edge fields: `from_atom`, `from_module`, `to_atom`, `to_module`, `link_type`, `reason`. Keep existing prompt structure, just change the wiring schema.

- [ ] **Step 5: Add edge cap constant and truncation**

```go
const maxWiringEdges = 50
```

After parsing the LLM response in `AnalyzeModule`, truncate:

```go
if len(analysis.Wiring) > maxWiringEdges {
	analysis.Wiring = analysis.Wiring[:maxWiringEdges]
}
```

- [ ] **Step 6: Fix all references to old Dependency type and fields**

Search for `Dependency` across the codebase (pipeline, patterns) and update to `WiringEdge`. Also update `buildSynthesisPrompt` at `deep.go:175-200` which iterates `m.Wiring` and accesses `.From`, `.To` — change to `.FromAtom`, `.ToAtom` (the `.Reason` field name is unchanged).

Run: `cd go && go build ./...`
Expected: BUILD SUCCESS

- [ ] **Step 7: Run all analyzer tests**

Run: `cd go && go test ./internal/analyzer/ -v`
Expected: ALL PASS

- [ ] **Step 8: Commit**

```bash
git add go/internal/analyzer/deep.go go/internal/analyzer/deep_test.go
git commit -m "feat(analyzer): replace Dependency with WiringEdge for graph-native output"
```

---

## Task 9: Pipeline Phase 2 — Atom Metadata and ID Map

**Files:**
- Modify: `go/internal/pipeline/pipeline.go:197-287` (Phase 2)
- Modify: `go/internal/pipeline/pipeline.go:685-699` (formatAtomEntry)
- Test: `go/internal/pipeline/pipeline_test.go`

- [ ] **Step 1: Add AtomIDMap type and extend Result**

In `pipeline.go`, add at the top:

```go
// AtomIDMap maps composite atom keys to their Memories IDs.
// Key format: "{module}:{filepath}:{name}:{kind}"
type AtomIDMap map[string]int

func atomKey(module, filepath, name, kind string) string {
	return module + ":" + filepath + ":" + name + ":" + kind
}
```

Add `AtomIDs AtomIDMap` to `Result`:

```go
type Result struct {
	Modules         int
	FilesIndexed    int
	AtomsCreated    int
	ModuleAnalyses  []analyzer.ModuleAnalysis
	Synthesis       *analyzer.SystemSynthesis
	Errors          []error
	AtomIDs         AtomIDMap
}
```

- [ ] **Step 2: Update Phase 2 to store atoms with metadata and collect IDs**

In the Phase 2 section (~lines 197-287), after atoms are analyzed:

1. Set `Module` on each atom: `atom.Module = mod.Name`
2. Build `Memory` objects with metadata fields
3. Call `store.UpsertBatchWithMeta()` (new Store method — see next step)
4. Collect returned IDs into `AtomIDMap`

Replace the `store.StoreBatch()` call with:

```go
// Build memories with metadata for each atom
memories := make([]storage.Memory, len(moduleAtoms))
for i, a := range moduleAtoms {
	a.Module = mod.Name
	memories[i] = storage.Memory{
		Text:   a.Summary + "\n\n" + a.ClarifiedCode,
		Source: fmt.Sprintf("carto/%s/%s/layer:atoms", cfg.ProjectName, mod.Name),
		Metadata: map[string]any{
			"name":     a.Name,
			"kind":     a.Kind,
			"filepath": a.FilePath,
			"module":   a.Module,
			"language": a.Language,
		},
	}
}
results, err := cfg.MemoriesClient.UpsertBatch(memories)
if err != nil {
	// log error, continue
} else {
	for i, r := range results {
		key := atomKey(mod.Name, moduleAtoms[i].FilePath, moduleAtoms[i].Name, moduleAtoms[i].Kind)
		atomIDs[key] = r.ID
	}
}
```

- [ ] **Step 3: Remove formatAtomEntry function**

The `formatAtomEntry` function at lines 685-699 is no longer needed — atom text is now `Summary + ClarifiedCode`, with structured data in metadata. Delete it.

- [ ] **Step 4: Run full test suite**

Run: `cd go && go test ./... -short`
Expected: ALL PASS (some pipeline tests may need updating for the new atom storage format)

- [ ] **Step 5: Commit**

```bash
git add go/internal/pipeline/pipeline.go go/internal/pipeline/pipeline_test.go
git commit -m "feat(pipeline): Phase 2 stores atoms with metadata and returns ID map"
```

---

## Task 10: Pipeline Phase 5 — Graph Link Creation

**Files:**
- Modify: `go/internal/pipeline/pipeline.go:435-555` (Phase 5)
- Test: `go/internal/pipeline/pipeline_test.go`

- [ ] **Step 1: Add link creation logic to Phase 5**

After wiring edges are generated in Phase 4, Phase 5 resolves atom names to IDs and creates links:

```go
// Create graph links from wiring edges
for _, analysis := range result.ModuleAnalyses {
	for _, edge := range analysis.Wiring {
		fromKey := atomKey(edge.FromModule, "", edge.FromAtom, "")
		toKey := atomKey(edge.ToModule, "", edge.ToAtom, "")

		// Fuzzy match: iterate atomIDs to find matching module+name
		fromID := findAtomID(atomIDs, edge.FromModule, edge.FromAtom)
		toID := findAtomID(atomIDs, edge.ToModule, edge.ToAtom)

		if fromID == 0 || toID == 0 {
			logFn("warn", fmt.Sprintf("skip link: %s/%s -> %s/%s (atom not found)",
				edge.FromModule, edge.FromAtom, edge.ToModule, edge.ToAtom))
			continue
		}

		if err := cfg.MemoriesClient.CreateLink(fromID, toID, edge.LinkType); err != nil {
			logFn("warn", fmt.Sprintf("create link failed: %v", err))
		}
	}
}
```

- [ ] **Step 2: Add findAtomID helper**

```go
func findAtomID(ids AtomIDMap, module, name string) int {
	for key, id := range ids {
		// Key format: "{module}:{filepath}:{name}:{kind}"
		parts := strings.SplitN(key, ":", 4)
		if len(parts) >= 3 && parts[0] == module && parts[2] == name {
			return id
		}
	}
	return 0
}
```

- [ ] **Step 3: Remove old wiring JSON blob storage**

In Phase 5, remove the `store.StoreLayer(mod.Name, storage.LayerWiring, ...)` call that stored wiring as a JSON blob. Wiring is now represented as graph links, not text.

- [ ] **Step 4: Run full test suite**

Run: `cd go && go test ./... -short`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add go/internal/pipeline/pipeline.go go/internal/pipeline/pipeline_test.go
git commit -m "feat(pipeline): Phase 5 creates graph links from wiring edges"
```

---

## Task 11: Writeback Command

**Files:**
- Create: `go/cmd/carto/cmd_writeback.go`
- Create: `go/cmd/carto/cmd_writeback_test.go`

- [ ] **Step 1: Write test for writeback command registration**

```go
func TestWritebackCmd_Flags(t *testing.T) {
	cmd := writebackCmd()
	if cmd.Use != "writeback <path>" {
		t.Errorf("unexpected use: %s", cmd.Use)
	}
	flags := []string{"file", "module", "project"}
	for _, f := range flags {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("missing flag: %s", f)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go && go test ./cmd/carto/ -run TestWritebackCmd_Flags -v`
Expected: FAIL

- [ ] **Step 3: Implement writebackCmd**

Create `go/cmd/carto/cmd_writeback.go`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/divyekant/carto/internal/atoms"
	"github.com/divyekant/carto/internal/chunker"
	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/manifest"
	"github.com/divyekant/carto/internal/scanner"
	"github.com/divyekant/carto/internal/storage"
	"github.com/spf13/cobra"
)

func writebackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "writeback <path>",
		Short: "Update index for changed files without full re-index",
		Long:  "Re-index specific files or modules, superseding old atoms and updating graph links.",
		Args:  cobra.ExactArgs(1),
		RunE:  runWriteback,
	}
	cmd.Flags().StringSlice("file", nil, "File(s) to re-index (repeatable)")
	cmd.Flags().String("module", "", "Module to re-index")
	cmd.Flags().String("project", "", "Project name (default: directory name)")
	return cmd
}

func runWriteback(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	files, _ := cmd.Flags().GetStringSlice("file")
	module, _ := cmd.Flags().GetString("module")
	project, _ := cmd.Flags().GetString("project")

	if project == "" {
		project = filepath.Base(absPath)
	}

	// Load manifest
	mf, err := manifest.Load(absPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	// If no files specified, detect changes from manifest
	if len(files) == 0 && module == "" {
		scanResult, err := scanner.Scan(absPath)
		if err != nil {
			return fmt.Errorf("scan: %w", err)
		}
		var allFiles []string
		for _, mod := range scanResult.Modules {
			allFiles = append(allFiles, mod.Files...)
		}
		changes, err := mf.DetectChanges(allFiles, absPath)
		if err != nil {
			return fmt.Errorf("detect changes: %w", err)
		}
		files = append(changes.Added, changes.Modified...)
		if len(files) == 0 {
			fmt.Fprintln(os.Stdout, "No changes detected")
			return nil
		}
	}

	// Create clients
	cfg := config.Load()
	llmClient, err := llm.NewClient(llm.Options{
		Provider: cfg.LLMProvider,
		APIKey:   cfg.LLMAPIKey,
		BaseURL:  cfg.LLMBaseURL,
		FastModel: cfg.FastModel,
		DeepModel: cfg.DeepModel,
	})
	if err != nil {
		return fmt.Errorf("llm client: %w", err)
	}
	memoriesClient := storage.NewMemoriesClient(config.ResolveURL(cfg.MemoriesURL), cfg.MemoriesKey)

	// Process each file
	atomAnalyzer := atoms.NewAnalyzer(llmClient)
	var superseded, added, removed, linksUpdated int

	for _, file := range files {
		s, a, r, l, err := writebackFile(absPath, project, file, mf, atomAnalyzer, memoriesClient)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warn: %s: %v\n", file, err)
			continue
		}
		superseded += s
		added += a
		removed += r
		linksUpdated += l
	}

	// Save manifest
	if err := mf.Save(); err != nil {
		return fmt.Errorf("save manifest: %w", err)
	}

	fmt.Fprintf(os.Stdout, "%d atoms superseded, %d added, %d removed, %d links updated\n",
		superseded, added, removed, linksUpdated)
	return nil
}
```

- [ ] **Step 4: Implement writebackFile helper**

```go
func writebackFile(root, project, relPath string, mf *manifest.Manifest,
	atomAnalyzer *atoms.Analyzer, client *storage.MemoriesClient) (superseded, added, removed, links int, err error) {

	absFile := filepath.Join(root, relPath)

	// Check if file changed
	newHash, err := mf.ComputeHash(absFile)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("hash: %w", err)
	}
	if entry, ok := mf.Files[relPath]; ok && entry.Hash == newHash {
		return 0, 0, 0, 0, nil // unchanged
	}

	// Detect module for this file
	module := detectModule(root, relPath)

	// Chunk and analyze
	code, _ := os.ReadFile(absFile)
	lang := scanner.DetectLanguage(relPath)
	chunks, _ := chunker.ChunkFile(absFile, code, lang, nil)
	atomChunks := make([]atoms.Chunk, len(chunks))
	for i, c := range chunks {
		atomChunks[i] = atoms.Chunk{
			Name: c.Name, Kind: c.Kind, Language: c.Language,
			FilePath: relPath, StartLine: c.StartLine, EndLine: c.EndLine,
			Code: c.Code,
		}
	}
	newAtoms, err := atomAnalyzer.AnalyzeBatch(atomChunks, 5, nil)
	if err != nil {
		return 0, 0, 0, 0, fmt.Errorf("analyze: %w", err)
	}

	// Find existing atoms for this filepath
	sourcePrefix := fmt.Sprintf("carto/%s/%s/layer:atoms", project, module)
	existing, _ := client.ListBySource(sourcePrefix, 500, 0)
	oldAtoms := filterByFilepath(existing, relPath)

	// Match and supersede/add/delete
	matched := map[int]bool{}
	for _, newAtom := range newAtoms {
		newAtom.Module = module
		oldID := findMatchingAtom(oldAtoms, newAtom)
		mem := storage.Memory{
			Text:   newAtom.Summary + "\n\n" + newAtom.ClarifiedCode,
			Source: sourcePrefix,
			Metadata: map[string]any{
				"name": newAtom.Name, "kind": newAtom.Kind,
				"filepath": relPath, "module": module, "language": newAtom.Language,
			},
		}
		if oldID > 0 {
			matched[oldID] = true
			_, err := client.Supersede(oldID, mem.Text, mem.Metadata)
			if err == nil {
				superseded++
			}
		} else {
			_, err := client.UpsertBatch([]storage.Memory{mem})
			if err == nil {
				added++
			}
		}
	}

	// Delete atoms with no match (removed functions)
	for _, old := range oldAtoms {
		if !matched[old.ID] {
			client.DeleteMemory(old.ID)
			removed++
		}
	}

	// Update manifest
	info, _ := os.Stat(absFile)
	mf.UpdateFile(relPath, newHash, info.Size())

	return superseded, added, removed, links, nil
}
```

- [ ] **Step 5: Add helper functions**

```go
func filterByFilepath(results []storage.SearchResult, filepath string) []storage.SearchResult {
	var filtered []storage.SearchResult
	for _, r := range results {
		if meta, ok := r.Metadata["filepath"]; ok && meta == filepath {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func findMatchingAtom(existing []storage.SearchResult, atom *atoms.Atom) int {
	for _, e := range existing {
		name, _ := e.Metadata["name"].(string)
		kind, _ := e.Metadata["kind"].(string)
		if name == atom.Name && kind == atom.Kind {
			return e.ID
		}
	}
	return 0
}

func detectModule(root, relPath string) string {
	scanResult, err := scanner.Scan(root)
	if err != nil {
		return filepath.Base(root)
	}
	for _, m := range scanResult.Modules {
		for _, f := range m.Files {
			if f == relPath {
				return m.Name
			}
		}
	}
	return filepath.Base(root)
}
```

- [ ] **Step 6: Register in main.go**

Add `writebackCmd()` to the subcommand list in `main.go`.

- [ ] **Step 7: Run tests**

Run: `cd go && go test ./cmd/carto/ -run TestWritebackCmd -v && go build ./cmd/carto/`
Expected: PASS + BUILD SUCCESS

- [ ] **Step 8: Commit**

```bash
git add go/cmd/carto/cmd_writeback.go go/cmd/carto/cmd_writeback_test.go go/cmd/carto/main.go
git commit -m "feat(cli): add writeback command for file-level index updates"
```

---

## Task 12: CLI — Query Command Advanced Params

**Files:**
- Modify: `go/cmd/carto/cmd_query.go:12-89`
- Test: `go/cmd/carto/cmd_query_test.go`

- [ ] **Step 1: Add new flags to queryCmd**

```go
cmd.Flags().Float64("graph-weight", 0.1, "Graph traversal weight (0-1)")
cmd.Flags().Float64("confidence-weight", 0.0, "Confidence decay weight (0-1)")
cmd.Flags().Float64("feedback-weight", 0.1, "Feedback signal weight (0-1)")
cmd.Flags().String("since", "", "Filter atoms after date (ISO 8601)")
cmd.Flags().String("until", "", "Filter atoms before date (ISO 8601)")
```

- [ ] **Step 2: Thread new flags into SearchOptions in runQuery**

```go
graphWeight, _ := cmd.Flags().GetFloat64("graph-weight")
confidenceWeight, _ := cmd.Flags().GetFloat64("confidence-weight")
feedbackWeight, _ := cmd.Flags().GetFloat64("feedback-weight")
since, _ := cmd.Flags().GetString("since")
until, _ := cmd.Flags().GetString("until")

opts := storage.SearchOptions{
	K: count, Hybrid: true,
	GraphWeight: graphWeight, ConfidenceWeight: confidenceWeight,
	FeedbackWeight: feedbackWeight, Since: since, Until: until,
}
```

- [ ] **Step 3: Update result display for graph/confidence fields**

In TTY output, add `[graph]` tag when `result.MatchType == "graph"`:

```go
tag := ""
if r.MatchType == "graph" {
	tag = " [graph]"
}
fmt.Fprintf(os.Stdout, "  %d. (%.2f%s) %s\n", i+1, r.Score, tag, preview)
```

- [ ] **Step 4: Run tests**

Run: `cd go && go test ./cmd/carto/ -run TestQuery -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add go/cmd/carto/cmd_query.go
git commit -m "feat(cli): add graph/confidence/feedback/temporal flags to query command"
```

---

## Task 13: CLI — Status Command with Memories Integration

**Files:**
- Modify: `go/cmd/carto/cmd_status.go:22-75`

- [ ] **Step 1: Add Memories client to status command**

After loading the manifest, attempt to query Memories for live stats:

```go
cfg := config.Load()
memoriesClient := storage.NewMemoriesClient(config.ResolveURL(cfg.MemoriesURL), cfg.MemoriesKey)

sourcePrefix := fmt.Sprintf("carto/%s/", mf.Project)
atomCount, countErr := memoriesClient.Count(sourcePrefix)

// Build status data
data := statusData{
	Project:   mf.Project,
	Files:     len(mf.Files),
	IndexedAt: mf.IndexedAt.Format(time.RFC3339),
}

if countErr == nil {
	data.Atoms = atomCount
}
```

- [ ] **Step 2: Add Atoms field to statusData struct**

```go
type statusData struct {
	Project   string `json:"project"`
	Files     int    `json:"files"`
	Atoms     int    `json:"atoms,omitempty"`
	TotalSize string `json:"total_size"`
	IndexedAt string `json:"indexed_at"`
}
```

- [ ] **Step 3: Add graceful degradation when Memories unavailable**

```go
if countErr != nil {
	fmt.Fprintln(os.Stderr, "warn: Memories unavailable — showing local manifest only")
}
```

- [ ] **Step 4: Run tests**

Run: `cd go && go test ./cmd/carto/ -run TestStatus -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add go/cmd/carto/cmd_status.go
git commit -m "feat(cli): status command shows atom count from Memories with offline fallback"
```

---

## Task 14: CLI — Export/Import v2 Format

**Files:**
- Modify: `go/cmd/carto/cmd_export.go:42-129`
- Modify: `go/cmd/carto/cmd_import.go:46-159`

- [ ] **Step 1: Update exportEntry with type field**

```go
type exportEntry struct {
	Type       string         `json:"type"`
	ID         int            `json:"id"`
	Text       string         `json:"text"`
	Source     string         `json:"source"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	DocumentAt string         `json:"document_at,omitempty"`
}

type exportLinkEntry struct {
	Type     string `json:"type"`
	FromID   int    `json:"from_id"`
	ToID     int    `json:"to_id"`
	LinkType string `json:"link_type"`
}
```

- [ ] **Step 2: Update export to emit atom and link lines**

In `runExport`, set `Type: "atom"` on each entry. After emitting all atoms, query links for each atom and emit link lines.

- [ ] **Step 3: Update importRecord with type awareness**

```go
type importRecord struct {
	Type     string         `json:"type"`
	Text     string         `json:"text,omitempty"`
	Source   string         `json:"source,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	FromID   int            `json:"from_id,omitempty"`
	ToID     int            `json:"to_id,omitempty"`
	LinkType string         `json:"link_type,omitempty"`
}
```

- [ ] **Step 4: Update import to handle two-pass (atoms then links)**

In `runImport`, first pass: upsert all `type: "atom"` records, build `oldID → newID` map. Second pass: create links with remapped IDs.

- [ ] **Step 5: Run tests**

Run: `cd go && go test ./cmd/carto/ -run "TestExport|TestImport" -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add go/cmd/carto/cmd_export.go go/cmd/carto/cmd_import.go
git commit -m "feat(cli): export/import v2 format with type field and link support"
```

---

## Task 15: Server Handler — Advanced Search Params

**Files:**
- Modify: `go/internal/server/handlers.go:99-194`

- [ ] **Step 1: Extend queryRequest struct**

```go
type queryRequest struct {
	Text             string  `json:"text"`
	Project          string  `json:"project"`
	Tier             string  `json:"tier"`
	K                int     `json:"k"`
	ConfidenceWeight float64 `json:"confidence_weight,omitempty"`
	FeedbackWeight   float64 `json:"feedback_weight,omitempty"`
	GraphWeight      float64 `json:"graph_weight,omitempty"`
	Since            string  `json:"since,omitempty"`
	Until            string  `json:"until,omitempty"`
}
```

- [ ] **Step 2: Update handleQuery to pass new params**

```go
opts := storage.SearchOptions{
	K:                req.K,
	Hybrid:           true,
	SourcePrefix:     sourcePrefix,
	ConfidenceWeight: req.ConfidenceWeight,
	FeedbackWeight:   req.FeedbackWeight,
	GraphWeight:      req.GraphWeight,
	Since:            req.Since,
	Until:            req.Until,
}
```

- [ ] **Step 3: Update queryResultItem to include new fields**

```go
type queryResultItem struct {
	ID           int            `json:"id"`
	Text         string         `json:"text"`
	Source       string         `json:"source"`
	Score        float64        `json:"score"`
	Layer        string         `json:"layer,omitempty"`
	MatchType    string         `json:"match_type,omitempty"`
	Confidence   float64        `json:"confidence,omitempty"`
	GraphSupport float64        `json:"graph_support,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}
```

- [ ] **Step 4: Remove 3x over-fetch and ListBySource fallback**

Remove `opts.K = req.K * 3` and the client-side filtering loop. Remove the `ListBySource` fallback block.

- [ ] **Step 5: Run server tests**

Run: `cd go && go test ./internal/server/ -v`
Expected: ALL PASS

- [ ] **Step 6: Commit**

```bash
git add go/internal/server/handlers.go
git commit -m "feat(server): pass advanced search params through to Memories v5"
```

---

## Task 16: Web UI — Query Page Advanced Filters

**Files:**
- Modify: `go/web/src/pages/Query.tsx`
- Modify: `go/web/src/components/QueryResult.tsx`

- [ ] **Step 1: Add advanced filter state to Query.tsx**

```tsx
const [showAdvanced, setShowAdvanced] = useState(false)
const [graphWeight, setGraphWeight] = useState(0.1)
const [confidenceWeight, setConfidenceWeight] = useState(0)
const [feedbackWeight, setFeedbackWeight] = useState(0.1)
const [since, setSince] = useState('')
const [until, setUntil] = useState('')
```

- [ ] **Step 2: Update Result interface**

```tsx
interface Result {
  id: number
  text: string
  score: number
  source: string
  match_type?: string
  confidence?: number
  graph_support?: number
  metadata?: Record<string, string>
}
```

- [ ] **Step 3: Send advanced params in search()**

```tsx
body: JSON.stringify({
  text: text.trim(), project, tier, k,
  ...(showAdvanced && {
    graph_weight: graphWeight,
    confidence_weight: confidenceWeight,
    feedback_weight: feedbackWeight,
    ...(since && { since }),
    ...(until && { until }),
  }),
}),
```

- [ ] **Step 4: Add collapsible Advanced Filters section**

Below the search bar, add a toggle button and filter controls (sliders for weights, date inputs for since/until). Collapsed by default.

- [ ] **Step 5: Update QueryResult component**

Add `match_type`, `confidence`, `metadata` to `QueryResultProps`. Show `[graph]` badge when `match_type === 'graph'`. Show metadata tags (module, language, kind) as small badges under the result text.

- [ ] **Step 6: Build and verify**

Run: `cd go/web && npm run build`
Expected: BUILD SUCCESS

- [ ] **Step 7: Commit**

```bash
git add go/web/src/pages/Query.tsx go/web/src/components/QueryResult.tsx
git commit -m "feat(web): add advanced search filters and graph/confidence display to Query page"
```

---

## Task 17: Patterns Generator — Writeback Instructions

**Files:**
- Modify: `go/internal/patterns/generator.go:85-147`

- [ ] **Step 1: Update GenerateCLAUDE writeback instructions**

In the "Working with the Carto Index" section, replace the `memory_add` write-back instructions with:

```go
section += "### After Changes: Write Back\n\n"
section += "After editing files, update the Carto index:\n\n"
section += "```bash\n"
section += fmt.Sprintf("carto writeback %s --file <changed-file>\n", input.ProjectName)
section += "```\n\n"
section += "This re-analyzes the file, supersedes old atoms, and updates graph links.\n"
section += "The index stays current without a full re-index.\n\n"
```

- [ ] **Step 2: Update GenerateCursorRules similarly**

Same change in the cursor rules generator.

- [ ] **Step 3: Run tests**

Run: `cd go && go test ./internal/patterns/ -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add go/internal/patterns/generator.go
git commit -m "feat(patterns): skill file instructions use carto writeback instead of memory_add"
```

---

## Task 18: Integration Test — Full Pipeline with Memories v5

**Files:**
- Create: `go/internal/pipeline/pipeline_v2_test.go`

- [ ] **Step 1: Write integration test for full pipeline with metadata + links**

Test that:
1. A full pipeline run stores atoms with metadata fields
2. Wiring edges create graph links between atom IDs
3. SearchAdvanced with graph_weight returns graph-traversed results
4. Writeback of a changed file supersedes the old atom

This test requires a running Memories instance — skip with `-short` flag.

```go
func TestPipeline_V2_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires Memories server")
	}
	// ... full pipeline integration test
}
```

- [ ] **Step 2: Write integration test for writeback**

```go
func TestWriteback_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires Memories server")
	}
	// Index a test file, modify it, writeback, verify supersede
}
```

- [ ] **Step 3: Run integration tests**

Run: `cd go && go test ./internal/pipeline/ -run TestPipeline_V2 -v` (requires Memories running)
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add go/internal/pipeline/pipeline_v2_test.go
git commit -m "test(pipeline): add v2 integration tests for metadata, links, and writeback"
```

---

## Task 19: Version Bump and Changelog

**Files:**
- Modify: `go/cmd/carto/main.go:11` (version)
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Bump version to 2.0.0**

In `main.go` line 11:
```go
var version = "2.0.0"
```

- [ ] **Step 2: Update CHANGELOG.md**

Add v2.0.0 entry with:
- Memories v5 native storage (metadata, document_at, advanced search)
- Graph-native wiring (links instead of JSON blobs)
- Supersede-based incremental updates
- `carto writeback` command
- Breaking: manifest v2.0, new export/import format

- [ ] **Step 3: Run full test suite**

Run: `cd go && go test ./... -short`
Expected: ALL PASS

- [ ] **Step 4: Build binary**

Run: `cd go && go build -o carto ./cmd/carto/`
Expected: BUILD SUCCESS

- [ ] **Step 5: Commit**

```bash
git add go/cmd/carto/main.go CHANGELOG.md
git commit -m "chore: bump version to 2.0.0 for Memories-native storage release"
```

---

## Execution Order & Dependencies

```
Task 1 (Atom struct) ──┐
Task 2 (Manifest)      ├──→ Task 7 (Interface) ──→ Task 9 (Pipeline Phase 2)
Task 3 (Structs)       │                           ──→ Task 10 (Pipeline Phase 5)
Task 4 (Upsert/Super)  │                           ──→ Task 11 (Writeback cmd)
Task 5 (Links)         │
Task 6 (SearchAdvanced)┘
Task 8 (WiringEdge) ───────→ Task 10 (Pipeline Phase 5)
Task 12 (Query CLI) ───────→ (independent after Task 7)
Task 13 (Status CLI) ──────→ (independent after Task 7)
Task 14 (Export/Import) ───→ (independent after Task 7)
Task 15 (Server handler) ──→ (independent after Task 7)
Task 16 (Web UI) ──────────→ (independent after Task 15)
Task 17 (Patterns) ────────→ (independent after Task 11)
Task 18 (Integration) ─────→ (after Tasks 9, 10, 11)
Task 19 (Version bump) ────→ (last)
```

**Parallelizable groups after Task 7:**
- Group A: Tasks 9, 10, 11 (pipeline + writeback) — sequential within group
- Group B: Tasks 12, 13, 14, 15 (CLI + server) — all parallel
- Group C: Task 16 (Web UI) — after Task 15
- Group D: Task 17 (Patterns) — after Task 11

## Task 20: Pipeline — document_at Patching in Phase 5

**Files:**
- Modify: `go/internal/pipeline/pipeline.go` (Phase 5, after history extraction)

- [ ] **Step 1: Write test for document_at patching**

```go
func TestPipeline_DocumentAtPatching(t *testing.T) {
	// Verify that after Phase 5, atoms have document_at set from git history
	// Use mock MemoriesClient that records PATCH calls
}
```

- [ ] **Step 2: In Phase 5, after history extraction, patch document_at on stored atoms**

After Phase 3 extracts git history, build a map of `filepath → lastCommitDate`. In Phase 5, for each stored atom ID, call the Memories PATCH endpoint (or use metadata fallback):

```go
// Build filepath→date map from history
fileDates := map[string]string{}
for _, fh := range moduleHistory {
	if len(fh.Commits) > 0 {
		fileDates[fh.FilePath] = fh.Commits[0].Date.Format(time.RFC3339)
	}
}

// Patch document_at on stored atoms
for key, atomID := range atomIDs {
	parts := strings.SplitN(key, ":", 4)
	if len(parts) >= 2 {
		if date, ok := fileDates[parts[1]]; ok {
			// Use PATCH /memory/{id} if available, otherwise store as metadata
			cfg.MemoriesClient.PatchDocumentAt(atomID, date)
		}
	}
}
```

Note: If Memories lacks a PATCH endpoint for `document_at`, fall back to storing it as `metadata.document_at` during Phase 2 using `os.Stat` mtime. This is a degraded mode — `since`/`until` won't work, but the data is preserved.

- [ ] **Step 3: Run tests**

Run: `cd go && go test ./internal/pipeline/ -v`
Expected: ALL PASS

- [ ] **Step 4: Commit**

```bash
git add go/internal/pipeline/pipeline.go
git commit -m "feat(pipeline): patch document_at on atoms in Phase 5 using git dates"
```

---

## Task 21: Pipeline — Incremental Re-indexing with Supersede

**Files:**
- Modify: `go/internal/pipeline/pipeline.go` (incremental path, ~lines 138-189)

- [ ] **Step 1: Write test for incremental supersede behavior**

```go
func TestPipeline_Incremental_UsesSupersede(t *testing.T) {
	// Mock MemoriesClient that tracks Supersede calls
	// Index a file, then re-index incrementally with a changed file
	// Verify Supersede was called (not DeleteBySource + AddBatch)
}
```

- [ ] **Step 2: Update incremental path to use Supersede**

In the incremental re-indexing section of `Run()`:
- For changed files: re-chunk → re-analyze → find existing atoms by filepath metadata → `Supersede()` each matched atom, `UpsertBatch()` new atoms, `DeleteMemory()` removed atoms
- For removed files: `DeleteMemory()` for each atom with that filepath
- For removed modules (all files gone): `DeleteBySource(modulePrefix)`
- After atom updates: re-run Phase 4 for affected modules, delete old links, create new links

```go
if cfg.Incremental {
	for _, file := range changes.Modified {
		s, a, r, _, err := writebackFileInternal(...)
		if err != nil {
			logFn("warn", fmt.Sprintf("incremental %s: %v", file, err))
		}
		superseded += s; added += a; removed += r
	}
	for _, file := range changes.Removed {
		// Find and delete atoms for this file
		existing, _ := cfg.MemoriesClient.ListBySource(sourcePrefix, 500, 0)
		for _, e := range filterByFilepath(existing, file) {
			cfg.MemoriesClient.DeleteMemory(e.ID)
		}
	}
}
```

- [ ] **Step 3: Add parallelism for supersede calls**

Use a semaphore bounded by `cfg.MaxWorkers` (default 10) to run supersede calls concurrently:

```go
sem := make(chan struct{}, cfg.MaxWorkers)
var wg sync.WaitGroup
for _, file := range changes.Modified {
	sem <- struct{}{}
	wg.Add(1)
	go func(f string) {
		defer wg.Done()
		defer func() { <-sem }()
		// ... supersede logic
	}(file)
}
wg.Wait()
```

- [ ] **Step 4: Run tests**

Run: `cd go && go test ./internal/pipeline/ -v`
Expected: ALL PASS

- [ ] **Step 5: Commit**

```bash
git add go/internal/pipeline/pipeline.go
git commit -m "feat(pipeline): incremental re-indexing uses supersede with parallel execution"
```

---

## Updated Execution Order & Dependencies

```
Task 1 (Atom struct) ──┐
Task 2 (Manifest)      ├──→ Task 7 (Interface) ──→ Task 9 (Pipeline Phase 2)
Task 3 (Structs)       │                           ──→ Task 10 (Pipeline Phase 5)
Task 4 (Upsert/Super)  │                           ──→ Task 20 (document_at)
Task 5 (Links)         │                           ──→ Task 21 (Incremental)
Task 6 (SearchAdvanced)┘                           ──→ Task 11 (Writeback cmd)
Task 8 (WiringEdge) ───────→ Task 10 (Pipeline Phase 5)
Task 12 (Query CLI) ───────→ (independent after Task 7)
Task 13 (Status CLI) ──────→ (independent after Task 7)
Task 14 (Export/Import) ───→ (independent after Task 7)
Task 15 (Server handler) ──→ (independent after Task 7)
Task 16 (Web UI) ──────────→ (independent after Task 15)
Task 17 (Patterns) ────────→ (independent after Task 11)
Task 18 (Integration) ─────→ (after Tasks 9, 10, 11, 20, 21)
Task 19 (Version bump) ────→ (last)
```

**Parallelizable groups after Task 7:**
- Group A: Tasks 9 → 10 → 20 → 21 → 11 (pipeline + writeback) — sequential
- Group B: Tasks 12, 13, 14, 15 (CLI + server) — all parallel
- Group C: Task 16 (Web UI) — after Task 15
- Group D: Task 17 (Patterns) — after Task 11

**Estimated total: ~21 commits, ~3-4 hours of implementation time.**
