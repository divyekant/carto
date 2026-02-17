# Carto v2 (Go) Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite carto as a Go CLI with intent-aware indexing, parallel pipeline, module detection, plugin system, and deobfuscation.

**Architecture:** Go binary using goroutine worker pools for parallel Haiku/git extraction, per-module Opus analysis, FAISS storage via REST API. Tree-sitter for AST-based chunking. Plugin interface for external signal sources (Jira, Confluence, etc).

**Tech Stack:** Go 1.25, tree-sitter/go-tree-sitter, cobra (CLI), Anthropic API (HTTP), FAISS REST API

---

### Task 1: Go Project Scaffolding

**Files:**
- Create: `go/cmd/carto/main.go`
- Create: `go/go.mod`
- Create: `go/internal/config/config.go`
- Create: `go/internal/config/config_test.go`

**Step 1: Initialize Go module**

```bash
mkdir -p go/cmd/carto go/internal/config
cd go && go mod init github.com/anthropic/indexer
```

**Step 2: Install dependencies**

```bash
cd go && go get github.com/spf13/cobra@latest
```

**Step 3: Write config test**

```go
// go/internal/config/config_test.go
package config

import (
	"os"
	"testing"
)

func TestLoadConfig_Defaults(t *testing.T) {
	cfg := Load()
	if cfg.FaissURL != "http://localhost:8900" {
		t.Errorf("expected default FAISS URL, got %s", cfg.FaissURL)
	}
	if cfg.HaikuModel != "claude-haiku-4-5-20251001" {
		t.Errorf("expected default haiku model, got %s", cfg.HaikuModel)
	}
	if cfg.MaxConcurrent != 10 {
		t.Errorf("expected default concurrency 10, got %d", cfg.MaxConcurrent)
	}
}

func TestLoadConfig_EnvOverrides(t *testing.T) {
	os.Setenv("ANTHROPIC_API_KEY", "test-key")
	os.Setenv("FAISS_URL", "http://custom:9999")
	defer os.Unsetenv("ANTHROPIC_API_KEY")
	defer os.Unsetenv("FAISS_URL")

	cfg := Load()
	if cfg.AnthropicKey != "test-key" {
		t.Errorf("expected test-key, got %s", cfg.AnthropicKey)
	}
	if cfg.FaissURL != "http://custom:9999" {
		t.Errorf("expected custom URL, got %s", cfg.FaissURL)
	}
}

func TestIsOAuthToken(t *testing.T) {
	if !IsOAuthToken("sk-ant-oat01-abc123") {
		t.Error("should detect OAuth token")
	}
	if IsOAuthToken("sk-ant-api03-abc123") {
		t.Error("should not detect API key as OAuth")
	}
	if IsOAuthToken("") {
		t.Error("should not detect empty string as OAuth")
	}
}
```

**Step 4: Run test to verify it fails**

Run: `cd go && go test ./internal/config/ -v`
Expected: FAIL — functions not defined

**Step 5: Write config implementation**

```go
// go/internal/config/config.go
package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	FaissURL      string
	FaissAPIKey   string
	AnthropicKey  string
	HaikuModel    string
	OpusModel     string
	MaxConcurrent int
}

func Load() Config {
	return Config{
		FaissURL:      envOr("FAISS_URL", "http://localhost:8900"),
		FaissAPIKey:   envOr("FAISS_API_KEY", "god-is-an-astronaut"),
		AnthropicKey:  os.Getenv("ANTHROPIC_API_KEY"),
		HaikuModel:    envOr("CARTO_HAIKU_MODEL", "claude-haiku-4-5-20251001"),
		OpusModel:     envOr("CARTO_OPUS_MODEL", "claude-opus-4-6"),
		MaxConcurrent: envOrInt("CARTO_MAX_CONCURRENT", 10),
	}
}

func IsOAuthToken(key string) bool {
	return len(key) > 0 && strings.HasPrefix(key, "sk-ant-oat01-")
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envOrInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
```

**Step 6: Write CLI skeleton**

