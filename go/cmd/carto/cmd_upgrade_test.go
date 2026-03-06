package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// =========================================================================
// Test helpers
// =========================================================================

// newRootWithUpgrade builds a minimal root command with the persistent flags
// needed by isJSONMode, confirmAction, and logAuditEvent, then attaches the
// upgrade subcommand.
func newRootWithUpgrade() *cobra.Command {
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("pretty", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	root.PersistentFlags().BoolP("verbose", "v", false, "")
	root.PersistentFlags().String("log-file", "", "")
	root.PersistentFlags().String("profile", "", "")
	root.AddCommand(upgradeCmd())
	return root
}

// mockGitHubRelease starts an httptest server that returns a GitHub-style
// release JSON with the given tag. It overrides githubReleaseURL for the
// duration of the test.
func mockGitHubRelease(t *testing.T, tag string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"tag_name": "%s"}`, tag)
	}))
	t.Cleanup(srv.Close)

	orig := githubReleaseURL
	githubReleaseURL = srv.URL
	t.Cleanup(func() { githubReleaseURL = orig })
}

// mockGitHubReleaseError starts an httptest server that returns a non-200
// status code. It overrides githubReleaseURL for the duration of the test.
func mockGitHubReleaseError(t *testing.T, statusCode int) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(statusCode)
	}))
	t.Cleanup(srv.Close)

	orig := githubReleaseURL
	githubReleaseURL = srv.URL
	t.Cleanup(func() { githubReleaseURL = orig })
}

// =========================================================================
// compareVersions — table-driven
// =========================================================================

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name string
		a, b string
		want int
	}{
		// Equal versions
		{"equal", "1.1.0", "1.1.0", 0},
		{"equal with v prefix", "v1.1.0", "v1.1.0", 0},
		{"equal mixed prefix", "v1.1.0", "1.1.0", 0},

		// Newer (a > b)
		{"newer patch", "1.1.1", "1.1.0", 1},
		{"newer minor", "1.2.0", "1.1.0", 1},
		{"newer major", "2.0.0", "1.9.9", 1},

		// Older (a < b)
		{"older patch", "1.1.0", "1.1.1", -1},
		{"older minor", "1.0.0", "1.1.0", -1},
		{"older major", "1.9.9", "2.0.0", -1},

		// With v prefix
		{"v prefix newer", "v2.0.0", "v1.0.0", 1},
		{"v prefix older", "v1.0.0", "v2.0.0", -1},

		// Different segment counts (short versions)
		{"short a", "1.0", "1.0.0", 0},
		{"short b", "1.0.0", "1.0", 0},
		{"short a less", "1.0", "1.0.1", -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := compareVersions(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

// =========================================================================
// carto upgrade --check — no update available
// =========================================================================

func TestUpgradeCmd_Check_NoUpdate(t *testing.T) {
	// Mock GitHub returning the same version as current.
	mockGitHubRelease(t, "v"+version)

	root := newRootWithUpgrade()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"upgrade", "--check", "--pretty"})

	if err := root.Execute(); err != nil {
		t.Fatalf("upgrade --check failed: %v", err)
	}

	combined := stderr.String()
	if !strings.Contains(combined, "up to date") {
		t.Errorf("expected 'up to date' in output, got:\n%s", combined)
	}
	if !strings.Contains(combined, version) {
		t.Errorf("expected current version %q in output, got:\n%s", version, combined)
	}
}

// =========================================================================
// carto upgrade --check — update available
// =========================================================================

func TestUpgradeCmd_Check_UpdateAvailable(t *testing.T) {
	// Mock GitHub returning a newer version.
	mockGitHubRelease(t, "v99.0.0")

	root := newRootWithUpgrade()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"upgrade", "--check", "--pretty"})

	if err := root.Execute(); err != nil {
		t.Fatalf("upgrade --check failed: %v", err)
	}

	combined := stderr.String()
	if !strings.Contains(combined, "Update available") {
		t.Errorf("expected 'Update available' in output, got:\n%s", combined)
	}
	if !strings.Contains(combined, "99.0.0") {
		t.Errorf("expected latest version '99.0.0' in output, got:\n%s", combined)
	}
}

// =========================================================================
// carto upgrade --check --json — envelope fields
// =========================================================================

func TestUpgradeCmd_Check_JSONEnvelope(t *testing.T) {
	mockGitHubRelease(t, "v99.0.0")

	root := newRootWithUpgrade()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"upgrade", "--check", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("upgrade --check --json failed: %v", err)
	}

	var env struct {
		OK   bool          `json:"ok"`
		Data upgradeResult `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse JSON envelope: %v\nraw: %s", err, stdout.String())
	}

	if !env.OK {
		t.Error("expected ok:true in envelope")
	}
	if env.Data.Current != version {
		t.Errorf("expected current=%q, got %q", version, env.Data.Current)
	}
	if env.Data.Latest != "99.0.0" {
		t.Errorf("expected latest=%q, got %q", "99.0.0", env.Data.Latest)
	}
	if !env.Data.UpdateAvailable {
		t.Error("expected update_available:true")
	}
}

