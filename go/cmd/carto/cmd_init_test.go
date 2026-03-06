package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
)

// newRootWithInit builds a minimal root command with the persistent flags
// that the output layer expects, then attaches initCmd() as a subcommand.
func newRootWithInit() *cobra.Command {
	root := &cobra.Command{Use: "carto"}
	root.PersistentFlags().Bool("json", false, "")
	root.PersistentFlags().Bool("pretty", false, "")
	root.PersistentFlags().BoolP("yes", "y", false, "")
	root.PersistentFlags().BoolP("verbose", "v", false, "")
	root.PersistentFlags().String("log-file", "", "")
	root.PersistentFlags().String("profile", "", "")
	root.AddCommand(initCmd())
	return root
}

// =========================================================================
// Non-interactive: writes config with all flags
// =========================================================================

func TestInitCmd_NonInteractive_WritesConfig(t *testing.T) {
	withCleanEnv(t)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	origPath := config.ConfigPath
	config.ConfigPath = cfgPath
	t.Cleanup(func() { config.ConfigPath = origPath })

	root := newRootWithInit()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init",
		"--non-interactive",
		"--llm-provider", "anthropic",
		"--api-key", "sk-ant-test-key-abcdef1234567890",
		"--memories-url", "http://localhost:8900",
		"--memories-key", "mem-key-xyz",
		"--projects-dir", dir,
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("init --non-interactive failed: %v", err)
	}

	// Verify config file was created.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	// Verify key values persisted.
	content := string(data)
	if !strings.Contains(content, "anthropic") {
		t.Error("config file missing llm_provider 'anthropic'")
	}
	if !strings.Contains(content, "sk-ant-test-key-abcdef1234567890") {
		t.Error("config file missing api key")
	}
	if !strings.Contains(content, "http://localhost:8900") {
		t.Error("config file missing memories_url")
	}
	if !strings.Contains(content, "mem-key-xyz") {
		t.Error("config file missing memories_key")
	}
}

// =========================================================================
// Non-interactive: missing --api-key errors
// =========================================================================

func TestInitCmd_NonInteractive_MissingAPIKey_Errors(t *testing.T) {
	withCleanEnv(t)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	origPath := config.ConfigPath
	config.ConfigPath = cfgPath
	t.Cleanup(func() { config.ConfigPath = origPath })

	root := newRootWithInit()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init",
		"--non-interactive",
		"--llm-provider", "anthropic",
	})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when --api-key is missing in non-interactive mode")
	}
	if !strings.Contains(err.Error(), "--api-key is required") {
		t.Errorf("expected '--api-key is required' error, got: %v", err)
	}
}

// =========================================================================
// Non-interactive: JSON envelope output
// =========================================================================

func TestInitCmd_NonInteractive_JSONOutput(t *testing.T) {
	withCleanEnv(t)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	origPath := config.ConfigPath
	config.ConfigPath = cfgPath
	t.Cleanup(func() { config.ConfigPath = origPath })

	root := newRootWithInit()
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs([]string{
		"--json",
		"init",
		"--non-interactive",
		"--api-key", "sk-ant-test-json-key-abcdef12345",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("init --non-interactive --json failed: %v", err)
	}

	// Parse the JSON envelope from stdout.
	var env struct {
		OK   bool `json:"ok"`
		Data struct {
			ConfigPath  string `json:"config_path"`
			Provider    string `json:"provider"`
			MemoriesURL string `json:"memories_url"`
		} `json:"data"`
	}
	if jsonErr := json.Unmarshal(stdout.Bytes(), &env); jsonErr != nil {
		t.Fatalf("not valid JSON: %v\nraw: %s", jsonErr, stdout.String())
	}
	if !env.OK {
		t.Error("expected ok=true in envelope")
	}
	if env.Data.ConfigPath == "" {
		t.Error("expected config_path in envelope data")
	}
	if env.Data.Provider == "" {
		t.Error("expected provider in envelope data")
	}
}

// =========================================================================
// Non-interactive: creates config directory if missing
// =========================================================================

func TestInitCmd_NonInteractive_CreatesConfigDir(t *testing.T) {
	withCleanEnv(t)

	dir := t.TempDir()
	nestedPath := filepath.Join(dir, "nested", "deep", "config.json")

	origPath := config.ConfigPath
	config.ConfigPath = nestedPath
	t.Cleanup(func() { config.ConfigPath = origPath })

	root := newRootWithInit()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init",
		"--non-interactive",
		"--api-key", "sk-ant-test-dir-create-abcdef1234",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	// Verify the nested directory was created.
	if _, err := os.Stat(filepath.Dir(nestedPath)); os.IsNotExist(err) {
		t.Error("expected nested config directory to be created")
	}

	// Verify config file exists.
	if _, err := os.Stat(nestedPath); os.IsNotExist(err) {
		t.Error("expected config file to be written")
	}
}

// =========================================================================
// Non-interactive: partial flags use defaults from config.Load
// =========================================================================

func TestInitCmd_NonInteractive_PartialFlags_UsesDefaults(t *testing.T) {
	withCleanEnv(t)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.json")

	origPath := config.ConfigPath
	config.ConfigPath = cfgPath
	t.Cleanup(func() { config.ConfigPath = origPath })

	// Only provide --api-key; provider and memories-url should come from defaults.
	root := newRootWithInit()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{
		"init",
		"--non-interactive",
		"--api-key", "sk-ant-test-partial-abcdef123456",
	})

	if err := root.Execute(); err != nil {
		t.Fatalf("init with partial flags failed: %v", err)
	}

	// Read back config and verify defaults were applied.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("config file not written: %v", err)
	}

	content := string(data)
	// Default provider is "anthropic".
	if !strings.Contains(content, "anthropic") {
		t.Error("config should contain default provider 'anthropic'")
	}
	// Default memories URL.
	if !strings.Contains(content, "http://localhost:8900") {
		t.Error("config should contain default memories_url")
	}
}
