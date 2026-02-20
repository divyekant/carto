package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/scanner"
)

func modulesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "modules <path>",
		Short: "List detected modules",
		Args:  cobra.ExactArgs(1),
		RunE:  runModules,
	}
}

func runModules(cmd *cobra.Command, args []string) error {
	absPath, err := filepath.Abs(args[0])
	if err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}

	result, err := scanner.Scan(absPath)
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}

	type moduleInfo struct {
		Name  string `json:"name"`
		Type  string `json:"type"`
		Path  string `json:"path"`
		Files int    `json:"files"`
	}

	modules := make([]moduleInfo, 0, len(result.Modules))
	for _, mod := range result.Modules {
		relPath := mod.RelPath
		if relPath == "" {
			relPath = "."
		}
		modules = append(modules, moduleInfo{
			Name:  mod.Name,
			Type:  mod.Type,
			Path:  relPath,
			Files: len(mod.Files),
		})
	}

	writeOutput(cmd, modules, func() {
		fmt.Printf("%s%sDetected modules in %s%s\n\n", bold, cyan, absPath, reset)

		if len(modules) == 0 {
			fmt.Println("  No modules detected.")
			return
		}

		fmt.Printf("  %-30s %-15s %-40s %s\n", "NAME", "TYPE", "PATH", "FILES")
		fmt.Printf("  %-30s %-15s %-40s %s\n",
			strings.Repeat("-", 30),
			strings.Repeat("-", 15),
			strings.Repeat("-", 40),
			strings.Repeat("-", 6))

		for _, mod := range modules {
			fmt.Printf("  %-30s %-15s %-40s %d\n", mod.Name, mod.Type, mod.Path, mod.Files)
		}

		fmt.Printf("\n  %sTotal:%s %d module(s), %d file(s)\n", bold, reset, len(result.Modules), len(result.Files))
	})

	return nil
}
