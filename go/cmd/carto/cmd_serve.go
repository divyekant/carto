package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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

	// Warn operators when auth is disabled so it is not overlooked in production.
	if cfg.ServerToken == "" {
		fmt.Fprintf(os.Stderr,
			"%s%sWARN:%s Authentication is disabled. Set CARTO_SERVER_TOKEN to require a Bearer token.\n",
			amber, bold, reset,
		)
	}

	fmt.Printf("%s%sCarto server%s starting on http://localhost:%s\n", bold, gold, reset, port)

	// Build an http.Server with sane production timeouts.
	// WriteTimeout is generous (10 min) because SSE progress streams for large
	// codebases can legitimately run for several minutes.
	httpSrv := &http.Server{
		Addr:         ":" + port,
		Handler:      srv,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Minute,
		IdleTimeout:  120 * time.Second,
	}

	// Start the server in a goroutine so we can listen for OS shutdown signals.
	serverErr := make(chan error, 1)
	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- fmt.Errorf("server error: %w", err)
		}
	}()

	// Block until we receive SIGINT/SIGTERM or the server errors out.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		return err
	case sig := <-quit:
		// Use structured slog for consistency with the rest of the server stack.
		slog.Info("shutdown_signal",
			"signal", sig.String(),
			"grace_period_s", 30,
		)
	}

	// Allow up to 30 seconds for in-flight requests (indexing runs) to complete
	// before forcibly closing connections. This prevents manifest corruption on
	// docker restart or kubernetes rolling deployments.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := httpSrv.Shutdown(ctx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	slog.Info("server_stopped", "status", "clean")
	return nil
}
