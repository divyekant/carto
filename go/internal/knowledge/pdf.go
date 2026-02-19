package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	pdflib "github.com/ledongthuc/pdf"
)

// LocalPDFSource reads PDF files from a configured directory.
type LocalPDFSource struct {
	dir string
}

// NewLocalPDFSource creates a PDF knowledge source.
func NewLocalPDFSource() *LocalPDFSource {
	return &LocalPDFSource{}
}

func (p *LocalPDFSource) Name() string { return "local-pdf" }

func (p *LocalPDFSource) Configure(cfg map[string]string) error {
	dir, ok := cfg["dir"]
	if !ok || dir == "" {
		return fmt.Errorf("local-pdf: 'dir' is required")
	}
	p.dir = dir
	return nil
}

func (p *LocalPDFSource) FetchDocuments(project string) ([]Document, error) {
	entries, err := os.ReadDir(p.dir)
	if err != nil {
		return nil, fmt.Errorf("local-pdf: read dir: %w", err)
	}

	var docs []Document
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

		docs = append(docs, Document{
			Title:   strings.TrimSuffix(entry.Name(), filepath.Ext(entry.Name())),
			Content: text,
			URL:     "file://" + absPath,
			Type:    "pdf",
		})
	}
	return docs, nil
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
