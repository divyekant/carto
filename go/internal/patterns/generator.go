// Package patterns generates skill files (CLAUDE.md, .cursorrules) from
// discovered architectural patterns, business zones, and blueprint data
// produced during Carto indexing.
package patterns

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Input contains the data needed to generate pattern files.
type Input struct {
	ProjectName string
	Blueprint   string          // from SystemSynthesis
	Patterns    []string        // from SystemSynthesis
	Zones       []Zone          // aggregated from all modules
	Modules     []ModuleSummary // brief info about each module
}

// Zone is a business domain grouping.
type Zone struct {
	Name   string
	Intent string
	Files  []string
}

// ModuleSummary is brief info about a module.
type ModuleSummary struct {
	Name   string
	Intent string
	Type   string // "go", "node", "python", etc.
}

// GenerateCLAUDE produces a CLAUDE.md file content from the given input.
func GenerateCLAUDE(input Input) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n\n", input.ProjectName)

	// Architecture section.
	b.WriteString("## Architecture\n\n")
	if input.Blueprint != "" {
		b.WriteString(input.Blueprint)
		b.WriteString("\n\n")
	}

	// Modules section.
	if len(input.Modules) > 0 {
		b.WriteString("## Modules\n\n")
		for _, m := range input.Modules {
			fmt.Fprintf(&b, "### %s (%s)\n", m.Name, m.Type)
			b.WriteString(m.Intent)
			b.WriteString("\n\n")
		}
	}

	// Business Domains section.
	if len(input.Zones) > 0 {
		b.WriteString("## Business Domains\n\n")
		for _, z := range input.Zones {
			fmt.Fprintf(&b, "### %s\n", z.Name)
			b.WriteString(z.Intent)
			b.WriteString("\n\n")
			if len(z.Files) > 0 {
				b.WriteString("Files:\n")
				for _, f := range z.Files {
					fmt.Fprintf(&b, "- %s\n", f)
				}
				b.WriteString("\n")
			}
		}
	}

	// Coding Patterns section.
	if len(input.Patterns) > 0 {
		b.WriteString("## Coding Patterns\n\n")
		for _, p := range input.Patterns {
			fmt.Fprintf(&b, "- %s\n", p)
		}
		b.WriteString("\n")
	}

	// Working with the Carto Index section.
	b.WriteString("## Working with the Carto Index\n\n")
	b.WriteString("This project is indexed by Carto. The index is stored in the Memories MCP server ")
	b.WriteString("and provides semantic understanding of every code unit, cross-component wiring, and ")
	b.WriteString("architectural patterns. **You MUST query it before editing and update it after changes.**\n\n")

	// Before Editing subsection.
	b.WriteString("### Before Editing: Query for Context\n\n")
	b.WriteString("Before modifying any file, search the index for existing knowledge about that code. ")
	b.WriteString("This prevents regressions, respects existing patterns, and surfaces hidden dependencies.\n\n")
	b.WriteString("**Using Memories MCP** (preferred):\n")
	b.WriteString("```\n")
	b.WriteString("memory_search({ query: \"functionName OR fileName\", hybrid: true, k: 5 })\n")
	b.WriteString("```\n\n")
	b.WriteString("**Using curl** (fallback):\n")
	b.WriteString("```bash\n")
	b.WriteString("curl -s -X POST \"$MEMORIES_URL/search\" \\\n")
	b.WriteString("  -H \"Content-Type: application/json\" -H \"X-API-Key: $MEMORIES_API_KEY\" \\\n")
	fmt.Fprintf(&b, "  -d '{\"query\": \"functionName OR fileName\", \"k\": 5, \"hybrid\": true, \"source_prefix\": \"carto/%s/\"}'\n", input.ProjectName)
	b.WriteString("```\n\n")
	b.WriteString("**What to search for:** the function/class you are changing, the file path, ")
	b.WriteString("and related component names to check wiring dependencies.\n\n")

	// After Changes subsection.
	b.WriteString("### After Changes: Write Back\n\n")
	b.WriteString("After completing a feature, fix, or refactor, write the change back so the index ")
	b.WriteString("stays current without a full re-index.\n\n")
	b.WriteString("**Source tag convention:** `carto/")
	b.WriteString(input.ProjectName)
	b.WriteString("/{module}/layer:{layer}`\n\n")
	b.WriteString("Use `layer:atoms` for code-level changes. Use `layer:wiring` for new cross-component dependencies.\n\n")
	b.WriteString("**Atom format** (match this exactly):\n")
	b.WriteString("```\n")
	b.WriteString("name (kind) in path/to/file.ext:startLine-endLine\n")
	b.WriteString("Summary: What it does and why it exists\n")
	b.WriteString("Imports: dep1, dep2\n")
	b.WriteString("Exports: exportedSymbol\n")
	b.WriteString("```\n\n")
	b.WriteString("**Using Memories MCP** (preferred):\n")
	b.WriteString("```\n")
	b.WriteString("memory_add({\n")
	b.WriteString("  text: \"handleAuth (function) in src/auth/handler.go:15-42\\nSummary: Validates JWT tokens and extracts user claims.\\nImports: jwt, context\\nExports: handleAuth\",\n")
	fmt.Fprintf(&b, "  source: \"carto/%s/MODULE_NAME/layer:atoms\"\n", input.ProjectName)
	b.WriteString("})\n")
	b.WriteString("```\n\n")
	b.WriteString("**Using curl** (fallback):\n")
	b.WriteString("```bash\n")
	b.WriteString("curl -s -X POST \"$MEMORIES_URL/memory/add\" \\\n")
	b.WriteString("  -H \"Content-Type: application/json\" -H \"X-API-Key: $MEMORIES_API_KEY\" \\\n")
	fmt.Fprintf(&b, "  -d '{\"text\": \"SUMMARY\", \"source\": \"carto/%s/MODULE_NAME/layer:atoms\"}'\n", input.ProjectName)
	b.WriteString("```\n\n")
	b.WriteString("Replace `MODULE_NAME` with the relevant module.\n\n")
	b.WriteString("**When to write back:** new functions/types, changed signatures, new dependencies, ")
	b.WriteString("bug fixes that alter behavior, deleted code (note the deletion).\n\n")

	// Follow Discovered Patterns subsection.
	b.WriteString("### Follow Discovered Patterns\n\n")
	b.WriteString("The Coding Patterns section above reflects conventions discovered across this codebase. ")
	b.WriteString("Follow them when writing new code. When you discover a new pattern, add it:\n\n")
	b.WriteString("```\n")
	fmt.Fprintf(&b, "memory_add({ text: \"Pattern: description\", source: \"carto/%s/_system/layer:patterns\" })\n", input.ProjectName)
	b.WriteString("```\n\n")

	b.WriteString("---\n*Generated by Carto v0.3.0*\n")

	return b.String()
}

