package sources

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// Compile-time interface check.
var _ Source = (*WebSource)(nil)

// WebSource fetches web pages and extracts their text content.
type WebSource struct {
	urls []string
	http http.Client
}

// NewWebSource creates an unconfigured web source with sensible defaults.
func NewWebSource() *WebSource {
	return &WebSource{
		http: http.Client{Timeout: 30 * time.Second},
	}
}

func (w *WebSource) Name() string { return "web" }
func (w *WebSource) Scope() Scope { return ProjectScope }

func (w *WebSource) Configure(cfg SourceConfig) error {
	raw := cfg.Settings["urls"]
	if raw == "" {
		return fmt.Errorf("web: 'urls' setting is required (comma-separated list)")
	}

	var urls []string
	for _, u := range strings.Split(raw, ",") {
		u = strings.TrimSpace(u)
		if u != "" {
			urls = append(urls, u)
		}
	}
	if len(urls) == 0 {
		return fmt.Errorf("web: at least one URL is required")
	}
	w.urls = urls
	return nil
}

func (w *WebSource) Fetch(ctx context.Context, _ FetchRequest) ([]Artifact, error) {
	var artifacts []Artifact
	for _, u := range w.urls {
		a, err := w.fetchURL(ctx, u)
		if err != nil {
			log.Printf("web: skipping %s: %v", u, err)
			continue
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, nil
}

func (w *WebSource) fetchURL(ctx context.Context, url string) (Artifact, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return Artifact{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", "Carto/1.0 (+https://github.com/divyekant/carto)")

	resp, err := w.http.Do(req)
	if err != nil {
		return Artifact{}, fmt.Errorf("GET: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Artifact{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return Artifact{}, fmt.Errorf("read body: %w", err)
	}

	html := string(body)
	title := extractTitle(html)
	if title == "" {
		title = url
	}
	text := stripHTML(html)
	text = truncateBody(text, 5000)

	contentType := resp.Header.Get("Content-Type")

	return Artifact{
		Source:   "web",
		Category: Knowledge,
		ID:       url,
		Title:    title,
		Body:     text,
		URL:      url,
		Date:     time.Now(),
		Author:   "",
		Tags:     map[string]string{"type": "webpage", "content_type": contentType},
	}, nil
}

var reTitle = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// extractTitle pulls the content of the first <title> tag.
func extractTitle(html string) string {
	m := reTitle.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	return strings.TrimSpace(m[1])
}

var (
	reScript = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reStyle  = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reTags   = regexp.MustCompile(`<[^>]+>`)
	reSpaces = regexp.MustCompile(`\s+`)
)

// stripHTML removes script/style blocks, HTML tags, and collapses whitespace.
func stripHTML(html string) string {
	s := reScript.ReplaceAllString(html, " ")
	s = reStyle.ReplaceAllString(s, " ")
	s = reTags.ReplaceAllString(s, " ")
	s = reSpaces.ReplaceAllString(s, " ")
	return strings.TrimSpace(s)
}
