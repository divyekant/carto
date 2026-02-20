package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// LinearSource fetches issues from the Linear GraphQL API.
type LinearSource struct {
	teamKey    string
	token      string
	apiURL     string
	maxResults int
	http       http.Client
}

// Compile-time interface check.
var _ Source = (*LinearSource)(nil)

// NewLinearSource creates an unconfigured Linear source with sensible defaults.
func NewLinearSource() *LinearSource {
	return &LinearSource{
		apiURL:     "https://api.linear.app/graphql",
		maxResults: 50,
		http:       http.Client{Timeout: 15 * time.Second},
	}
}

func (l *LinearSource) Name() string { return "linear" }
func (l *LinearSource) Scope() Scope { return ProjectScope }

func (l *LinearSource) Configure(cfg SourceConfig) error {
	l.teamKey = cfg.Settings["team_key"]
	if t, ok := cfg.Credentials["linear_token"]; ok {
		l.token = t
	}
	if l.teamKey == "" {
		return fmt.Errorf("linear: team_key is required")
	}
	return nil
}

func (l *LinearSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	query := fmt.Sprintf(`{
  issues(filter: { team: { key: { eq: "%s" } } }, first: %d, orderBy: updatedAt) {
    nodes {
      identifier
      title
      description
      url
      updatedAt
      creator { name }
      state { name }
      priority
      labels { nodes { name } }
    }
  }
}`, l.teamKey, l.maxResults)

	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		return nil, fmt.Errorf("linear: marshal query: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", l.apiURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("linear: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if l.token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+l.token)
	}

	resp, err := l.http.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("linear: request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("linear: API returned %d", resp.StatusCode)
	}

	var gqlResp linearGraphQLResponse
	if err := json.NewDecoder(resp.Body).Decode(&gqlResp); err != nil {
		return nil, fmt.Errorf("linear: decode response: %w", err)
	}

	var artifacts []Artifact
	for _, node := range gqlResp.Data.Issues.Nodes {
		updatedAt, _ := time.Parse(time.RFC3339, node.UpdatedAt)

		tags := map[string]string{
			"type":     "issue",
			"status":   node.State.Name,
			"priority": fmt.Sprintf("%d", node.Priority),
		}
		if labelNames := collectLabelNames(node.Labels.Nodes); labelNames != "" {
			tags["labels"] = labelNames
		}

		artifacts = append(artifacts, Artifact{
			Source:   "linear",
			Category: Signal,
			ID:       node.Identifier,
			Title:    node.Title,
			Body:     truncateBody(node.Description, 500),
			URL:      node.URL,
			Date:     updatedAt,
			Author:   node.Creator.Name,
			Tags:     tags,
		})
	}

	return artifacts, nil
}

// collectLabelNames joins label names into a comma-separated string.
func collectLabelNames(labels []linearLabel) string {
	names := make([]string, 0, len(labels))
	for _, l := range labels {
		if l.Name != "" {
			names = append(names, l.Name)
		}
	}
	return strings.Join(names, ",")
}

// GraphQL response types for the Linear API.

type linearGraphQLResponse struct {
	Data struct {
		Issues struct {
			Nodes []linearIssue `json:"nodes"`
		} `json:"issues"`
	} `json:"data"`
}

type linearIssue struct {
	Identifier  string       `json:"identifier"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	URL         string       `json:"url"`
	UpdatedAt   string       `json:"updatedAt"`
	Creator     linearUser   `json:"creator"`
	State       linearState  `json:"state"`
	Priority    int          `json:"priority"`
	Labels      linearLabels `json:"labels"`
}

type linearUser struct {
	Name string `json:"name"`
}

type linearState struct {
	Name string `json:"name"`
}

type linearLabels struct {
	Nodes []linearLabel `json:"nodes"`
}

type linearLabel struct {
	Name string `json:"name"`
}
