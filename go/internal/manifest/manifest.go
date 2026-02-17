package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// FileEntry tracks the hash and metadata of a single indexed file.
type FileEntry struct {
	Hash      string    `json:"hash"`
	Size      int64     `json:"size"`
	IndexedAt time.Time `json:"indexed_at"`
}

// Manifest tracks the state of all indexed files for a project.
type Manifest struct {
	Version   string               `json:"version"`
	Project   string               `json:"project"`
	IndexedAt time.Time            `json:"indexed_at"`
	Files     map[string]FileEntry `json:"files"` // keyed by relative path
	path      string               // on-disk path to manifest.json (not serialized)
}

// ChangeSet describes what changed since the last index.
type ChangeSet struct {
	Added    []string // new files not in manifest
	Modified []string // files with different hash
	Removed  []string // files in manifest but no longer on disk
}

// NewManifest creates a new empty manifest for a project.
// The manifest file path is set to {projectRoot}/.carto/manifest.json.
func NewManifest(projectRoot, projectName string) *Manifest {
	return &Manifest{
		Version: "1.0",
		Project: projectName,
		Files:   make(map[string]FileEntry),
		path:    filepath.Join(projectRoot, ".carto", "manifest.json"),
	}
}

// Load reads a manifest from {projectRoot}/.carto/manifest.json.
// If the file does not exist, it returns a new empty manifest (not an error).
func Load(projectRoot string) (*Manifest, error) {
	p := filepath.Join(projectRoot, ".carto", "manifest.json")

	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			m := NewManifest(projectRoot, "")
			return m, nil
		}
		return nil, fmt.Errorf("read manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal manifest: %w", err)
	}
	m.path = p

	if m.Files == nil {
		m.Files = make(map[string]FileEntry)
	}

	return &m, nil
}

// Save writes the manifest to disk as JSON.
// It creates the .carto/ directory if it does not already exist.
func (m *Manifest) Save() error {
	dir := filepath.Dir(m.path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create manifest dir: %w", err)
	}

	m.IndexedAt = time.Now()

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal manifest: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0o644); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	return nil
}

// ComputeHash reads the file at filePath and returns its SHA-256 hex digest.
func (m *Manifest) ComputeHash(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open file for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// DetectChanges compares a list of current file paths (relative to projectRoot)
// against the manifest to determine what has been added, modified, or removed.
func (m *Manifest) DetectChanges(currentFiles []string, projectRoot string) (*ChangeSet, error) {
	cs := &ChangeSet{}

	// Build a set of current files for fast lookup.
	currentSet := make(map[string]struct{}, len(currentFiles))
	for _, f := range currentFiles {
		currentSet[f] = struct{}{}
	}

	// Check each current file against the manifest.
	for _, relPath := range currentFiles {
		entry, exists := m.Files[relPath]
		if !exists {
			cs.Added = append(cs.Added, relPath)
			continue
		}

		absPath := filepath.Join(projectRoot, relPath)
		hash, err := m.ComputeHash(absPath)
		if err != nil {
			return nil, fmt.Errorf("compute hash for %s: %w", relPath, err)
		}

		if hash != entry.Hash {
			cs.Modified = append(cs.Modified, relPath)
		}
	}

	// Check for files removed from disk.
	for relPath := range m.Files {
		if _, exists := currentSet[relPath]; !exists {
			cs.Removed = append(cs.Removed, relPath)
		}
	}

	return cs, nil
}

// UpdateFile adds or updates a file entry in the manifest with the current timestamp.
func (m *Manifest) UpdateFile(relPath, hash string, size int64) {
	m.Files[relPath] = FileEntry{
		Hash:      hash,
		Size:      size,
		IndexedAt: time.Now(),
	}
}

// RemoveFile deletes a file entry from the manifest.
func (m *Manifest) RemoveFile(relPath string) {
	delete(m.Files, relPath)
}

// IsEmpty returns true if no files are tracked in the manifest.
func (m *Manifest) IsEmpty() bool {
	return len(m.Files) == 0
}
