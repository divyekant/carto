package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/divyekant/carto/internal/indexplan"
)

func TestIndexDryRunScansWithoutIndexing(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LLM_PROVIDER", "anthropic")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/dryrun\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := indexCmd()
	out, err := execCmd(t, cmd, []string{dir, "--dry-run"})
	if err != nil {
		t.Fatalf("index --dry-run returned error: %v", err)
	}

	for _, want := range []string{"Index plan", "modules: 1", "files:", "LLM calls: none"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, out)
		}
	}
	if !strings.Contains(out, "2") {
		t.Fatalf("dry-run output should include file count 2:\n%s", out)
	}

	if _, err := os.Stat(filepath.Join(dir, ".carto")); !os.IsNotExist(err) {
		t.Fatalf("dry-run should not create .carto, stat err=%v", err)
	}
}

func TestIndexMaxFilesRequiresRepairMissingAtomsBeforeProviderSetup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("LLM_PROVIDER", "anthropic")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("ANTHROPIC_API_KEY", "")

	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/maxfiles\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := indexCmd()
	_, err := execCmd(t, cmd, []string{dir, "--max-files", "1"})
	if err == nil {
		t.Fatal("expected --max-files without --repair-missing-atoms to fail")
	}
	if !strings.Contains(err.Error(), "--max-files requires --repair-missing-atoms") {
		t.Fatalf("error = %v, want max-files repair mode validation", err)
	}
}

func TestBuildIndexPlanWarnsForLargeCodebase(t *testing.T) {
	modules := make([]indexplan.Module, 0, indexplan.LargeModuleThreshold+1)
	for i := 0; i < indexplan.LargeModuleThreshold+1; i++ {
		modules = append(modules, indexplan.Module{Name: "mod", Files: 1})
	}

	plan := indexplan.FromModules("proj", "/tmp/proj", modules, indexplan.LargeFileThreshold+1, 1024, map[string]int{"go": 1})

	if !plan.Large {
		t.Fatal("expected large plan")
	}
	if len(plan.Recommendations) == 0 {
		t.Fatal("expected recommendations for large plan")
	}
	if !strings.Contains(strings.Join(plan.Recommendations, "\n"), "foreground") {
		t.Fatalf("recommendations should mention foreground runs:\n%v", plan.Recommendations)
	}
}
