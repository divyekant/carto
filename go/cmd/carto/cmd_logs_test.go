package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =========================================================================
// Helpers
// =========================================================================

// newRootWithLogs creates a minimal root command with the persistent flags
// that logsCmd (and its helpers like isJSONMode) depend on, then attaches
// the logs subcommand.
func newRootWithLogs(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("pretty", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	root.PersistentFlags().String("log-file", "", "")
	root.AddCommand(logsCmd())
	return root
}

// sampleAuditLines returns NDJSON lines for test fixtures.
func sampleAuditLines() string {
	lines := []string{
		`{"ts":"2025-06-01T10:00:00Z","level":"audit","command":"carto index","args":["myproject"],"result":"ok"}`,
		`{"ts":"2025-06-01T10:01:00Z","level":"audit","command":"carto query","args":["what is main"],"result":"ok"}`,
		`{"ts":"2025-06-01T10:02:00Z","level":"audit","command":"carto index","args":["other"],"result":"error","error":"LLM timeout"}`,
		`{"ts":"2025-06-01T10:03:00Z","level":"audit","command":"carto status","args":[],"result":"ok"}`,
		`{"ts":"2025-06-01T10:04:00Z","level":"audit","command":"carto config get","args":[],"result":"ok"}`,
	}
	return strings.Join(lines, "\n") + "\n"
}

// writeTempLog writes NDJSON content to a temp file and returns its path.
func writeTempLog(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "audit.ndjson")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp log: %v", err)
	}
	return p
}

// =========================================================================
// Tests
// =========================================================================

func TestLogsCmd_NoLogConfigured_Error(t *testing.T) {
	// Ensure CARTO_AUDIT_LOG is not set.
	t.Setenv("CARTO_AUDIT_LOG", "")

	root := newRootWithLogs(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	// No --log-file, no env var → should return CONFIG_ERROR.
	root.SetArgs([]string{"logs"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when no audit log is configured, got nil")
	}
	if !strings.Contains(err.Error(), "no audit log configured") {
		t.Errorf("expected 'no audit log configured' error, got: %v", err)
	}
}

func TestLogsCmd_MissingFile_Error(t *testing.T) {
	root := newRootWithLogs(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"logs", "--log-file", "/tmp/nonexistent-carto-test-audit.ndjson"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for missing log file, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestLogsCmd_DisplaysEntries(t *testing.T) {
	logPath := writeTempLog(t, sampleAuditLines())

	root := newRootWithLogs(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"logs", "--log-file", logPath, "--pretty"})
	if err := root.Execute(); err != nil {
		t.Fatalf("logs command failed: %v", err)
	}

	out := buf.String()
	// Should display all 5 entries.
	if !strings.Contains(out, "5 entries") {
		t.Errorf("expected '5 entries' in output, got:\n%s", out)
	}
	// Should show command names.
	if !strings.Contains(out, "carto index") {
		t.Errorf("expected 'carto index' in output, got:\n%s", out)
	}
	if !strings.Contains(out, "carto query") {
		t.Errorf("expected 'carto query' in output, got:\n%s", out)
	}
	// Should show the error message for the failed entry.
	if !strings.Contains(out, "LLM timeout") {
		t.Errorf("expected 'LLM timeout' error in output, got:\n%s", out)
	}
}

func TestLogsCmd_FilterByCommand(t *testing.T) {
	logPath := writeTempLog(t, sampleAuditLines())

	root := newRootWithLogs(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"logs", "--log-file", logPath, "--command", "index", "--pretty"})
	if err := root.Execute(); err != nil {
		t.Fatalf("logs --command failed: %v", err)
	}

	out := buf.String()
	// "carto index" appears twice in the sample data (ok + error).
	if !strings.Contains(out, "2 entries") {
		t.Errorf("expected '2 entries' for --command index, got:\n%s", out)
	}
	// Should not contain "carto query" or "carto status".
	if strings.Contains(out, "carto query") {
		t.Errorf("--command index should not include 'carto query', got:\n%s", out)
	}
}

func TestLogsCmd_FilterByResult(t *testing.T) {
	logPath := writeTempLog(t, sampleAuditLines())

	root := newRootWithLogs(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"logs", "--log-file", logPath, "--result", "error", "--pretty"})
	if err := root.Execute(); err != nil {
		t.Fatalf("logs --result failed: %v", err)
	}

	out := buf.String()
	// Only 1 error entry in the sample data.
	if !strings.Contains(out, "1 entries") {
		t.Errorf("expected '1 entries' for --result error, got:\n%s", out)
	}
	if !strings.Contains(out, "LLM timeout") {
		t.Errorf("expected error entry with 'LLM timeout', got:\n%s", out)
	}
}

func TestLogsCmd_LastN(t *testing.T) {
	logPath := writeTempLog(t, sampleAuditLines())

	root := newRootWithLogs(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"logs", "--log-file", logPath, "--last", "2", "--pretty"})
	if err := root.Execute(); err != nil {
		t.Fatalf("logs --last failed: %v", err)
	}

	out := buf.String()
	// Should show only the last 2 entries.
	if !strings.Contains(out, "2 entries") {
		t.Errorf("expected '2 entries' for --last 2, got:\n%s", out)
	}
	// The last 2 entries are "carto status" and "carto config get".
	if !strings.Contains(out, "carto status") {
		t.Errorf("expected 'carto status' (4th entry) in last 2, got:\n%s", out)
	}
	if !strings.Contains(out, "carto config get") {
		t.Errorf("expected 'carto config get' (5th entry) in last 2, got:\n%s", out)
	}
}

