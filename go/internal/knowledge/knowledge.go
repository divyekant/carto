package knowledge

import "log"

// Document represents a standalone knowledge document not tied to a specific module.
type Document struct {
	Title   string `json:"title"`
	Content string `json:"content"`
	URL     string `json:"url"`
	Type    string `json:"type"` // "pdf", "gdoc", etc.
}

// KnowledgeSource is the plugin interface for project-level documents.
type KnowledgeSource interface {
	Name() string
	Configure(cfg map[string]string) error
	FetchDocuments(project string) ([]Document, error)
}

// Registry holds all configured knowledge sources.
type Registry struct {
	sources []KnowledgeSource
}

// NewRegistry creates an empty knowledge source registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a knowledge source.
func (r *Registry) Register(s KnowledgeSource) {
	r.sources = append(r.sources, s)
}

// FetchAll collects documents from every registered source. Individual source
// errors are logged but do not prevent other sources from being queried.
func (r *Registry) FetchAll(project string) ([]Document, error) {
	var all []Document
	for _, s := range r.sources {
		docs, err := s.FetchDocuments(project)
		if err != nil {
			log.Printf("knowledge: warning: source %s failed: %v", s.Name(), err)
			continue
		}
		all = append(all, docs...)
	}
	return all, nil
}
