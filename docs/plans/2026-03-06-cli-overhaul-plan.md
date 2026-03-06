# CLI Overhaul Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make Carto's CLI agent-usable and human-usable with JSON envelope contract, TTY auto-detection, gold brand, and 6 new commands — matching Memories CLI patterns.

**Architecture:** Layer-by-layer approach. Foundation (output.go, errors.go) first, then gold brand swap, then retrofit all existing commands, then add new commands, then fill test gaps. Each layer builds on the previous. No breaking changes.

**Tech Stack:** Go, Cobra CLI framework, `golang.org/x/term` for TTY detection, existing `internal/config` and `internal/storage` packages.

---

### Task 1: Add `golang.org/x/term` dependency

**Files:**
- Modify: `go/go.mod`

**Step 1: Add the dependency**

Run: `cd /Users/divyekant/Projects/carto/go && go get golang.org/x/term`

**Step 2: Verify**

Run: `grep "golang.org/x/term" go.mod`
Expected: dependency listed

**Step 3: Commit**

```bash
git add go/go.mod go/go.sum
git commit -m "chore: add golang.org/x/term for TTY detection"
```

---

### Task 2: Create `output.go` — envelope, TTY detection, stdin helper

**Files:**
- Create: `go/cmd/carto/output.go`
- Test: `go/cmd/carto/output_test.go`

**Step 1: Write the failing tests**

Create `go/cmd/carto/output_test.go`:

```go
package main

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// ── Envelope structure for test assertions ──

type envelope struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
	Code  string `json:"code,omitempty"`
}

func parseEnvelope(t *testing.T, raw string) envelope {
	t.Helper()
	var env envelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &env); err != nil {
		t.Fatalf("not a valid JSON envelope: %v\nraw: %s", err, raw)
	}
	return env
}

// ── Test: isJSONMode ──

func TestIsJSONMode_ExplicitFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().Bool("json", false, "")
	cmd.PersistentFlags().Bool("pretty", false, "")
	cmd.PersistentFlags().Set("json", "true")

	if !isJSONMode(cmd) {
		t.Error("expected JSON mode when --json is set")
	}
}

func TestIsJSONMode_PrettyOverrides(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().Bool("json", false, "")
	cmd.PersistentFlags().Bool("pretty", false, "")
	cmd.PersistentFlags().Set("pretty", "true")

	if isJSONMode(cmd) {
		t.Error("--pretty should force non-JSON mode")
	}
}

// ── Test: writeEnvelope success ──

func TestWriteEnvelope_Success(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().Bool("json", false, "")
	cmd.PersistentFlags().Bool("pretty", false, "")
	cmd.PersistentFlags().Set("json", "true")

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	writeEnvelope(cmd, map[string]string{"key": "value"}, nil)

	env := parseEnvelope(t, buf.String())
	if !env.OK {
		t.Error("expected ok: true")
	}
	if env.Error != "" {
		t.Error("expected no error in success envelope")
	}
}

// ── Test: writeEnvelope error ──

func TestWriteEnvelope_Error(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().Bool("json", false, "")
	cmd.PersistentFlags().Bool("pretty", false, "")
	cmd.PersistentFlags().Set("json", "true")

	var buf bytes.Buffer
	cmd.SetErr(&buf)

	writeEnvelope(cmd, nil, &cliError{msg: "test error", code: "TEST_ERROR", exit: 1})

	env := parseEnvelope(t, buf.String())
	if env.OK {
		t.Error("expected ok: false for error envelope")
	}
	if env.Code != "TEST_ERROR" {
		t.Errorf("expected code TEST_ERROR, got %q", env.Code)
	}
}

// ── Test: readInputOrStdin with argument ──

func TestReadInputOrStdin_Argument(t *testing.T) {
	data, err := readInputOrStdin("hello world")
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(data))
	}
}

// ── Test: writeEnvelope human mode calls humanFn ──

func TestWriteEnvelope_HumanMode(t *testing.T) {
	cmd := &cobra.Command{Use: "test"}
	cmd.PersistentFlags().Bool("json", false, "")
	cmd.PersistentFlags().Bool("pretty", false, "")
	// json=false, pretty=false, but we're in a test (isatty=false)
	// Force pretty to get human mode:
	cmd.PersistentFlags().Set("pretty", "true")

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	called := false
	writeEnvelopeHuman(cmd, map[string]string{"key": "value"}, nil, func() {
		called = true
	})

	if !called {
		t.Error("expected humanFn to be called in non-JSON mode")
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -run "TestIsJSONMode|TestWriteEnvelope|TestReadInputOrStdin" -v`
Expected: compilation errors (functions not defined)

