package scanner

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// helper: create a file with optional content
func createFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to create file %s: %v", path, err)
	}
}

// --- Language Detection Tests ---

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		filename string
		want     string
	}{
		{"main.go", "go"},
		{"app.js", "javascript"},
		{"component.jsx", "javascript"},
		{"index.ts", "typescript"},
		{"page.tsx", "typescript"},
		{"script.py", "python"},
		{"Main.java", "java"},
		{"lib.rs", "rust"},
		{"app.rb", "ruby"},
		{"styles.css", "css"},
		{"index.html", "html"},
		{"config.yaml", "yaml"},
		{"config.yml", "yaml"},
		{"data.json", "json"},
		{"Cargo.toml", "toml"},
		{"schema.sql", "sql"},
		{"app.swift", "swift"},
		{"lib.c", "c"},
		{"lib.cpp", "cpp"},
		{"Program.cs", "csharp"},
		{"app.vue", "vue"},
		{"page.svelte", "svelte"},
		{"query.graphql", "graphql"},
		{"Dockerfile", "dockerfile"},
		{"Makefile", "makefile"},
		{"Rakefile", "ruby"},
		{"unknown.xyz", ""},
		{"noextension", ""},
		{"style.scss", "scss"},
		{"style.less", "less"},
		{"main.kt", "kotlin"},
		{"app.dart", "dart"},
		{"main.zig", "zig"},
		{"Lib.hs", "haskell"},
		{"core.clj", "clojure"},
		{"main.tf", "terraform"},
		{"api.proto", "protobuf"},
		{"module.mjs", "javascript"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := DetectLanguage(tt.filename)
			if got != tt.want {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

// --- Skip Directories Tests ---

func TestScan_SkipsCommonDirs(t *testing.T) {
	root := t.TempDir()

	// Create files in normal directories
	createFile(t, filepath.Join(root, "src", "main.go"), "package main")

	// Create files in directories that should be skipped
	createFile(t, filepath.Join(root, "node_modules", "pkg", "index.js"), "module.exports = {}")
	createFile(t, filepath.Join(root, ".git", "config"), "[core]")
	createFile(t, filepath.Join(root, "__pycache__", "mod.pyc"), "bytecode")
	createFile(t, filepath.Join(root, "dist", "bundle.js"), "bundled")
	createFile(t, filepath.Join(root, ".carto", "index.json"), "{}")
	createFile(t, filepath.Join(root, ".next", "build-manifest.json"), "{}")
	createFile(t, filepath.Join(root, ".cache", "data"), "cached")

	result, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	for _, f := range result.Files {
		for dir := range skipDirs {
			if containsPathComponent(f.RelPath, dir) {
				t.Errorf("found file from skipped dir %q: %s", dir, f.RelPath)
			}
		}
	}

	// Verify the normal file IS found
	found := false
	for _, f := range result.Files {
		if f.RelPath == filepath.Join("src", "main.go") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find src/main.go but it was missing")
	}
}

// --- Lock Files Tests ---

func TestScan_SkipsLockFiles(t *testing.T) {
	root := t.TempDir()

	createFile(t, filepath.Join(root, "main.go"), "package main")
	createFile(t, filepath.Join(root, "package-lock.json"), "{}")
	createFile(t, filepath.Join(root, "yarn.lock"), "")
	createFile(t, filepath.Join(root, "pnpm-lock.yaml"), "")
	createFile(t, filepath.Join(root, "Gemfile.lock"), "")
	createFile(t, filepath.Join(root, "Cargo.lock"), "")
	createFile(t, filepath.Join(root, "go.sum"), "")
	createFile(t, filepath.Join(root, "composer.lock"), "")

	result, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	lockFileNames := map[string]bool{}
	for name := range lockFiles {
		lockFileNames[name] = true
	}

	for _, f := range result.Files {
		base := filepath.Base(f.RelPath)
		if lockFileNames[base] {
			t.Errorf("lock file should have been skipped: %s", f.RelPath)
		}
	}

	// Verify non-lock file IS found
	found := false
	for _, f := range result.Files {
		if f.RelPath == "main.go" {
			found = true
		}
	}
	if !found {
		t.Error("expected to find main.go")
	}
}

// --- Gitignore Tests ---

func TestScan_RespectsGitignore(t *testing.T) {
	root := t.TempDir()

	// Create a .gitignore
	createFile(t, filepath.Join(root, ".gitignore"), "*.log\nsecrets/\ntmp/*.txt\n!important.log\n")

	// Files that should be ignored
	createFile(t, filepath.Join(root, "debug.log"), "log data")
	createFile(t, filepath.Join(root, "error.log"), "error data")
	createFile(t, filepath.Join(root, "secrets", "key.pem"), "secret key")
	createFile(t, filepath.Join(root, "tmp", "notes.txt"), "temp notes")

	// Files that should NOT be ignored
	createFile(t, filepath.Join(root, "main.go"), "package main")
	createFile(t, filepath.Join(root, "important.log"), "keep this")
	createFile(t, filepath.Join(root, "tmp", "data.csv"), "1,2,3")

	result, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	relPaths := map[string]bool{}
	for _, f := range result.Files {
		relPaths[f.RelPath] = true
	}

	// Should be ignored
	shouldBeIgnored := []string{"debug.log", "error.log", filepath.Join("secrets", "key.pem"), filepath.Join("tmp", "notes.txt")}
	for _, p := range shouldBeIgnored {
		if relPaths[p] {
			t.Errorf("expected %q to be ignored by .gitignore", p)
		}
	}

	// Should NOT be ignored
	shouldBePresent := []string{"main.go", "important.log", filepath.Join("tmp", "data.csv")}
	for _, p := range shouldBePresent {
		if !relPaths[p] {
			t.Errorf("expected %q to be present (not ignored)", p)
		}
	}
}

func TestScan_GitignoreDirectoryPattern(t *testing.T) {
	root := t.TempDir()

	createFile(t, filepath.Join(root, ".gitignore"), "logs/\n")
	createFile(t, filepath.Join(root, "logs", "app.log"), "log")
	createFile(t, filepath.Join(root, "logs", "error.log"), "error")
	createFile(t, filepath.Join(root, "src", "main.go"), "package main")

	result, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	for _, f := range result.Files {
		if containsPathComponent(f.RelPath, "logs") {
			t.Errorf("file in ignored 'logs/' dir should be skipped: %s", f.RelPath)
		}
	}
}

// --- Module Detection Tests ---

func TestDetectModules_GoModule(t *testing.T) {
	root := t.TempDir()

	createFile(t, filepath.Join(root, "go.mod"), "module github.com/example/myapp\n\ngo 1.21\n")
	createFile(t, filepath.Join(root, "main.go"), "package main")
	createFile(t, filepath.Join(root, "pkg", "lib.go"), "package pkg")

	files := []FileInfo{
		{Path: filepath.Join(root, "go.mod"), RelPath: "go.mod", Language: ""},
		{Path: filepath.Join(root, "main.go"), RelPath: "main.go", Language: "go"},
		{Path: filepath.Join(root, "pkg", "lib.go"), RelPath: filepath.Join("pkg", "lib.go"), Language: "go"},
	}

	modules := DetectModules(root, files)
	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(modules))
	}

	m := modules[0]
	if m.Name != "github.com/example/myapp" {
		t.Errorf("module name = %q, want %q", m.Name, "github.com/example/myapp")
	}
	if m.Type != "go" {
		t.Errorf("module type = %q, want %q", m.Type, "go")
	}
	if m.Manifest != "go.mod" {
		t.Errorf("manifest = %q, want %q", m.Manifest, "go.mod")
	}
	if len(m.Files) != 3 {
		t.Errorf("expected 3 files in module, got %d", len(m.Files))
	}
}

func TestDetectModules_NodeModule(t *testing.T) {
	root := t.TempDir()

	createFile(t, filepath.Join(root, "package.json"), `{"name": "@scope/my-app", "version": "1.0.0"}`)
	createFile(t, filepath.Join(root, "index.js"), "console.log('hello')")

	files := []FileInfo{
		{Path: filepath.Join(root, "package.json"), RelPath: "package.json", Language: "json"},
		{Path: filepath.Join(root, "index.js"), RelPath: "index.js", Language: "javascript"},
	}

	modules := DetectModules(root, files)
	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(modules))
	}

	m := modules[0]
	if m.Name != "@scope/my-app" {
		t.Errorf("module name = %q, want %q", m.Name, "@scope/my-app")
	}
	if m.Type != "node" {
		t.Errorf("module type = %q, want %q", m.Type, "node")
	}
}

