package main

// cmd_export.go -- export index data as NDJSON.
//
// Streams memories from the Memories store for a given project, optionally
// filtered by layer. Default output is NDJSON (one JSON object per line)
// for piping. With --json, outputs an envelope with export count.

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/storage"
)

func exportCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export index data as NDJSON",
		Long: `Export all indexed data for a project as newline-delimited JSON (NDJSON).

Each line is a JSON object with text, source, and metadata fields.
Use --layer to filter to a specific index layer (atoms, wiring, zones, blueprint, patterns).

Examples:
  carto export --project myapp > backup.ndjson
  carto export --project myapp --layer atoms
  carto export --project myapp | jq '.text'`,
		RunE: runExport,
	}

	cmd.Flags().StringP("project", "p", "", "Project name (required)")
	cmd.Flags().String("layer", "", "Filter to specific layer (atoms, wiring, zones, blueprint, patterns)")
	cmd.MarkFlagRequired("project")

	return cmd
}

// exportEntry is a single exported memory record.
type exportEntry struct {
	ID       int            `json:"id"`
	Text     string         `json:"text"`
	Source   string         `json:"source"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func runExport(cmd *cobra.Command, _ []string) error {
	project, _ := cmd.Flags().GetString("project")
	layer, _ := cmd.Flags().GetString("layer")

	cfg := config.Load()
	if cfg.MemoriesURL == "" {
		return newConfigError("memories URL not configured (set MEMORIES_URL or run carto init)")
	}

	client := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// Build source prefix filter.
	sourcePrefix := "carto/" + project + "/"
	if layer != "" {
		sourcePrefix = "carto/" + project + "/layer:" + layer
	}

	// Stream mode (default) vs envelope mode (--json).
	jsonMode := isJSONMode(cmd)

	const pageSize = 100
	offset := 0
	exported := 0
	enc := json.NewEncoder(cmd.OutOrStdout())

	for {
		results, err := client.ListBySource(sourcePrefix, pageSize, offset)
		if err != nil {
			if exported == 0 {
				return newConnectionError("failed to connect to Memories: " + err.Error())
			}
			// Partial failure -- log warning and break.
			printWarn("export interrupted after %d entries: %v", exported, err)
			break
		}

		if len(results) == 0 {
			break
		}

		if !jsonMode {
			// Stream NDJSON: one entry per line.
			for _, r := range results {
				entry := exportEntry{
					ID:       r.ID,
					Text:     r.Text,
					Source:   r.Source,
					Metadata: r.Meta,
				}
				enc.Encode(entry) //nolint:errcheck
			}
		}

		exported += len(results)
		offset += len(results)

		if len(results) < pageSize {
			break
		}
	}

	if jsonMode {
		// Envelope mode: summary only.
		data := map[string]any{
			"exported": exported,
			"project":  project,
		}
		if layer != "" {
			data["layer"] = layer
		}
		writeEnvelopeHuman(cmd, data, nil, nil)
	} else if exported == 0 {
		fmt.Fprintf(cmd.ErrOrStderr(), "%sNo entries found for project %q%s\n", stone, project, reset)
	} else {
		fmt.Fprintf(cmd.ErrOrStderr(), "%sExported %d entries%s\n", stone, exported, reset)
	}

	logAuditEvent(cmd, "ok", "", map[string]any{"project": project, "exported": exported})
	return nil
}
