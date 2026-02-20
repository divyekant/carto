package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/manifest"
	"github.com/divyekant/carto/internal/sources"
)

func projectsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage indexed projects",
	}
	cmd.AddCommand(projectsListCmd())
	cmd.AddCommand(projectsShowCmd())
	cmd.AddCommand(projectsDeleteCmd())
	return cmd
}

func projectsListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all indexed projects",
		RunE:  runProjectsList,
	}
}

func runProjectsList(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return fmt.Errorf("read projects dir: %w", err)
	}

	type projectInfo struct {
		Name      string `json:"name"`
		Path      string `json:"path"`
		Files     int    `json:"files"`
		IndexedAt string `json:"indexed_at"`
	}

	var projects []projectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectPath := filepath.Join(projectsDir, entry.Name())
		mf, err := manifest.Load(projectPath)
		if err != nil || mf.IsEmpty() {
			continue
		}
		name := mf.Project
		if name == "" {
			name = entry.Name()
		}
		projects = append(projects, projectInfo{
			Name:      name,
			Path:      projectPath,
			Files:     len(mf.Files),
			IndexedAt: mf.IndexedAt.Format(time.RFC3339),
		})
	}

	writeOutput(cmd, projects, func() {
		if len(projects) == 0 {
			fmt.Println("No indexed projects found.")
			return
		}
		fmt.Printf("%s%sIndexed projects%s\n\n", bold, cyan, reset)
		fmt.Printf("  %-25s %-8s %s\n", "NAME", "FILES", "INDEXED AT")
		fmt.Printf("  %-25s %-8s %s\n",
			strings.Repeat("-", 25),
			strings.Repeat("-", 8),
			strings.Repeat("-", 20))
		for _, p := range projects {
			fmt.Printf("  %-25s %-8d %s\n", p.Name, p.Files, p.IndexedAt)
		}
		fmt.Printf("\n  %sTotal:%s %d project(s)\n", bold, reset, len(projects))
	})
	return nil
}

func projectsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show details of an indexed project",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectsShow,
	}
}

func runProjectsShow(cmd *cobra.Command, args []string) error {
	name := args[0]
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	projectPath := filepath.Join(projectsDir, name)
	mf, err := manifest.Load(projectPath)
	if err != nil {
		return fmt.Errorf("load manifest: %w", err)
	}
	if mf.IsEmpty() {
		return fmt.Errorf("project %q not found or has no index", name)
	}

	// Calculate total size.
	var totalSize int64
	for _, entry := range mf.Files {
		totalSize += entry.Size
	}

	// Load sources config if present.
	srcCfg, _ := sources.LoadSourcesConfig(projectPath)
	var sourceNames []string
	if srcCfg != nil {
		for k := range srcCfg.Sources {
			sourceNames = append(sourceNames, k)
		}
	}

	type showData struct {
		Name      string   `json:"name"`
		Path      string   `json:"path"`
		Files     int      `json:"files"`
		TotalSize string   `json:"total_size"`
		IndexedAt string   `json:"indexed_at"`
		Sources   []string `json:"sources,omitempty"`
	}

	data := showData{
		Name:      mf.Project,
		Path:      projectPath,
		Files:     len(mf.Files),
		TotalSize: formatBytes(totalSize),
		IndexedAt: mf.IndexedAt.Format(time.RFC3339),
		Sources:   sourceNames,
	}

	writeOutput(cmd, data, func() {
		fmt.Printf("%s%sProject: %s%s\n\n", bold, cyan, data.Name, reset)
		fmt.Printf("  %sPath:%s        %s\n", cyan, reset, data.Path)
		fmt.Printf("  %sFiles:%s       %d\n", cyan, reset, data.Files)
		fmt.Printf("  %sTotal size:%s  %s\n", cyan, reset, data.TotalSize)
		fmt.Printf("  %sIndexed at:%s  %s\n", cyan, reset, data.IndexedAt)
		if len(data.Sources) > 0 {
			fmt.Printf("  %sSources:%s     %s\n", cyan, reset, strings.Join(data.Sources, ", "))
		}
	})
	return nil
}

func projectsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <name>",
		Short: "Delete a project's .carto directory",
		Args:  cobra.ExactArgs(1),
		RunE:  runProjectsDelete,
	}
}

func runProjectsDelete(cmd *cobra.Command, args []string) error {
	name := args[0]
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	cartoDir := filepath.Join(projectsDir, name, ".carto")
	info, err := os.Stat(cartoDir)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("project %q has no .carto directory", name)
	}

	if err := os.RemoveAll(cartoDir); err != nil {
		return fmt.Errorf("delete .carto: %w", err)
	}

	type deleteResult struct {
		Name    string `json:"name"`
		Deleted bool   `json:"deleted"`
	}

	writeOutput(cmd, deleteResult{Name: name, Deleted: true}, func() {
		fmt.Printf("%s✓%s Deleted .carto directory for project %q\n", green, reset, name)
	})
	return nil
}