func TestDetectModules_MultipleModules(t *testing.T) {
	root := t.TempDir()

	// Root Go module
	createFile(t, filepath.Join(root, "go.mod"), "module github.com/example/root\n\ngo 1.21\n")
	createFile(t, filepath.Join(root, "main.go"), "package main")

	// Nested Node module
	createFile(t, filepath.Join(root, "frontend", "package.json"), `{"name": "frontend"}`)
	createFile(t, filepath.Join(root, "frontend", "index.js"), "export default {}")

	files := []FileInfo{
		{Path: filepath.Join(root, "go.mod"), RelPath: "go.mod"},
		{Path: filepath.Join(root, "main.go"), RelPath: "main.go", Language: "go"},
		{Path: filepath.Join(root, "frontend", "package.json"), RelPath: filepath.Join("frontend", "package.json"), Language: "json"},
		{Path: filepath.Join(root, "frontend", "index.js"), RelPath: filepath.Join("frontend", "index.js"), Language: "javascript"},
	}

	modules := DetectModules(root, files)
	if len(modules) != 2 {
		t.Fatalf("expected 2 modules, got %d", len(modules))
	}

	// Sort by type for deterministic testing
	sort.Slice(modules, func(i, j int) bool {
		return modules[i].Type < modules[j].Type
	})

	goMod := modules[0]
	nodeMod := modules[1]

	if goMod.Type != "go" {
		t.Errorf("expected go module, got %q", goMod.Type)
	}
	if nodeMod.Type != "node" {
		t.Errorf("expected node module, got %q", nodeMod.Type)
	}
	if nodeMod.Name != "frontend" {
		t.Errorf("node module name = %q, want %q", nodeMod.Name, "frontend")
	}
}