```go
// go/cmd/carto/main.go
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var version = "0.2.0"

func main() {
	root := &cobra.Command{
		Use:     "carto",
		Short:   "Carto — intent-aware codebase intelligence",
		Version: version,
	}

	root.AddCommand(indexCmd())
	root.AddCommand(queryCmd())
	root.AddCommand(modulesCmd())
	root.AddCommand(patternsCmd())
	root.AddCommand(statusCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func indexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "index <path>",
		Short: "Index a codebase",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("index: not yet implemented")
			return nil
		},
	}
	cmd.Flags().Bool("full", false, "Force full re-index")
	cmd.Flags().String("module", "", "Index a single module")
	cmd.Flags().Bool("incremental", false, "Only re-index changed files")
	return cmd
}

func queryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "query <question>",
		Short: "Query the indexed codebase",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("query: not yet implemented")
			return nil
		},
	}
}

func modulesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "modules <path>",
		Short: "List detected modules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("modules: not yet implemented")
			return nil
		},
	}
}

func patternsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "patterns <path>",
		Short: "Generate CLAUDE.md and .cursorrules",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("patterns: not yet implemented")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <path>",
		Short: "Show index status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("status: not yet implemented")
			return nil
		},
	}
}
```

**Step 7: Run tests and build**

Run: `cd go && go test ./... -v && go build -o carto ./cmd/carto && ./carto --version`
Expected: Tests PASS, binary outputs `carto version 0.2.0`

**Step 8: Commit**

```bash
git add go/
git commit -m "feat(go): project scaffolding — CLI skeleton, config, OAuth detection"
```

---

### Task 2: FAISS Client

**Files:**
- Create: `go/internal/storage/faiss.go`
- Create: `go/internal/storage/faiss_test.go`

**Step 1: Write FAISS client test**

```go
// go/internal/storage/faiss_test.go
package storage

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFaissClient_Health(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()

	client := NewFaissClient(srv.URL, "test-key")
	ok, err := client.Health()
	if err != nil || !ok {
		t.Errorf("expected healthy, got ok=%v err=%v", ok, err)
	}
}

func TestFaissClient_AddMemory(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/memory/add" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("X-API-Key") != "test-key" {
			t.Error("missing API key header")
		}
		json.NewEncoder(w).Encode(map[string]int{"id": 42})
	}))
	defer srv.Close()

	client := NewFaissClient(srv.URL, "test-key")
	id, err := client.AddMemory(Memory{Text: "test", Source: "test/source"})
	if err != nil {
		t.Fatal(err)
	}
	if id != 42 {
		t.Errorf("expected id 42, got %d", id)
	}
}

func TestFaissClient_Search(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"results": []map[string]any{
				{"id": 1, "text": "hello", "score": 0.95, "source": "test"},
			},
		})
	}))
	defer srv.Close()

	client := NewFaissClient(srv.URL, "test-key")
	results, err := client.Search("hello", SearchOptions{K: 5})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
	if results[0].Score != 0.95 {
		t.Errorf("expected score 0.95, got %f", results[0].Score)
	}
}

func TestFaissClient_DeleteBySource(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(map[string]any{
				"memories": []map[string]any{
					{"id": 1, "text": "a", "score": 1.0, "source": "test"},
					{"id": 2, "text": "b", "score": 1.0, "source": "test"},
				},
			})
		} else if r.Method == "DELETE" {
			calls++
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	client := NewFaissClient(srv.URL, "test-key")
	n, err := client.DeleteBySource("test")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("expected 2 deletions, got %d", n)
	}
	if calls != 2 {
		t.Errorf("expected 2 DELETE calls, got %d", calls)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/storage/ -v`
Expected: FAIL

**Step 3: Write FAISS client implementation**

```go
// go/internal/storage/faiss.go
package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

type Memory struct {
	Text        string         `json:"text"`
	Source      string         `json:"source"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	Deduplicate bool           `json:"deduplicate,omitempty"`
}

type SearchResult struct {
	ID     int            `json:"id"`
	Text   string         `json:"text"`
	Score  float64        `json:"score"`
	Source string         `json:"source"`
	Meta   map[string]any `json:"metadata,omitempty"`
}

type SearchOptions struct {
	K         int
	Threshold float64
	Hybrid    bool
	Source    string
}

type FaissClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewFaissClient(baseURL, apiKey string) *FaissClient {
	return &FaissClient{baseURL: baseURL, apiKey: apiKey, http: &http.Client{}}
}

func (c *FaissClient) request(method, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, c.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		// Tolerate 404 on deletes
		if resp.StatusCode == 404 {
			return data, nil
		}
		return nil, fmt.Errorf("FAISS API error %d: %s", resp.StatusCode, string(data))
	}
	return data, nil
}

