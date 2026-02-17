package history

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"
)

// gitCmd runs a git command in the given directory with deterministic author info.
func gitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	fullArgs := append([]string{
		"-c", "user.name=test",
		"-c", "user.email=test@test.com",
	}, args...)
	cmd := exec.Command("git", fullArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@test.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\noutput: %s", args, err, out)
	}
}

// initTestRepo creates a temp dir with a git repo containing two commits
// on a single file. Returns the repo root and a cleanup function.
func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	gitCmd(t, dir, "init")

	// First commit.
	filePath := filepath.Join(dir, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "hello.txt")
	gitCmd(t, dir, "commit", "-m", "Initial commit (#10)")

	// Second commit.
	if err := os.WriteFile(filePath, []byte("hello world\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	gitCmd(t, dir, "add", "hello.txt")
	gitCmd(t, dir, "commit", "-m", "Update hello PR-42: add world")

	return dir
}

func TestParsePRReference(t *testing.T) {
	tests := []struct {
		message string
		want    string
	}{
		{"Fix bug (#123)", "#123"},
		{"PR-456: add feature", "PR-456"},
		{"Merge pull request #789 from branch", "#789"},
		{"no PR here", ""},
		{"just some commit", ""},
		{"PR 99 something", "PR-99"},
		{"PR100 hotfix", "PR-100"},
		{"multiple #1 and #2", "#1"}, // returns the first match
		{"lowercase pr-55 ref", "PR-55"},
	}

	for _, tt := range tests {
		t.Run(tt.message, func(t *testing.T) {
			got := ParsePRReference(tt.message)
			if got != tt.want {
				t.Errorf("ParsePRReference(%q) = %q, want %q", tt.message, got, tt.want)
			}
		})
	}
}

func TestExtractFileHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := initTestRepo(t)

	// Use a long-ago since value so both commits are captured.
	opts := &ExtractOptions{
		MaxCommits: 50,
		Since:      "10 years ago",
	}

	h, err := ExtractFileHistory(dir, "hello.txt", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if h.FilePath != "hello.txt" {
		t.Errorf("FilePath = %q, want %q", h.FilePath, "hello.txt")
	}

	if len(h.Commits) != 2 {
		t.Fatalf("expected 2 commits, got %d", len(h.Commits))
	}

	// Most recent commit first in git log output.
	if h.Commits[0].Message != "Update hello PR-42: add world" {
		t.Errorf("first commit message = %q", h.Commits[0].Message)
	}
	if h.Commits[0].PRRef != "PR-42" {
		t.Errorf("first commit PRRef = %q, want %q", h.Commits[0].PRRef, "PR-42")
	}

	if h.Commits[1].Message != "Initial commit (#10)" {
		t.Errorf("second commit message = %q", h.Commits[1].Message)
	}
	if h.Commits[1].PRRef != "#10" {
		t.Errorf("second commit PRRef = %q, want %q", h.Commits[1].PRRef, "#10")
	}

	// Both commits are by "test".
	if len(h.Authors) != 1 || h.Authors[0] != "test" {
		t.Errorf("Authors = %v, want [test]", h.Authors)
	}

	if h.ChurnScore != 2.0 {
		t.Errorf("ChurnScore = %f, want 2.0", h.ChurnScore)
	}

	// Verify commit hashes are non-empty.
	for i, c := range h.Commits {
		if len(c.Hash) < 7 {
			t.Errorf("commit %d has short hash: %q", i, c.Hash)
		}
		if c.Author != "test" {
			t.Errorf("commit %d Author = %q, want %q", i, c.Author, "test")
		}
		if c.Date == "" {
			t.Errorf("commit %d has empty Date", i)
		}
	}
}

func TestExtractFileHistory_DefaultOptions(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := initTestRepo(t)

	// nil opts should use defaults and not panic.
	h, err := ExtractFileHistory(dir, "hello.txt", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With default "6 months ago", the commits we just made should appear.
	if len(h.Commits) != 2 {
		t.Fatalf("expected 2 commits with default opts, got %d", len(h.Commits))
	}
}

func TestExtractFileHistory_NonGitDir(t *testing.T) {
	dir := t.TempDir()

	h, err := ExtractFileHistory(dir, "nonexistent.go", nil)
	if err != nil {
		t.Fatalf("expected no error for non-git dir, got %v", err)
	}

	if h.FilePath != "nonexistent.go" {
		t.Errorf("FilePath = %q, want %q", h.FilePath, "nonexistent.go")
	}
	if len(h.Commits) != 0 {
		t.Errorf("expected 0 commits for non-git dir, got %d", len(h.Commits))
	}
}

func TestExtractFileHistory_NonexistentFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := initTestRepo(t)

	h, err := ExtractFileHistory(dir, "does_not_exist.txt", nil)
	if err != nil {
		t.Fatalf("expected no error for missing file, got %v", err)
	}

	if len(h.Commits) != 0 {
		t.Errorf("expected 0 commits for missing file, got %d", len(h.Commits))
	}
}

func TestExtractFileHistory_MaxCommits(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := initTestRepo(t)

	opts := &ExtractOptions{
		MaxCommits: 1,
		Since:      "10 years ago",
	}

	h, err := ExtractFileHistory(dir, "hello.txt", opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(h.Commits) != 1 {
		t.Fatalf("expected 1 commit with MaxCommits=1, got %d", len(h.Commits))
	}
}

func TestExtractBulkHistory(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	dir := t.TempDir()
	gitCmd(t, dir, "init")

	// Create two files with commits.
	for _, name := range []string{"a.txt", "b.txt"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("content of "+name+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		gitCmd(t, dir, "add", name)
		gitCmd(t, dir, "commit", "-m", "Add "+name)
	}

	opts := &ExtractOptions{Since: "10 years ago"}
	results, err := ExtractBulkHistory(dir, []string{"a.txt", "b.txt"}, opts, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Collect filenames and sort so order doesn't matter.
	names := []string{results[0].FilePath, results[1].FilePath}
	sort.Strings(names)
	if names[0] != "a.txt" || names[1] != "b.txt" {
		t.Errorf("unexpected file paths: %v", names)
	}

	for _, h := range results {
		if len(h.Commits) != 1 {
			t.Errorf("file %s: expected 1 commit, got %d", h.FilePath, len(h.Commits))
		}
		if h.ChurnScore != 1.0 {
			t.Errorf("file %s: ChurnScore = %f, want 1.0", h.FilePath, h.ChurnScore)
		}
	}
}

func TestExtractBulkHistory_EmptyInput(t *testing.T) {
	results, err := ExtractBulkHistory("/tmp", nil, nil, 4)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for empty input, got %d", len(results))
	}
}

func TestExtractBulkHistory_NonGitDir(t *testing.T) {
	dir := t.TempDir()

	results, err := ExtractBulkHistory(dir, []string{"foo.go", "bar.go"}, nil, 2)
	if err != nil {
		t.Fatalf("expected no error for non-git dir, got %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	for _, h := range results {
		if len(h.Commits) != 0 {
			t.Errorf("expected 0 commits for non-git dir file %s, got %d", h.FilePath, len(h.Commits))
		}
	}
}
