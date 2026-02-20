package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/sources"
)

func sourcesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sources",
		Short: "Manage project source configurations",
	}
	cmd.AddCommand(sourcesListCmd())
	cmd.AddCommand(sourcesSetCmd())
	cmd.AddCommand(sourcesRmCmd())
	return cmd
}

func sourcesListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <project>",
		Short: "List configured sources for a project",
		Args:  cobra.ExactArgs(1),
		RunE:  runSourcesList,
	}
}

func runSourcesList(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	projectPath := filepath.Join(projectsDir, args[0])
	srcCfg, err := sources.LoadSourcesConfig(projectPath)
	if err != nil {
		return fmt.Errorf("load sources: %w", err)
	}

	if srcCfg == nil || len(srcCfg.Sources) == 0 {
		writeOutput(cmd, map[string]interface{}{"sources": map[string]interface{}{}}, func() {
			fmt.Println("No sources configured.")
		})
		return nil
	}

	// Build a JSON-friendly representation.
	type sourceDetail struct {
		Type     string            `json:"type"`
		Settings map[string]string `json:"settings,omitempty"`
	}
	var details []sourceDetail
	for name, entry := range srcCfg.Sources {
		details = append(details, sourceDetail{
			Type:     name,
			Settings: entry.Settings,
		})
	}

	writeOutput(cmd, details, func() {
		fmt.Printf("%s%sSources for %s%s\n\n", bold, cyan, args[0], reset)
		for name, entry := range srcCfg.Sources {
			fmt.Printf("  %s%s%s\n", bold, name, reset)
			for k, v := range entry.Settings {
				fmt.Printf("    %s: %s\n", k, v)
			}
			for k, vals := range entry.ListSettings {
				fmt.Printf("    %s: [%s]\n", k, strings.Join(vals, ", "))
			}
		}
	})
	return nil
}

func sourcesSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <project> <type> [key=value ...]",
		Short: "Set or update a source for a project",
		Args:  cobra.MinimumNArgs(2),
		RunE:  runSourcesSet,
	}
}

func runSourcesSet(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	projectName := args[0]
	sourceType := args[1]

	projectPath := filepath.Join(projectsDir, projectName)
	srcCfg, err := sources.LoadSourcesConfig(projectPath)
	if err != nil {
		return fmt.Errorf("load sources: %w", err)
	}
	if srcCfg == nil {
		srcCfg = &sources.SourcesYAML{
			Sources: make(map[string]sources.SourceEntry),
		}
	}

	// Get or create the entry.
	entry, exists := srcCfg.Sources[sourceType]
	if !exists {
		entry = sources.SourceEntry{
			Settings:     make(map[string]string),
			ListSettings: make(map[string][]string),
			Raw:          make(map[string]interface{}),
		}
	}
	if entry.Settings == nil {
		entry.Settings = make(map[string]string)
	}

	// Parse key=value pairs from remaining args.
	for _, kv := range args[2:] {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid key=value pair: %q", kv)
		}
		entry.Settings[parts[0]] = parts[1]
	}

	srcCfg.Sources[sourceType] = entry
	if err := sources.SaveSourcesConfig(projectPath, srcCfg); err != nil {
		return fmt.Errorf("save sources: %w", err)
	}

	writeOutput(cmd, map[string]string{"project": projectName, "source": sourceType, "status": "updated"}, func() {
		fmt.Printf("%s✓%s Source %q updated for project %q\n", green, reset, sourceType, projectName)
	})
	return nil
}

func sourcesRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <project> <type>",
		Short: "Remove a source from a project",
		Args:  cobra.ExactArgs(2),
		RunE:  runSourcesRm,
	}
}

func runSourcesRm(cmd *cobra.Command, args []string) error {
	projectsDir := os.Getenv("PROJECTS_DIR")
	if projectsDir == "" {
		return fmt.Errorf("PROJECTS_DIR environment variable is not set")
	}

	projectName := args[0]
	sourceType := args[1]

	projectPath := filepath.Join(projectsDir, projectName)
	srcCfg, err := sources.LoadSourcesConfig(projectPath)
	if err != nil {
		return fmt.Errorf("load sources: %w", err)
	}
	if srcCfg == nil || len(srcCfg.Sources) == 0 {
		return fmt.Errorf("no sources configured for project %q", projectName)
	}

	if _, exists := srcCfg.Sources[sourceType]; !exists {
		return fmt.Errorf("source %q not found for project %q", sourceType, projectName)
	}

	delete(srcCfg.Sources, sourceType)
	if err := sources.SaveSourcesConfig(projectPath, srcCfg); err != nil {
		return fmt.Errorf("save sources: %w", err)
	}

	writeOutput(cmd, map[string]string{"project": projectName, "source": sourceType, "status": "removed"}, func() {
		fmt.Printf("%s✓%s Source %q removed from project %q\n", green, reset, sourceType, projectName)
	})
	return nil
}