func (c *FaissClient) Health() (bool, error) {
	_, err := c.request("GET", "/health", nil)
	return err == nil, err
}

func (c *FaissClient) AddMemory(m Memory) (int, error) {
	data, err := c.request("POST", "/memory/add", m)
	if err != nil {
		return 0, err
	}
	var resp struct{ ID int `json:"id"` }
	if err := json.Unmarshal(data, &resp); err != nil {
		return 0, err
	}
	return resp.ID, nil
}

func (c *FaissClient) AddBatch(memories []Memory) error {
	const batchSize = 500
	for i := 0; i < len(memories); i += batchSize {
		end := i + batchSize
		if end > len(memories) {
			end = len(memories)
		}
		payload := map[string]any{
			"memories":    memories[i:end],
			"deduplicate": false,
		}
		if _, err := c.request("POST", "/memory/add-batch", payload); err != nil {
			return err
		}
	}
	return nil
}

func (c *FaissClient) Search(query string, opts SearchOptions) ([]SearchResult, error) {
	k := opts.K
	if k == 0 {
		k = 10
	}
	payload := map[string]any{
		"query":  query,
		"k":      k,
		"hybrid": true,
	}
	if opts.Threshold > 0 {
		payload["threshold"] = opts.Threshold
	}
	data, err := c.request("POST", "/search", payload)
	if err != nil {
		return nil, err
	}
	var resp struct{ Results []SearchResult `json:"results"` }
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Results, nil
}

func (c *FaissClient) ListBySource(source string, limit int) ([]SearchResult, error) {
	if limit == 0 {
		limit = 50
	}
	path := "/memories?source=" + url.QueryEscape(source) + "&limit=" + strconv.Itoa(limit)
	data, err := c.request("GET", path, nil)
	if err != nil {
		return nil, err
	}
	var resp struct{ Memories []SearchResult `json:"memories"` }
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Memories, nil
}

func (c *FaissClient) DeleteMemory(id int) error {
	_, err := c.request("DELETE", fmt.Sprintf("/memory/%d", id), nil)
	return err
}

func (c *FaissClient) DeleteBySource(prefix string) (int, error) {
	memories, err := c.ListBySource(prefix, 1000)
	if err != nil {
		return 0, err
	}
	for _, m := range memories {
		if err := c.DeleteMemory(m.ID); err != nil {
			return 0, err
		}
	}
	return len(memories), nil
}
```

**Step 4: Run tests**

Run: `cd go && go test ./internal/storage/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add go/internal/storage/
git commit -m "feat(go): FAISS REST client with batch, search, delete"
```

---

### Task 3: LLM Client (Anthropic + OAuth)

**Files:**
- Create: `go/internal/llm/client.go`
- Create: `go/internal/llm/client_test.go`

**Step 1: Write LLM client test**

```go
// go/internal/llm/client_test.go
package llm

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_Complete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "Hello from LLM"},
			},
			"stop_reason": "end_turn",
		})
	}))
	defer srv.Close()

	client := NewClient(Options{
		APIKey:  "test-key",
		BaseURL: srv.URL,
	})

	result, err := client.Complete("test prompt", TierHaiku, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result != "Hello from LLM" {
		t.Errorf("expected 'Hello from LLM', got '%s'", result)
	}
}

func TestClient_Semaphore(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "ok"},
			},
			"stop_reason": "end_turn",
		})
	}))
	defer srv.Close()

	client := NewClient(Options{
		APIKey:      "test-key",
		BaseURL:     srv.URL,
		MaxConcurrent: 2,
	})

	// Fire 5 requests concurrently
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			_, err := client.Complete("test", TierHaiku, nil)
			done <- err
		}()
	}
	for i := 0; i < 5; i++ {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
	if calls != 5 {
		t.Errorf("expected 5 calls, got %d", calls)
	}
}