**Step 3: Write `output.go`**

Create `go/cmd/carto/output.go`:

```go
package main

import (
	"encoding/json"
	"io"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// ── TTY detection ─────────────────────────────────────────────────────────

// isJSONMode determines whether to output JSON. Priority:
//  1. --json flag explicitly set → true
//  2. --pretty flag explicitly set → false
//  3. stdout is not a TTY (piped / agent) → true
//  4. otherwise → false (human terminal)
func isJSONMode(cmd *cobra.Command) bool {
	root := cmd.Root()
	if f := root.PersistentFlags().Lookup("json"); f != nil && f.Changed {
		return true
	}
	if f := root.PersistentFlags().Lookup("pretty"); f != nil && f.Changed {
		return false
	}
	return !term.IsTerminal(int(os.Stdout.Fd()))
}

// isYes returns true when the --yes/-y flag is set.
func isYes(cmd *cobra.Command) bool {
	root := cmd.Root()
	v, _ := root.PersistentFlags().GetBool("yes")
	return v
}

// ── JSON envelope ─────────────────────────────────────────────────────────

// writeEnvelope outputs data as a JSON envelope (agent mode) or
// does nothing in human mode (caller handles human output separately).
// If err is non-nil, writes an error envelope to stderr.
func writeEnvelope(cmd *cobra.Command, data any, err error) {
	writeEnvelopeHuman(cmd, data, err, nil)
}

// writeEnvelopeHuman outputs data as JSON envelope or calls humanFn.
// This is the core output function. Use writeEnvelope when you don't
// need a human callback (e.g., inside runWithEnvelope).
func writeEnvelopeHuman(cmd *cobra.Command, data any, err error, humanFn func()) {
	if !isJSONMode(cmd) {
		if humanFn != nil {
			humanFn()
		}
		return
	}

	type successEnvelope struct {
		OK   bool `json:"ok"`
		Data any  `json:"data"`
	}
	type errorEnvelope struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}

	if err != nil {
		cErr := toCliError(err)
		env := errorEnvelope{OK: false, Error: cErr.msg, Code: cErr.code}
		w := cmd.ErrOrStderr()
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		enc.Encode(env) //nolint:errcheck
		return
	}

	env := successEnvelope{OK: true, Data: data}
	w := cmd.OutOrStdout()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.Encode(env) //nolint:errcheck
}

// ── Stdin support ─────────────────────────────────────────────────────────

// readInputOrStdin reads from the argument if provided, or from stdin if
// the argument is "-" or stdin is a pipe (not a terminal).
func readInputOrStdin(arg string) ([]byte, error) {
	if arg != "" && arg != "-" {
		return []byte(arg), nil
	}
	if arg == "-" || !term.IsTerminal(int(os.Stdin.Fd())) {
		return io.ReadAll(os.Stdin)
	}
	return []byte(arg), nil
}

// ── Confirmation prompt ───────────────────────────────────────────────────

// confirmAction prompts the user for confirmation unless --yes is set.
// Returns true if the action should proceed.
func confirmAction(cmd *cobra.Command, prompt string) bool {
	if isYes(cmd) {
		return true
	}
	if isJSONMode(cmd) {
		// Agents must use --yes; never block on stdin prompt in JSON mode.
		return false
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "%s [y/N]: ", prompt)
	var answer string
	fmt.Fscanln(cmd.InOrStdin(), &answer)
	return answer == "y" || answer == "Y" || answer == "yes"
}
```

Note: add `"fmt"` to the imports for `confirmAction`.

**Step 4: Run tests to verify they pass**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -run "TestIsJSONMode|TestWriteEnvelope|TestReadInputOrStdin" -v`
Expected: PASS

**Step 5: Commit**

```bash
git add go/cmd/carto/output.go go/cmd/carto/output_test.go
git commit -m "feat(cli): add output.go — JSON envelope, TTY detection, stdin support"
```

---

### Task 3: Create `errors.go` — typed errors, classifier, runWithEnvelope

**Files:**
- Create: `go/cmd/carto/errors.go`
- Test: `go/cmd/carto/errors_test.go`

**Step 1: Write the failing tests**

Create `go/cmd/carto/errors_test.go`:

```go
package main

import (
	"errors"
	"testing"
)

func TestClassifyError_Connection(t *testing.T) {
	err := &cliError{msg: "connection refused", code: ErrCodeConnection, exit: ExitConnRefused}
	cErr := toCliError(err)
	if cErr.code != ErrCodeConnection {
		t.Errorf("expected %s, got %s", ErrCodeConnection, cErr.code)
	}
	if cErr.exit != ExitConnRefused {
		t.Errorf("expected exit %d, got %d", ExitConnRefused, cErr.exit)
	}
}

