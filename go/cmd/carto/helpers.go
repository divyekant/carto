package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// ANSI escape codes for colored output.
const (
	bold   = "\033[1m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	red    = "\033[31m"
	reset  = "\033[0m"
)

// spinner frames for progress display.
var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// truncateText shortens a string to the given max length, appending "..." if
// truncation occurs. It also replaces newlines with spaces for single-line display.
func truncateText(s string, maxLen int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", "")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// formatBytes returns a human-readable byte size string.
func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// writeOutput renders data as JSON (if --json flag is set) or invokes
// the human-readable callback.
func writeOutput(cmd *cobra.Command, data any, humanFn func()) {
	jsonMode, _ := cmd.Flags().GetBool("json")
	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(data)
		return
	}
	humanFn()
}
