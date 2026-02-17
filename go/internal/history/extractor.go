package history

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
)

// CommitInfo holds metadata for a single git commit.
type CommitInfo struct {
	Hash    string
	Author  string
	Date    string
	Message string
	PRRef   string // extracted PR reference like "#247" or "PR-123"
}

// FileHistory holds the git history for a single file.
type FileHistory struct {
	FilePath   string
	Commits    []CommitInfo
	Authors    []string  // unique authors
	ChurnScore float64   // number of commits as a proxy for complexity
}

// ExtractOptions controls how much history to fetch.
type ExtractOptions struct {
	MaxCommits int    // default 50 per file
	Since      string // git date format, default "6 months ago"
}

func (o *ExtractOptions) maxCommits() int {
	if o != nil && o.MaxCommits > 0 {
		return o.MaxCommits
	}
	return 50
}

func (o *ExtractOptions) since() string {
	if o != nil && o.Since != "" {
		return o.Since
	}
	return "6 months ago"
}

// prRefRe matches PR references in commit messages:
//   - "#123" (GitHub-style)
//   - "PR-123" or "PR 123" or "PR123" (Jira/other-style)
var prRefRe = regexp.MustCompile(`(?i)(?:#(\d+)|PR[- ]?(\d+))`)

// ParsePRReference extracts the first PR reference from a commit message.
// Returns strings like "#123" or "PR-456", or "" if none found.
func ParsePRReference(message string) string {
	m := prRefRe.FindStringSubmatch(message)
	if m == nil {
		return ""
	}
	// m[1] is the GitHub-style number (from #(\d+))
	// m[2] is the PR-style number (from PR[- ]?(\d+))
	if m[1] != "" {
		return "#" + m[1]
	}
	return "PR-" + m[2]
}

// ExtractFileHistory runs `git log` for a single file and parses the output
// into structured commit data. If git is unavailable or the path is not inside
// a git repo, it returns an empty history without an error.
func ExtractFileHistory(repoRoot string, relPath string, opts *ExtractOptions) (*FileHistory, error) {
	maxCommits := opts.maxCommits()
	since := opts.since()

	args := []string{
		"log",
		"--follow",
		"--pretty=format:%H|%an|%aI|%s",
		fmt.Sprintf("-n%d", maxCommits),
		fmt.Sprintf("--since=%s", since),
		"--",
		relPath,
	}

	cmd := exec.Command("git", args...)
	cmd.Dir = repoRoot

	out, err := cmd.Output()
	if err != nil {
		// git not installed, not a repo, or file has no history — return empty.
		return &FileHistory{FilePath: relPath}, nil
	}

	output := strings.TrimSpace(string(out))
	if output == "" {
		return &FileHistory{FilePath: relPath}, nil
	}

	lines := strings.Split(output, "\n")
	commits := make([]CommitInfo, 0, len(lines))
	authorSet := make(map[string]struct{})

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Split into at most 4 parts: hash, author, date, subject.
		// The subject itself may contain "|" so we limit splits.
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}

		ci := CommitInfo{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    parts[2],
			Message: parts[3],
			PRRef:   ParsePRReference(parts[3]),
		}
		commits = append(commits, ci)
		authorSet[ci.Author] = struct{}{}
	}

	authors := make([]string, 0, len(authorSet))
	for a := range authorSet {
		authors = append(authors, a)
	}

	return &FileHistory{
		FilePath:   relPath,
		Commits:    commits,
		Authors:    authors,
		ChurnScore: float64(len(commits)),
	}, nil
}

// ExtractBulkHistory extracts history for multiple files in parallel.
// maxWorkers controls the concurrency level (goroutine count).
func ExtractBulkHistory(repoRoot string, relPaths []string, opts *ExtractOptions, maxWorkers int) ([]*FileHistory, error) {
	if maxWorkers <= 0 {
		maxWorkers = 4
	}

	results := make([]*FileHistory, len(relPaths))
	errs := make([]error, len(relPaths))

	sem := make(chan struct{}, maxWorkers)
	var wg sync.WaitGroup

	for i, p := range relPaths {
		wg.Add(1)
		go func(idx int, path string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			h, err := ExtractFileHistory(repoRoot, path, opts)
			results[idx] = h
			errs[idx] = err
		}(i, p)
	}

	wg.Wait()

	// Return the first error encountered, if any.
	for _, err := range errs {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}