func TestClassifyError_Auth(t *testing.T) {
	err := &cliError{msg: "unauthorized", code: ErrCodeAuth, exit: ExitAuthFailure}
	cErr := toCliError(err)
	if cErr.code != ErrCodeAuth {
		t.Errorf("expected %s, got %s", ErrCodeAuth, cErr.code)
	}
}

func TestClassifyError_NotFound(t *testing.T) {
	err := &cliError{msg: "not found", code: ErrCodeNotFound, exit: ExitNotFound}
	cErr := toCliError(err)
	if cErr.code != ErrCodeNotFound {
		t.Errorf("expected %s, got %s", ErrCodeNotFound, cErr.code)
	}
}

func TestClassifyError_Config(t *testing.T) {
	err := &cliError{msg: "missing config", code: ErrCodeConfig, exit: ExitConfig}
	cErr := toCliError(err)
	if cErr.code != ErrCodeConfig {
		t.Errorf("expected %s, got %s", ErrCodeConfig, cErr.code)
	}
}

func TestClassifyError_GenericError(t *testing.T) {
	err := errors.New("something went wrong")
	cErr := toCliError(err)
	if cErr.code != ErrCodeGeneral {
		t.Errorf("expected %s, got %s", ErrCodeGeneral, cErr.code)
	}
	if cErr.exit != ExitErr {
		t.Errorf("expected exit %d, got %d", ExitErr, cErr.exit)
	}
}

func TestNewConnectionError(t *testing.T) {
	err := newConnectionError("cannot reach server")
	var cErr *cliError
	if !errors.As(err, &cErr) {
		t.Fatal("expected *cliError type")
	}
	if cErr.code != ErrCodeConnection {
		t.Errorf("expected %s, got %s", ErrCodeConnection, cErr.code)
	}
}

func TestNewAuthError(t *testing.T) {
	err := newAuthError("bad key")
	var cErr *cliError
	if !errors.As(err, &cErr) {
		t.Fatal("expected *cliError type")
	}
	if cErr.code != ErrCodeAuth {
		t.Errorf("expected %s, got %s", ErrCodeAuth, cErr.code)
	}
}

func TestNewNotFoundError(t *testing.T) {
	err := newNotFoundError("project not found")
	var cErr *cliError
	if !errors.As(err, &cErr) {
		t.Fatal("expected *cliError type")
	}
	if cErr.code != ErrCodeNotFound {
		t.Errorf("expected %s, got %s", ErrCodeNotFound, cErr.code)
	}
}

func TestNewConfigError(t *testing.T) {
	err := newConfigError("missing key")
	var cErr *cliError
	if !errors.As(err, &cErr) {
		t.Fatal("expected *cliError type")
	}
	if cErr.code != ErrCodeConfig {
		t.Errorf("expected %s, got %s", ErrCodeConfig, cErr.code)
	}
}
```

**Step 2: Run tests to verify they fail**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -run "TestClassifyError|TestNew.*Error" -v`
Expected: compilation errors

**Step 3: Write `errors.go`**

Create `go/cmd/carto/errors.go`:

