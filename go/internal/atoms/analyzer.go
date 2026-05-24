package atoms

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
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
	Language      string   `json:"language"`
	Module        string   `json:"module"`
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
	llm       LLMClient
	maxTokens int
}

const (
	maxBatchChunks      = 50
	maxBatchPromptChars = 50000
)

// NewAnalyzer creates an Analyzer that uses the given LLM client.
// Optional maxTokens overrides the default 4096 output token limit.
func NewAnalyzer(client LLMClient, maxTokens ...int) *Analyzer {
	mt := 4096
	if len(maxTokens) > 0 && maxTokens[0] > 0 {
		mt = maxTokens[0]
	}
	return &Analyzer{llm: client, maxTokens: mt}
}

// llmResponse is the expected JSON shape returned by the LLM.
type llmResponse struct {
	ClarifiedCode string   `json:"clarified_code"`
	Summary       string   `json:"summary"`
	Imports       []string `json:"imports"`
	Exports       []string `json:"exports"`
}

type batchLLMResponse struct {
	Index int `json:"index"`
	llmResponse
}

// buildPrompt constructs the prompt sent to the fast tier for a given chunk.
func buildPrompt(chunk Chunk) string {
	return fmt.Sprintf(`Analyze this %s code unit (%s: %s) from %s.

1. CLARIFY: Write a concise clarified_code excerpt or pseudocode sketch (max 300 chars). Do not copy the full code unless the unit is already tiny.
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

func buildBatchPrompt(chunks []Chunk) string {
	var b strings.Builder
	b.WriteString(`Analyze these code units.

For each input item:
1. CLARIFY: Write a concise clarified_code excerpt or pseudocode sketch (max 300 chars). Do not copy the full code unless the unit is already tiny.
2. SUMMARIZE: Write a 1-3 sentence summary of what this code does and WHY it exists.
3. IMPORTS: List external dependencies.
4. EXPORTS: List symbols this code makes available.

Respond as a JSON array. Each item must include the original "index":
{"items":[
  {"index": 0, "clarified_code": "...", "summary": "...", "imports": ["..."], "exports": ["..."]}
]}

`)
	for i, chunk := range chunks {
		fmt.Fprintf(&b, "## Item %d\n", i)
		fmt.Fprintf(&b, "Language: %s\nKind: %s\nName: %s\nPath: %s\nLines: %d-%d\n",
			chunk.Language, chunk.Kind, chunk.Name, chunk.FilePath, chunk.StartLine, chunk.EndLine)
		fmt.Fprintf(&b, "```%s\n%s\n```\n\n", chunk.Language, chunk.Code)
	}
	return b.String()
}

func chunkBatches(chunks []Chunk) [][]Chunk {
	var batches [][]Chunk
	var current []Chunk
	currentChars := 0
	for _, chunk := range chunks {
		chunkChars := len(chunk.Code) + len(chunk.Name) + len(chunk.FilePath) + 256
		if len(current) > 0 && (len(current) >= maxBatchChunks || currentChars+chunkChars > maxBatchPromptChars) {
			batches = append(batches, current)
			current = nil
			currentChars = 0
		}
		current = append(current, chunk)
		currentChars += chunkChars
	}
	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func atomFromResponse(chunk Chunk, resp llmResponse) *Atom {
	return &Atom{
		Name:          chunk.Name,
		Kind:          chunk.Kind,
		Language:      chunk.Language,
		FilePath:      chunk.FilePath,
		Summary:       resp.Summary,
		ClarifiedCode: resp.ClarifiedCode,
		Imports:       resp.Imports,
		Exports:       resp.Exports,
		StartLine:     chunk.StartLine,
		EndLine:       chunk.EndLine,
	}
}

// AnalyzeChunk sends a single code chunk to the fast tier for clarification and
// summarization, returning the resulting Atom.
func (a *Analyzer) AnalyzeChunk(chunk Chunk) (*Atom, error) {
	prompt := buildPrompt(chunk)

	raw, err := a.llm.CompleteJSON(prompt, llm.TierFast, &llm.CompleteOptions{
		System:    "You are a code analysis assistant. Respond only with valid JSON.",
		MaxTokens: a.maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("atoms: LLM call failed: %w", err)
	}

	var resp llmResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("atoms: failed to parse LLM response: %w", err)
	}

	return atomFromResponse(chunk, resp), nil
}

func (a *Analyzer) AnalyzeChunkBatch(chunks []Chunk) ([]*Atom, error) {
	if len(chunks) == 0 {
		return nil, nil
	}
	prompt := buildBatchPrompt(chunks)
	raw, err := a.llm.CompleteJSON(prompt, llm.TierFast, &llm.CompleteOptions{
		System:    "You are a code analysis assistant. Respond only with valid JSON.",
		MaxTokens: a.maxTokens,
	})
	if err != nil {
		return nil, fmt.Errorf("atoms: batch LLM call failed: %w", err)
	}

	var batchResp struct {
		Items []batchLLMResponse `json:"items"`
	}
	if err := json.Unmarshal(raw, &batchResp); err != nil {
		var items []batchLLMResponse
		if arrayErr := json.Unmarshal(raw, &items); arrayErr != nil {
			return nil, fmt.Errorf("atoms: failed to parse batch LLM response: %w", err)
		}
		batchResp.Items = items
	}
	if len(batchResp.Items) == 0 {
		return nil, fmt.Errorf("atoms: batch LLM response contained no items")
	}

	atoms := make([]*Atom, 0, len(batchResp.Items))
	seen := make(map[int]bool, len(batchResp.Items))
	for _, resp := range batchResp.Items {
		if resp.Index < 0 || resp.Index >= len(chunks) || seen[resp.Index] {
			continue
		}
		seen[resp.Index] = true
		atoms = append(atoms, atomFromResponse(chunks[resp.Index], resp.llmResponse))
	}
	return atoms, nil
}

// AnalyzeBatch processes multiple chunks in parallel using up to maxWorkers
// goroutines. The progress callback, if non-nil, is called after each chunk
// completes with (done, total) counts. Chunks that fail analysis are skipped
// with a logged warning. Results are returned in the same order as input.
func (a *Analyzer) AnalyzeBatch(chunks []Chunk, maxWorkers int, progress func(done, total int)) ([]*Atom, error) {
	return a.AnalyzeBatchCtx(context.Background(), chunks, maxWorkers, progress)
}

// AnalyzeBatchCtx is like AnalyzeBatch but accepts a context for cancellation.
func (a *Analyzer) AnalyzeBatchCtx(ctx context.Context, chunks []Chunk, maxWorkers int, progress func(done, total int)) ([]*Atom, error) {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	total := len(chunks)
	batches := chunkBatches(chunks)
	var results []*Atom

	sem := make(chan struct{}, maxWorkers)
	var mu sync.Mutex
	var done int
	var wg sync.WaitGroup

	for _, batch := range batches {
		select {
		case <-ctx.Done():
			break
		default:
		}
		if ctx.Err() != nil {
			break
		}

		wg.Add(1)

		acquired := false
		select {
		case sem <- struct{}{}:
			acquired = true
		case <-ctx.Done():
		}
		if !acquired {
			wg.Done()
			break
		}

		go func(batch []Chunk) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}

			atoms, err := a.AnalyzeChunkBatch(batch)

			mu.Lock()
			if err != nil {
				log.Printf("atoms: warning: batch analysis failed (%d chunks), retrying individually: %v", len(batch), err)
				mu.Unlock()
				for _, ch := range batch {
					if ctx.Err() != nil {
						return
					}
					atom, chunkErr := a.AnalyzeChunk(ch)
					mu.Lock()
					if chunkErr != nil {
						log.Printf("atoms: warning: skipping chunk %q (%s): %v", ch.Name, ch.FilePath, chunkErr)
					} else {
						results = append(results, atom)
					}
					done++
					if progress != nil {
						progress(done, total)
					}
					mu.Unlock()
				}
			} else {
				results = append(results, atoms...)
				for range batch {
					done++
					if progress != nil {
						progress(done, total)
					}
				}
				mu.Unlock()
			}
		}(batch)
	}

	wg.Wait()

	return results, nil
}
