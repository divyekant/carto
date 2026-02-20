package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GitHubSource fetches issues and PRs from the GitHub API.
type GitHubSource struct {
	owner    string
	repo     string
	token    string
	baseURL  string
	maxPages int
	http     http.Client
}

// NewGitHubSource creates an unconfigured GitHub source.
func NewGitHubSource() *GitHubSource {
	return &GitHubSource{
		baseURL:  "https://api.github.com",
		maxPages: 3,
		http:     http.Client{Timeout: 15 * time.Second},
	}
}

func (g *GitHubSource) Name() string { return "github" }
func (g *GitHubSource) Scope() Scope { return ProjectScope }

func (g *GitHubSource) Configure(cfg SourceConfig) error {
	g.owner = cfg.Settings["owner"]
	g.repo = cfg.Settings["repo"]
	if t, ok := cfg.Credentials["github_token"]; ok {
		g.token = t
	}
	if g.owner == "" || g.repo == "" {
		return fmt.Errorf("github: owner and repo are required")
	}
	return nil
}

func (g *GitHubSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	var artifacts []Artifact

	issues, err := g.fetchIssues(ctx)
	if err != nil {
		return nil, fmt.Errorf("github: fetch issues: %w", err)
	}
	artifacts = append(artifacts, issues...)

	prs, err := g.fetchPRs(ctx)
	if err != nil {
		return nil, fmt.Errorf("github: fetch PRs: %w", err)
	}
	artifacts = append(artifacts, prs...)

	return artifacts, nil
}

type ghIssue struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	CreatedAt   time.Time     `json:"created_at"`
	User        ghUser        `json:"user"`
	PullRequest *ghPullReqRef `json:"pull_request"`
	State       string        `json:"state"`
}

type ghPullReqRef struct {
	URL string `json:"url"`
}

type ghPR struct {
	Number    int       `json:"number"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	CreatedAt time.Time `json:"created_at"`
	User      ghUser    `json:"user"`
	State     string    `json:"state"`
}

type ghUser struct {
	Login string `json:"login"`
}

func (g *GitHubSource) apiGet(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", g.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if g.token != "" {
		req.Header.Set("Authorization", "Bearer "+g.token)
	}

	resp, err := g.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (g *GitHubSource) fetchIssues(ctx context.Context) ([]Artifact, error) {
	var ghIssues []ghIssue
	path := fmt.Sprintf("/repos/%s/%s/issues?state=all&per_page=30&sort=updated", g.owner, g.repo)
	if err := g.apiGet(ctx, path, &ghIssues); err != nil {
		return nil, err
	}

	var artifacts []Artifact
	for _, issue := range ghIssues {
		if issue.PullRequest != nil {
			continue
		}
		artifacts = append(artifacts, Artifact{
			Source:   "github",
			Category: Signal,
			ID:       fmt.Sprintf("#%d", issue.Number),
			Title:    issue.Title,
			Body:     truncateBody(issue.Body, 500),
			URL:      issue.HTMLURL,
			Date:     issue.CreatedAt,
			Author:   issue.User.Login,
			Tags:     map[string]string{"type": "issue", "state": issue.State},
		})
	}
	return artifacts, nil
}

func (g *GitHubSource) fetchPRs(ctx context.Context) ([]Artifact, error) {
	var ghPRs []ghPR
	path := fmt.Sprintf("/repos/%s/%s/pulls?state=all&per_page=30&sort=updated", g.owner, g.repo)
	if err := g.apiGet(ctx, path, &ghPRs); err != nil {
		return nil, err
	}

	var artifacts []Artifact
	for _, pr := range ghPRs {
		artifacts = append(artifacts, Artifact{
			Source:   "github",
			Category: Signal,
			ID:       fmt.Sprintf("#%d", pr.Number),
			Title:    pr.Title,
			Body:     truncateBody(pr.Body, 500),
			URL:      pr.HTMLURL,
			Date:     pr.CreatedAt,
			Author:   pr.User.Login,
			Tags:     map[string]string{"type": "pr", "state": pr.State},
		})
	}
	return artifacts, nil
}

func truncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
