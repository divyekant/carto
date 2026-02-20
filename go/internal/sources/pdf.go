package sources

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
)

// PDFSource reads PDF files from a configured directory.
type PDFSource struct {
	dir string
}

// NewPDFSource creates a PDF knowledge source.
func NewPDFSource() *PDFSource {
	return &PDFSource{}
}

func (p *PDFSource) Name() string { return "local-pdf" }
func (p *PDFSource) Scope() Scope { return ProjectScope }

func (p *PDFSource) Configure(cfg SourceConfig) error {
	dir := cfg.Settings["dir"]
	if dir == "" {
		return fmt.Errorf("local-pdf: 'dir' setting is required")
	}
	p.dir = dir
	return nil
}

func (p *PDFSource) Fetch(_ context.Context, _ FetchRequest) ([]Artifact, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, fmt.Errorf("local-pdf: read dir: %w", err)
	}

	var artifacts []Artifact
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".pdf") {
			continue
		}

		absPath := filepath.Join(p.dir, entry.Name())
		text, err := extractPDFText(absPath)
		if err != nil {
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}

		title := strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name()))
		artifacts = append(artifacts, Artifact{
			Source:   "local-pdf",
			Category: Knowledge,
			ID:       entry.Name(),
			Title:    title,
			Body:     text,
			URL:      "file://" + absPath,
			Tags:     map[string]string{"format": "pdf"},
		})
	}
	return artifacts, nil
}

func extractPDFText(path string) (string, error) {
	f, reader, err := pdflib.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var sb strings.Builder
	for i := 1; i <= reader.NumPage(); i++ {
		page := reader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	return sb.String(), nil
}
