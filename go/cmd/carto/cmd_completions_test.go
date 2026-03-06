package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func newRootWithCompletions(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("pretty", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	root.AddCommand(completionsCmd())
	// Add a dummy subcommand so completions have something to complete
	root.AddCommand(&cobra.Command{Use: "version", Short: "Show version"})
	return root
}

func TestCompletionsCmd_Bash(t *testing.T) {
	root := newRootWithCompletions(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completions", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if len(out) == 0 {
		t.Error("expected non-empty bash completion output")
	}
	if !strings.Contains(out, "carto") {
		t.Error("expected completion script to contain 'carto'")
	}
}

func TestCompletionsCmd_Zsh(t *testing.T) {
	root := newRootWithCompletions(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completions", "zsh"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty zsh completion output")
	}
}

func TestCompletionsCmd_Fish(t *testing.T) {
	root := newRootWithCompletions(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"completions", "fish"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty fish completion output")
	}
}

func TestCompletionsCmd_InvalidShell(t *testing.T) {
	root := newRootWithCompletions(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"completions", "invalid"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error for unsupported shell")
	}
}

func TestCompletionsCmd_NoArgs(t *testing.T) {
	root := newRootWithCompletions(t)
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"completions"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no shell specified")
	}
}
