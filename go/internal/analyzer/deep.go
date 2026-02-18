package analyzer

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/divyekant/carto/internal/atoms"
	"github.com/divyekant/carto/internal/history"
	"github.com/divyekant/carto/internal/llm"
	"github.com/divyekant/carto/internal/signals"
)

// LLMClient is the interface needed for deep-tier calls.
type LLMClient interface {
	CompleteJSON(prompt string, tier llm.Tier, opts *llm.CompleteOptions) (json.RawMessage, error)
}

// ModuleInput is the data needed to analyze one module.
type ModuleInput struct {
	Name    string
	Path    string
	Atoms   []*atoms.Atom
	History []*history.FileHistory
	Signals []signals.Signal
}

// Dependency represents a cross-unit connection with intent.
type Dependency struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Reason string `json:"reason"`
}

// Zone represents a business domain grouping.
type Zone struct {
	Name   string   `json:"name"`
	Intent string   `json:"intent"`
	Files  []string `json:"files"`
}

// ModuleAnalysis is the output of per-module deep-tier analysis.
type ModuleAnalysis struct {
	ModuleName   string       `json:"module_name"`
	Wiring       []Dependency `json:"wiring"`
	Zones        []Zone       `json:"zones"`
	ModuleIntent string       `json:"module_intent"`
}

// SystemSynthesis is the output of system-wide deep-tier synthesis.
type SystemSynthesis struct {
	Blueprint string   `json:"blueprint"`
	Patterns  []string `json:"patterns"`
}

// DeepAnalyzer runs deep-tier analysis on modules and system-wide.
type DeepAnalyzer struct {
	llm LLMClient
}

// NewDeepAnalyzer creates a DeepAnalyzer that uses the given LLM client.
func NewDeepAnalyzer(client LLMClient) *DeepAnalyzer {
	return &DeepAnalyzer{llm: client}
}

// buildModulePrompt constructs the user prompt for per-module analysis.
func buildModulePrompt(input ModuleInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Analyze the module %q (path: %s).\n\n", input.Name, input.Path)

	// Atom summaries.
	b.WriteString("## Code Units (Atoms)\n\n")
	if len(input.Atoms) == 0 {
		b.WriteString("(none)\n\n")
	} else {
		for _, a := range input.Atoms {
			fmt.Fprintf(&b, "- **%s** (%s) in `%s`\n", a.Name, a.Kind, a.FilePath)
			fmt.Fprintf(&b, "  Summary: %s\n", a.Summary)
			if len(a.Imports) > 0 {
				fmt.Fprintf(&b, "  Imports: %s\n", strings.Join(a.Imports, ", "))
			}
			if len(a.Exports) > 0 {
				fmt.Fprintf(&b, "  Exports: %s\n", strings.Join(a.Exports, ", "))
			}
		}
		b.WriteString("\n")
	}

	// File history.
	b.WriteString("## File History\n\n")
	if len(input.History) == 0 {
		b.WriteString("(none)\n\n")
	} else {
		for _, h := range input.History {
			commitCount := len(h.Commits)
			authors := strings.Join(h.Authors, ", ")
			fmt.Fprintf(&b, "- `%s`: %d commits, churn=%.0f, authors=[%s]\n",
				h.FilePath, commitCount, h.ChurnScore, authors)
		}
		b.WriteString("\n")
	}

	// Signals.
	b.WriteString("## External Signals\n\n")
	if len(input.Signals) == 0 {
		b.WriteString("(none)\n\n")
	} else {
		for _, s := range input.Signals {
			fmt.Fprintf(&b, "- [%s] %s: %s\n", s.Type, s.ID, s.Title)
		}
		b.WriteString("\n")
	}

	b.WriteString(`Produce a JSON object with these fields:
- "module_name": the module name
- "wiring": array of {"from": "<unit>", "to": "<unit>", "reason": "<why connected>"}
- "zones": array of {"name": "<domain>", "intent": "<purpose statement>", "files": ["<path>", ...]}
- "module_intent": a 1-3 sentence summary of the module's purpose
`)

	return b.String()
}

