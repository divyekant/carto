package main

// branding.go — Carto brand identity constants.
//
// All user-visible product copy, taglines, and colour documentation live here
// so they can be kept in sync across the CLI help text, the web UI, and the
// /api/about JSON endpoint without hunting through multiple files.
//
// ─── Color Palette ─────────────────────────────────────────────────────────
//
//  Name              Hex        OKLCH                      Role
//  ──────────────── ────────── ─────────────────────────  ───────────────────
//  Brand Indigo      #5B50F5   oklch(0.511 0.233 267.4)  Primary actions,
//                                                          logo mark, active
//                                                          nav, focus rings
//  Brand Teal        #0EA5A0   oklch(0.630 0.118 185.4)  Info callouts,
//                                                          success badges
//  Slate (neutral)             monochrome oklch slate     Backgrounds, text,
//                                                          borders, cards
//  Amber             #F59E0B   oklch(0.748 0.183 73.8)   Warnings
//  Rose              #F43F5E   oklch(0.614 0.239 11.5)   Errors / destructive
//  Emerald           #10B981   oklch(0.726 0.175 158.4)  Success indicators
//
// CLI ANSI palette (terminals do not render arbitrary hex colours):
//   cyan  (\033[36m) — maps to Brand Indigo role in terminal output
//   green (\033[32m) — success indicators
//   yellow(\033[33m) — warnings
//   red   (\033[31m) — errors

const (
	// AppName is the canonical product name.
	AppName = "Carto"

	// Tagline is the one-line brand statement shown at the top of help text
	// and on the About page.
	Tagline = "Map your codebase. Navigate with intent."

	// ShortDescription is a single-sentence summary of what Carto does.
	ShortDescription = "Intent-aware codebase intelligence for engineering teams."

	// Description is the full product description used in --help output and
	// the /api/about endpoint.
	Description = `Carto indexes your source code, documentation, issues, and knowledge
bases into a semantic vector store, making every file, pattern, and
architectural decision retrievable by meaning — not just keyword.`

	// WhoItIsFor describes the target audience.
	WhoItIsFor = `Engineering teams that want AI assistants to understand their whole
project. Platform engineers building internal developer portals. CTOs
who need codebase-wide insights, automated documentation, and
dependency graphs on demand.`

	// HowItWorks is a concise step-by-step description of the Carto pipeline.
	HowItWorks = `1. Index  — Carto scans your codebase, extracts modules, analyses patterns
           with LLMs, and stores semantic embeddings in a layered Memories
           vector store.
2. Query  — Ask natural-language questions. Carto retrieves the right code,
           docs, and context across your entire project history.
3. Generate — Produce CLAUDE.md and .cursorrules files so AI assistants
           receive a detailed map of your project's intent and architecture.
4. Integrate — Connect GitHub Issues, Jira, Notion, Slack, and PDFs into one
           unified knowledge graph.`

	// Features is a short list of headline capabilities.
	Features = `• Semantic code search across your entire repository
• LLM-powered module intent extraction (Anthropic, OpenAI, Ollama)
• Layered storage: atoms → modules → blueprints → patterns
• CLAUDE.md and .cursorrules generator for AI assistant context
• GitHub, Jira, Linear, Notion, Slack, PDF source connectors
• Incremental re-indexing — only changed files are re-processed
• Docker-native deployment with bearer-auth and audit logging`

	// ProjectURL is the canonical homepage / repository URL.
	ProjectURL = "https://github.com/divyekant/carto"
)
