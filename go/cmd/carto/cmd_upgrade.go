package main

// cmd_upgrade.go — check for and install newer versions of Carto.
//
// `carto upgrade --check` queries GitHub releases to find the latest version.
// `carto upgrade` (without --check) would download and replace the binary,
// but for now this is stubbed with a message.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func upgradeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Check for and install new versions",
		Long: `Check GitHub for newer versions of Carto and optionally upgrade.

Examples:
  carto upgrade --check       # Check only, don't download
  carto upgrade --yes         # Upgrade without confirmation`,
		RunE: runUpgrade,
	}

	cmd.Flags().Bool("check", false, "Check for updates without installing")

	return cmd
}

// upgradeResult is the envelope data.
type upgradeResult struct {
	Current         string `json:"current"`
	Latest          string `json:"latest"`
	UpdateAvailable bool   `json:"update_available"`
}

// compareVersions compares two semver strings (without "v" prefix).
// Returns:
//
//	-1 if a < b
//	 0 if a == b
//	+1 if a > b
func compareVersions(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")

	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	for i := 0; i < 3; i++ {
		var va, vb int
		if i < len(partsA) {
			fmt.Sscanf(partsA[i], "%d", &va)
		}
		if i < len(partsB) {
			fmt.Sscanf(partsB[i], "%d", &vb)
		}
		if va < vb {
			return -1
		}
		if va > vb {
			return 1
		}
	}
	return 0
}

// githubReleaseURL is the GitHub API URL. Exported as a var for testing.
var githubReleaseURL = "https://api.github.com/repos/divyekant/carto/releases/latest"

func fetchLatestVersion() (string, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(githubReleaseURL)
	if err != nil {
		return "", fmt.Errorf("failed to check for updates: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("failed to parse release info: %w", err)
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	checkOnly, _ := cmd.Flags().GetBool("check")

	latest, err := fetchLatestVersion()
	if err != nil {
		return newConnectionError(err.Error())
	}

	result := upgradeResult{
		Current:         version,
		Latest:          latest,
		UpdateAvailable: compareVersions(version, latest) < 0,
	}

	if checkOnly || !result.UpdateAvailable {
		writeEnvelopeHuman(cmd, result, nil, func() {
			w := cmd.ErrOrStderr()
			fmt.Fprintf(w, "  Current: %s%s%s\n", bold, result.Current, reset)
			fmt.Fprintf(w, "  Latest:  %s%s%s\n", bold, result.Latest, reset)
			if result.UpdateAvailable {
				fmt.Fprintf(w, "\n  %s%sUpdate available!%s Run %scarto upgrade%s to install.\n",
					bold, gold, reset, bold, reset)
			} else {
				fmt.Fprintf(w, "\n  %s%sYou are up to date.%s\n", bold, green, reset)
			}
		})
		logAuditEvent(cmd, "ok", "", map[string]any{
			"current": result.Current,
			"latest":  result.Latest,
			"action":  "check",
		})
		return nil
	}

	// Upgrade flow (stub for now).
	if !confirmAction(cmd, fmt.Sprintf("Upgrade Carto from %s to %s?", version, latest)) {
		printError("upgrade cancelled")
		return nil
	}

	// TODO: implement actual binary download and replacement.
	fmt.Fprintf(cmd.ErrOrStderr(), "\n  %s%sUpgrade not yet implemented.%s\n", bold, amber, reset)
	fmt.Fprintf(cmd.ErrOrStderr(), "  Download the latest version from: %shttps://github.com/divyekant/carto/releases%s\n", bold, reset)

	writeEnvelopeHuman(cmd, result, nil, nil)
	logAuditEvent(cmd, "ok", "", map[string]any{
		"current": result.Current,
		"latest":  result.Latest,
		"action":  "upgrade-stub",
	})
	return nil
}
