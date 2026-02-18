package signals

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"
)

// prRefPattern matches PR references like "PR #42", "#123", "pull #7".
var prRefPattern = regexp.MustCompile(`(?i)(?:PR\s*#|pull\s*(?:request)?\s*#|#)(\d+)`)

// GitSignalSource extracts commit-based signals from git history.
type GitSignalSource struct {
	repoRoot   string
	maxCommits int
}

// NewGitSignalSource creates a git signal source rooted at the given directory.
func NewGitSignalSource(repoRoot string) *GitSignalSource {
	return &GitSignalSource{repoRoot: repoRoot, maxCommits: 20}
}

// Name returns the identifier for this signal source.
func (g *GitSignalSource) Name() string {
	return "git"
}

// Configure applies key-value settings. Recognised keys: "repo_root".
func (g *GitSignalSource) Configure(cfg map[string]string) error {
	if root, ok := cfg["repo_root"]; ok {
		g.repoRoot = root
	}
	return nil
}

// FetchSignals runs git log for the given module and returns commit and PR
// signals sorted by date (newest first). Returns an empty slice (not an
// error) when the directory is not inside a git repository.
func (g *GitSignalSource) FetchSignals(module Module) ([]Signal, error) {
	// Build the git log command.
	args := []string{
		"-C", g.repoRoot,
		"log",
		fmt.Sprintf("--pretty=format:%%H|%%an|%%aI|%%s"),
		fmt.Sprintf("-n%d", g.maxCommits),
	}
	if module.RelPath != "" {
		args = append(args, "--", module.RelPath)
	}

	cmd := exec.Command("git", args...)
	out, err := cmd.Output()
	if err != nil {
		// Not a git repo or git not installed — return empty, not error.
		return nil, nil
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}

	var signals []Signal
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		hash := parts[0]
		author := parts[1]
		dateStr := parts[2]
		subject := parts[3]

		date, err := time.Parse(time.RFC3339, dateStr)
		if err != nil {
			date = time.Time{}
		}

		// Commit signal.
		signals = append(signals, Signal{
			Type:   "commit",
			ID:     hash,
			Title:  subject,
			Date:   date,
			Author: author,
		})

		// Extract PR references from the commit subject.
		matches := prRefPattern.FindAllStringSubmatch(subject, -1)
		for _, m := range matches {
			signals = append(signals, Signal{
				Type:   "pr",
				ID:     "#" + m[1],
				Title:  subject,
				Date:   date,
				Author: author,
			})
		}
	}

	// Sort newest first.
	sort.Slice(signals, func(i, j int) bool {
		return signals[i].Date.After(signals[j].Date)
	})

	return signals, nil
}