func TestLogsCmd_JSONEnvelope(t *testing.T) {
	logPath := writeTempLog(t, sampleAuditLines())

	root := newRootWithLogs(t)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	root.SetArgs([]string{"logs", "--log-file", logPath, "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("logs --json failed: %v", err)
	}

	// JSON envelope is written to stdout.
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Entries []auditEvent `json:"entries"`
			Total   int          `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("logs --json not valid JSON: %v\nraw stdout: %s\nraw stderr: %s",
			err, stdout.String(), stderr.String())
	}
	if !env.OK {
		t.Error("expected ok:true in JSON envelope")
	}
	if env.Data.Total != 5 {
		t.Errorf("expected total 5, got %d", env.Data.Total)
	}
	if len(env.Data.Entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(env.Data.Entries))
	}
}

func TestLogsCmd_JSONEnvelope_WithFilter(t *testing.T) {
	logPath := writeTempLog(t, sampleAuditLines())

	root := newRootWithLogs(t)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)

	root.SetArgs([]string{"logs", "--log-file", logPath, "--json", "--result", "ok"})
	if err := root.Execute(); err != nil {
		t.Fatalf("logs --json --result ok failed: %v", err)
	}

	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			Entries []auditEvent `json:"entries"`
			Total   int          `json:"total"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("JSON parse failed: %v\nraw: %s", err, stdout.String())
	}
	// 4 "ok" entries in sample data.
	if env.Data.Total != 4 {
		t.Errorf("expected total 4 ok entries, got %d", env.Data.Total)
	}
	for _, e := range env.Data.Entries {
		if e.Result != "ok" {
			t.Errorf("expected all entries to be 'ok', got %q", e.Result)
		}
	}
}

func TestLogsCmd_SkipsMalformedLines(t *testing.T) {
	content := `{"ts":"2025-06-01T10:00:00Z","level":"audit","command":"carto index","result":"ok"}
this is not json
{"ts":"2025-06-01T10:01:00Z","level":"audit","command":"carto query","result":"ok"}
`
	logPath := writeTempLog(t, content)

	root := newRootWithLogs(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"logs", "--log-file", logPath, "--pretty"})
	if err := root.Execute(); err != nil {
		t.Fatalf("logs command failed: %v", err)
	}

	out := buf.String()
	// Should show 2 valid entries, silently skipping the malformed line.
	if !strings.Contains(out, "2 entries") {
		t.Errorf("expected '2 entries' (skipping malformed line), got:\n%s", out)
	}
}

func TestLogsCmd_EmptyLog(t *testing.T) {
	logPath := writeTempLog(t, "")

	root := newRootWithLogs(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	root.SetArgs([]string{"logs", "--log-file", logPath, "--pretty"})
	if err := root.Execute(); err != nil {
		t.Fatalf("logs with empty file failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "0 entries") {
		t.Errorf("expected '0 entries' for empty log, got:\n%s", out)
	}
}

func TestLogsCmd_EnvVarFallback(t *testing.T) {
	logPath := writeTempLog(t, sampleAuditLines())
	t.Setenv("CARTO_AUDIT_LOG", logPath)

	root := newRootWithLogs(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)

	// No --log-file flag; should pick up CARTO_AUDIT_LOG from env.
	root.SetArgs([]string{"logs", "--pretty"})
	if err := root.Execute(); err != nil {
		t.Fatalf("logs with CARTO_AUDIT_LOG env failed: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "5 entries") {
		t.Errorf("expected '5 entries' via env var, got:\n%s", out)
	}
}
