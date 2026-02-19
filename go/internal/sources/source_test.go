package sources

import (
	"context"
	"testing"
	"time"
)

func TestArtifact_CategoryConstants(t *testing.T) {
	// Verify the three categories exist and are distinct.
	cats := []Category{Signal, Knowledge, Context}
	seen := map[Category]bool{}
	for _, c := range cats {
		if seen[c] {
			t.Errorf("duplicate category: %s", c)
		}
		seen[c] = true
	}
	if len(seen) != 3 {
		t.Errorf("expected 3 categories, got %d", len(seen))
	}
}

func TestScope_Constants(t *testing.T) {
	if ProjectScope == ModuleScope {
		t.Error("ProjectScope and ModuleScope should be different")
	}
}

func TestArtifact_Fields(t *testing.T) {
	a := Artifact{
		Source:   "github",
		Category: Signal,
		ID:       "#42",
		Title:    "Fix login",
		Body:     "Details here",
		URL:      "https://github.com/user/repo/issues/42",
		Files:    []string{"auth/login.go"},
		Module:   "root",
		Date:     time.Now(),
		Author:   "alice",
		Tags:     map[string]string{"state": "closed"},
	}
	if a.Source != "github" {
		t.Errorf("Source = %q, want %q", a.Source, "github")
	}
	if a.Category != Signal {
		t.Errorf("Category = %q, want %q", a.Category, Signal)
	}
	if len(a.Files) != 1 || a.Files[0] != "auth/login.go" {
		t.Errorf("Files = %v, want [auth/login.go]", a.Files)
	}
	if a.Tags["state"] != "closed" {
		t.Errorf("Tags[state] = %q, want %q", a.Tags["state"], "closed")
	}
}

// mockSource is a test double implementing Source.
type mockSource struct {
	name      string
	scope     Scope
	configErr error
	artifacts []Artifact
	fetchErr  error
}

func (m *mockSource) Name() string                                             { return m.name }
func (m *mockSource) Scope() Scope                                             { return m.scope }
func (m *mockSource) Configure(cfg SourceConfig) error                         { return m.configErr }
func (m *mockSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	return m.artifacts, m.fetchErr
}

func TestSourceInterface_Compliance(t *testing.T) {
	// Verify mockSource satisfies Source at compile time.
	var _ Source = (*mockSource)(nil)
}
