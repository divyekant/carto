package main

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/spf13/cobra"
)

// =========================================================================
// Helpers for output tests
// =========================================================================

// newTestRoot creates a root command with the --json, --pretty, and --yes
// persistent flags, matching the real root setup. This enables tests to
// exercise isJSONMode / isYes without depending on main.go's root command.
func newTestRoot() *cobra.Command {
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "Output machine-readable JSON")
	root.PersistentFlags().Bool("pretty", false, "Force human-readable output")
	root.PersistentFlags().Bool("yes", false, "Skip confirmation prompts")
	return root
}

// =========================================================================
// isJSONMode
// =========================================================================

func TestIsJSONMode_ExplicitFlag(t *testing.T) {
	root := newTestRoot()
	child := &cobra.Command{Use: "sub", Run: func(_ *cobra.Command, _ []string) {}}
	root.AddCommand(child)

	root.SetArgs([]string{"sub", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !isJSONMode(child) {
		t.Error("isJSONMode should return true when --json flag is set")
	}
}

func TestIsJSONMode_PrettyOverrides(t *testing.T) {
	root := newTestRoot()
	child := &cobra.Command{Use: "sub", Run: func(_ *cobra.Command, _ []string) {}}
	root.AddCommand(child)

	root.SetArgs([]string{"sub", "--json", "--pretty"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if isJSONMode(child) {
		t.Error("isJSONMode should return false when --pretty overrides --json")
	}
}

// =========================================================================
// writeEnvelope (success path)
// =========================================================================

func TestWriteEnvelope_Success(t *testing.T) {
	root := newTestRoot()
	var stdout, stderr bytes.Buffer

	child := &cobra.Command{
		Use: "sub",
		Run: func(cmd *cobra.Command, _ []string) {
			writeEnvelope(cmd, map[string]string{"foo": "bar"}, nil)
		},
	}
	root.AddCommand(child)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	child.SetOut(&stdout)
	child.SetErr(&stderr)
	root.SetArgs([]string{"sub", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var env struct {
		OK   bool            `json:"ok"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("parse envelope: %v\nraw: %s", err, stdout.String())
	}
	if !env.OK {
		t.Error("expected ok:true")
	}
	if env.Data == nil {
		t.Error("expected non-nil data field")
	}
}

// =========================================================================
// writeEnvelope (error path)
// =========================================================================

func TestWriteEnvelope_Error(t *testing.T) {
	root := newTestRoot()
	var stdout, stderr bytes.Buffer

	child := &cobra.Command{
		Use: "sub",
		Run: func(cmd *cobra.Command, _ []string) {
			writeEnvelope(cmd, nil, newConnectionError("connection refused"))
		},
	}
	root.AddCommand(child)
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	child.SetOut(&stdout)
	child.SetErr(&stderr)
	root.SetArgs([]string{"sub", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var env struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		Code  string `json:"code"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &env); err != nil {
		t.Fatalf("parse error envelope: %v\nraw: %s", err, stderr.String())
	}
	if env.OK {
		t.Error("expected ok:false")
	}
	if env.Error == "" {
		t.Error("expected non-empty error message")
	}
	if env.Code != ErrCodeConnection {
		t.Errorf("expected code %q, got %q", ErrCodeConnection, env.Code)
	}
}

// =========================================================================
// readInputOrStdin
// =========================================================================

func TestReadInputOrStdin_Argument(t *testing.T) {
	data, err := readInputOrStdin("hello world")
	if err != nil {
		t.Fatalf("readInputOrStdin: %v", err)
	}
	if string(data) != "hello world" {
		t.Errorf("expected %q, got %q", "hello world", string(data))
	}
}

func TestReadInputOrStdin_EmptyArg(t *testing.T) {
	// With an empty arg and a real terminal on stdin, we should get back
	// an empty byte slice (the function returns []byte(""), nil).
	data, err := readInputOrStdin("")
	if err != nil {
		t.Fatalf("readInputOrStdin: %v", err)
	}
	if string(data) != "" {
		t.Errorf("expected empty string, got %q", string(data))
	}
}

// =========================================================================
// writeEnvelopeHuman — humanFn invocation
// =========================================================================

func TestWriteEnvelopeHuman_CallsHumanFn(t *testing.T) {
	root := newTestRoot()
	var stdout bytes.Buffer
	called := false

	child := &cobra.Command{
		Use: "sub",
		Run: func(cmd *cobra.Command, _ []string) {
			writeEnvelopeHuman(cmd, map[string]string{"key": "val"}, nil, func() {
				called = true
			})
		},
	}
	root.AddCommand(child)
	root.SetOut(&stdout)
	child.SetOut(&stdout)

	// Execute with --pretty to force human mode (in tests, stdout is not
	// a terminal so the TTY fallback would otherwise choose JSON mode).
	root.SetArgs([]string{"sub", "--pretty"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if !called {
		t.Error("humanFn should have been called in non-JSON mode")
	}
}

func TestWriteEnvelopeHuman_SkipsHumanFnInJSON(t *testing.T) {
	root := newTestRoot()
	var stdout bytes.Buffer
	called := false

	child := &cobra.Command{
		Use: "sub",
		Run: func(cmd *cobra.Command, _ []string) {
			writeEnvelopeHuman(cmd, map[string]string{"key": "val"}, nil, func() {
				called = true
			})
		},
	}
	root.AddCommand(child)
	root.SetOut(&stdout)
	child.SetOut(&stdout)

	root.SetArgs([]string{"sub", "--json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if called {
		t.Error("humanFn should NOT have been called in JSON mode")
	}
}