// GenerateCursorRules produces a .cursorrules file content from the given input.
func GenerateCursorRules(input Input) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Project: %s\n\n", input.ProjectName)

	// Architecture.
	b.WriteString("Architecture:\n")
	if input.Blueprint != "" {
		b.WriteString(input.Blueprint)
	}
	b.WriteString("\n\n")

	// Modules.
	if len(input.Modules) > 0 {
		b.WriteString("Modules:\n")
		for _, m := range input.Modules {
			fmt.Fprintf(&b, "- %s (%s): %s\n", m.Name, m.Type, m.Intent)
		}
		b.WriteString("\n")
	}

	// Patterns.
	if len(input.Patterns) > 0 {
		b.WriteString("Patterns:\n")
		for _, p := range input.Patterns {
			fmt.Fprintf(&b, "- %s\n", p)
		}
		b.WriteString("\n")
	}

	// Business Domains.
	if len(input.Zones) > 0 {
		b.WriteString("Business Domains:\n")
		for _, z := range input.Zones {
			fmt.Fprintf(&b, "- %s: %s\n", z.Name, z.Intent)
		}
		b.WriteString("\n")
	}

	// Working with the Carto Index instructions.
	b.WriteString("Working with the Carto Index:\n")
	b.WriteString("This project is indexed by Carto. Query before editing, write back after changes.\n\n")
	b.WriteString("Query before editing (search for the function/file you are changing):\n")
	fmt.Fprintf(&b, "  curl -s -X POST \"$MEMORIES_URL/search\" -H \"Content-Type: application/json\" -H \"X-API-Key: $MEMORIES_API_KEY\" -d '{\"query\": \"SEARCH_TERM\", \"k\": 5, \"hybrid\": true, \"source_prefix\": \"carto/%s/\"}'\n\n", input.ProjectName)
	b.WriteString("Write back after changes (use atom format: name (kind) in file:line-line | Summary | Imports | Exports):\n")
	fmt.Fprintf(&b, "  curl -s -X POST \"$MEMORIES_URL/memory/add\" -H \"Content-Type: application/json\" -H \"X-API-Key: $MEMORIES_API_KEY\" -d '{\"text\": \"SUMMARY\", \"source\": \"carto/%s/MODULE/layer:atoms\"}'\n\n", input.ProjectName)
	b.WriteString("Layers: atoms (code units), wiring (dependencies), patterns (conventions).\n")
	b.WriteString("Write back after: new functions, changed signatures, bug fixes, refactors, deletions.\n")
	b.WriteString("Follow the Patterns section above when writing new code.\n\n")

	return b.String()
}

