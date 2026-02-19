package sources

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
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

func setupGitTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	gitCmd(t, dir, "init")

	modDir := filepath.Join(dir, "mymodule")
	os.MkdirAll(modDir, 0o755)

	os.WriteFile(filepath.Join(modDir, "main.go"), []byte("package main\n"), 0o644)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "Initial commit")

	os.WriteFile(filepath.Join(modDir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "Add main function")

	os.WriteFile(filepath.Join(modDir, "util.go"), []byte("package main\n\nfunc helper() {}\n"), 0o644)
	gitCmd(t, dir, "add", ".")
	gitCmd(t, dir, "commit", "-m", "Fix bug from PR #42")

	return dir
}

func TestGitSource_Name(t *testing.T) {
	src := NewGitSource("")
	if src.Name() != "git" {
		t.Errorf("Name() = %q, want %q", src.Name(), "git")
	}
}

func TestGitSource_Scope(t *testing.T) {
	src := NewGitSource("")
	if src.Scope() != ModuleScope {
		t.Errorf("Scope() = %d, want ModuleScope", src.Scope())
	}
}

func TestGitSource_Fetch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found in PATH")
	}

	repoDir := setupGitTestRepo(t)
	src := NewGitSource(repoDir)

	req := FetchRequest{
		Project:    "test",
		Module:     "mymodule",
		ModulePath: filepath.Join(repoDir, "mymodule"),
		RepoRoot:   repoDir,
	}

	artifacts, err := src.Fetch(context.Background(), req)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	var commits int
	for _, a := range artifacts {
		if a.Category != Signal {
			t.Errorf("expected Signal category, got %s", a.Category)
		}
		if a.Source != "git" {
			t.Errorf("expected source=git, got %s", a.Source)
		}
		if a.Tags["type"] == "commit" {
			commits++
		}
	}

	if commits < 3 {
		t.Errorf("expected at least 3 commits, got %d", commits)
	}

	// Verify PR reference extraction.
	var foundPR42 bool
	for _, a := range artifacts {
		if a.Tags["type"] == "pr" && a.ID == "#42" {
			foundPR42 = true
		}
	}
	if !foundPR42 {
		t.Error("expected PR #42 artifact from commit message")
	}

	// Verify sorted newest first.
	for i := 1; i < len(artifacts); i++ {
		if artifacts[i].Date.After(artifacts[i-1].Date) {
			t.Errorf("not sorted newest-first at index %d", i)
		}
	}
}

func TestGitSource_NonGitDir(t *testing.T) {
	dir := t.TempDir()
	src := NewGitSource(dir)

	artifacts, err := src.Fetch(context.Background(), FetchRequest{
		Module:     "test",
		ModulePath: dir,
		RepoRoot:   dir,
	})
	if err != nil {
		t.Fatalf("expected nil error for non-git dir, got: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(artifacts))
	}
}

var _ Source = (*GitSource)(nil)
