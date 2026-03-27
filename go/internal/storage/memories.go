package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Memory represents a document to store in the Memories index.
type Memory struct {
	Text       string         `json:"text"`
	Source     string         `json:"source"`
	Key        string         `json:"key,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	DocumentAt string         `json:"document_at,omitempty"`
}

// SearchResult represents a single result returned from Memories.
type SearchResult struct {
	ID           int            `json:"id"`
	Text         string         `json:"text"`
	Score        float64        `json:"score"`
	Source       string         `json:"source"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	MatchType    string         `json:"match_type,omitempty"`
	Confidence   float64        `json:"confidence,omitempty"`
	GraphSupport float64        `json:"graph_support,omitempty"`
}

// SearchOptions controls search behaviour.
type SearchOptions struct {
	K                int     `json:"k,omitempty"`
	Threshold        float64 `json:"threshold,omitempty"`
	Hybrid           bool    `json:"hybrid,omitempty"`
	SourcePrefix     string  `json:"source_prefix,omitempty"`
	ConfidenceWeight float64 `json:"confidence_weight,omitempty"`
	FeedbackWeight   float64 `json:"feedback_weight,omitempty"`
	GraphWeight      float64 `json:"graph_weight,omitempty"`
	Since            string  `json:"since,omitempty"`
	Until            string  `json:"until,omitempty"`
}

// UpsertResult represents the outcome of a single memory upsert.
type UpsertResult struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

// Link represents a graph edge between two memories.
type Link struct {
	ToID      int    `json:"to_id"`
	Type      string `json:"type"`
	CreatedAt string `json:"created_at"`
}

// MemoriesClient talks to the Memories REST API.
type MemoriesClient struct {
	baseURL string
	apiKey  string
	http    http.Client
}

// NewMemoriesClient creates a client for the given base URL and API key.
func NewMemoriesClient(baseURL, apiKey string) *MemoriesClient {
	return &MemoriesClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		http: http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// request is the shared helper for all HTTP calls.
func (c *MemoriesClient) request(method, path string, body any) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("do request: %w", err)
	}
	return resp, nil
}