```go
package main

import (
	"errors"
	"os"

	"github.com/spf13/cobra"
)

// ─── Error codes (matching Memories CLI contract) ────────────────────────

const (
	ErrCodeGeneral    = "GENERAL_ERROR"
	ErrCodeNotFound   = "NOT_FOUND"
	ErrCodeConnection = "CONNECTION_ERROR"
	ErrCodeAuth       = "AUTH_FAILURE"
	ErrCodeConfig     = "CONFIG_ERROR"
)

// ─── Exit codes ──────────────────────────────────────────────────────────
// Note: ExitOK (0), ExitErr (1) are in helpers.go.
// Adding NOT_FOUND (2) — helpers.go has ExitUsage=2, ExitConfig=3,
// ExitConnRefused=4, ExitAuthFailure=5.
// We remap to match the Memories convention:
//   0=ok, 1=general, 2=not_found, 3=connection, 4=auth, 5=config

const (
	ExitNotFound = 2 // replaces ExitUsage for envelope-aware commands
)

// ─── Typed error ─────────────────────────────────────────────────────────

// cliError carries error metadata for the JSON envelope contract.
type cliError struct {
	msg  string
	code string
	exit int
}

func (e *cliError) Error() string { return e.msg }

// ─── Constructors ────────────────────────────────────────────────────────

func newConnectionError(msg string) error {
	return &cliError{msg: msg, code: ErrCodeConnection, exit: ExitConnRefused}
}

func newAuthError(msg string) error {
	return &cliError{msg: msg, code: ErrCodeAuth, exit: ExitAuthFailure}
}

func newNotFoundError(msg string) error {
	return &cliError{msg: msg, code: ErrCodeNotFound, exit: ExitNotFound}
}

func newConfigError(msg string) error {
	return &cliError{msg: msg, code: ErrCodeConfig, exit: ExitConfig}
}

// ─── Classifier ──────────────────────────────────────────────────────────

// toCliError extracts a *cliError from err, or wraps it as GENERAL_ERROR.
func toCliError(err error) *cliError {
	var cErr *cliError
	if errors.As(err, &cErr) {
		return cErr
	}
	return &cliError{msg: err.Error(), code: ErrCodeGeneral, exit: ExitErr}
}

// ─── runWithEnvelope ─────────────────────────────────────────────────────

// runWithEnvelope executes fn, writes the result or error as a JSON
// envelope, and exits with the appropriate code on error.
// humanFn is called in human mode for success output.
func runWithEnvelope(cmd *cobra.Command, humanFn func(data any), fn func() (any, error)) {
	data, err := fn()
	if err != nil {
		cErr := toCliError(err)
		// Human mode: print colored error to stderr.
		if !isJSONMode(cmd) {
			printError("%s", cErr.msg)
		}
		writeEnvelope(cmd, nil, err)
		logAuditEvent(cmd, "error", cErr.msg, nil)
		os.Exit(cErr.exit)
	}

	writeEnvelopeHuman(cmd, data, nil, func() {
		if humanFn != nil {
			humanFn(data)
		}
	})
	logAuditEvent(cmd, "ok", "", nil)
}
```

**Step 4: Run tests to verify they pass**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -run "TestClassifyError|TestNew.*Error" -v`
Expected: PASS

**Step 5: Run all tests to verify no regressions**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -v`
Expected: all existing tests still pass

**Step 6: Commit**

```bash
git add go/cmd/carto/errors.go go/cmd/carto/errors_test.go
git commit -m "feat(cli): add errors.go — typed errors, classifier, runWithEnvelope"
```

---

### Task 4: Add `--pretty` and `--yes` global flags

**Files:**
- Modify: `go/cmd/carto/main.go` (lines 43-52)

**Step 1: Add the new flags after existing global flags**

In `main.go`, after line 52 (`--profile` flag), add:

```go
	// --pretty forces human-readable output even when piped (inverse of --json).
	root.PersistentFlags().Bool("pretty", false, "Force human-readable output even when piped")
	// --yes skips confirmation prompts for automation and agent usage.
	root.PersistentFlags().BoolP("yes", "y", false, "Skip confirmation prompts")
```

**Step 2: Run tests**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -v`
Expected: PASS (no regressions)

**Step 3: Commit**

```bash
git add go/cmd/carto/main.go
git commit -m "feat(cli): add --pretty and --yes/-y global flags"
```

---

### Task 5: Gold brand — update color constants

**Files:**
- Modify: `go/cmd/carto/helpers.go` (lines 13-23, 109-111, 129, 215)
- Modify: `go/cmd/carto/branding.go` (lines 9-28)

**Step 1: Update ANSI color palette in `helpers.go`**

Replace lines 13-23:

```go
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
```

**Step 2: Update `printWarn` to use `amber`**

Replace line 111 (`printWarn`):

```go
func printWarn(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s%swarn:%s %s\n", bold, amber, reset, fmt.Sprintf(format, args...))
}
```

**Step 3: Update `verboseLog` to use `gold`**

Replace line 129:

```go
	fmt.Fprintf(os.Stderr, "%s[debug]%s %s\n", gold, reset, fmt.Sprintf(format, args...))