func TestDetectModules_NoManifest(t *testing.T) {
	root := t.TempDir()

	createFile(t, filepath.Join(root, "main.py"), "print('hello')")
	createFile(t, filepath.Join(root, "lib.py"), "def helper(): pass")

	files := []FileInfo{
		{Path: filepath.Join(root, "main.py"), RelPath: "main.py", Language: "python"},
		{Path: filepath.Join(root, "lib.py"), RelPath: "lib.py", Language: "python"},
	}

	modules := DetectModules(root, files)
	if len(modules) != 1 {
		t.Fatalf("expected 1 fallback module, got %d", len(modules))
	}

	m := modules[0]
	if m.Type != "unknown" {
		t.Errorf("module type = %q, want %q", m.Type, "unknown")
	}
	if m.Manifest != "" {
		t.Errorf("expected empty manifest, got %q", m.Manifest)
	}
	if len(m.Files) != 2 {
		t.Errorf("expected 2 files, got %d", len(m.Files))
	}
}

func TestDetectModules_RustCargo(t *testing.T) {
	root := t.TempDir()

	createFile(t, filepath.Join(root, "Cargo.toml"), "[package]\nname = \"my-crate\"\nversion = \"0.1.0\"\n")
	createFile(t, filepath.Join(root, "src", "main.rs"), "fn main() {}")

	files := []FileInfo{
		{Path: filepath.Join(root, "Cargo.toml"), RelPath: "Cargo.toml", Language: "toml"},
		{Path: filepath.Join(root, "src", "main.rs"), RelPath: filepath.Join("src", "main.rs"), Language: "rust"},
	}

	modules := DetectModules(root, files)
	if len(modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(modules))
	}

	m := modules[0]
	if m.Name != "my-crate" {
		t.Errorf("module name = %q, want %q", m.Name, "my-crate")
	}
	if m.Type != "rust" {
		t.Errorf("module type = %q, want %q", m.Type, "rust")
	}
}

// --- Full Scan Integration Test ---

