package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunPatterns_WritesFiles(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)

	cartoDir := filepath.Join(dir, ".carto")
	os.MkdirAll(cartoDir, 0o755)

	cmd := patternsCmd()
	cmd.SetArgs([]string{dir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("patterns command failed: %v", err)
	}

	claudePath := filepath.Join(dir, "CLAUDE.md")
	if _, err := os.Stat(claudePath); os.IsNotExist(err) {
		t.Error("CLAUDE.md was not created")
	}

	cursorPath := filepath.Join(dir, ".cursorrules")
	if _, err := os.Stat(cursorPath); os.IsNotExist(err) {
		t.Error(".cursorrules was not created")
	}
}

func TestCLI_HelpExitsClean(t *testing.T) {
	root := &cobra.Command{Use: "carto", Version: version}
	root.AddCommand(indexCmd())
	root.AddCommand(queryCmd())
	root.AddCommand(modulesCmd())
	root.AddCommand(patternsCmd())
	root.AddCommand(statusCmd())

	root.SetArgs([]string{"--help"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("--help should not error: %v", err)
	}
}

func TestCLI_ModulesCommand(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/test\n\ngo 1.21\n"), 0o644)
	os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644)

	cmd := modulesCmd()
	cmd.SetArgs([]string{dir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("modules command failed: %v", err)
	}
}

func TestCLI_StatusNoIndex(t *testing.T) {
	dir := t.TempDir()
	cmd := statusCmd()
	cmd.SetArgs([]string{dir})
	err := cmd.Execute()
	if err != nil {
		t.Fatalf("status command failed: %v", err)
	}
}

func TestCLI_IndexNoArgs(t *testing.T) {
	cmd := indexCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("index with no args should error")
	}
}
