package sources

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Compile-time interface check.
var _ Source = (*JiraSource)(nil)

// JiraSource fetches issues from the Jira REST API v3.
type JiraSource struct {
	baseURL    string
	email      string
	token      string
	projectKey string
	maxResults int
	http       http.Client
}

// NewJiraSource creates an unconfigured Jira source with sensible defaults.
func NewJiraSource() *JiraSource {
	return &JiraSource{
		maxResults: 50,
		http:       http.Client{Timeout: 15 * time.Second},
	}
}

func (j *JiraSource) Name() string { return "jira" }
func (j *JiraSource) Scope() Scope { return ProjectScope }

func (j *JiraSource) Configure(cfg SourceConfig) error {
	j.baseURL = strings.TrimRight(cfg.Settings["base_url"], "/")
	j.projectKey = cfg.Settings["project_key"]
	if t, ok := cfg.Credentials["jira_token"]; ok {
		j.token = t
	}
	if e, ok := cfg.Credentials["jira_email"]; ok {
		j.email = e
	}
	if j.baseURL == "" {
		return fmt.Errorf("jira: base_url is required")
	}
	if j.projectKey == "" {
		return fmt.Errorf("jira: project_key is required")
	}
	return nil
}

func (j *JiraSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	issues, err := j.searchIssues(ctx)
	if err != nil {
		return nil, fmt.Errorf("jira: search issues: %w", err)
	}
	return issues, nil
}

// jiraSearchResponse is the top-level response from /rest/api/3/search.
type jiraSearchResponse struct {
	Issues []jiraIssue `json:"issues"`
}

type jiraIssue struct {
	Key    string          `json:"key"`
	Fields jiraIssueFields `json:"fields"`
}

type jiraIssueFields struct {
	Summary     string        `json:"summary"`
	Description string        `json:"description"`
	Updated     string        `json:"updated"`
	Creator     jiraUser      `json:"creator"`
	IssueType   jiraNameField `json:"issuetype"`
	Status      jiraNameField `json:"status"`
	Priority    jiraNameField `json:"priority"`
}

type jiraUser struct {
	DisplayName string `json:"displayName"`
}

type jiraNameField struct {
	Name string `json:"name"`
}

func (j *JiraSource) searchIssues(ctx context.Context) ([]Artifact, error) {
	jql := fmt.Sprintf("project=%s ORDER BY updated DESC", j.projectKey)
	params := url.Values{}
	params.Set("jql", jql)
	params.Set("maxResults", strconv.Itoa(j.maxResults))
	path := "/rest/api/3/search?" + params.Encode()

	var result jiraSearchResponse
	if err := j.apiGet(ctx, path, &result); err != nil {
		return nil, err
	}

	var artifacts []Artifact
	for _, issue := range result.Issues {
		updated, _ := time.Parse("2006-01-02T15:04:05.000-0700", issue.Fields.Updated)

		artifacts = append(artifacts, Artifact{
			Source:   "jira",
			Category: Signal,
			ID:       issue.Key,
			Title:    issue.Fields.Summary,
			Body:     truncateBody(issue.Fields.Description, 500),
			URL:      j.baseURL + "/browse/" + issue.Key,
			Date:     updated,
			Author:   issue.Fields.Creator.DisplayName,
			Tags: map[string]string{
				"type":     issue.Fields.IssueType.Name,
				"status":   issue.Fields.Status.Name,
				"priority": issue.Fields.Priority.Name,
			},
		})
	}
	return artifacts, nil
}

func (j *JiraSource) apiGet(ctx context.Context, path string, v any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", j.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if j.email != "" && j.token != "" {
		creds := base64.StdEncoding.EncodeToString([]byte(j.email + ":" + j.token))
		req.Header.Set("Authorization", "Basic "+creds)
	}

	resp, err := j.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
