package main

// cmd_import.go — import index data from NDJSON.
//
// Reads NDJSON from stdin (one memory per line) and stores each entry
// in Memories for the given project. Supports two strategies:
//   - add (default): appends entries alongside existing data
//   - replace: deletes all existing entries for the project first

import (
	"bufio"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/storage"
)

func importCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import",
		Short: "Import index data from NDJSON",
		Long: `Import index data from newline-delimited JSON (NDJSON) into the Memories store.

Reads from stdin. Each line should be a JSON object with at least "text" and "source" fields.

Strategies:
  add     — append to existing index data (default)
  replace — delete all existing entries for the project, then import

Examples:
  cat backup.ndjson | carto import --project myapp
  carto import --project myapp --strategy replace --yes < data.ndjson`,
		RunE: runImport,
	}

	cmd.Flags().StringP("project", "p", "", "Project name (required)")
	cmd.Flags().String("strategy", "add", "Import strategy: add or replace")
	cmd.MarkFlagRequired("project")

	return cmd
}

// importRecord is a single NDJSON line from the import stream.
type importRecord struct {
	Text     string         `json:"text"`
	Source   string         `json:"source"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

func runImport(cmd *cobra.Command, _ []string) error {
	project, _ := cmd.Flags().GetString("project")
	strategy, _ := cmd.Flags().GetString("strategy")

	if strategy != "add" && strategy != "replace" {
		return newConfigError("invalid strategy: " + strategy + " (use add or replace)")
	}

	cfg := config.Load()
	if cfg.MemoriesURL == "" {
		return newConfigError("memories URL not configured (set MEMORIES_URL or run carto init)")
	}

	client := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// Replace strategy: delete existing entries first.
	if strategy == "replace" {
		if !confirmAction(cmd, fmt.Sprintf("Delete all existing entries for project %q before import?", project)) {
			printError("import cancelled")
			return nil
		}

		prefix := "carto/" + project + "/"
		deleted, err := client.DeleteBySource(prefix)
		if err != nil {
			return newConnectionError("failed to delete existing entries: " + err.Error())
		}
		verboseLog(cmd, "deleted %d existing entries", deleted)
	}

	// Read NDJSON from stdin.
	reader := cmd.InOrStdin()

	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1MB line buffer

	imported := 0
	var batch []storage.Memory

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var rec importRecord
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			printWarn("skipping invalid JSON line: %v", err)
			continue
		}

		if rec.Text == "" {
			printWarn("skipping entry with empty text")
			continue
		}

		// Ensure source has the project prefix.
		source := rec.Source
		if source == "" {
			source = "carto/" + project + "/import"
		}

		batch = append(batch, storage.Memory{
			Text:     rec.Text,
			Source:   source,
			Metadata: rec.Metadata,
		})

		if len(batch) >= 100 {
			if _, err := client.UpsertBatch(batch); err != nil {
				return newConnectionError("failed to store batch: " + err.Error())
			}
			imported += len(batch)
			batch = batch[:0]
		}
	}

	if err := scanner.Err(); err != nil {
		return newConnectionError("error reading input: " + err.Error())
	}

	// Flush remaining batch.
	if len(batch) > 0 {
		if _, err := client.UpsertBatch(batch); err != nil {
			return newConnectionError("failed to store final batch: " + err.Error())
		}
		imported += len(batch)
	}

	data := map[string]any{
		"imported": imported,
		"project":  project,
		"strategy": strategy,
	}

	writeEnvelopeHuman(cmd, data, nil, func() {
		fmt.Fprintf(cmd.ErrOrStderr(), "%s%sImported %d entries%s into project %q (strategy: %s)\n",
			bold, gold, imported, reset, project, strategy)
	})

	logAuditEvent(cmd, "ok", "", map[string]any{
		"project":  project,
		"imported": imported,
		"strategy": strategy,
	})
	return nil
}