```

**Step 4: Update `warnMark` to use `amber`**

Replace line 215:

```go
func warnMark() string { return amber + "⚠" + reset }
```

**Step 5: Update branding.go color documentation**

Replace the color palette comment block (lines 9-28) in `branding.go`:

```go
// ─── Color Palette ─────────────────────────────────────────────────────────
//
//  Name              Hex        Role
//  ──────────────── ────────── ───────────────────────────────────────────
//  Brand Gold        #d4af37   Primary actions, headers, active states,
//                               logo mark, focus rings
//  Stone             #78716c   Neutral text, borders, de-emphasis
//  Amber             #F59E0B   Warnings
//  Rose              #F43F5E   Errors / destructive actions
//  Emerald           #10B981   Success indicators
//
// CLI ANSI palette (terminals do not render arbitrary hex colours):
//   gold  (\033[33m)       — maps to Brand Gold role in terminal output
//   green (\033[32m)       — success indicators
//   amber (\033[38;5;214m) — warnings (256-color, distinct from gold)
//   red   (\033[31m)       — errors
//   stone (\033[38;5;249m) — de-emphasis, neutral text
```

**Step 6: Fix all references to old color names**

Run: `cd /Users/divyekant/Projects/carto/go && grep -rn 'cyan\|yellow' cmd/carto/ --include="*.go" | grep -v "_test.go" | grep -v "//"`

Replace every `cyan` reference with `gold` and every `yellow` reference with `amber` across all `cmd_*.go` files. This is a mechanical find-and-replace. Key files:
- `cmd_about.go` — all `cyan` → `gold`
- `cmd_doctor.go` — all `cyan` → `gold`, `yellow` → `amber`
- `cmd_index.go` — spinner color `cyan` → `gold`
- `cmd_version.go` — `cyan` → `gold`
- `cmd_auth.go` — `cyan` → `gold`, `yellow` → `amber`
- `cmd_config.go` — `cyan` → `gold`
- `cmd_projects.go` — `cyan` → `gold`
- `cmd_query.go` — `cyan` → `gold`
- `cmd_status.go` — `cyan` → `gold`
- `cmd_sources.go` — `cyan` → `gold`
- `cmd_modules.go` — `cyan` → `gold`
- `cmd_serve.go` — `cyan` → `gold`, `yellow` → `amber`

**Step 7: Update `about` command brand colors section**

In `cmd_about.go`, replace the "Brand colors" section (lines 107-113):

```go
		fmt.Printf("%s%sBrand colors%s\n", bold, gold, reset)
		fmt.Printf("  %-14s %s#d4af37%s  primary actions, headers, active states\n", "Brand Gold", bold, reset)
		fmt.Printf("  %-14s %s#78716c%s  neutral text, borders, de-emphasis\n", "Stone", bold, reset)
		fmt.Printf("  %-14s %s#F59E0B%s  warnings\n", "Amber", bold, reset)
		fmt.Printf("  %-14s %s#F43F5E%s  errors and destructive actions\n", "Rose", bold, reset)
		fmt.Printf("  %-14s %s#10B981%s  success indicators\n", "Emerald", bold, reset)
