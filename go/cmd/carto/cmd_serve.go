package main

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/server"
	"github.com/divyekant/carto/internal/storage"
	cartoWeb "github.com/divyekant/carto/web"
)

func serveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the Carto web UI",
		RunE:  runServe,
	}
	cmd.Flags().String("port", "8950", "Port to listen on")
	cmd.Flags().String("projects-dir", "", "Directory containing indexed projects")
	return cmd
}

func runServe(cmd *cobra.Command, args []string) error {
	port, _ := cmd.Flags().GetString("port")
	projectsDir, _ := cmd.Flags().GetString("projects-dir")

	// Set config persistence path inside the projects directory so it
	// survives container restarts (the projects dir is a mounted volume).
	if projectsDir != "" {
		config.ConfigPath = filepath.Join(projectsDir, ".carto-server.json")
	}

	cfg := config.Load()

	memoriesClient := storage.NewMemoriesClient(config.ResolveURL(cfg.MemoriesURL), cfg.MemoriesKey)

	// Extract the dist subdirectory from the embedded FS.
	distFS, err := fs.Sub(cartoWeb.DistFS, "dist")
	if err != nil {
		return fmt.Errorf("embedded web assets: %w", err)
	}

	srv := server.New(cfg, memoriesClient, projectsDir, distFS)
	fmt.Printf("%s%sCarto server%s starting on http://localhost:%s\n", bold, cyan, reset, port)
	return srv.Start(":" + port)
}
