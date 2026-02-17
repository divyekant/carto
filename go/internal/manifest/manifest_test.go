package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewManifest(t *testing.T) {
	root := "/tmp/fake-project"
	m := NewManifest(root, "my-project")

	// path must end with .carto/manifest.json
	wantSuffix := filepath.Join(".carto", "manifest.json")
	if !strings.HasSuffix(m.path, wantSuffix) {
		t.Errorf("path = %q, want suffix %q", m.path, wantSuffix)
	}

	// Files map initialized (not nil)
	if m.Files == nil {
		t.Fatal("Files map should be initialized, got nil")
	}

	// Version set
	if m.Version != "1.0" {
		t.Errorf("Version = %q, want %q", m.Version, "1.0")
	}

	// Project name set
	if m.Project != "my-project" {
		t.Errorf("Project = %q, want %q", m.Project, "my-project")
	}
}

func TestSaveAndLoad(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test-project")

	m.UpdateFile("src/main.go", "abc123", 1024)
	m.UpdateFile("README.md", "def456", 512)

	if err := m.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(root)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.Project != "test-project" {
		t.Errorf("Project = %q, want %q", loaded.Project, "test-project")
	}

	if loaded.Version != "1.0" {
		t.Errorf("Version = %q, want %q", loaded.Version, "1.0")
	}

	if len(loaded.Files) != 2 {
		t.Fatalf("len(Files) = %d, want 2", len(loaded.Files))
	}

	entry, ok := loaded.Files["src/main.go"]
	if !ok {
		t.Fatal("missing entry for src/main.go")
	}
	if entry.Hash != "abc123" {
		t.Errorf("src/main.go Hash = %q, want %q", entry.Hash, "abc123")
	}
	if entry.Size != 1024 {
		t.Errorf("src/main.go Size = %d, want %d", entry.Size, 1024)
	}

	entry2, ok := loaded.Files["README.md"]
	if !ok {
		t.Fatal("missing entry for README.md")
	}
	if entry2.Hash != "def456" {
		t.Errorf("README.md Hash = %q, want %q", entry2.Hash, "def456")
	}
	if entry2.Size != 512 {
		t.Errorf("README.md Size = %d, want %d", entry2.Size, 512)
	}
}

func TestLoad_NoFile(t *testing.T) {
	root := t.TempDir()

	m, err := Load(root)
	if err != nil {
		t.Fatalf("Load from empty dir should not error, got: %v", err)
	}

	if !m.IsEmpty() {
		t.Errorf("IsEmpty() = false, want true for manifest loaded from empty dir")
	}

	if m.Files == nil {
		t.Error("Files map should be initialized even when no file exists")
	}
}

func TestComputeHash(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test")

	content := []byte("hello, world\n")
	filePath := filepath.Join(root, "test.txt")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	got, err := m.ComputeHash(filePath)
	if err != nil {
		t.Fatalf("ComputeHash: %v", err)
	}

	// Compute expected SHA-256 hex digest.
	h := sha256.Sum256(content)
	want := hex.EncodeToString(h[:])

	if got != want {
		t.Errorf("ComputeHash = %q, want %q", got, want)
	}
}

func TestComputeHash_Deterministic(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test")

	content := []byte("deterministic content check")
	filePath := filepath.Join(root, "same.txt")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	hash1, err := m.ComputeHash(filePath)
	if err != nil {
		t.Fatalf("ComputeHash (1st): %v", err)
	}

	hash2, err := m.ComputeHash(filePath)
	if err != nil {
		t.Fatalf("ComputeHash (2nd): %v", err)
	}

	if hash1 != hash2 {
		t.Errorf("hashes differ for same file: %q vs %q", hash1, hash2)
	}
}

func TestDetectChanges_Added(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test")
	// Manifest has no files; disk has one file.

	cs, err := m.DetectChanges([]string{"new-file.go"}, root)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}

	if len(cs.Added) != 1 || cs.Added[0] != "new-file.go" {
		t.Errorf("Added = %v, want [new-file.go]", cs.Added)
	}
	if len(cs.Modified) != 0 {
		t.Errorf("Modified = %v, want empty", cs.Modified)
	}
	if len(cs.Removed) != 0 {
		t.Errorf("Removed = %v, want empty", cs.Removed)
	}
}