```

**Step 8: Run all tests**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -v`
Expected: PASS (color changes don't break logic)

**Step 9: Commit**

```bash
git add go/cmd/carto/helpers.go go/cmd/carto/branding.go go/cmd/carto/cmd_*.go
git commit -m "feat(cli): gold brand — replace indigo/cyan with gold/stone palette"
```

---

### Task 6: Retrofit `version` and `about` commands to use envelope

**Files:**
- Modify: `go/cmd/carto/cmd_version.go`
- Modify: `go/cmd/carto/cmd_about.go`

**Step 1: Write failing test for version envelope**

Add to `output_test.go`:

```go
func TestVersionCmd_JSONEnvelope(t *testing.T) {
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("pretty", false, "")
	root.PersistentFlags().Bool("yes", false, "")

	cmd := versionCmd("2.0.0-test")
	root.AddCommand(cmd)

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"version", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	env := parseEnvelope(t, buf.String())
	if !env.OK {
		t.Error("expected ok: true")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -run TestVersionCmd_JSONEnvelope -v`
Expected: FAIL (version still uses writeOutput, not envelope)

**Step 3: Update `cmd_version.go`**

Replace the `RunE` function body to use `runWithEnvelope`:

```go
		RunE: func(cmd *cobra.Command, _ []string) error {
			type versionInfo struct {
				Version   string `json:"version"`
				GoVersion string `json:"go_version"`
				OS        string `json:"os"`
				Arch      string `json:"arch"`
			}

			info := versionInfo{
				Version:   ver,
				GoVersion: runtime.Version(),
				OS:        runtime.GOOS,
				Arch:      runtime.GOARCH,
			}

			writeEnvelopeHuman(cmd, info, nil, func() {
				fmt.Printf("%s%scarto%s %s\n", bold, gold, reset, ver)
				fmt.Printf("  go:   %s\n", info.GoVersion)
				fmt.Printf("  os:   %s/%s\n", info.OS, info.Arch)
			})

			logAuditEvent(cmd, "ok", "", map[string]any{"version": ver})
			return nil
		},
```

**Step 4: Update `cmd_about.go`**

Replace `writeOutput(cmd, data, func() {` with `writeEnvelopeHuman(cmd, data, nil, func() {` and update the closing to match.

**Step 5: Run tests**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add go/cmd/carto/cmd_version.go go/cmd/carto/cmd_about.go go/cmd/carto/output_test.go
git commit -m "feat(cli): retrofit version and about to use JSON envelope"
```

---

### Task 7: Retrofit remaining existing commands to use envelope

**Files:**
- Modify: `go/cmd/carto/cmd_auth.go`
- Modify: `go/cmd/carto/cmd_config.go`
- Modify: `go/cmd/carto/cmd_doctor.go`
- Modify: `go/cmd/carto/cmd_index.go`
- Modify: `go/cmd/carto/cmd_query.go`
- Modify: `go/cmd/carto/cmd_modules.go`
- Modify: `go/cmd/carto/cmd_patterns.go`
- Modify: `go/cmd/carto/cmd_status.go`
- Modify: `go/cmd/carto/cmd_projects.go`
- Modify: `go/cmd/carto/cmd_sources.go`
- Modify: `go/cmd/carto/cmd_serve.go`

This is a mechanical migration. For each command:

1. Replace `writeOutput(cmd, data, func() { ... })` with `writeEnvelopeHuman(cmd, data, nil, func() { ... })`
2. Where commands manually call `printError` + `os.Exit`, wrap the logic in `runWithEnvelope` instead
3. Use typed errors: `newConnectionError()`, `newAuthError()`, `newNotFoundError()`, `newConfigError()` where appropriate

**Key error mappings:**
- `cmd_auth.go` validate: connection errors → `newConnectionError`, 401/403 → `newAuthError`
- `cmd_config.go` validate: missing keys → `newConfigError`
- `cmd_index.go`: LLM/Memories connection → `newConnectionError`, auth → `newAuthError`
- `cmd_query.go`: project not found → `newNotFoundError`, connection → `newConnectionError`
- `cmd_status.go`: no index → `newNotFoundError`
- `cmd_projects.go` show/delete: project not found → `newNotFoundError`
- `cmd_projects.go` delete: add confirmation prompt using `confirmAction(cmd, "Delete project X?")`
- `cmd_sources.go`: project not found → `newNotFoundError`

**Config source attribution in `cmd_config.go` `get` subcommand:**

Add a `resolveSource` helper that checks where each config value came from:

```go
func resolveSource(key string, cfg *config.Config) string {
	// Check if flag was set → "flag"
	// Check if env var is set → "env"
	// Check if file has the key → "file"
	// Otherwise → "default"
}
```

Integrate into the config get output structure.

**Step: Run all tests after each command file**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -v`
Expected: PASS after each file

**Step: Commit per batch (group related commands)**

```bash
git add go/cmd/carto/cmd_auth.go go/cmd/carto/cmd_config.go go/cmd/carto/cmd_doctor.go
git commit -m "feat(cli): retrofit auth, config, doctor to JSON envelope"

git add go/cmd/carto/cmd_index.go go/cmd/carto/cmd_query.go go/cmd/carto/cmd_modules.go
git commit -m "feat(cli): retrofit index, query, modules to JSON envelope"

git add go/cmd/carto/cmd_patterns.go go/cmd/carto/cmd_status.go go/cmd/carto/cmd_serve.go
git commit -m "feat(cli): retrofit patterns, status, serve to JSON envelope"

git add go/cmd/carto/cmd_projects.go go/cmd/carto/cmd_sources.go
git commit -m "feat(cli): retrofit projects, sources to JSON envelope + --yes confirmation"
```

---

### Task 8: Remove old `writeOutput` from helpers.go

**Files:**
- Modify: `go/cmd/carto/helpers.go`

**Step 1: Delete `writeOutput` function (lines 69-100)**

Remove the entire `writeOutput` function. All commands now use `writeEnvelopeHuman`.

**Step 2: Run all tests**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -v`
Expected: PASS (no remaining references)

**Step 3: Verify no references remain**

Run: `cd /Users/divyekant/Projects/carto/go && grep -rn "writeOutput" cmd/carto/`
Expected: no results

**Step 4: Commit**

```bash
git add go/cmd/carto/helpers.go
git commit -m "refactor(cli): remove deprecated writeOutput — all commands use envelope"
```

---

### Task 9: New command — `carto completions`

**Files:**
- Create: `go/cmd/carto/cmd_completions.go`
- Modify: `go/cmd/carto/main.go` (register command)

**Step 1: Write the test**

Add to `output_test.go` or a new `cmd_completions_test.go`:

```go
func TestCompletionsCmd_Bash(t *testing.T) {
	root := &cobra.Command{Use: "carto"}
	root.AddCommand(completionsCmd())

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completions", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "bash") && !strings.Contains(buf.String(), "complete") {
		t.Error("expected bash completion script")
	}
}

