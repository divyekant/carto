package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/patterns"
	"github.com/divyekant/carto/internal/scanner"
	"github.com/divyekant/carto/internal/storage"
)

func patternsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "patterns <path>",
		Short: "Generate CLAUDE.md and .cursorrules",
		Args:  cobra.ExactArgs(1),
		RunE:  runPatterns,
	}
	cmd.Flags().String("format", "all", "Output format: claude, cursor, all")
	return cmd
}

func runPatterns(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	format, _ := cmd.Flags().GetString("format")

	cfg := config.Load()
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// Scan to discover modules.
	result, err := scanner.Scan(absPath)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	projectName := filepath.Base(absPath)

	// Try to load existing analysis from Memories.
	store := storage.NewStore(memoriesClient, projectName)

	// Build module summaries from scan.
	var moduleSummaries []patterns.ModuleSummary
	for _, mod := range result.Modules {
		moduleSummaries = append(moduleSummaries, patterns.ModuleSummary{
			Name:   mod.Name,
			Type:   mod.Type,
			Intent: "",
		})
	}

	// Attempt to retrieve stored blueprint and patterns.
	var blueprint string
	var pats []string
	var zones []patterns.Zone

	if blueprintResults, err := store.RetrieveLayer("_system", "blueprint"); err == nil && len(blueprintResults) > 0 {
		blueprint = blueprintResults[0].Text
	}

	if patResults, err := store.RetrieveLayer("_system", "patterns"); err == nil && len(patResults) > 0 {
		var parsed []string
		if jsonErr := json.Unmarshal([]byte(patResults[0].Text), &parsed); jsonErr == nil {
			pats = parsed
		}
	}

	// Retrieve zones from each module.
	for _, mod := range result.Modules {
		if zoneResults, err := store.RetrieveLayer(mod.Name, "zones"); err == nil && len(zoneResults) > 0 {
			var modZones []patterns.Zone
			if jsonErr := json.Unmarshal([]byte(zoneResults[0].Text), &modZones); jsonErr == nil {
				zones = append(zones, modZones...)
			}
		}
	}

	input := patterns.Input{
		ProjectName: projectName,
		Blueprint:   blueprint,
		Patterns:    pats,
		Zones:       zones,
		Modules:     moduleSummaries,
	}

	fmt.Printf("%s%sGenerating patterns for %s%s\n", bold, cyan, absPath, reset)
	fmt.Printf("  modules: %d, format: %s\n\n", len(result.Modules), format)

	if err := patterns.WriteFiles(absPath, input, format); err != nil {
		return fmt.Errorf("write patterns: %w", err)
	}

	switch format {
	case "claude":
		fmt.Printf("  %s✓%s CLAUDE.md\n", green, reset)
	case "cursor":
		fmt.Printf("  %s✓%s .cursorrules\n", green, reset)
	default:
		fmt.Printf("  %s✓%s CLAUDE.md\n", green, reset)
		fmt.Printf("  %s✓%s .cursorrules\n", green, reset)
	}

	return nil
}