func TestDetectChanges_Modified(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test")

	// Write a file on disk with new content.
	filePath := filepath.Join(root, "changed.txt")
	if err := os.WriteFile(filePath, []byte("new content"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Record an old (different) hash in the manifest.
	m.UpdateFile("changed.txt", "old-hash-that-will-not-match", 100)

	cs, err := m.DetectChanges([]string{"changed.txt"}, root)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}

	if len(cs.Modified) != 1 || cs.Modified[0] != "changed.txt" {
		t.Errorf("Modified = %v, want [changed.txt]", cs.Modified)
	}
	if len(cs.Added) != 0 {
		t.Errorf("Added = %v, want empty", cs.Added)
	}
	if len(cs.Removed) != 0 {
		t.Errorf("Removed = %v, want empty", cs.Removed)
	}
}

func TestDetectChanges_Removed(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test")

	// Manifest tracks a file, but it is not in currentFiles.
	m.UpdateFile("deleted.txt", "somehash", 64)

	cs, err := m.DetectChanges([]string{}, root)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}

	if len(cs.Removed) != 1 || cs.Removed[0] != "deleted.txt" {
		t.Errorf("Removed = %v, want [deleted.txt]", cs.Removed)
	}
	if len(cs.Added) != 0 {
		t.Errorf("Added = %v, want empty", cs.Added)
	}
	if len(cs.Modified) != 0 {
		t.Errorf("Modified = %v, want empty", cs.Modified)
	}
}

func TestDetectChanges_Mixed(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test")

	// "existing.txt" is in manifest with a stale hash -> will be Modified.
	existingPath := filepath.Join(root, "existing.txt")
	if err := os.WriteFile(existingPath, []byte("updated"), 0o644); err != nil {
		t.Fatalf("write existing.txt: %v", err)
	}
	m.UpdateFile("existing.txt", "stale-hash", 50)

	// "gone.txt" is in manifest but NOT in currentFiles -> Removed.
	m.UpdateFile("gone.txt", "anyhash", 30)

	// "brand-new.txt" is in currentFiles but NOT in manifest -> Added.
	currentFiles := []string{"existing.txt", "brand-new.txt"}

	cs, err := m.DetectChanges(currentFiles, root)
	if err != nil {
		t.Fatalf("DetectChanges: %v", err)
	}

	if len(cs.Added) != 1 || cs.Added[0] != "brand-new.txt" {
		t.Errorf("Added = %v, want [brand-new.txt]", cs.Added)
	}
	if len(cs.Modified) != 1 || cs.Modified[0] != "existing.txt" {
		t.Errorf("Modified = %v, want [existing.txt]", cs.Modified)
	}
	if len(cs.Removed) != 1 || cs.Removed[0] != "gone.txt" {
		t.Errorf("Removed = %v, want [gone.txt]", cs.Removed)
	}
}

func TestUpdateFile(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test")

	m.UpdateFile("pkg/util.go", "hashvalue", 2048)

	entry, ok := m.Files["pkg/util.go"]
	if !ok {
		t.Fatal("entry not found after UpdateFile")
	}
	if entry.Hash != "hashvalue" {
		t.Errorf("Hash = %q, want %q", entry.Hash, "hashvalue")
	}
	if entry.Size != 2048 {
		t.Errorf("Size = %d, want %d", entry.Size, 2048)
	}
	if entry.IndexedAt.IsZero() {
		t.Error("IndexedAt should be set to a non-zero time")
	}
}

func TestRemoveFile(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test")

	m.UpdateFile("to-remove.go", "hash", 100)
	if _, ok := m.Files["to-remove.go"]; !ok {
		t.Fatal("entry should exist before removal")
	}

	m.RemoveFile("to-remove.go")

	if _, ok := m.Files["to-remove.go"]; ok {
		t.Error("entry should be gone after RemoveFile")
	}
}

func TestIsEmpty(t *testing.T) {
	root := t.TempDir()
	m := NewManifest(root, "test")

	if !m.IsEmpty() {
		t.Error("new manifest should be empty")
	}

	m.UpdateFile("file.go", "h", 1)

	if m.IsEmpty() {
		t.Error("manifest with a file should not be empty")
	}
}