// AnalyzeModule sends a single module's data to the deep tier and returns wiring,
// zones, and intent analysis.
func (d *DeepAnalyzer) AnalyzeModule(module ModuleInput) (*ModuleAnalysis, error) {
	prompt := buildModulePrompt(module)

	raw, err := d.llm.CompleteJSON(prompt, llm.TierDeep, &llm.CompleteOptions{
		System:    "You are a software architecture analyst. Analyze this module and respond with JSON.",
		MaxTokens: 8192,
	})
	if err != nil {
		return nil, fmt.Errorf("analyzer: LLM call failed for module %q: %w", module.Name, err)
	}

	var result ModuleAnalysis
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("analyzer: failed to parse LLM response for module %q: %w", module.Name, err)
	}

	// Ensure the module name is set even if the LLM omitted it.
	if result.ModuleName == "" {
		result.ModuleName = module.Name
	}

	return &result, nil
}

// buildSynthesisPrompt constructs the user prompt for system-level synthesis.
func buildSynthesisPrompt(modules []ModuleAnalysis) string {
	var b strings.Builder

	b.WriteString("Synthesize the following module analyses into a system-level understanding.\n\n")

	for _, m := range modules {
		fmt.Fprintf(&b, "## Module: %s\n", m.ModuleName)
		fmt.Fprintf(&b, "Intent: %s\n", m.ModuleIntent)

		if len(m.Zones) > 0 {
			b.WriteString("Zones:\n")
			for _, z := range m.Zones {
				fmt.Fprintf(&b, "  - %s: %s (files: %s)\n", z.Name, z.Intent, strings.Join(z.Files, ", "))
			}
		}

		if len(m.Wiring) > 0 {
			b.WriteString("Wiring:\n")
			for _, w := range m.Wiring {
				fmt.Fprintf(&b, "  - %s -> %s: %s\n", w.From, w.To, w.Reason)
			}
		}

		b.WriteString("\n")
	}

	b.WriteString(`Produce a JSON object with these fields:
- "blueprint": a narrative description of the overall system architecture, cross-module interactions, and business purpose
- "patterns": an array of strings, each describing a coding convention or architectural pattern discovered across the codebase
`)

	return b.String()
}

// SynthesizeSystem takes all module analyses, sends them to the deep tier, and returns
// a system-level blueprint and discovered patterns.
func (d *DeepAnalyzer) SynthesizeSystem(modules []ModuleAnalysis) (*SystemSynthesis, error) {
	prompt := buildSynthesisPrompt(modules)

	raw, err := d.llm.CompleteJSON(prompt, llm.TierDeep, &llm.CompleteOptions{
		System:    "You are a senior software architect. Synthesize these module analyses into a system-level understanding. Respond with JSON.",
		MaxTokens: 8192,
	})
	if err != nil {
		return nil, fmt.Errorf("analyzer: LLM call failed for system synthesis: %w", err)
	}

	var result SystemSynthesis
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("analyzer: failed to parse LLM synthesis response: %w", err)
	}

	return &result, nil
}

// AnalyzeModules processes multiple modules in parallel using up to maxWorkers
// goroutines. The progress callback, if non-nil, is called after each module
// completes with (done, total) counts. Modules that fail analysis are skipped
// with a logged warning; successfully analyzed modules are returned along with
// any accumulated errors.
func (d *DeepAnalyzer) AnalyzeModules(modules []ModuleInput, maxWorkers int, progress func(done, total int)) ([]ModuleAnalysis, error) {
	if maxWorkers <= 0 {
		maxWorkers = 1
	}

	total := len(modules)
	results := make([]*ModuleAnalysis, total)

	sem := make(chan struct{}, maxWorkers)
	var mu sync.Mutex
	var done int
	var errs []error
	var wg sync.WaitGroup

	for i, mod := range modules {
		wg.Add(1)
		sem <- struct{}{} // acquire semaphore slot

		go func(idx int, m ModuleInput) {
			defer wg.Done()
			defer func() { <-sem }() // release semaphore slot

			analysis, err := d.AnalyzeModule(m)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				log.Printf("analyzer: warning: skipping module %q: %v", m.Name, err)
				errs = append(errs, err)
			} else {
				results[idx] = analysis
			}

			done++
			if progress != nil {
				progress(done, total)
			}
		}(i, mod)
	}

	wg.Wait()

	// Compact results: remove nil entries from skipped modules.
	compact := make([]ModuleAnalysis, 0, total)
	for _, r := range results {
		if r != nil {
			compact = append(compact, *r)
		}
	}

	// If there were errors, return them joined. The caller still gets
	// partial results for modules that succeeded.
	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return compact, fmt.Errorf("analyzer: %d module(s) failed: %s", len(errs), strings.Join(msgs, "; "))
	}

	return compact, nil
}
