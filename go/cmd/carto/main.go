package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
)

var version = "1.1.0"

func main() {
	// Sync version into the config package so the server /api/health
	// response and audit log entries reflect the same build version.
	config.Version = version

	root := &cobra.Command{
		Use:   "carto",
		Short: AppName + " — " + Tagline,
		Long: AppName + ` — ` + Tagline + `

` + Description + `

Run 'carto about' for full product information and brand color palette.

Environment variables:
  ANTHROPIC_API_KEY    API key for Anthropic (Claude)
  LLM_API_KEY          Generic LLM API key (overrides ANTHROPIC_API_KEY)
  LLM_PROVIDER         Provider: anthropic | openai | ollama (default: anthropic)
  LLM_BASE_URL         Base URL for OpenAI-compatible providers
  MEMORIES_URL         URL of the Memories vector store (default: http://localhost:8900)
  MEMORIES_API_KEY     API key for the Memories store
  PROJECTS_DIR         Directory containing indexed project subdirectories
  CARTO_SERVER_TOKEN   Bearer token for the web server (empty = dev mode, no auth)
  CARTO_CORS_ORIGINS   Comma-separated allowed CORS origins
  CARTO_AUDIT_LOG      File path for structured JSON audit logs
  CARTO_PROFILE        Config profile name (default: "default")`,
		Version: version,
	}

	// ── Global flags ───────────────────────────────────────────────────────
	// --json emits machine-readable JSON output — consumed by CI pipelines.
	root.PersistentFlags().Bool("json", false, "Output machine-readable JSON")
	// --quiet suppresses spinners and progress — useful for scripting.
	root.PersistentFlags().BoolP("quiet", "q", false, "Suppress progress spinners, only output result")
	// --verbose enables detailed diagnostic output to stderr.
	root.PersistentFlags().BoolP("verbose", "v", false, "Print verbose/debug output to stderr")
	// --log-file writes structured JSON audit events to a file.
	root.PersistentFlags().String("log-file", "", "Append structured JSON audit events to this file")
	// --profile selects the named config profile (multi-environment support).
	root.PersistentFlags().String("profile", "", "Config profile to use (overrides CARTO_PROFILE env var)")
	// --pretty forces human-readable output even when piped (inverse of --json).
	root.PersistentFlags().Bool("pretty", false, "Force human-readable output even when piped")
	// --yes skips confirmation prompts for automation and agent usage.
	root.PersistentFlags().BoolP("yes", "y", false, "Skip confirmation prompts")

	// ── Subcommands ────────────────────────────────────────────────────────
	root.AddCommand(indexCmd())
	root.AddCommand(queryCmd())
	root.AddCommand(modulesCmd())
	root.AddCommand(patternsCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(projectsCmd())
	root.AddCommand(sourcesCmd())
	root.AddCommand(configCmdGroup())
	root.AddCommand(authCmd())        // B2B: credential management
	root.AddCommand(doctorCmd())      // B2B: pre-flight environment diagnostics
	root.AddCommand(versionCmd(version)) // structured version info (JSON-capable)
	root.AddCommand(aboutCmd())          // product identity card and branding guide

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
