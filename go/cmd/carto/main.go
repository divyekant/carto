package main

import (
	"os"

	"github.com/spf13/cobra"
)

var version = "0.3.0"

func main() {
	root := &cobra.Command{
		Use:     "carto",
		Short:   "Carto -- intent-aware codebase intelligence",
		Version: version,
	}

	root.PersistentFlags().Bool("json", false, "Output machine-readable JSON")
	root.PersistentFlags().BoolP("quiet", "q", false, "Suppress progress spinners, only output result")

	root.AddCommand(indexCmd())
	root.AddCommand(queryCmd())
	root.AddCommand(modulesCmd())
	root.AddCommand(patternsCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(serveCmd())
	root.AddCommand(projectsCmd())
	root.AddCommand(sourcesCmd())
	root.AddCommand(configCmdGroup())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