func TestScan_Integration(t *testing.T) {
	root := t.TempDir()

	// Set up a realistic project structure
	createFile(t, filepath.Join(root, ".gitignore"), "*.log\ncoverage/\n")
	createFile(t, filepath.Join(root, "go.mod"), "module github.com/test/proj\n\ngo 1.21\n")
	createFile(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}")
	createFile(t, filepath.Join(root, "pkg", "util.go"), "package pkg")
	createFile(t, filepath.Join(root, "pkg", "util_test.go"), "package pkg")
	createFile(t, filepath.Join(root, "README.md"), "# Project")

	// Files that should be ignored
	createFile(t, filepath.Join(root, "node_modules", "dep", "index.js"), "")
	createFile(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main")
	createFile(t, filepath.Join(root, "debug.log"), "log data")
	createFile(t, filepath.Join(root, "coverage", "report.html"), "<html></html>")
	createFile(t, filepath.Join(root, "go.sum"), "hash data")

	result, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if result.Root != root {
		t.Errorf("root = %q, want %q", result.Root, root)
	}

	relPaths := map[string]bool{}
	for _, f := range result.Files {
		relPaths[f.RelPath] = true
	}

	// Expected files
	expected := []string{
		"go.mod",
		"main.go",
		filepath.Join("pkg", "util.go"),
		filepath.Join("pkg", "util_test.go"),
		"README.md",
		".gitignore",
	}
	for _, p := range expected {
		if !relPaths[p] {
			t.Errorf("expected file %q not found in scan results", p)
		}
	}

	// Unexpected files
	unexpected := []string{
		filepath.Join("node_modules", "dep", "index.js"),
		filepath.Join(".git", "HEAD"),
		"debug.log",
		filepath.Join("coverage", "report.html"),
		"go.sum",
	}
	for _, p := range unexpected {
		if relPaths[p] {
			t.Errorf("file %q should have been excluded", p)
		}
	}

	// Check module detection
	if len(result.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(result.Modules))
	}
	if result.Modules[0].Type != "go" {
		t.Errorf("module type = %q, want %q", result.Modules[0].Type, "go")
	}
	if result.Modules[0].Name != "github.com/test/proj" {
		t.Errorf("module name = %q, want %q", result.Modules[0].Name, "github.com/test/proj")
	}
}

func TestScan_FileInfoHasCorrectLanguage(t *testing.T) {
	root := t.TempDir()

	createFile(t, filepath.Join(root, "main.go"), "package main")
	createFile(t, filepath.Join(root, "app.ts"), "export default {}")
	createFile(t, filepath.Join(root, "style.css"), "body {}")
	createFile(t, filepath.Join(root, "README.md"), "# Readme")

	result, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	langByFile := map[string]string{}
	for _, f := range result.Files {
		langByFile[f.RelPath] = f.Language
	}

	checks := map[string]string{
		"main.go":   "go",
		"app.ts":    "typescript",
		"style.css": "css",
		"README.md": "markdown",
	}

	for file, wantLang := range checks {
		if got := langByFile[file]; got != wantLang {
			t.Errorf("file %q: language = %q, want %q", file, got, wantLang)
		}
	}
}

func TestScan_FileInfoHasCorrectSize(t *testing.T) {
	root := t.TempDir()

	content := "hello, world!"
	createFile(t, filepath.Join(root, "test.txt"), content)

	result, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan() error: %v", err)
	}

	if len(result.Files) == 0 {
		t.Fatal("expected at least 1 file")
	}

	for _, f := range result.Files {
		if f.RelPath == "test.txt" {
			if f.Size != int64(len(content)) {
				t.Errorf("size = %d, want %d", f.Size, len(content))
			}
			return
		}
	}
	t.Error("test.txt not found in results")
}

// --- Glob Match Tests ---

func TestGlobMatch(t *testing.T) {
	tests := []struct {
		pattern string
		name    string
		want    bool
	}{
		{"*.log", "debug.log", true},
		{"*.log", "app.txt", false},
		{"*.log", filepath.Join("dir", "debug.log"), false}, // * does not cross /
		{"**/*.log", filepath.Join("dir", "debug.log"), true},
		{"src/*.go", filepath.Join("src", "main.go"), true},
		{"src/*.go", filepath.Join("src", "sub", "main.go"), false},
		{"?at", "cat", true},
		{"?at", "at", false},
		{"exact.txt", "exact.txt", true},
		{"exact.txt", "other.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.name, func(t *testing.T) {
			got := globMatch(tt.pattern, tt.name)
			if got != tt.want {
				t.Errorf("globMatch(%q, %q) = %v, want %v", tt.pattern, tt.name, got, tt.want)
			}
		})
	}
}

// --- Helper ---

func containsPathComponent(path, component string) bool {
	parts := filepath.SplitList(path)
	if len(parts) <= 1 {
		// filepath.SplitList splits by os.PathListSeparator which is : or ;
		// We need to split by /
		parts = splitPath(path)
	}
	for _, p := range parts {
		if p == component {
			return true
		}
	}
	return false
}

func splitPath(path string) []string {
	var parts []string
	for {
		dir, file := filepath.Split(path)
		if file != "" {
			parts = append([]string{file}, parts...)
		}
		if dir == "" || dir == path {
			break
		}
		path = filepath.Clean(dir)
	}
	return parts
}
