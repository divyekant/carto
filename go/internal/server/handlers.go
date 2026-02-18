package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/anthropic/indexer/internal/manifest"
)

// ProjectInfo describes an indexed project discovered in the projects directory.
type ProjectInfo struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	IndexedAt time.Time `json:"indexed_at"`
	FileCount int       `json:"file_count"`
}

// writeJSON marshals v as JSON and writes it to the response with the given status.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// handleListProjects scans projectsDir for subdirectories that contain a
// .carto/manifest.json and returns their metadata as a JSON array.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	if s.projectsDir == "" {
		writeJSON(w, http.StatusOK, []ProjectInfo{})
		return
	}

	entries, err := os.ReadDir(s.projectsDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to read projects directory")
		return
	}

	var projects []ProjectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectRoot := filepath.Join(s.projectsDir, entry.Name())
		mf, err := manifest.Load(projectRoot)
		if err != nil {
			continue
		}
		// Skip directories without a manifest file (empty manifest = no project).
		if mf.IsEmpty() && mf.Project == "" {
			continue
		}

		projects = append(projects, ProjectInfo{
			Name:      mf.Project,
			Path:      projectRoot,
			IndexedAt: mf.IndexedAt,
			FileCount: len(mf.Files),
		})
	}

	if projects == nil {
		projects = []ProjectInfo{}
	}

	writeJSON(w, http.StatusOK, projects)
}
