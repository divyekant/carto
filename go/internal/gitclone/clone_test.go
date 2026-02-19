package gitclone

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsGitURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://github.com/user/repo", true},
		{"https://github.com/user/repo.git", true},
		{"git@github.com:user/repo.git", true},
		{"/Users/dk/projects/my-project", false},
		{"./relative/path", false},
		{"", false},
	}
	for _, tt := range tests {
		got := IsGitURL(tt.input)
		if got != tt.expected {
			t.Errorf("IsGitURL(%q) = %v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestParseRepoName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"https://github.com/user/my-repo", "my-repo"},
		{"https://github.com/user/my-repo.git", "my-repo"},
		{"git@github.com:user/my-repo.git", "my-repo"},
	}
	for _, tt := range tests {
		got := ParseRepoName(tt.input)
		if got != tt.expected {
			t.Errorf("ParseRepoName(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestParseOwnerRepo(t *testing.T) {
	tests := []struct {
		input       string
		expectOwner string
		expectRepo  string
	}{
		{"https://github.com/octocat/Hello-World", "octocat", "Hello-World"},
		{"https://github.com/octocat/Hello-World.git", "octocat", "Hello-World"},
		{"git@github.com:octocat/Hello-World.git", "octocat", "Hello-World"},
		{"https://gitlab.com/user/repo", "", ""},
	}
	for _, tt := range tests {
		owner, repo := ParseOwnerRepo(tt.input)
		if owner != tt.expectOwner || repo != tt.expectRepo {
			t.Errorf("ParseOwnerRepo(%q) = (%q, %q), want (%q, %q)",
				tt.input, owner, repo, tt.expectOwner, tt.expectRepo)
		}
	}
}

func TestClone_PublicRepo(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping clone test in short mode")
	}

	result, err := Clone(CloneOptions{
		URL:   "https://github.com/octocat/Hello-World",
		Depth: 1,
	})
	if err != nil {
		t.Fatalf("Clone failed: %v", err)
	}
	defer result.Cleanup()

	if _, err := os.Stat(filepath.Join(result.Dir, ".git")); err != nil {
		t.Error("expected .git directory in clone")
	}
}
