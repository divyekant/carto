// Package carto provides a thin Go SDK for programmatic access to Carto
// indexing and querying. It wraps the internal packages with a stable API.
package carto

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/pipeline"
	"github.com/divyekant/carto/internal/sources"
	"github.com/divyekant/carto/internal/storage"
)

// IndexOptions configures an indexing run.
type IndexOptions struct {
	Incremental bool
	Module      string
	Project     string // defaults to directory name
}

// IndexResult contains the output of an indexing run.
type IndexResult struct {
	Modules int
	Files   int
	Atoms   int
	Errors  int
}

// Index runs the Carto indexing pipeline on the given path.
func Index(path string, opts IndexOptions) (*IndexResult, error) {
	cfg := config.Load()
	apiKey := cfg.LLMApiKey
	if apiKey == "" {
		apiKey = cfg.AnthropicKey
	}
	if apiKey == "" && cfg.LLMProvider != "ollama" {
		return nil, fmt.Errorf("no API key set")
	}

	llmClient := llm.NewClient(llm.Options{
		APIKey:        apiKey,
		FastModel:     cfg.FastModel,
		DeepModel:     cfg.DeepModel,
		MaxConcurrent: cfg.MaxConcurrent,
		BaseURL:       cfg.LLMBaseURL,
	})

	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	registry := sources.NewRegistry()
	registry.Register(sources.NewGitSource(path))

	projectName := opts.Project
	if projectName == "" {
		projectName = filepath.Base(path)
	}

	result, err := pipeline.Run(pipeline.Config{
		ProjectName:    projectName,
		RootPath:       path,
		LLMClient:      llmClient,
		MemoriesClient: memoriesClient,
		SourceRegistry: registry,
		MaxWorkers:     cfg.MaxConcurrent,
		Incremental:    opts.Incremental,
		ModuleFilter:   opts.Module,
	})
	if err != nil {
		return nil, err
	}

	return &IndexResult{
		Modules: result.Modules,
		Files:   result.FilesIndexed,
		Atoms:   result.AtomsCreated,
		Errors:  len(result.Errors),
	}, nil
}

// QueryOptions configures a query.
type QueryOptions struct {
	Project string
	Tier    string // mini, standard, full
	K       int
}

// QueryResult is a single search result.
type QueryResult struct {
	Text   string
	Source string
	Score  float64
}

// Query searches the Carto index.
func Query(text string, opts QueryOptions) ([]QueryResult, error) {
	cfg := config.Load()
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	if opts.K == 0 {
		opts.K = 10
	}

	searchOpts := storage.SearchOptions{K: opts.K, Hybrid: true}
	if opts.Project != "" {
		searchOpts.Source = fmt.Sprintf("carto/%s/", opts.Project)
		searchOpts.K = opts.K * 3
	}

	results, err := memoriesClient.Search(text, searchOpts)
	if err != nil {
		return nil, err
	}

	var out []QueryResult
	for _, r := range results {
		out = append(out, QueryResult{
			Text:   r.Text,
			Source: r.Source,
			Score:  r.Score,
		})
		if len(out) >= opts.K {
			break
		}
	}
	return out, nil
}

// Sources returns the configured sources for a project.
func Sources(projectName string) (map[string]map[string]string, error) {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return nil, fmt.Errorf("PROJECTS_DIR not set")
	}
	root := filepath.Join(projectsDir, projectName)
	yamlCfg, err := sources.LoadSourcesConfig(root)
	if err != nil {
		return nil, err
	}
	result := map[string]map[string]string{}
	if yamlCfg != nil {
		for k, v := range yamlCfg.Sources {
			result[k] = v.Settings
		}
	}
	return result, nil
}
