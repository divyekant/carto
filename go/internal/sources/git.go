package sources

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

var prRefPattern = regexp.MustCompile(`(?i)(?:PR\s*#|pull\s*(?:request)?\s*#|#)(\d+)`)

// GitSource extracts commit-based artifacts from git history.
type GitSource struct {
	repoRoot   string
	maxCommits int
}

// NewGitSource creates a git source rooted at the given directory.
func NewGitSource(repoRoot string) *GitSource {
	return &GitSource{repoRoot: repoRoot, maxCommits: 20}
}

func (g *GitSource) Name() string { return "git" }
func (g *GitSource) Scope() Scope { return ModuleScope }

func (g *GitSource) Configure(cfg SourceConfig) error {
	if root, ok := cfg.Settings["repo_root"]; ok {
		g.repoRoot = root
	}
	if max, ok := cfg.Settings["max_commits"]; ok {
		var n int
		if _, err := fmt.Sscanf(max, "%d", &n); err == nil && n > 0 {
			g.maxCommits = n
		}
	}
	return nil
}

func (g *GitSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	root := g.repoRoot
	if root == "" {
		root = req.RepoRoot
	}

	args := []string{
		"-C", root,
		"log",
		fmt.Sprintf("--pretty=format:%%H|%%an|%%aI|%%s"),
		fmt.Sprintf("-n%d", g.maxCommits),
	}

	// Scope to module's relative path within the repo.
	if req.ModulePath != "" && req.ModulePath != root {
		relPath := strings.TrimPrefix(req.ModulePath, root+"/")
		if relPath != "" {
			args = append(args, "--", relPath)
		}
	}

	cmd := exec.CommandContext(ctx, "git", args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, nil // not a git repo — return empty
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	var artifacts []Artifact
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		hash, author, dateStr, subject := parts[0], parts[1], parts[2], parts[3]
		date, _ := time.Parse(time.RFC3339, dateStr)

		artifacts = append(artifacts, Artifact{
			Source:   "git",
			Category: Signal,
			ID:       hash,
			Title:    subject,
			Date:     date,
			Author:   author,
			Module:   req.Module,
			Tags:     map[string]string{"type": "commit"},
		})

		// Extract PR references.
		matches := prRefPattern.FindAllStringSubmatch(subject, -1)
		for _, m := range matches {
			artifacts = append(artifacts, Artifact{
				Source:   "git",
				Category: Signal,
				ID:       "#" + m[1],
				Title:    subject,
				Date:     date,
				Author:   author,
				Module:   req.Module,
				Tags:     map[string]string{"type": "pr"},
			})
		}
	}

	sort.Slice(artifacts, func(i, j int) bool {
		return artifacts[i].Date.After(artifacts[j].Date)
	})

	return artifacts, nil
}
