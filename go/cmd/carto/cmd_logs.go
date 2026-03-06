package main

// cmd_logs.go — query and tail the Carto audit log.
//
// Reads the NDJSON audit log file and displays entries with optional
// filters. Supports --follow for live tailing.

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func logsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Query or tail the audit log",
		Long: `Display entries from the Carto audit log.

The audit log path is set via --log-file or the CARTO_AUDIT_LOG environment variable.

Examples:
  carto logs --last 20
  carto logs --command index --result error
  carto logs --follow`,
		RunE: runLogs,
	}

	cmd.Flags().BoolP("follow", "f", false, "Tail the log file for new entries")
	cmd.Flags().IntP("last", "n", 20, "Number of recent entries to display")
	cmd.Flags().String("command", "", "Filter by command name (substring match)")
	cmd.Flags().String("result", "", "Filter by result: ok or error")

	return cmd
}

func runLogs(cmd *cobra.Command, _ []string) error {
	follow, _ := cmd.Flags().GetBool("follow")
	last, _ := cmd.Flags().GetInt("last")
	commandFilter, _ := cmd.Flags().GetString("command")
	resultFilter, _ := cmd.Flags().GetString("result")

	// Resolve audit log path.
	logFile, _ := cmd.Root().PersistentFlags().GetString("log-file")
	if logFile == "" {
		logFile = os.Getenv("CARTO_AUDIT_LOG")
	}
	if logFile == "" {
		return newConfigError("no audit log configured (set CARTO_AUDIT_LOG or use --log-file)")
	}

	f, err := os.Open(logFile)
	if err != nil {
		if os.IsNotExist(err) {
			return newNotFoundError("audit log file not found: " + logFile)
		}
		return fmt.Errorf("open audit log: %w", err)
	}
	defer f.Close()

	// Read all entries.
	var allEntries []auditEvent
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var ev auditEvent
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue // skip malformed lines
		}
		allEntries = append(allEntries, ev)
	}

	// Apply filters.
	var filtered []auditEvent
	for _, ev := range allEntries {
		if commandFilter != "" && !strings.Contains(ev.Command, commandFilter) {
			continue
		}
		if resultFilter != "" && ev.Result != resultFilter {
			continue
		}
		filtered = append(filtered, ev)
	}

	// Apply --last limit.
	if last > 0 && len(filtered) > last {
		filtered = filtered[len(filtered)-last:]
	}

	// Output.
	if isJSONMode(cmd) {
		data := map[string]any{
			"entries": filtered,
			"total":   len(filtered),
		}
		writeEnvelopeHuman(cmd, data, nil, nil)
	} else {
		for _, ev := range filtered {
			printLogEntry(cmd, ev)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "\n%s%d entries%s\n", stone, len(filtered), reset)
	}

	// Follow mode.
	if follow && !isJSONMode(cmd) {
		fmt.Fprintf(cmd.ErrOrStderr(), "%sTailing %s (Ctrl+C to stop)...%s\n", stone, logFile, reset)
		tailFile(cmd, f, commandFilter, resultFilter)
	}

	return nil
}

// printLogEntry formats a single audit event for human reading.
func printLogEntry(cmd *cobra.Command, ev auditEvent) {
	w := cmd.ErrOrStderr()
	resultColor := green
	resultMark := "\u2713"
	if ev.Result == "error" {
		resultColor = red
		resultMark = "\u2717"
	}

	ts := ev.Timestamp
	if t, err := time.Parse(time.RFC3339, ev.Timestamp); err == nil {
		ts = t.Local().Format("15:04:05")
	}

	fmt.Fprintf(w, "  %s%s%s %s%-8s%s %s", resultColor, resultMark, reset, stone, ts, reset, ev.Command)
	if ev.Error != "" {
		fmt.Fprintf(w, " %s%s%s", red, ev.Error, reset)
	}
	fmt.Fprintln(w)
}

// tailFile watches the file for new lines.
func tailFile(cmd *cobra.Command, f *os.File, commandFilter, resultFilter string) {
	scanner := bufio.NewScanner(f)
	for {
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				continue
			}
			var ev auditEvent
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				continue
			}
			if commandFilter != "" && !strings.Contains(ev.Command, commandFilter) {
				continue
			}
			if resultFilter != "" && ev.Result != resultFilter {
				continue
			}
			printLogEntry(cmd, ev)
		}
		time.Sleep(500 * time.Millisecond)
		// Reset scanner for next poll cycle.
		scanner = bufio.NewScanner(f)
	}
}
