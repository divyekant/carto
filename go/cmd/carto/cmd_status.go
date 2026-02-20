package main

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/manifest"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <path>",
		Short: "Show index status",
		Args:  cobra.ExactArgs(1),
		RunE:  runStatus,
	}
}

func runStatus(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	mf, err := manifest.Load(absPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}

	if mf.IsEmpty() {
		writeOutput(cmd, map[string]interface{}{"indexed": false, "path": absPath}, func() {
			fmt.Printf("%s%sIndex status for %s%s\n\n", bold, cyan, absPath, reset)
			fmt.Printf("  %sNo index found.%s Run %scarto index %s%s to create one.\n", yellow, reset, bold, absPath, reset)
		})
		return nil
	}

	projectName := mf.Project
	if projectName == "" {
		projectName = filepath.Base(absPath)
	}

	// Calculate total size across indexed files.
	var totalSize int64
	for _, entry := range mf.Files {
		totalSize += entry.Size
	}

	type statusData struct {
		Project   string `json:"project"`
		Files     int    `json:"files"`
		TotalSize string `json:"total_size"`
		IndexedAt string `json:"indexed_at"`
	}

	data := statusData{
		Project:   projectName,
		Files:     len(mf.Files),
		TotalSize: formatBytes(totalSize),
		IndexedAt: mf.IndexedAt.Format(time.RFC3339),
	}

	writeOutput(cmd, data, func() {
		fmt.Printf("%s%sIndex status for %s%s\n\n", bold, cyan, absPath, reset)
		fmt.Printf("  %sProject:%s     %s\n", cyan, reset, data.Project)
		fmt.Printf("  %sLast indexed:%s %s\n", cyan, reset, data.IndexedAt)
		fmt.Printf("  %sFiles:%s       %d\n", cyan, reset, data.Files)
		fmt.Printf("  %sTotal size:%s  %s\n", cyan, reset, data.TotalSize)
	})

	return nil
}
