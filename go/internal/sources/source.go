// Package sources provides a unified interface for all external integrations
// that feed context into the Carto indexing pipeline. Each source produces
// Artifact values classified as Signal (file-linked), Knowledge (project-level),
// or Context (hybrid).
package sources

import (
	"context"
	"time"
)

// Category classifies what an artifact represents.
type Category string

const (
	Signal    Category = "signal"    // file-linked: commits, PRs, issues, tickets
	Knowledge Category = "knowledge" // project-level: PDFs, docs, RFCs, web pages
	Context   Category = "context"   // hybrid: Slack threads, GitHub discussions
)

// Scope controls when a source is fetched during the pipeline.
type Scope int

const (
	ProjectScope Scope = iota // fetch once per project (most sources)
	ModuleScope               // fetch per module (git commits only)
)

// Artifact is the universal output of every source.
type Artifact struct {
	Source   string            // source name: "github", "jira", "notion", etc.
	Category Category
	ID       string            // unique within source: "#42", "PROJ-123", "page-id"
	Title    string
	Body     string
	URL      string
	Files    []string          // optional: linked file paths relative to repo root
	Module   string            // optional: linked module name
	Date     time.Time
	Author   string
	Tags     map[string]string // structured metadata: "state", "priority", etc.
}

// FetchRequest provides context to a source's Fetch method.
type FetchRequest struct {
	Project    string // project name
	Module     string // set only for ModuleScope sources
	ModulePath string // filesystem path, ModuleScope only
	RepoRoot   string // root of the codebase
}

// SourceConfig holds credentials and settings for a source.
type SourceConfig struct {
	Credentials map[string]string // from Settings UI / env vars
	Settings    map[string]string // from .carto/sources.yaml or auto-detect
}

// Source is the unified interface for all external integrations.
type Source interface {
	Name() string
	Scope() Scope
	Configure(cfg SourceConfig) error
	Fetch(ctx context.Context, req FetchRequest) ([]Artifact, error)
}