// =========================================================================
// carto upgrade --check --json — no update envelope
// =========================================================================

func TestUpgradeCmd_Check_JSONEnvelope_NoUpdate(t *testing.T) {
	mockGitHubRelease(t, "v"+version)

	root := newRootWithUpgrade()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"upgrade", "--check", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("upgrade --check --json failed: %v", err)
	}

	var env struct {
		OK   bool          `json:"ok"`
		Data upgradeResult `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("failed to parse JSON envelope: %v\nraw: %s", err, stdout.String())
	}

	if !env.OK {
		t.Error("expected ok:true")
	}
	if env.Data.UpdateAvailable {
		t.Error("expected update_available:false when versions match")
	}
	if env.Data.Current != env.Data.Latest {
		t.Errorf("expected current == latest, got %q vs %q", env.Data.Current, env.Data.Latest)
	}
}

// =========================================================================
// carto upgrade — connection error
// =========================================================================

func TestUpgradeCmd_ConnectionError(t *testing.T) {
	mockGitHubReleaseError(t, http.StatusInternalServerError)

	root := newRootWithUpgrade()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"upgrade", "--check", "--pretty"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when GitHub returns 500")
	}

	// The error should be a connection error.
	ce := toCliError(err)
	if ce.code != ErrCodeConnection {
		t.Errorf("expected error code %q, got %q", ErrCodeConnection, ce.code)
	}
}

// =========================================================================
// carto upgrade (no --check) — stub path, no update
// =========================================================================

func TestUpgradeCmd_NoUpdate_SkipsStub(t *testing.T) {
	// When already up to date, the stub path should not be reached.
	mockGitHubRelease(t, "v"+version)

	root := newRootWithUpgrade()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{"upgrade", "--pretty"})

	if err := root.Execute(); err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	combined := stderr.String()
	if strings.Contains(combined, "not yet implemented") {
		t.Error("should not show stub message when already up to date")
	}
	if !strings.Contains(combined, "up to date") {
		t.Errorf("expected 'up to date' message, got:\n%s", combined)
	}
}

// =========================================================================
// fetchLatestVersion — valid response
// =========================================================================

func TestFetchLatestVersion_ParsesTagName(t *testing.T) {
	mockGitHubRelease(t, "v2.3.4")

	got, err := fetchLatestVersion()
	if err != nil {
		t.Fatalf("fetchLatestVersion: %v", err)
	}
	if got != "2.3.4" {
		t.Errorf("expected %q, got %q", "2.3.4", got)
	}
}

func TestFetchLatestVersion_StripsVPrefix(t *testing.T) {
	mockGitHubRelease(t, "v10.0.1")

	got, err := fetchLatestVersion()
	if err != nil {
		t.Fatalf("fetchLatestVersion: %v", err)
	}
	if strings.HasPrefix(got, "v") {
		t.Errorf("expected v prefix to be stripped, got %q", got)
	}
}

func TestFetchLatestVersion_ErrorOnNon200(t *testing.T) {
	mockGitHubReleaseError(t, http.StatusNotFound)

	_, err := fetchLatestVersion()
	if err == nil {
		t.Fatal("expected error on 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Errorf("expected error to mention status code 404, got: %v", err)
	}
}
