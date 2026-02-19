package knowledge

import (
	"testing"
)

func TestRegistry_Empty(t *testing.T) {
	r := NewRegistry()
	docs, err := r.FetchAll("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Errorf("expected 0 docs, got %d", len(docs))
	}
}

type mockSource struct {
	docs []Document
}

func (m *mockSource) Name() string                                      { return "mock" }
func (m *mockSource) Configure(cfg map[string]string) error             { return nil }
func (m *mockSource) FetchDocuments(project string) ([]Document, error) { return m.docs, nil }

func TestRegistry_FetchAll(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockSource{docs: []Document{
		{Title: "Doc A", Content: "content a", Type: "mock"},
	}})
	r.Register(&mockSource{docs: []Document{
		{Title: "Doc B", Content: "content b", Type: "mock"},
	}})

	docs, err := r.FetchAll("test")
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 2 {
		t.Errorf("expected 2 docs, got %d", len(docs))
	}
}

func TestLocalPDFSource_Configure(t *testing.T) {
	src := NewLocalPDFSource()
	err := src.Configure(map[string]string{})
	if err == nil {
		t.Error("expected error when dir not set")
	}

	err = src.Configure(map[string]string{"dir": "/tmp"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