func TestClient_OAuthHeaders(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth != "Bearer sk-ant-oat01-test" {
			t.Errorf("expected Bearer token, got %s", auth)
		}
		if r.Header.Get("X-Api-Key") != "" {
			t.Error("should not have x-api-key with OAuth")
		}
		beta := r.Header.Get("Anthropic-Beta")
		if beta == "" {
			t.Error("missing anthropic-beta header")
		}
		json.NewEncoder(w).Encode(map[string]any{
			"content": []map[string]any{
				{"type": "text", "text": "ok"},
			},
			"stop_reason": "end_turn",
		})
	}))
	defer srv.Close()

	client := NewClient(Options{
		APIKey:  "sk-ant-oat01-test",
		BaseURL: srv.URL,
		IsOAuth: true,
	})
	_, err := client.Complete("test", TierHaiku, nil)
	if err != nil {
		t.Fatal(err)
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd go && go test ./internal/llm/ -v`
Expected: FAIL

**Step 3: Write LLM client** — HTTP-based Anthropic client with semaphore, OAuth support, streaming for large requests.

```go
// go/internal/llm/client.go
package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Tier string

const (
	TierHaiku Tier = "haiku"
	TierOpus  Tier = "opus"
)

type Options struct {
	APIKey        string
	BaseURL       string
	HaikuModel    string
	OpusModel     string
	MaxConcurrent int
	IsOAuth       bool
}

type CompleteOptions struct {
	System    string
	MaxTokens int
}

type Client struct {
	opts Options
	sem  chan struct{}
	http *http.Client
}

func NewClient(opts Options) *Client {
	if opts.BaseURL == "" {
		opts.BaseURL = "https://api.anthropic.com"
	}
	if opts.HaikuModel == "" {
		opts.HaikuModel = "claude-haiku-4-5-20251001"
	}
	if opts.OpusModel == "" {
		opts.OpusModel = "claude-opus-4-6"
	}
	if opts.MaxConcurrent <= 0 {
		opts.MaxConcurrent = 10
	}
	return &Client{
		opts: opts,
		sem:  make(chan struct{}, opts.MaxConcurrent),
		http: &http.Client{},
	}
}

func (c *Client) Complete(prompt string, tier Tier, opts *CompleteOptions) (string, error) {
	c.sem <- struct{}{}
	defer func() { <-c.sem }()

	model := c.opts.HaikuModel
	if tier == TierOpus {
		model = c.opts.OpusModel
	}

	maxTokens := 4096
	var system string
	if opts != nil {
		if opts.MaxTokens > 0 {
			maxTokens = opts.MaxTokens
		}
		system = opts.System
	}

	payload := map[string]any{
		"model":      model,
		"max_tokens": maxTokens,
		"messages":   []map[string]any{{"role": "user", "content": prompt}},
	}
	if system != "" {
		payload["system"] = system
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", c.opts.BaseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Anthropic-Version", "2023-06-01")

	if c.opts.IsOAuth {
		req.Header.Set("Authorization", "Bearer "+c.opts.APIKey)
		req.Header.Set("Anthropic-Beta", "oauth-2025-04-20,interleaved-thinking-2025-05-14")
	} else {
		req.Header.Set("X-Api-Key", c.opts.APIKey)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("LLM API error %d: %s", resp.StatusCode, string(data))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}

	for _, block := range result.Content {
		if block.Type == "text" {
			return block.Text, nil
		}
	}
	return "", nil
}

func (c *Client) CompleteJSON(prompt string, tier Tier, opts *CompleteOptions) (json.RawMessage, error) {
	text, err := c.Complete(prompt, tier, opts)
	if err != nil {
		return nil, err
	}
	text = strings.TrimSpace(text)

	// Strip markdown fences
	if i := strings.Index(text, "```"); i >= 0 {
		if j := strings.LastIndex(text, "```"); j > i {
			inner := text[i+3 : j]
			if nl := strings.IndexByte(inner, '\n'); nl >= 0 {
				inner = inner[nl+1:]
			}
			text = strings.TrimSpace(inner)
		}
	}

	// Find JSON object
	if !strings.HasPrefix(text, "{") {
		if i := strings.Index(text, "{"); i >= 0 {
			if j := strings.LastIndex(text, "}"); j > i {
				text = text[i : j+1]
			}
		}
	}

	return json.RawMessage(text), nil
}
```

**Step 4: Run tests**

Run: `cd go && go test ./internal/llm/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add go/internal/llm/
git commit -m "feat(go): LLM client with OAuth, semaphore concurrency, JSON extraction"
```

---

### Task 4: Scanner + Module Detection (Layer 0)

**Files:**
- Create: `go/internal/scanner/scanner.go`
- Create: `go/internal/scanner/scanner_test.go`
- Create: `go/internal/scanner/languages.go`
- Create: `go/internal/scanner/modules.go`

**Step 1: Write scanner test**

Test covers: file walking, gitignore respect, language detection, module boundary detection (package.json, pom.xml, go.mod).

**Step 2: Run test to verify fails**

Run: `cd go && go test ./internal/scanner/ -v`

**Step 3: Implement scanner** — walks file tree, respects .gitignore, detects languages by extension, detects module boundaries by manifest files (package.json, pom.xml, go.mod, Cargo.toml, build.gradle, pyproject.toml), infers directory roles, finds entry points.

**Step 4: Run tests, commit**

```bash
git commit -m "feat(go): scanner with module detection, language inference, gitignore"
```

---

### Task 5: Chunker (Tree-sitter based)

**Files:**
- Create: `go/internal/chunker/chunker.go`
- Create: `go/internal/chunker/chunker_test.go`

**Step 1: Install tree-sitter**

```bash
cd go && go get github.com/tree-sitter/go-tree-sitter@latest
go get github.com/tree-sitter/tree-sitter-go@latest
go get github.com/tree-sitter/tree-sitter-javascript@latest
go get github.com/tree-sitter/tree-sitter-python@latest
go get github.com/tree-sitter/tree-sitter-typescript@latest
go get github.com/tree-sitter/tree-sitter-java@latest
go get github.com/tree-sitter/tree-sitter-rust@latest
```

**Step 2: Write chunker test** — splits Go/JS/Python/Java code into function/class/type chunks using AST.

**Step 3: Implement chunker** — uses tree-sitter to parse code into AST, walks tree to find declaration nodes (function_declaration, class_declaration, method_definition, etc.), extracts each as a chunk with name, kind, line range, raw code. Falls back to heuristic regex for unsupported languages.

**Step 4: Run tests, commit**

```bash
git commit -m "feat(go): tree-sitter chunker with AST-based code splitting"
```

---

### Task 6: History Extractor (Layer 1b)

**Files:**
- Create: `go/internal/history/extractor.go`
- Create: `go/internal/history/extractor_test.go`

**Step 1: Write history test** — parses git log output, extracts blame data, handles missing git repos.

**Step 2: Implement history extractor** — runs `git log --follow --pretty=format:...` per file, `git log --oneline` for recent commits, parses PR references from commit messages. For large repos: samples last 6 months + high-churn files only.

**Step 3: Run tests, commit**

```bash
git commit -m "feat(go): git history extractor with blame, commit parsing, smart sampling"
```

---

### Task 7: Signals Plugin Interface (Layer 1c)

**Files:**
- Create: `go/internal/signals/source.go`
- Create: `go/internal/signals/git.go`
- Create: `go/internal/signals/git_test.go`

**Step 1: Define SignalSource interface** — `Name()`, `Configure()`, `FetchSignals()`.

**Step 2: Implement built-in git signal source** — extracts PR descriptions via GitHub API (if GITHUB_TOKEN set), otherwise falls back to commit messages.

**Step 3: Run tests, commit**

```bash
git commit -m "feat(go): signal plugin interface + built-in git source"
```

---

### Task 8: Atoms Analyzer (Layer 1a — Haiku)

**Files:**
- Create: `go/internal/atoms/analyzer.go`
- Create: `go/internal/atoms/analyzer_test.go`

**Step 1: Write atoms test** — mock LLM, verify clarification prompt includes deobfuscation instructions, verify output structure.

**Step 2: Implement atoms analyzer** — sends each code chunk to Haiku with prompt: "Clarify this code (rename cryptic variables to meaningful names) and summarize in 1-3 sentences." Returns Atom with clarifiedCode, summary, imports, exports.

**Step 3: Run tests, commit**

```bash
git commit -m "feat(go): atoms analyzer with deobfuscation + Haiku summarization"
```

---

### Task 9: Deep Analyzer (Layers 2-4 — Opus)

**Files:**
- Create: `go/internal/analyzer/deep.go`
- Create: `go/internal/analyzer/deep_test.go`

**Step 1: Write deep analyzer test** — mock LLM, verify per-module analysis, verify system synthesis, verify JSON parsing + repair.

**Step 2: Implement deep analyzer** — two methods:
- `AnalyzeModule(atoms, history, signals)` → wiring + zones + module intent (Opus per module)
- `SynthesizeSystem(moduleSummaries)` → blueprint + patterns (Opus system-wide)
Includes JSON repair logic from TS prototype.

**Step 3: Run tests, commit**

```bash
git commit -m "feat(go): deep analyzer with per-module Opus + system synthesis"
```

---

### Task 10: Storage Layer

**Files:**
- Create: `go/internal/storage/store.go`
- Create: `go/internal/storage/store_test.go`

**Step 1: Write storage test** — verify layer serialization, source tagging, truncation for >50k entries.

**Step 2: Implement storage** — serializes all layers into FAISS with source tags: `carto/{project}/{module}/layer:N`. Handles 50k char limit. Supports tiered retrieval (mini/standard/full).

**Step 3: Run tests, commit**

```bash
git commit -m "feat(go): storage layer with tiered retrieval + FAISS serialization"
```

---

### Task 11: Manifest

**Files:**
- Create: `go/internal/manifest/manifest.go`
- Create: `go/internal/manifest/manifest_test.go`

**Step 1: Write manifest test** — file hash tracking, change detection, JSON persistence.

**Step 2: Implement manifest** — tracks file hashes in `.carto/manifest.json`, detects added/modified/removed files for incremental indexing.

**Step 3: Run tests, commit**

```bash
git commit -m "feat(go): manifest for incremental indexing"
```

---

### Task 12: Pipeline Orchestrator

**Files:**
- Create: `go/internal/pipeline/pipeline.go`
- Create: `go/internal/pipeline/pipeline_test.go`

**Step 1: Write pipeline test** — mock all components, verify phase ordering, verify goroutine parallelism.

**Step 2: Implement pipeline** — orchestrates all 5 phases with goroutine worker pools. Logs progress via callback. Handles errors gracefully (skip bad files, retry LLM calls).

**Step 3: Run tests, commit**

```bash
git commit -m "feat(go): pipeline orchestrator with parallel goroutine phases"
```

---

### Task 13: Wire CLI Commands

**Files:**
- Modify: `go/cmd/carto/main.go`

**Step 1: Wire `index` command** — creates all components, runs pipeline, displays progress with colors.

**Step 2: Wire `query` command** — searches FAISS with tier flag (mini/standard/full).

**Step 3: Wire `modules` command** — runs scanner, displays detected modules.

**Step 4: Wire `patterns` command** — fetches patterns from FAISS, generates CLAUDE.md + .cursorrules.

**Step 5: Wire `status` command** — reads manifest, displays index stats.

**Step 6: Build and test all commands**

Run: `cd go && go build -o carto ./cmd/carto && ./carto index --help && ./carto query --help`

**Step 7: Commit**

```bash
git commit -m "feat(go): wire all CLI commands to pipeline"
```

---

### Task 14: Skill Generator (CLAUDE.md / .cursorrules)

**Files:**
- Create: `go/internal/patterns/generator.go`
- Create: `go/internal/patterns/generator_test.go`

**Step 1: Write generator test** — verify CLAUDE.md output format, verify .cursorrules format.

**Step 2: Implement generator** — produces CLAUDE.md and .cursorrules from discovered patterns, zones, and blueprint.

**Step 3: Run tests, commit**

```bash
git commit -m "feat(go): CLAUDE.md and .cursorrules generator from indexed patterns"
```

---

### Task 15: Integration Test

**Files:**
- Create: `go/internal/pipeline/integration_test.go`

**Step 1: Write integration test** — creates temp directory with sample Go/JS files, runs full pipeline with mocked LLM + FAISS, verifies all layers produced.

**Step 2: Run all tests**

Run: `cd go && go test ./... -v -count=1`
Expected: ALL PASS

**Step 3: Build final binary**

Run: `cd go && go build -o carto ./cmd/carto && ./carto --version`
Expected: `carto version 0.2.0`

**Step 4: Commit**

```bash
git commit -m "test(go): integration test for full pipeline"
```

---

## Execution Notes

- Tasks 1-3 are sequential (foundations)
- Tasks 4-8 can run in parallel (independent packages)
- Tasks 9-11 can run in parallel (depend on types from 4-8 but not each other)
- Task 12 depends on 4-11 (wires everything)
- Task 13 depends on 12 (CLI wiring)
- Tasks 14-15 can run in parallel after 12
