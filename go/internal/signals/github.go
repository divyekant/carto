package signals

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// GitHubSignalSource fetches issues and PRs from the GitHub API.
type GitHubSignalSource struct {
	owner   string
	repo    string
	token   string
	baseURL string
	http    http.Client
}

// NewGitHubSignalSource creates an unconfigured GitHub signal source.
func NewGitHubSignalSource() *GitHubSignalSource {
	return &GitHubSignalSource{
		baseURL: "https://api.github.com",
		http:    http.Client{Timeout: 15 * time.Second},
	}
}

func (g *GitHubSignalSource) Name() string { return "github" }

func (g *GitHubSignalSource) Configure(cfg map[string]string) error {
	g.owner = cfg["owner"]
	g.repo = cfg["repo"]
	if t, ok := cfg["token"]; ok {
		g.token = t
	}
	if g.owner == "" || g.repo == "" {
		return fmt.Errorf("github: owner and repo are required")
	}
	return nil
}

func (g *GitHubSignalSource) FetchSignals(module Module) ([]Signal, error) {
	var signals []Signal

	issues, err := g.fetchIssues()
	if err != nil {
		return nil, fmt.Errorf("github: fetch issues: %w", err)
	}
	signals = append(signals, issues...)

	prs, err := g.fetchPRs()
	if err != nil {
		return nil, fmt.Errorf("github: fetch PRs: %w", err)
	}
	signals = append(signals, prs...)

	return signals, nil
}

type ghIssue struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Body        string        `json:"body"`
	HTMLURL     string        `json:"html_url"`
	CreatedAt   time.Time     `json:"created_at"`
	User        ghUser        `json:"user"`
	PullRequest *ghPullReqRef `json:"pull_request"`
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
}

type ghUser struct {
	Login string `json:"login"`
}

func (g *GitHubSignalSource) apiGet(path string, v any) error {
	req, err := http.NewRequest("GET", g.baseURL+path, nil)
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

func (g *GitHubSignalSource) fetchIssues() ([]Signal, error) {
	var ghIssues []ghIssue
	path := fmt.Sprintf("/repos/%s/%s/issues?state=all&per_page=30&sort=updated", g.owner, g.repo)
	if err := g.apiGet(path, &ghIssues); err != nil {
		return nil, err
	}

	var signals []Signal
	for _, issue := range ghIssues {
		if issue.PullRequest != nil {
			continue
		}
		signals = append(signals, Signal{
			Type:   "issue",
			ID:     fmt.Sprintf("#%d", issue.Number),
			Title:  issue.Title,
			Body:   truncateBody(issue.Body, 500),
			URL:    issue.HTMLURL,
			Date:   issue.CreatedAt,
			Author: issue.User.Login,
		})
	}
	return signals, nil
}

func (g *GitHubSignalSource) fetchPRs() ([]Signal, error) {
	var ghPRs []ghPR
	path := fmt.Sprintf("/repos/%s/%s/pulls?state=all&per_page=30&sort=updated", g.owner, g.repo)
	if err := g.apiGet(path, &ghPRs); err != nil {
		return nil, err
	}

	var signals []Signal
	for _, pr := range ghPRs {
		signals = append(signals, Signal{
			Type:   "pr",
			ID:     fmt.Sprintf("#%d", pr.Number),
			Title:  pr.Title,
			Body:   truncateBody(pr.Body, 500),
			URL:    pr.HTMLURL,
			Date:   pr.CreatedAt,
			Author: pr.User.Login,
		})
	}
	return signals, nil
}

func truncateBody(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
