package signals

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

// gitCmd runs a git command in the given directory with test user config.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	fullArgs := append([]string{
		"-C", dir,
		"-c", "user.name=test",
		"-c", "user.email=test@test.com",
	}, args...)
	cmd := exec.Command("git", fullArgs...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

// setupTestRepo creates a temporary git repo with a module subdirectory
// and three commits, one of which references PR #42.
func setupTestRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()

	gitCmd(t, dir, "init")

	// Create a module subdirectory with a file.
	modDir := filepath.Join(dir, "mymodule")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Commit 1: initial file.
	if err := os.WriteFile(filepath.Join(modDir, "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "Initial commit")

	// Commit 2: update file.
	if err := os.WriteFile(filepath.Join(modDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "Add main function")

	// Commit 3: references PR #42.
	if err := os.WriteFile(filepath.Join(modDir, "util.go"), []byte("package main\n\nfunc helper() {}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "Fix bug from PR #42")

	return dir
}

func TestNewGitSignalSource(t *testing.T) {
	src := NewGitSignalSource("/some/repo")

	if src.repoRoot != "/some/repo" {
		t.Errorf("repoRoot = %q, want %q", src.repoRoot, "/some/repo")
	}
	if src.maxCommits != 20 {
		t.Errorf("maxCommits = %d, want 20", src.maxCommits)
	}
	if src.Name() != "git" {
		t.Errorf("Name() = %q, want %q", src.Name(), "git")
	}
}

func TestGitSignalSource_Configure(t *testing.T) {
	src := NewGitSignalSource("/original")
	err := src.Configure(map[string]string{"repo_root": "/updated"})
	if err != nil {
		t.Fatalf("Configure returned error: %v", err)
	}
	if src.repoRoot != "/updated" {
		t.Errorf("repoRoot after Configure = %q, want %q", src.repoRoot, "/updated")
	}
}

func TestGitSignalSource_FetchSignals(t *testing.T) {
	// Skip if git is not available.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	repoDir := setupTestRepo(t)
	src := NewGitSignalSource(repoDir)

	module := Module{
		Name:    "mymodule",
		Path:    filepath.Join(repoDir, "mymodule"),
		RelPath: "mymodule",
		Files:   []string{"main.go", "util.go"},
	}

	signals, err := src.FetchSignals(module)
	if err != nil {
		t.Fatalf("FetchSignals returned error: %v", err)
	}

	// Count signal types.
	var commits, prs int
	for _, s := range signals {
		switch s.Type {
		case "commit":
			commits++
		case "pr":
			prs++
		}
	}

	if commits < 3 {
		t.Errorf("expected at least 3 commit signals, got %d", commits)
	}
	if prs < 1 {
		t.Errorf("expected at least 1 PR signal, got %d", prs)
	}

	// Verify the PR signal has the correct ID.
	var foundPR42 bool
	for _, s := range signals {
		if s.Type == "pr" && s.ID == "#42" {
			foundPR42 = true
			break
		}
	}
	if !foundPR42 {
		t.Error("expected a PR signal with ID '#42'")
	}

	// Verify signals are sorted newest first.
	for i := 1; i < len(signals); i++ {
		if signals[i].Date.After(signals[i-1].Date) {
			t.Errorf("signals not sorted newest-first: signal[%d].Date=%v > signal[%d].Date=%v",
				i, signals[i].Date, i-1, signals[i-1].Date)
		}
	}

	// Verify commit signals have non-empty fields.
	for _, s := range signals {
		if s.Type == "commit" {
			if s.ID == "" {
				t.Error("commit signal has empty ID (hash)")
			}
			if s.Title == "" {
				t.Error("commit signal has empty Title")
			}
			if s.Author == "" {
				t.Error("commit signal has empty Author")
			}
			if s.Date.IsZero() {
				t.Error("commit signal has zero Date")
			}
		}
	}
}

func TestGitSignalSource_FetchSignals_NonGitDir(t *testing.T) {
	// FetchSignals on a non-git directory should return empty, not an error.
	dir := t.TempDir()
	src := NewGitSignalSource(dir)

	module := Module{
		Name:    "test",
		Path:    dir,
		RelPath: ".",
	}

	signals, err := src.FetchSignals(module)
	if err != nil {
		t.Fatalf("expected nil error for non-git dir, got: %v", err)
	}
	if len(signals) != 0 {
		t.Errorf("expected 0 signals for non-git dir, got %d", len(signals))
	}
}

// mockSignalSource is a test double implementing SignalSource.
type mockSignalSource struct {
	name    string
	signals []Signal
	err     error
}

func (m *mockSignalSource) Name() string                            { return m.name }
func (m *mockSignalSource) Configure(cfg map[string]string) error   { return nil }
func (m *mockSignalSource) FetchSignals(mod Module) ([]Signal, error) {
	return m.signals, m.err
}

func TestRegistry_FetchAll(t *testing.T) {
	reg := NewRegistry()

	// Source A: returns 2 signals successfully.
	reg.Register(&mockSignalSource{
		name: "sourceA",
		signals: []Signal{
			{Type: "ticket", ID: "JIRA-100", Title: "Fix login"},
			{Type: "doc", ID: "DOC-1", Title: "API docs"},
		},
	})

	// Source B: returns an error (should be skipped, not fail).
	reg.Register(&mockSignalSource{
		name: "sourceB",
		err:  fmt.Errorf("connection refused"),
	})

	// Source C: returns 1 signal.
	reg.Register(&mockSignalSource{
		name: "sourceC",
		signals: []Signal{
			{Type: "pr", ID: "#99", Title: "Add feature"},
		},
	})

	module := Module{Name: "test"}
	all, err := reg.FetchAll(module)
	if err != nil {
		t.Fatalf("FetchAll returned error: %v", err)
	}

	if len(all) != 3 {
		t.Errorf("expected 3 signals from FetchAll, got %d", len(all))
	}

	// Verify we got the expected types.
	types := map[string]int{}
	for _, s := range all {
		types[s.Type]++
	}
	if types["ticket"] != 1 {
		t.Errorf("expected 1 ticket signal, got %d", types["ticket"])
	}
	if types["doc"] != 1 {
		t.Errorf("expected 1 doc signal, got %d", types["doc"])
	}
	if types["pr"] != 1 {
		t.Errorf("expected 1 pr signal, got %d", types["pr"])
	}
}

func TestRegistry_FetchAll_Empty(t *testing.T) {
	reg := NewRegistry()
	module := Module{Name: "test"}
	all, err := reg.FetchAll(module)
	if err != nil {
		t.Fatalf("FetchAll on empty registry returned error: %v", err)
	}
	if len(all) != 0 {
		t.Errorf("expected 0 signals from empty registry, got %d", len(all))
	}
}

func TestPRRefPattern(t *testing.T) {
	cases := []struct {
		input string
		want  []string // expected PR number strings
	}{
		{"Fix bug from PR #42", []string{"42"}},
		{"Merge pull #7 into main", []string{"7"}},
		{"Refs #100, #200", []string{"100", "200"}},
		{"No PR reference here", nil},
		{"PR #1 and PR #2", []string{"1", "2"}},
		{"pull request #55 fix", []string{"55"}},
	}

	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			matches := prRefPattern.FindAllStringSubmatch(tc.input, -1)
			var got []string
			for _, m := range matches {
				got = append(got, m[1])
			}
			if len(got) != len(tc.want) {
				t.Errorf("input %q: got %v, want %v", tc.input, got, tc.want)
				return
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("input %q: match[%d] = %q, want %q", tc.input, i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestGitSignalSource_FetchSignals_EmptyModule(t *testing.T) {
	// Verify FetchSignals works with an empty RelPath (scans whole repo).
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	repoDir := setupTestRepo(t)
	src := NewGitSignalSource(repoDir)

	module := Module{
		Name:    "root",
		Path:    repoDir,
		RelPath: "", // empty — scan entire repo
	}

	signals, err := src.FetchSignals(module)
	if err != nil {
		t.Fatalf("FetchSignals returned error: %v", err)
	}
	if len(signals) == 0 {
		t.Error("expected signals for whole-repo scan, got 0")
	}
}

// Ensure GitSignalSource satisfies the SignalSource interface at compile time.
var _ SignalSource = (*GitSignalSource)(nil)

// Suppress unused import warning for time if tests don't reference it directly.
var _ = time.Now
