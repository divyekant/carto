package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// ─── ANSI colour codes ─────────────────────────────────────────────────────

// ANSI escape codes for colored output.
// Maps to the Carto gold brand palette for terminal rendering.
const (
	bold  = "\033[1m"
	gold  = "\033[33m"        // brand gold #d4af37 — primary accent
	green = "\033[32m"        // success #10B981
	amber = "\033[38;5;214m"  // warnings #F59E0B — distinct from gold
	red   = "\033[31m"        // errors #F43F5E
	stone = "\033[38;5;249m"  // de-emphasis — warm neutral
	reset = "\033[0m"
)

// ─── Exit codes (Unix convention) ─────────────────────────────────────────
// Follows sysexits.h where well-known; custom codes start at 3.

const (
	ExitOK          = 0 // success
	ExitErr         = 1 // general runtime error
	ExitUsage       = 2 // invalid CLI arguments
	ExitConfig      = 3 // missing or invalid configuration
	ExitConnRefused = 4 // could not reach a required service
	ExitAuthFailure = 5 // authentication / authorisation failure
)

// ─── Spinner ───────────────────────────────────────────────────────────────

// spinnerFrames are Braille dot frames for the CLI progress spinner.
var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// ─── Output helpers ────────────────────────────────────────────────────────

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
// the human-readable callback. It traverses up to the root command to find
// the persistent --json flag set at any level.
func writeOutput(cmd *cobra.Command, data any, humanFn func()) {
	jsonMode := false
	// Walk up the command chain to find the --json persistent flag.
	c := cmd
	for c != nil {
		if f := c.PersistentFlags().Lookup("json"); f != nil {
			if v, err := c.PersistentFlags().GetBool("json"); err == nil && v {
				jsonMode = true
				break
			}
		}
		// Also check non-persistent flags in case the flag was defined locally.
		if f := c.Flags().Lookup("json"); f != nil {
			if v, err := c.Flags().GetBool("json"); err == nil && v {
				jsonMode = true
				break
			}
		}
		c = c.Parent()
	}

	if jsonMode {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(data) //nolint:errcheck
		return
	}
	humanFn()
}

// printError writes a formatted error line to stderr with ANSI colour.
// Use this for user-visible errors; the returned error is for cobra.
func printError(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s%serror:%s %s\n", bold, red, reset, fmt.Sprintf(format, args...))
}

// printWarn writes a formatted warning to stderr without stopping execution.
func printWarn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s%swarn:%s %s\n", bold, amber, reset, fmt.Sprintf(format, args...))
}

// isVerbose returns true when the --verbose persistent flag is active in the
// command hierarchy.
func isVerbose(cmd *cobra.Command) bool {
	root := cmd
	for root.Parent() != nil {
		root = root.Parent()
	}
	v, _ := root.PersistentFlags().GetBool("verbose")
	return v
}

// verboseLog prints a debug line to stderr when --verbose is active.
func verboseLog(cmd *cobra.Command, format string, args ...any) {
	if !isVerbose(cmd) {
		return
	}
	fmt.Fprintf(os.Stderr, "%s[debug]%s %s\n", gold, reset, fmt.Sprintf(format, args...))
}

// ─── Structured audit log ─────────────────────────────────────────────────

// auditEvent is the JSON shape written to the audit log file.
type auditEvent struct {
	Timestamp string         `json:"ts"`
	Level     string         `json:"level"`
	Command   string         `json:"command"`
	Args      []string       `json:"args,omitempty"`
	Result    string         `json:"result"` // "ok" | "error"
	Error     string         `json:"error,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
}

// logAuditEvent appends a JSON audit event to the file configured via
// --log-file (flag) or CARTO_AUDIT_LOG (env). Silently no-ops when neither
// is set so it never breaks normal operation.
func logAuditEvent(cmd *cobra.Command, result, errMsg string, extra map[string]any) {
	logFile, _ := cmd.Root().PersistentFlags().GetString("log-file")
	if logFile == "" {
		logFile = os.Getenv("CARTO_AUDIT_LOG")
	}
	if logFile == "" {
		return
	}

	// Build "carto config set" style command name from the cobra chain.
	var parts []string
	c := cmd
	for c != nil {
		parts = append([]string{c.Name()}, parts...)
		c = c.Parent()
	}

	ev := auditEvent{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Level:     "audit",
		Command:   strings.Join(parts, " "),
		Args:      cmd.Flags().Args(),
		Result:    result,
		Error:     errMsg,
		Extra:     extra,
	}

	data, err := json.Marshal(ev)
	if err != nil {
		return
	}

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
	if err != nil {
		return
	}
	defer f.Close()
	f.Write(append(data, '\n')) //nolint:errcheck
}

// ─── Profile helpers ───────────────────────────────────────────────────────

// resolveProfile returns the active config profile. Priority:
//  1. --profile flag
//  2. CARTO_PROFILE env var
//  3. "default"
func resolveProfile(cmd *cobra.Command) string {
	if p, _ := cmd.Root().PersistentFlags().GetString("profile"); p != "" {
		return p
	}
	if p := os.Getenv("CARTO_PROFILE"); p != "" {
		return p
	}
	return "default"
}

// ─── Status-indicator helpers ─────────────────────────────────────────────

// checkMark returns a coloured ✓ or ✗.
func checkMark(ok bool) string {
	if ok {
		return green + "✓" + reset
	}
	return red + "✗" + reset
}

// warnMark returns an amber ⚠.
func warnMark() string { return amber + "⚠" + reset }

// stoneText wraps s in the warm-neutral stone colour for de-emphasis.
func stoneText(s string) string { return stone + s + reset }
