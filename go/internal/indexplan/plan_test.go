package indexplan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildScansWithoutSideEffects(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/plan\n\ngo 1.25\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0o644); err != nil {
		t.Fatalf("write main.go: %v", err)
	}

	plan, err := Build(dir, "plan-project", "")
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if plan.Project != "plan-project" {
		t.Fatalf("Project = %q, want plan-project", plan.Project)
	}
	if plan.Modules != 1 {
		t.Fatalf("Modules = %d, want 1", plan.Modules)
	}
	if plan.Files != 2 {
		t.Fatalf("Files = %d, want 2", plan.Files)
	}
	if plan.Languages["go"] != 1 {
		t.Fatalf("go language count = %d, want 1", plan.Languages["go"])
	}
	if _, err := os.Stat(filepath.Join(dir, ".carto")); !os.IsNotExist(err) {
		t.Fatalf("Build should not create .carto, stat err=%v", err)
	}
}

func TestFromModulesWarnsForLargeCodebase(t *testing.T) {
	modules := []Module{{Name: "large", Files: LargeFileThreshold + 1}}
	plan := FromModules("proj", "/tmp/proj", modules, LargeFileThreshold+1, 1024, map[string]int{"go": 1})
	if !plan.Large {
		t.Fatal("expected large plan")
	}
	if len(plan.Recommendations) == 0 {
		t.Fatal("expected recommendations for large plan")
	}
}

func TestBuildModuleFilterSupportsPathForDuplicateNames(t *testing.T) {
	dir := t.TempDir()
	writeFile := func(rel string) {
		t.Helper()
		path := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte("class Example {}\n"), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	writeFile("core/dao/pom.xml")
	writeFile("core/dao/src/Dao.java")
	writeFile("core/cdp/dao/pom.xml")
	writeFile("core/cdp/dao/src/CdpDao.java")

	plan, err := Build(dir, "dup", "core/dao")
	if err != nil {
		t.Fatalf("Build by path returned error: %v", err)
	}
	if plan.Modules != 1 || plan.TopModules[0].Path != "core/dao" || plan.Files != 2 {
		t.Fatalf("plan = %+v, want only core/dao with 2 files", plan)
	}

	_, err = Build(dir, "dup", "dao")
	if err == nil {
		t.Fatal("expected ambiguous module name to fail")
	}
	if !strings.Contains(err.Error(), "ambiguous") || !strings.Contains(err.Error(), "core/dao") || !strings.Contains(err.Error(), "core/cdp/dao") {
		t.Fatalf("ambiguous error = %v, want both module paths", err)
	}
}
