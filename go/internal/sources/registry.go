package sources

import (
	"context"
	"log"
	"sync"
)

// Registry holds all configured sources and dispatches fetch calls.
type Registry struct {
	sources []Source
}

// NewRegistry creates an empty source registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a source to the registry.
func (r *Registry) Register(s Source) {
	r.sources = append(r.sources, s)
}

// SourceNames returns the names of all registered sources.
func (r *Registry) SourceNames() []string {
	names := make([]string, len(r.sources))
	for i, s := range r.sources {
		names[i] = s.Name()
	}
	return names
}

// FetchAllProject fetches artifacts from all ProjectScope sources concurrently.
// Individual source errors are logged but do not prevent other sources from running.
func (r *Registry) FetchAllProject(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	var projectSources []Source
	for _, s := range r.sources {
		if s.Scope() == ProjectScope {
			projectSources = append(projectSources, s)
		}
	}

	if len(projectSources) == 0 {
		return nil, nil
	}

	type result struct {
		artifacts []Artifact
		err       error
		name      string
	}

	results := make(chan result, len(projectSources))
	var wg sync.WaitGroup

	for _, s := range projectSources {
		wg.Add(1)
		go func(src Source) {
			defer wg.Done()
			arts, err := src.Fetch(ctx, req)
			results <- result{artifacts: arts, err: err, name: src.Name()}
		}(s)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var all []Artifact
	for res := range results {
		if res.err != nil {
			log.Printf("sources: warning: %s failed: %v", res.name, res.err)
			continue
		}
		all = append(all, res.artifacts...)
	}

	return all, nil
}

// FetchModule fetches artifacts from all ModuleScope sources.
// Only module-scoped sources (e.g. git) are invoked.
func (r *Registry) FetchModule(ctx context.Context, req FetchRequest) ([]Artifact, error) {
	var all []Artifact
	for _, s := range r.sources {
		if s.Scope() != ModuleScope {
			continue
		}
		arts, err := s.Fetch(ctx, req)
		if err != nil {
			log.Printf("sources: warning: %s failed for module %s: %v", s.Name(), req.Module, err)
			continue
		}
		all = append(all, arts...)
	}
	return all, nil
}
