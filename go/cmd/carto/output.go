package main

// output.go — foundation output layer for the Carto CLI.
//
// Provides envelope-based output (JSON with ok/data/error structure),
// TTY detection, stdin reading, and confirmation prompts. This replaces
// the ad-hoc writeOutput() in helpers.go with a structured approach that
// every command can adopt incrementally.
//
// The two key entry points are:
//
//   - writeEnvelope(cmd, data, err)           — auto-routes to JSON or human
//   - writeEnvelopeHuman(cmd, data, err, fn)  — same, but with a custom human renderer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// ─── Mode detection ───────────────────────────────────────────────────────

// isJSONMode determines whether the command should emit JSON output.
//
// Priority:
//  1. --json flag explicitly set → true
//  2. --pretty flag explicitly set → false (overrides --json)
//  3. Fall back to TTY detection: non-terminal stdout → JSON
//
// Flags are read from the root command via cmd.Root().
func isJSONMode(cmd *cobra.Command) bool {
	root := cmd.Root()

	// Check --json (Changed means explicitly set by user).
	if f := root.PersistentFlags().Lookup("json"); f != nil && f.Changed {
		// --json was explicitly passed; but check --pretty override.
		if p := root.PersistentFlags().Lookup("pretty"); p != nil && p.Changed {
			return false
		}
		return true
	}

	// Check --pretty without --json: force human mode.
	if p := root.PersistentFlags().Lookup("pretty"); p != nil && p.Changed {
		return false
	}

	// Fall back: non-terminal stdout → JSON mode.
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

// isYes returns true when the --yes flag is active, indicating that
// confirmation prompts should be auto-accepted.
func isYes(cmd *cobra.Command) bool {
	root := cmd.Root()
	if f := root.PersistentFlags().Lookup("yes"); f != nil && f.Changed {
		v, err := root.PersistentFlags().GetBool("yes")
		return err == nil && v
	}
	return false
}

// ─── Envelope writers ─────────────────────────────────────────────────────

// writeEnvelope writes a JSON envelope or does nothing in human mode.
// Equivalent to writeEnvelopeHuman with a nil humanFn.
func writeEnvelope(cmd *cobra.Command, data any, err error) {
	writeEnvelopeHuman(cmd, data, err, nil)
}

// writeEnvelopeHuman is the core output function.
//
// In human mode (non-JSON): calls humanFn if non-nil and returns.
// In JSON mode:
//   - On error: writes {"ok":false,"error":"...","code":"..."} to stderr.
//   - On success: writes {"ok":true,"data":...} to stdout.
func writeEnvelopeHuman(cmd *cobra.Command, data any, err error, humanFn func()) {
	if !isJSONMode(cmd) {
		if humanFn != nil {
			humanFn()
		}
		return
	}

	if err != nil {
		ce := toCliError(err)
		env := struct {
			OK    bool   `json:"ok"`
			Error string `json:"error"`
			Code  string `json:"code"`
		}{
			OK:    false,
			Error: ce.msg,
			Code:  ce.code,
		}
		enc := json.NewEncoder(cmd.ErrOrStderr())
		enc.SetIndent("", "  ")
		enc.Encode(env) //nolint:errcheck
		return
	}

	env := struct {
		OK   bool `json:"ok"`
		Data any  `json:"data"`
	}{
		OK:   true,
		Data: data,
	}
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	enc.Encode(env) //nolint:errcheck
}

// ─── Input helpers ────────────────────────────────────────────────────────

// readInputOrStdin returns the argument as bytes. If the argument is "-"
// or empty and stdin is not a terminal, it reads from stdin instead.
func readInputOrStdin(arg string) ([]byte, error) {
	if arg != "" && arg != "-" {
		return []byte(arg), nil
	}

	if arg == "-" || !term.IsTerminal(int(os.Stdin.Fd())) {
		return io.ReadAll(os.Stdin)
	}

	return []byte(arg), nil
}

// ─── Confirmation ─────────────────────────────────────────────────────────

// confirmAction prompts the user for yes/no confirmation.
//
//   - If --yes is set: returns true immediately.
//   - If JSON mode: returns false (agents must pass --yes explicitly).
//   - Otherwise: prints prompt to stderr and reads answer from stdin.
func confirmAction(cmd *cobra.Command, prompt string) bool {
	if isYes(cmd) {
		return true
	}
	if isJSONMode(cmd) {
		return false
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N]: ", prompt)
	reader := bufio.NewReader(os.Stdin)
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer = strings.TrimSpace(strings.ToLower(answer))
	return answer == "y" || answer == "yes"
}