func TestCompletionsCmd_Zsh(t *testing.T) {
	root := &cobra.Command{Use: "carto"}
	root.AddCommand(completionsCmd())

	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completions", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected zsh completion output")
	}
}
```

**Step 2: Write `cmd_completions.go`**

```go
package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func completionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "completions <bash|zsh|fish|powershell>",
		Short:     "Generate shell completion scripts",
		Long:      `Generate autocompletion scripts for your shell. Source the output to enable tab completion.`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell: %s (use bash, zsh, fish, or powershell)", args[0])
			}
		},
	}
}
```

**Step 3: Register in `main.go`**

Add after line 67: `root.AddCommand(completionsCmd())`

**Step 4: Run tests**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -run TestCompletionsCmd -v`
Expected: PASS

**Step 5: Commit**

```bash
git add go/cmd/carto/cmd_completions.go go/cmd/carto/main.go go/cmd/carto/output_test.go
git commit -m "feat(cli): add completions command — bash, zsh, fish, powershell"
```

---

### Task 10: New command — `carto init`

**Files:**
- Create: `go/cmd/carto/cmd_init.go`
- Create: `go/cmd/carto/cmd_init_test.go`
- Modify: `go/cmd/carto/main.go` (register command)

**Step 1: Write failing tests**

```go
package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/divyekant/carto/internal/config"
)

func newRootWithInit(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("pretty", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	root.PersistentFlags().BoolP("verbose", "v", false, "")
	root.PersistentFlags().String("log-file", "", "")
	root.PersistentFlags().String("profile", "", "")
	root.AddCommand(initCmd())
	return root
}

func TestInitCmd_NonInteractive_WritesConfig(t *testing.T) {
	withCleanEnv(t)
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	origPath := config.ConfigPath
	config.ConfigPath = cfgPath
	t.Cleanup(func() { config.ConfigPath = origPath })

	root := newRootWithInit(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init", "--non-interactive",
		"--llm-provider", "anthropic",
		"--api-key", "sk-ant-test123",
		"--memories-url", "http://localhost:8900",
	})

	err := root.Execute()
	if err != nil {
		t.Fatalf("init --non-interactive failed: %v", err)
	}

	// Verify config file was written.
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		t.Error("config file was not created")
	}
}

func TestInitCmd_NonInteractive_MissingRequiredFlag_Errors(t *testing.T) {
	withCleanEnv(t)

	root := newRootWithInit(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"init", "--non-interactive"})
	// No --api-key provided

	err := root.Execute()
	if err == nil {
		t.Error("expected error for missing required flags")
	}
}
```

**Step 2: Write `cmd_init.go`**

Implement the init command with:
- `--non-interactive` flag
- `--llm-provider`, `--api-key`, `--memories-url`, `--memories-key`, `--projects-dir` flags
- Interactive mode: prompt for each value using `fmt.Fprintf` + `fmt.Fscanln`
- Non-interactive: validate all required flags present, fail with `CONFIG_ERROR` if missing
- Write config using `config.Save()`
- Run doctor checks at the end

**Step 3: Register in `main.go`**

Add: `root.AddCommand(initCmd())`

**Step 4: Run tests**

Run: `cd /Users/divyekant/Projects/carto/go && go test ./cmd/carto/ -run TestInitCmd -v`
Expected: PASS

**Step 5: Commit**

```bash
git add go/cmd/carto/cmd_init.go go/cmd/carto/cmd_init_test.go go/cmd/carto/main.go
git commit -m "feat(cli): add init command — interactive wizard + --non-interactive mode"
```

---

### Task 11: New command — `carto export`

**Files:**
- Create: `go/cmd/carto/cmd_export.go`
- Create: `go/cmd/carto/cmd_export_test.go`
- Modify: `go/cmd/carto/main.go`

**Step 1: Write failing tests**

Test that export with `--json` returns an envelope, and test that export without `--json` streams NDJSON.

**Step 2: Write `cmd_export.go`**

- Flags: `--project` (required), `--layer` (optional: atoms, wiring, zones, blueprint, patterns)
- Queries Memories for all entries with source prefix `carto/{project}/`
- If `--layer` specified, filter to `layer:{layer}`
- Default (no `--json`): stream NDJSON to stdout, one memory per line
- With `--json`: output envelope `{"exported": N, "project": "name"}`

**Step 3: Register, test, commit**

```bash
git commit -m "feat(cli): add export command — NDJSON streaming of index data"
```

---

### Task 12: New command — `carto import`

**Files:**
- Create: `go/cmd/carto/cmd_import.go`
- Create: `go/cmd/carto/cmd_import_test.go`
- Modify: `go/cmd/carto/main.go`

