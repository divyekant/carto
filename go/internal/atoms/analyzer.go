package atoms

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/divyekant/carto/internal/llm"
)

// Chunk represents a code unit to analyze (passed in from chunker).
type Chunk struct {
	Name      string
	Kind      string
	Language  string
	FilePath  string
	StartLine int
	EndLine   int
	Code      string
}

// Atom is the output of fast-tier analysis -- a clarified, summarized code unit.
type Atom struct {
	Name          string   `json:"name"`
	Kind          string   `json:"kind"`
	FilePath      string   `json:"file_path"`
	Summary       string   `json:"summary"`
	ClarifiedCode string   `json:"clarified_code"`
	Imports       []string `json:"imports"`
	Exports       []string `json:"exports"`
	StartLine     int      `json:"start_line"`
	EndLine       int      `json:"end_line"`
}

// LLMClient is the interface the analyzer needs from the LLM package.
type LLMClient interface {
	CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error)
}

// Analyzer processes code chunks through the fast tier.
type Analyzer struct {
	llm LLMClient
}

// NewAnalyzer creates an Analyzer that uses the given LLM client.
func NewAnalyzer(client LLMClient) *Analyzer {
	return &Analyzer{llm: client}
}

// llmResponse is the expected JSON shape returned by the LLM.
type llmResponse struct {
	ClarifiedCode string   `json:"clarified_code"`
	Summary       string   `json:"summary"`
	Imports       []string `json:"imports"`
	Exports       []string `json:"exports"`
}

// buildPrompt constructs the prompt sent to the fast tier for a given chunk.
func buildPrompt(chunk Chunk) string {
	return fmt.Sprintf(`Analyze this %s code unit (%s: %s) from %s.

1. CLARIFY: Rename any cryptic/single-letter variables to meaningful names. Add brief inline comments for complex logic. Keep the code structure identical.
2. SUMMARIZE: Write a 1-3 sentence summary of what this code does and WHY it exists.
3. IMPORTS: List any external dependencies this code uses.
4. EXPORTS: List any symbols this code makes available to other modules.

Respond as JSON:
{"clarified_code": "...", "summary": "...", "imports": ["..."], "exports": ["..."]}

Code:
`+"`"+"`"+"`"+`%s
%s
`+"`"+"`"+"`",
		chunk.Language, chunk.Kind, chunk.Name, chunk.FilePath,
		chunk.Language, chunk.Code)
}

// AnalyzeChunk sends a single code chunk to the fast tier for clarification and
// summarization, returning the resulting Atom.
func (a *Analyzer) AnalyzeChunk(chunk Chunk) (*Atom, error) {
	prompt := buildPrompt(chunk)

	raw, err := a.llm.CompleteJSON(prompt, llm.TierFast, &llm.CompleteOptions{
		System:    "You are a code analysis assistant. Respond only with valid JSON.",
		MaxTokens: 4096,
	})
	if err != nil {
		return nil, fmt.Errorf("atoms: LLM call failed: %w", err)
	}

	var resp llmResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("atoms: failed to parse LLM response: %w", err)
	}

	atom := &Atom{
		Name:          chunk.Name,
		Kind:          chunk.Kind,
		FilePath:      chunk.FilePath,
		Summary:       resp.Summary,
		ClarifiedCode: resp.ClarifiedCode,
		Imports:       resp.Imports,
		Exports:       resp.Exports,
		StartLine:     chunk.StartLine,
		EndLine:       chunk.EndLine,
	}

	return atom, nil
}

// AnalyzeBatch processes multiple chunks in parallel using up to maxWorkers
// goroutines. The progress callback, if non-nil, is called after each chunk
// completes with (done, total) counts. Chunks that fail analysis are skipped
// with a logged warning. Results are returned in the same order as input.
func (a *Analyzer) AnalyzeBatch(chunks []Chunk, maxWorkers int, progress func(done, total int)) ([]*Atom, error) {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	total := len(chunks)
	results := make([]*Atom, total)

	sem := make(chan struct{}, maxWorkers)
	var mu sync.Mutex
	var done int
	var wg sync.WaitGroup

	for i, chunk := range chunks {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore slot

		go func(idx int, ch Chunk) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore slot

			atom, err := a.AnalyzeChunk(ch)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				log.Printf("atoms: warning: skipping chunk %q (%s): %v", ch.Name, ch.FilePath, err)
			} else {
				results[idx] = atom
			}

			done++
			if progress != nil {
				progress(done, total)
			}
		}(i, chunk)
	}

	wg.Wait()

	// Compact results: remove nil entries from skipped chunks.
	compact := make([]*Atom, 0, total)
	for _, atom := range results {
		if atom != nil {
			compact = append(compact, atom)
		}
	}

	return compact, nil
}
