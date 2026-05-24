package main

import (
	"testing"

	"github.com/divyekant/carto/internal/atoms"
	"github.com/divyekant/carto/internal/scanner"
)

func TestWritebackCmd_Flags(t *testing.T) {
	cmd := writebackCmd()
	if cmd.Use != "writeback <path>" {
		t.Errorf("unexpected use: %s", cmd.Use)
	}
	for _, f := range []string{"file", "module", "project"} {
		if cmd.Flags().Lookup(f) == nil {
			t.Errorf("missing flag: %s", f)
		}
	}
}

func TestWritebackCmd_RequiresArg(t *testing.T) {
	root := testRoot(writebackCmd())
	root.SetArgs([]string{"writeback"})
	err := root.Execute()
	if err == nil {
		t.Error("expected error when no path argument given")
	}
}

func TestFindModule_RootModule(t *testing.T) {
	modules := []scanner.Module{
		{Name: "root", Path: "/project", RelPath: ".", Files: []string{"main.go"}},
	}
	got := findModule("main.go", modules)
	if got != "root" {
		t.Errorf("findModule() = %q, want %q", got, "root")
	}
}

func TestFindModule_SubModule(t *testing.T) {
	modules := []scanner.Module{
		{Name: "root", Path: "/project", RelPath: "."},
		{Name: "web", Path: "/project/web", RelPath: "web", Files: []string{"web/app.tsx"}},
	}
	got := findModule("web/app.tsx", modules)
	if got != "web" {
		t.Errorf("findModule() = %q, want %q", got, "web")
	}
}

func TestFindModule_LongestPrefixWins(t *testing.T) {
	modules := []scanner.Module{
		{Name: "internal", Path: "/project/internal", RelPath: "internal"},
		{Name: "storage", Path: "/project/internal/storage", RelPath: "internal/storage"},
	}
	got := findModule("internal/storage/memories.go", modules)
	if got != "storage" {
		t.Errorf("findModule() = %q, want %q", got, "storage")
	}
}

func TestFindModule_NoModules(t *testing.T) {
	got := findModule("main.go", nil)
	if got != "" {
		t.Errorf("findModule() = %q, want empty string", got)
	}
}

func TestBuildAtomMeta(t *testing.T) {
	a := &atoms.Atom{
		Name:      "handleAuth",
		Kind:      "function",
		Language:  "go",
		Module:    "auth",
		StartLine: 15,
		EndLine:   42,
	}
	meta := buildAtomMeta(a, "internal/auth/handler.go")

	// Only the 5 fields the pipeline stores (no start_line/end_line).
	checks := map[string]any{
		"name":     "handleAuth",
		"kind":     "function",
		"language": "go",
		"module":   "auth",
		"filepath": "internal/auth/handler.go",
	}
	for k, want := range checks {
		got, ok := meta[k]
		if !ok {
			t.Errorf("missing key %q", k)
			continue
		}
		if got != want {
			t.Errorf("meta[%q] = %v, want %v", k, got, want)
		}
	}
	// Verify start_line and end_line are NOT present.
	for _, forbidden := range []string{"start_line", "end_line"} {
		if _, ok := meta[forbidden]; ok {
			t.Errorf("meta should not contain %q", forbidden)
		}
	}
}