**Step 1: Write failing tests**

Test add strategy (reads NDJSON from stdin, adds to Memories). Test replace strategy requires `--yes`.

**Step 2: Write `cmd_import.go`**

- Flags: `--project` (required), `--strategy` (add|replace, default: add), stdin input
- Reads NDJSON from stdin (one memory per line)
- `replace` strategy: deletes existing entries with source prefix, then imports. Requires `confirmAction()`.
- Envelope: `{"imported": N, "project": "name", "strategy": "add"}`

**Step 3: Register, test, commit**

```bash
git commit -m "feat(cli): add import command — NDJSON ingestion with add/replace strategies"
```

---

### Task 13: New command — `carto logs`

**Files:**
- Create: `go/cmd/carto/cmd_logs.go`
- Create: `go/cmd/carto/cmd_logs_test.go`
- Modify: `go/cmd/carto/main.go`

**Step 1: Write failing tests**

Test: no audit log configured → CONFIG_ERROR. Test: filter by command. Test: `--last N`.

**Step 2: Write `cmd_logs.go`**

- Flags: `--follow`/`-f`, `--last`/`-n` (int, default 20), `--command` (string), `--result` (ok|error)
- Reads NDJSON audit log file (from `--log-file` flag or `CARTO_AUDIT_LOG` env)
- `--follow`: tail the file (use `os.File` seek + poll)
- Filter entries by command name and result
- Envelope: `{"entries": [...], "total": N}`

**Step 3: Register, test, commit**

```bash
git commit -m "feat(cli): add logs command — query and tail audit log"
```

---

### Task 14: New command — `carto upgrade`

**Files:**
- Create: `go/cmd/carto/cmd_upgrade.go`
- Create: `go/cmd/carto/cmd_upgrade_test.go`
- Modify: `go/cmd/carto/main.go`

**Step 1: Write failing tests**

Test: `--check` returns version comparison data. Test: version comparison logic (current >= latest → no update).

**Step 2: Write `cmd_upgrade.go`**

- Flags: `--check` (check only, don't download)
- Check: HTTP GET to GitHub releases API, parse latest tag, compare with `version` variable
- Upgrade: download binary, verify checksum, replace in-place. Requires `confirmAction()`.
- Envelope: `{"current": "1.1.0", "latest": "1.2.0", "update_available": true}`
- For unit tests: extract version comparison logic into a testable function

**Step 3: Register, test, commit**

```bash
git commit -m "feat(cli): add upgrade command — check for and install new versions"
```

---

### Task 15: Fill test coverage gaps

**Files:**
- Modify: `go/cmd/carto/cmd_modernization_test.go`
- Create or extend test files as needed

**Step 1: Add envelope tests for all existing commands**

For each command, add a test that:
1. Creates a root command with all persistent flags
2. Runs with `--json`
3. Parses the output as an envelope
4. Asserts `ok: true` and expected data structure

Cover: `auth status`, `config get`, `config path`, `doctor`, `projects list`, `modules`, `status`, `about`.

**Step 2: Add error envelope tests**

For commands with known error paths:
- `config validate` without API key → envelope with `code: CONFIG_ERROR`
- `status` on empty dir → envelope with `code: NOT_FOUND`
- `auth set-key` with bad provider → envelope with error

**Step 3: Run full test suite with race detector**

Run: `cd /Users/divyekant/Projects/carto/go && go test -race ./cmd/carto/ -v`
Expected: PASS

**Step 4: Commit**

```bash
git add go/cmd/carto/*_test.go
git commit -m "test(cli): fill coverage gaps — envelope tests for all commands"
```

---

### Task 16: Final verification and cleanup

**Step 1: Run full test suite**

Run: `cd /Users/divyekant/Projects/carto/go && go test -race ./...`
Expected: PASS

**Step 2: Run go vet**

Run: `cd /Users/divyekant/Projects/carto/go && go vet ./...`
Expected: clean

**Step 3: Build binary**

Run: `cd /Users/divyekant/Projects/carto/go && go build -o carto ./cmd/carto`
Expected: builds successfully

**Step 4: Smoke test key commands**

```bash
./carto version
./carto version --json
./carto about
./carto doctor --skip-network
./carto completions bash | head -5
echo '{"test": true}' | ./carto version  # should auto-detect JSON mode
```

**Step 5: Verify no references to old colors**

Run: `grep -rn 'cyan\|yellow' go/cmd/carto/ --include="*.go" | grep -v "_test.go" | grep -v "//"`
Expected: no results (all replaced with gold/amber)

**Step 6: Commit any cleanup**

```bash
git commit -m "chore(cli): final cleanup and verification"
```
