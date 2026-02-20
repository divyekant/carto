package sources

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPDFSource_Name(t *testing.T) {
	src := NewPDFSource()
	if src.Name() != "local-pdf" {
		t.Errorf("Name() = %q, want %q", src.Name(), "local-pdf")
	}
}

func TestPDFSource_Scope(t *testing.T) {
	src := NewPDFSource()
	if src.Scope() != ProjectScope {
		t.Errorf("Scope() = %d, want ProjectScope", src.Scope())
	}
}

func TestPDFSource_Configure_MissingDir(t *testing.T) {
	src := NewPDFSource()
	err := src.Configure(SourceConfig{Settings: map[string]string{}})
	if err == nil {
		t.Error("expected error when dir missing")
	}
}

func TestPDFSource_Fetch_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	src := NewPDFSource()
	src.Configure(SourceConfig{Settings: map[string]string{"dir": dir}})

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts from empty dir, got %d", len(artifacts))
	}
}

func TestPDFSource_Fetch_SkipsNonPDF(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# hello"), 0o644)
	os.WriteFile(filepath.Join(dir, "data.csv"), []byte("a,b\n1,2"), 0o644)

	src := NewPDFSource()
	src.Configure(SourceConfig{Settings: map[string]string{"dir": dir}})

	artifacts, err := src.Fetch(context.Background(), FetchRequest{Project: "test"})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(artifacts) != 0 {
		t.Errorf("expected 0 artifacts (no PDFs), got %d", len(artifacts))
	}
}

func TestPDFSource_ArtifactCategory(t *testing.T) {
	a := Artifact{
		Source:   "local-pdf",
		Category: Knowledge,
		Title:    "Test Doc",
	}
	if a.Category != Knowledge {
		t.Errorf("expected Knowledge category, got %s", a.Category)
	}
}

var _ Source = (*PDFSource)(nil)
