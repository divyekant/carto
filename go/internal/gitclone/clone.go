package gitclone

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// CloneOptions configures a git clone operation.
type CloneOptions struct {
	URL    string
	Branch string
	Token  string
	Depth  int
}

// CloneResult holds the result of a successful clone.
type CloneResult struct {
	Dir     string
	Cleanup func()
}

// IsGitURL returns true if the input looks like a Git URL rather than a local path.
func IsGitURL(input string) bool {
	if input == "" {
		return false
	}
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		return true
	}
	if strings.HasPrefix(input, "git@") {
		return true
	}
	return false
}

// ParseRepoName extracts the repository name from a Git URL.
func ParseRepoName(gitURL string) string {
	if strings.HasPrefix(gitURL, "git@") {
		parts := strings.SplitN(gitURL, ":", 2)
		if len(parts) == 2 {
			name := filepath.Base(parts[1])
			return strings.TrimSuffix(name, ".git")
		}
	}
	u, err := url.Parse(gitURL)
	if err != nil {
		return filepath.Base(gitURL)
	}
	name := filepath.Base(u.Path)
	return strings.TrimSuffix(name, ".git")
}

// ParseOwnerRepo extracts owner and repo name from a GitHub URL.
// Returns ("", "") if the URL is not a recognized GitHub URL.
func ParseOwnerRepo(gitURL string) (owner, repo string) {
	if strings.HasPrefix(gitURL, "git@github.com:") {
		path := strings.TrimPrefix(gitURL, "git@github.com:")
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) == 2 {
			return parts[0], parts[1]
		}
		return "", ""
	}
	u, err := url.Parse(gitURL)
	if err != nil || u.Host != "github.com" {
		return "", ""
	}
	path := strings.TrimPrefix(u.Path, "/")
	path = strings.TrimSuffix(path, ".git")
	parts := strings.SplitN(path, "/", 3)
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

// Clone performs a shallow git clone to a temporary directory.
func Clone(opts CloneOptions) (*CloneResult, error) {
	if opts.URL == "" {
		return nil, fmt.Errorf("gitclone: URL is required")
	}
	if opts.Depth == 0 {
		opts.Depth = 1
	}

	tmpDir, err := os.MkdirTemp("", "carto-clone-*")
	if err != nil {
		return nil, fmt.Errorf("gitclone: create temp dir: %w", err)
	}
	cleanup := func() { os.RemoveAll(tmpDir) }

	cloneURL := opts.URL
	if opts.Token != "" && strings.HasPrefix(cloneURL, "https://") {
		u, err := url.Parse(cloneURL)
		if err == nil {
			u.User = url.UserPassword("x-access-token", opts.Token)
			cloneURL = u.String()
		}
	}

	args := []string{"clone", "--depth", fmt.Sprintf("%d", opts.Depth)}
	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}
	args = append(args, cloneURL, tmpDir)

	cmd := exec.Command("git", args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		cleanup()
		return nil, fmt.Errorf("gitclone: git clone failed: %w", err)
	}

	return &CloneResult{Dir: tmpDir, Cleanup: cleanup}, nil
}
