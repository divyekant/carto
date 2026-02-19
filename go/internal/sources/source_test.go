package sources

import (
	"context"
	"fmt"
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

// ── Registry Tests ──────────────────────────────────────────────────────

func TestRegistry_FetchAll_ProjectScope(t *testing.T) {
	reg := NewRegistry()

	reg.Register(&mockSource{
		name:  "github",
		scope: ProjectScope,
		artifacts: []Artifact{
			{Source: "github", Category: Signal, ID: "#1", Title: "Issue 1"},
			{Source: "github", Category: Signal, ID: "#2", Title: "PR 2"},
		},
	})
	reg.Register(&mockSource{
		name:  "jira",
		scope: ProjectScope,
		artifacts: []Artifact{
			{Source: "jira", Category: Signal, ID: "PROJ-10", Title: "Ticket"},
		},
	})

	req := FetchRequest{Project: "test", RepoRoot: "/tmp/repo"}
	all, err := reg.FetchAllProject(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchAllProject: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 artifacts, got %d", len(all))
	}
}

func TestRegistry_FetchAll_SkipsErrors(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockSource{
		name:      "good",
		scope:     ProjectScope,
		artifacts: []Artifact{{Source: "good", ID: "1", Title: "OK"}},
	})
	reg.Register(&mockSource{
		name:     "bad",
		scope:    ProjectScope,
		fetchErr: fmt.Errorf("connection refused"),
	})
	reg.Register(&mockSource{
		name:      "also-good",
		scope:     ProjectScope,
		artifacts: []Artifact{{Source: "also-good", ID: "2", Title: "OK too"}},
	})

	req := FetchRequest{Project: "test", RepoRoot: "/tmp/repo"}
	all, err := reg.FetchAllProject(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchAllProject: %v", err)
	}
	if len(all) != 2 {
		t.Errorf("expected 2 artifacts (skipping failed source), got %d", len(all))
	}
}

func TestRegistry_FetchModule(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockSource{
		name:  "git",
		scope: ModuleScope,
		artifacts: []Artifact{
			{Source: "git", Category: Signal, ID: "abc123", Title: "commit"},
		},
	})
	// Project-scoped sources should be ignored by FetchModule.
	reg.Register(&mockSource{
		name:  "jira",
		scope: ProjectScope,
		artifacts: []Artifact{
			{Source: "jira", Category: Signal, ID: "J-1", Title: "ticket"},
		},
	})

	req := FetchRequest{Project: "test", Module: "mymod", ModulePath: "/tmp/repo/mymod", RepoRoot: "/tmp/repo"}
	all, err := reg.FetchModule(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchModule: %v", err)
	}
	if len(all) != 1 {
		t.Errorf("expected 1 artifact (module-scoped only), got %d", len(all))
	}
	if all[0].Source != "git" {
		t.Errorf("expected git artifact, got %s", all[0].Source)
	}
}

func TestRegistry_Empty(t *testing.T) {
	reg := NewRegistry()
	req := FetchRequest{Project: "test", RepoRoot: "/tmp/repo"}

	project, err := reg.FetchAllProject(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchAllProject on empty: %v", err)
	}
	if len(project) != 0 {
		t.Errorf("expected 0 from empty registry, got %d", len(project))
	}

	module, err := reg.FetchModule(context.Background(), req)
	if err != nil {
		t.Fatalf("FetchModule on empty: %v", err)
	}
	if len(module) != 0 {
		t.Errorf("expected 0 from empty registry, got %d", len(module))
	}
}

func TestRegistry_Sources(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockSource{name: "git", scope: ModuleScope})
	reg.Register(&mockSource{name: "github", scope: ProjectScope})

	names := reg.SourceNames()
	if len(names) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(names))
	}
}
