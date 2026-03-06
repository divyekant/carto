package main

// cmd_version.go — structured version information command.
//
// `carto version` emits build metadata in human-readable or JSON format.
// The JSON form is useful for CI pipelines that need to check the deployed
// version against an expected release tag.
//
// Usage:
//
//	carto version          # human-readable
//	carto version --json   # machine-readable JSON

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func versionCmd(ver string) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show build version and runtime information",
		Long: `Print the Carto build version along with Go runtime metadata.

The --json flag emits a machine-readable JSON object suitable for CI pipelines:

  {
    "version":    "1.2.3",
    "go_version": "go1.25.1",
    "os":         "linux",
    "arch":       "amd64"
  }`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			type versionInfo struct {
				Version   string `json:"version"`
				GoVersion string `json:"go_version"`
				OS        string `json:"os"`
				Arch      string `json:"arch"`
			}

			info := versionInfo{
				Version:   ver,
				GoVersion: runtime.Version(),
				OS:        runtime.GOOS,
				Arch:      runtime.GOARCH,
			}

			writeEnvelopeHuman(cmd, info, nil, func() {
				fmt.Printf("%s%scarto%s %s\n", bold, gold, reset, ver)
				fmt.Printf("  go:   %s\n", info.GoVersion)
				fmt.Printf("  os:   %s/%s\n", info.OS, info.Arch)
			})

			logAuditEvent(cmd, "ok", "", map[string]any{"version": ver})
			return nil
		},
	}
}

