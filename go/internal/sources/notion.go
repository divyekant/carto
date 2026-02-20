package sources

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Compile-time interface check.
var _ Source = (*NotionSource)(nil)

// NotionSource fetches pages from a Notion database via the Notion API.
type NotionSource struct {
	databaseID string
	token      string
	baseURL    string
	maxPages   int
	http       http.Client
}

// NewNotionSource creates an unconfigured Notion source with sensible defaults.
func NewNotionSource() *NotionSource {
	return &NotionSource{
		baseURL:  "https://api.notion.com/v1",
		maxPages: 50,
		http:     http.Client{Timeout: 15 * time.Second},
	}
}

func (n *NotionSource) Name() string { return "notion" }
func (n *NotionSource) Scope() Scope { return ProjectScope }

func (n *NotionSource) Configure(cfg SourceConfig) error {
	n.databaseID = cfg.Settings["database_id"]
	if t, ok := cfg.Credentials["notion_token"]; ok {
		n.token = t
	}
	if n.databaseID == "" {
		return fmt.Errorf("notion: database_id is required")
	}
	return nil
}

func (n *NotionSource) Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	pages, err := n.queryDatabase(ctx)
	if err != nil {
		return nil, fmt.Errorf("notion: query database: %w", err)
	}

	var artifacts []Artifact
	for _, page := range pages {
		body, err := n.fetchBlockContent(ctx, page.ID)
		if err != nil {
			// Skip pages whose content cannot be retrieved.
			continue
		}

		artifacts = append(artifacts, Artifact{
			Source:   "notion",
			Category: Knowledge,
			ID:       page.ID,
			Title:    extractNotionTitle(page.Properties),
			Body:     truncateBody(body, 2000),
			URL:      page.URL,
			Date:     page.LastEditedTime,
			Author:   "",
			Tags:     map[string]string{"type": "page"},
		})
	}

	return artifacts, nil
}

// --- Notion API types ---

type notionQueryRequest struct {
	PageSize int              `json:"page_size"`
	Sorts    []notionSort     `json:"sorts"`
}

type notionSort struct {
	Timestamp string `json:"timestamp"`
	Direction string `json:"direction"`
}

type notionQueryResponse struct {
	Results []notionPage `json:"results"`
}

type notionPage struct {
	ID             string                       `json:"id"`
	URL            string                       `json:"url"`
	LastEditedTime time.Time                    `json:"last_edited_time"`
	Properties     map[string]notionProperty    `json:"properties"`
}

type notionProperty struct {
	Type  string             `json:"type"`
	Title []notionRichText   `json:"title,omitempty"`
}

type notionRichText struct {
	PlainText string `json:"plain_text"`
}

type notionBlocksResponse struct {
	Results []notionBlock `json:"results"`
}

type notionBlock struct {
	Type      string              `json:"type"`
	Paragraph *notionRichTextWrap `json:"paragraph,omitempty"`
	Heading1  *notionRichTextWrap `json:"heading_1,omitempty"`
	Heading2  *notionRichTextWrap `json:"heading_2,omitempty"`
	Heading3  *notionRichTextWrap `json:"heading_3,omitempty"`
	BulletedListItem *notionRichTextWrap `json:"bulleted_list_item,omitempty"`
	NumberedListItem *notionRichTextWrap `json:"numbered_list_item,omitempty"`
	Toggle    *notionRichTextWrap `json:"toggle,omitempty"`
	Quote     *notionRichTextWrap `json:"quote,omitempty"`
	ToDo      *notionRichTextWrap `json:"to_do,omitempty"`
	Callout   *notionRichTextWrap `json:"callout,omitempty"`
	Code      *notionRichTextWrap `json:"code,omitempty"`
}

type notionRichTextWrap struct {
	RichText []notionRichText `json:"rich_text"`
}

// --- API helpers ---

func (n *NotionSource) notionRequest(ctx context.Context, method, path string, body any, v any) error {
	var reqBody *bytes.Buffer
	if body != nil {
		reqBody = &bytes.Buffer{}
		if err := json.NewEncoder(reqBody).Encode(body); err != nil {
			return err
		}
	}

	var httpReq *http.Request
	var err error
	if reqBody != nil {
		httpReq, err = http.NewRequestWithContext(ctx, method, n.baseURL+path, reqBody)
	} else {
		httpReq, err = http.NewRequestWithContext(ctx, method, n.baseURL+path, nil)
	}
	if err != nil {
		return err
	}

	httpReq.Header.Set("Authorization", "Bearer "+n.token)
	httpReq.Header.Set("Notion-Version", "2022-06-28")
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := n.http.Do(httpReq)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API returned %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func (n *NotionSource) queryDatabase(ctx context.Context) ([]notionPage, error) {
	reqBody := notionQueryRequest{
		PageSize: n.maxPages,
		Sorts: []notionSort{
			{Timestamp: "last_edited_time", Direction: "descending"},
		},
	}

	var resp notionQueryResponse
	path := fmt.Sprintf("/databases/%s/query", n.databaseID)
	if err := n.notionRequest(ctx, "POST", path, reqBody, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

func (n *NotionSource) fetchBlockContent(ctx context.Context, pageID string) (string, error) {
	var resp notionBlocksResponse
	path := fmt.Sprintf("/blocks/%s/children", pageID)
	if err := n.notionRequest(ctx, "GET", path, nil, &resp); err != nil {
		return "", err
	}
	return extractBlockText(resp.Results), nil
}

// --- Extraction helpers ---

// extractNotionTitle finds the first title-type property and returns its plain text.
func extractNotionTitle(props map[string]notionProperty) string {
	for _, prop := range props {
		if prop.Type == "title" && len(prop.Title) > 0 {
			var title string
			for _, rt := range prop.Title {
				title += rt.PlainText
			}
			return title
		}
	}
	return ""
}

// extractBlockText concatenates plain text from all supported block types.
func extractBlockText(blocks []notionBlock) string {
	var text string
	for _, block := range blocks {
		var wrap *notionRichTextWrap
		switch block.Type {
		case "paragraph":
			wrap = block.Paragraph
		case "heading_1":
			wrap = block.Heading1
		case "heading_2":
			wrap = block.Heading2
		case "heading_3":
			wrap = block.Heading3
		case "bulleted_list_item":
			wrap = block.BulletedListItem
		case "numbered_list_item":
			wrap = block.NumberedListItem
		case "toggle":
			wrap = block.Toggle
		case "quote":
			wrap = block.Quote
		case "to_do":
			wrap = block.ToDo
		case "callout":
			wrap = block.Callout
		case "code":
			wrap = block.Code
		default:
			continue
		}
		if wrap == nil {
			continue
		}
		for _, rt := range wrap.RichText {
			text += rt.PlainText
		}
		text += "\n"
	}
	return text
}
