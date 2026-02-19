package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Memory represents a document to store in the Memories index.
type Memory struct {
	Text        string         `json:"text"`
	Source      string         `json:"source"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Deduplicate bool           `json:"deduplicate"`
}

// SearchResult represents a single result returned from Memories.
type SearchResult struct {
	ID     int            `json:"id"`
	Text   string         `json:"text"`
	Score  float64        `json:"score"`
	Source string         `json:"source"`
	Meta   map[string]any `json:"metadata,omitempty"`
}

// SearchOptions controls search behaviour.
type SearchOptions struct {
	K         int     `json:"k,omitempty"`
	Threshold float64 `json:"threshold,omitempty"`
	Hybrid    bool    `json:"hybrid,omitempty"`
	Source    string  `json:"source,omitempty"`
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
			Timeout: 30 * time.Second,
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

// AddBatch stores memories in chunks of 500.
func (c *MemoriesClient) AddBatch(memories []Memory) error {
	for i := 0; i < len(memories); i += batchSize {
		end := i + batchSize
		if end > len(memories) {
			end = len(memories)
		}
		batch := memories[i:end]

		payload := struct {
			Memories []Memory `json:"memories"`
		}{Memories: batch}

		resp, err := c.request(http.MethodPost, "/memory/add-batch", payload)
		if err != nil {
			return fmt.Errorf("batch %d: %w", i/batchSize, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			text, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("batch %d: memories API error %d: %s", i/batchSize, resp.StatusCode, text)
		}
	}
	return nil
}

// Search queries the Memories index with the given options.
func (c *MemoriesClient) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	k := opts.K
	if k == 0 {
		k = 10
	}

	payload := struct {
		Query     string  `json:"query"`
		K         int     `json:"k"`
		Threshold float64 `json:"threshold,omitempty"`
		Hybrid    bool    `json:"hybrid"`
		Source    string  `json:"source,omitempty"`
	}{
		Query:     query,
		K:         k,
		Threshold: opts.Threshold,
		Hybrid:    opts.Hybrid,
		Source:    opts.Source,
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

// ListBySource fetches memories matching a source prefix.
func (c *MemoriesClient) ListBySource(source string, limit int) ([]SearchResult, error) {
	if limit == 0 {
		limit = 50
	}
	path := "/memories?source=" + url.QueryEscape(source) + "&limit=" + strconv.Itoa(limit)

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

// DeleteBySource lists memories matching the prefix and deletes each one.
// Returns the number of memories deleted.
func (c *MemoriesClient) DeleteBySource(prefix string) (int, error) {
	memories, err := c.ListBySource(prefix, 0)
	if err != nil {
		return 0, fmt.Errorf("list by source: %w", err)
	}
	for _, m := range memories {
		if err := c.DeleteMemory(m.ID); err != nil {
			return 0, fmt.Errorf("delete memory %d: %w", m.ID, err)
		}
	}
	return len(memories), nil
}
