package signals

import (
	"log"
	"time"
)

// Signal represents a piece of external context (ticket, PR, doc).
type Signal struct {
	Type   string    // "commit", "pr", "ticket", "doc"
	ID     string    // "abc123", "#247", "JIRA-1892"
	Title  string
	Body   string
	URL    string
	Files  []string  // linked file paths
	Date   time.Time
	Author string
}

// Module is a minimal struct representing a module for signal fetching.
type Module struct {
	Name    string
	Path    string
	RelPath string
	Files   []string
}

// SignalSource is the plugin interface.
type SignalSource interface {
	Name() string
	Configure(cfg map[string]string) error
	FetchSignals(module Module) ([]Signal, error)
}

// Registry holds all configured signal sources.
type Registry struct {
	sources []SignalSource
}

// NewRegistry creates an empty signal source registry.
func NewRegistry() *Registry {
	return &Registry{}
}

// Register adds a signal source to the registry.
func (r *Registry) Register(s SignalSource) {
	r.sources = append(r.sources, s)
}

// FetchAll collects signals from every registered source. Individual source
// errors are logged but do not prevent other sources from being queried.
func (r *Registry) FetchAll(module Module) ([]Signal, error) {
	var all []Signal
	for _, s := range r.sources {
		signals, err := s.FetchSignals(module)
		if err != nil {
			log.Printf("signals: warning: source %s failed for module %s: %v", s.Name(), module.Name, err)
			continue
		}
		all = append(all, signals...)
	}
	return all, nil
}
