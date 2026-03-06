package main

// cmd_about.go — Carto About command.
//
// `carto about` prints a rich identity card for the product: tagline,
// description, target audience, how it works, and headline features.
// The --json flag emits the same data in a machine-readable format for
// integrations that want to surface Carto's version/identity in dashboards.
//
// Usage:
//
//	carto about           # human-readable identity card
//	carto about --json    # machine-readable JSON

import (
	"fmt"

	"github.com/spf13/cobra"
)

func aboutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "about",
		Short: "Show product information and branding",
		Long: `Display Carto's identity card: tagline, description, target audience,
how it works, headline features, and the current build version.

Use --json for a machine-readable format suitable for dashboards or CI.`,
		RunE: runAbout,
	}
}

func runAbout(cmd *cobra.Command, _ []string) error {
	type aboutData struct {
		Name        string   `json:"name"`
		Version     string   `json:"version"`
		Tagline     string   `json:"tagline"`
		Description string   `json:"description"`
		ForWhom     string   `json:"for_whom"`
		HowItWorks  string   `json:"how_it_works"`
		Features    []string `json:"features"`
		ProjectURL  string   `json:"project_url"`
	}

	features := []string{
		"Semantic code search across your entire repository",
		"LLM-powered module intent extraction (Anthropic, OpenAI, Ollama)",
		"Layered storage: atoms → modules → blueprints → patterns",
		"CLAUDE.md and .cursorrules generator for AI assistant context",
		"GitHub, Jira, Linear, Notion, Slack, PDF source connectors",
		"Incremental re-indexing — only changed files are re-processed",
		"Docker-native deployment with bearer-auth and audit logging",
	}

	data := aboutData{
		Name:        AppName,
		Version:     version,
		Tagline:     Tagline,
		Description: Description,
		ForWhom:     WhoItIsFor,
		HowItWorks:  HowItWorks,
		Features:    features,
		ProjectURL:  ProjectURL,
	}

	writeOutput(cmd, data, func() {
		// ── Header ──────────────────────────────────────────────────────────
		fmt.Printf("\n  %s%s%s%s\n", bold, gold, AppName, reset)
		fmt.Printf("  %s\n", Tagline)
		fmt.Printf("  version %s%s%s\n\n", bold, version, reset)

		// ── Description ─────────────────────────────────────────────────────
		fmt.Printf("%s%sWhat is Carto?%s\n", bold, gold, reset)
		fmt.Printf("  Carto indexes your source code, documentation, issues, and\n")
		fmt.Printf("  knowledge bases into a semantic vector store, making every file,\n")
		fmt.Printf("  pattern, and architectural decision retrievable by meaning —\n")
		fmt.Printf("  not just keyword.\n\n")

		// ── For Whom ────────────────────────────────────────────────────────
		fmt.Printf("%s%sWho is it for?%s\n", bold, gold, reset)
		fmt.Printf("  • Engineering teams that want AI assistants to understand their\n")
		fmt.Printf("    whole project, not just the file currently open.\n")
		fmt.Printf("  • Platform engineers building internal developer portals.\n")
		fmt.Printf("  • CTOs who need codebase-wide insights, automated documentation,\n")
		fmt.Printf("    and dependency graphs on demand.\n\n")

		// ── How It Works ────────────────────────────────────────────────────
		fmt.Printf("%s%sHow it works%s\n", bold, gold, reset)
		fmt.Printf("  %s1. Index%s    Scan your codebase, extract modules, analyse patterns\n", bold, reset)
		fmt.Printf("            with LLMs, and store semantic embeddings in a layered\n")
		fmt.Printf("            Memories vector store.\n")
		fmt.Printf("  %s2. Query%s    Ask natural-language questions. Carto retrieves the\n", bold, reset)
		fmt.Printf("            right code, docs, and context across your entire history.\n")
		fmt.Printf("  %s3. Generate%s Produce CLAUDE.md and .cursorrules files so AI assistants\n", bold, reset)
		fmt.Printf("            receive a detailed map of your project's intent.\n")
		fmt.Printf("  %s4. Integrate%s Connect GitHub Issues, Jira, Notion, Slack, and PDFs\n", bold, reset)
		fmt.Printf("            into one unified knowledge graph.\n\n")

		// ── Features ────────────────────────────────────────────────────────
		fmt.Printf("%s%sHeadline features%s\n", bold, gold, reset)
		for _, f := range features {
			fmt.Printf("  %s•%s %s\n", gold, reset, f)
		}
		fmt.Printf("\n")

		// ── Color palette ───────────────────────────────────────────────────
		fmt.Printf("%s%sBrand colors%s\n", bold, gold, reset)
		fmt.Printf("  %-14s %s#d4af37%s  primary actions, headers, active states\n", "Brand Gold", bold, reset)
		fmt.Printf("  %-14s %s#78716c%s  neutral text, borders, de-emphasis\n", "Stone", bold, reset)
		fmt.Printf("  %-14s %s#F59E0B%s  warnings\n", "Amber", bold, reset)
		fmt.Printf("  %-14s %s#F43F5E%s  errors and destructive actions\n", "Rose", bold, reset)
		fmt.Printf("  %-14s %s#10B981%s  success indicators\n", "Emerald", bold, reset)
		fmt.Printf("\n")

		// ── Footer ──────────────────────────────────────────────────────────
		fmt.Printf("  %s%s%s\n\n", gold, ProjectURL, reset)
	})

	return nil
}