// Health returns true when the Memories server is reachable.
func (c *MemoriesClient) Health() (bool, error) {
	resp, err := c.request(http.MethodGet, "/health", nil)
	if err != nil {
		return false, nil
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

// AddMemory stores a single memory and returns its assigned ID.
func (c *MemoriesClient) AddMemory(m Memory) (int, error) {
	resp, err := c.request(http.MethodPost, "/memory/add", m)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("memories API error %d: %s", resp.StatusCode, text)
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	return result.ID, nil
}

const batchSize = 500

// UpsertBatch inserts or updates memories in chunks of batchSize.
// Returns the UpsertResult for each memory processed.
func (c *MemoriesClient) UpsertBatch(memories []Memory) ([]UpsertResult, error) {
	var all []UpsertResult
	total := (len(memories) + batchSize - 1) / batchSize

	for i := 0; i < len(memories); i += batchSize {
		end := i + batchSize
		if end > len(memories) {
			end = len(memories)
		}
		batch := memories[i:end]
		batchNum := i/batchSize + 1

		log.Printf("storage: upserting batch %d/%d (%d memories)", batchNum, total, len(batch))

		payload := struct {
			Memories []Memory `json:"memories"`
		}{Memories: batch}

		resp, err := c.request(http.MethodPost, "/memory/upsert-batch", payload)
		if err != nil {
			return all, fmt.Errorf("batch %d: %w", batchNum, err)
		}

		if resp.StatusCode != http.StatusOK {
			text, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return all, fmt.Errorf("batch %d: memories API error %d: %s", batchNum, resp.StatusCode, text)
		}

		var result struct {
			Results []UpsertResult `json:"results"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			resp.Body.Close()
			return all, fmt.Errorf("batch %d: decode response: %w", batchNum, err)
		}
		resp.Body.Close()

		all = append(all, result.Results...)
	}
	return all, nil
}

// Supersede replaces an existing memory with new content, preserving lineage.
// Returns the ID of the newly created memory.
func (c *MemoriesClient) Supersede(oldID int, newText string, newMeta map[string]any) (int, error) {
	payload := struct {
		OldID    int            `json:"old_id"`
		Text     string         `json:"text"`
		Metadata map[string]any `json:"metadata,omitempty"`
	}{
		OldID:    oldID,
		Text:     newText,
		Metadata: newMeta,
	}

	resp, err := c.request(http.MethodPost, "/memory/supersede", payload)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("memories API error %d: %s", resp.StatusCode, text)
	}

	var result struct {
		NewID int `json:"new_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	return result.NewID, nil
}

// CreateLink creates a directed graph edge from one memory to another.
func (c *MemoriesClient) CreateLink(fromID, toID int, linkType string) error {
	payload := struct {
		ToID int    `json:"to_id"`
		Type string `json:"type"`
	}{
		ToID: toID,
		Type: linkType,
	}

	path := fmt.Sprintf("/memory/%d/link", fromID)
	resp, err := c.request(http.MethodPost, path, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("memories API error %d: %s", resp.StatusCode, text)
	}
	return nil
}

// GetLinks returns all outgoing graph links from the given memory.
func (c *MemoriesClient) GetLinks(id int) ([]Link, error) {
	path := fmt.Sprintf("/memory/%d/links", id)
	resp, err := c.request(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("memories API error %d: %s", resp.StatusCode, text)
	}

	var result struct {
		Links []Link `json:"links"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Links, nil
}

// DeleteLinks removes all outgoing graph links from the given memory.
// Best-effort: logs warnings on failures and continues.
func (c *MemoriesClient) DeleteLinks(id int) error {
	links, err := c.GetLinks(id)
	if err != nil {
		log.Printf("storage: warning: get links for %d: %v", id, err)
		return nil
	}

	for _, link := range links {
		path := fmt.Sprintf("/memory/%d/link/%d", id, link.ToID)
		resp, err := c.request(http.MethodDelete, path, nil)
		if err != nil {
			log.Printf("storage: warning: delete link %d->%d: %v", id, link.ToID, err)
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			log.Printf("storage: warning: delete link %d->%d: API error %d", id, link.ToID, resp.StatusCode)
		}
	}
	return nil
}

// SearchAdvanced queries the Memories index with full 6-signal support.
// Only non-zero option fields are included in the request body.
func (c *MemoriesClient) SearchAdvanced(query string, opts SearchOptions) ([]SearchResult, error) {
	k := opts.K
	if k == 0 {
		k = 10
	}

	payload := map[string]any{
		"query": query,
		"k":     k,
	}
	if opts.Threshold != 0 {
		payload["threshold"] = opts.Threshold
	}
	if opts.Hybrid {
		payload["hybrid"] = opts.Hybrid
	}
	if opts.SourcePrefix != "" {
		payload["source_prefix"] = opts.SourcePrefix
	}
	if opts.ConfidenceWeight != 0 {
		payload["confidence_weight"] = opts.ConfidenceWeight
	}
	if opts.FeedbackWeight != 0 {
		payload["feedback_weight"] = opts.FeedbackWeight
	}
	if opts.GraphWeight != 0 {
		payload["graph_weight"] = opts.GraphWeight
	}
	if opts.Since != "" {
		payload["since"] = opts.Since
	}
	if opts.Until != "" {
		payload["until"] = opts.Until
	}

	resp, err := c.request(http.MethodPost, "/search", payload)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("memories API error %d: %s", resp.StatusCode, text)
	}

	var result struct {
		Results []SearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Results, nil
}

// ListBySource fetches memories matching a source prefix with pagination.
func (c *MemoriesClient) ListBySource(source string, limit, offset int) ([]SearchResult, error) {
	if limit == 0 {
		limit = 100
	}
	path := "/memories?source=" + url.QueryEscape(source) +
		"&limit=" + strconv.Itoa(limit) +
		"&offset=" + strconv.Itoa(offset)

	resp, err := c.request(http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("memories API error %d: %s", resp.StatusCode, text)
	}

	var result struct {
		Memories []SearchResult `json:"memories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return result.Memories, nil
}

// DeleteMemory removes a memory by ID. Tolerates 404 (already deleted).
func (c *MemoriesClient) DeleteMemory(id int) error {
	path := fmt.Sprintf("/memory/%d", id)
	resp, err := c.request(http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("memories API error %d: %s", resp.StatusCode, text)
	}
	return nil
}

// DeleteBySource bulk-deletes all memories matching the given source prefix
// using the Memories delete-by-prefix endpoint. Returns the count deleted.
func (c *MemoriesClient) DeleteBySource(prefix string) (int, error) {
	payload := struct {
		SourcePrefix string `json:"source_prefix"`
	}{SourcePrefix: prefix}

	resp, err := c.request(http.MethodPost, "/memory/delete-by-prefix", payload)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("memories API error %d: %s", resp.StatusCode, text)
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	return result.Count, nil
}

// Count returns the number of memories matching a source prefix.
func (c *MemoriesClient) Count(sourcePrefix string) (int, error) {
	path := "/memories/count"
	if sourcePrefix != "" {
		path += "?source=" + url.QueryEscape(sourcePrefix)
	}

	resp, err := c.request(http.MethodGet, path, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		text, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("memories API error %d: %s", resp.StatusCode, text)
	}

	var result struct {
		Count int `json:"count"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("decode response: %w", err)
	}
	return result.Count, nil
}
