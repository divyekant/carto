package main

import (
	"os"
	"path/filepath"
	"testing"
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