// Section markers used to delimit the Carto-generated section within
// existing files. This allows updating the Carto section without
// destroying user-authored content.
const (
	cartoBeginMarker = "<!-- BEGIN CARTO INDEX -->"
	cartoEndMarker   = "<!-- END CARTO INDEX -->"
)

// WriteFiles writes CLAUDE.md and/or .cursorrules to the given directory.
// The format parameter controls which files are written: "claude" writes only
// CLAUDE.md, "cursor" writes only .cursorrules, and "all" writes both.
//
// If the target file already exists, the Carto section is appended or
// updated in-place (between BEGIN/END markers) without disturbing
// user-authored content.
func WriteFiles(dir string, input Input, format string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("patterns: cannot create directory %s: %w", dir, err)
	}

	switch format {
	case "claude":
		return writeCLAUDE(dir, input)
	case "cursor":
		return writeCursorRules(dir, input)
	case "all":
		if err := writeCLAUDE(dir, input); err != nil {
			return err
		}
		return writeCursorRules(dir, input)
	default:
		return fmt.Errorf("patterns: unknown format %q (expected claude, cursor, or all)", format)
	}
}

// writeCLAUDE writes (or updates) a CLAUDE.md file in the given directory.
func writeCLAUDE(dir string, input Input) error {
	path := filepath.Join(dir, "CLAUDE.md")
	cartoSection := cartoBeginMarker + "\n" + GenerateCLAUDE(input) + cartoEndMarker + "\n"
	content := mergeWithExisting(path, cartoSection)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("patterns: failed to write %s: %w", path, err)
	}
	return nil
}

// writeCursorRules writes (or updates) a .cursorrules file in the given directory.
func writeCursorRules(dir string, input Input) error {
	path := filepath.Join(dir, ".cursorrules")
	cartoSection := cartoBeginMarker + "\n" + GenerateCursorRules(input) + cartoEndMarker + "\n"
	content := mergeWithExisting(path, cartoSection)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("patterns: failed to write %s: %w", path, err)
	}
	return nil
}

// mergeWithExisting reads the file at path (if it exists) and either replaces
// an existing Carto section or appends the new section. If the file doesn't
// exist, returns just the new section.
func mergeWithExisting(path, cartoSection string) string {
	existing, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist — return just the Carto section.
		return cartoSection
	}

	old := string(existing)
	beginIdx := strings.Index(old, cartoBeginMarker)
	endIdx := strings.Index(old, cartoEndMarker)

	if beginIdx >= 0 && endIdx >= 0 && endIdx > beginIdx {
		// Replace existing Carto section.
		return old[:beginIdx] + cartoSection + old[endIdx+len(cartoEndMarker)+1:]
	}

	// Append Carto section to existing content.
	if !strings.HasSuffix(old, "\n") {
		old += "\n"
	}
	return old + "\n" + cartoSection
}
