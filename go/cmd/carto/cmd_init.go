package main

// cmd_init.go — interactive and non-interactive configuration wizard.
//
// `carto init` sets up the essential configuration values (LLM provider,
// API key, Memories URL) via interactive prompts or flags for automation.
// It persists the result via config.Save and emits an envelope summary.

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
)

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize Carto configuration",
		Long: `Set up Carto configuration interactively or via flags for automation.

Interactive mode prompts for each setting. Non-interactive mode (--non-interactive)
requires all values via flags or environment variables.`,
		RunE: runInit,
	}

	cmd.Flags().Bool("non-interactive", false, "Skip prompts, use flags and env vars only")
	cmd.Flags().String("llm-provider", "", "LLM provider (anthropic, openai, ollama)")
	cmd.Flags().String("api-key", "", "LLM API key")
	cmd.Flags().String("memories-url", "", "Memories server URL")
	cmd.Flags().String("memories-key", "", "Memories API key")
	cmd.Flags().String("projects-dir", "", "Directory for indexed projects")

	return cmd
}

// initResult is the envelope data returned on success.
type initResult struct {
	ConfigPath  string `json:"config_path"`
	Provider    string `json:"provider"`
	MemoriesURL string `json:"memories_url"`
}

func runInit(cmd *cobra.Command, _ []string) error {
	nonInteractive, _ := cmd.Flags().GetBool("non-interactive")
	flagProvider, _ := cmd.Flags().GetString("llm-provider")
	flagAPIKey, _ := cmd.Flags().GetString("api-key")
	flagMemURL, _ := cmd.Flags().GetString("memories-url")
	flagMemKey, _ := cmd.Flags().GetString("memories-key")
	flagProjDir, _ := cmd.Flags().GetString("projects-dir")

	// Load current config for defaults.
	cfg := config.Load()

	// Ensure ConfigPath is set so Save() has somewhere to write.
	cfgPath := config.ConfigPath
	if cfgPath == "" {
		cfgPath = config.DefaultConfigFilePath()
		config.ConfigPath = cfgPath
	}

	if nonInteractive {
		return runInitNonInteractive(cmd, cfg, cfgPath, flagProvider, flagAPIKey, flagMemURL, flagMemKey, flagProjDir)
	}
	return runInitInteractive(cmd, cfg, cfgPath, flagProvider, flagAPIKey, flagMemURL, flagMemKey, flagProjDir)
}

// ── Non-interactive mode ────────────────────────────────────────────────────

func runInitNonInteractive(cmd *cobra.Command, cfg config.Config, cfgPath, provider, apiKey, memURL, memKey, projDir string) error {
	if apiKey == "" {
		return newConfigError("--api-key is required in non-interactive mode")
	}

	if provider != "" {
		cfg.LLMProvider = provider
	}
	cfg.LLMApiKey = apiKey
	if memURL != "" {
		cfg.MemoriesURL = memURL
	}
	if memKey != "" {
		cfg.MemoriesKey = memKey
	}

	if err := ensureConfigDir(cfgPath); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return newConfigError("failed to write config: " + err.Error())
	}

	if projDir != "" {
		os.Setenv("PROJECTS_DIR", projDir)
	}

	result := initResult{
		ConfigPath:  cfgPath,
		Provider:    cfg.LLMProvider,
		MemoriesURL: cfg.MemoriesURL,
	}

	writeEnvelopeHuman(cmd, result, nil, func() {
		printInitSummary(cmd, cfg, cfgPath)
	})

	logAuditEvent(cmd, "ok", "", map[string]any{"mode": "non-interactive"})
	return nil
}

// ── Interactive mode ────────────────────────────────────────────────────────

func runInitInteractive(cmd *cobra.Command, cfg config.Config, cfgPath, flagProvider, flagAPIKey, flagMemURL, flagMemKey, flagProjDir string) error {
	w := cmd.ErrOrStderr()
	fmt.Fprintf(w, "\n%s%sCarto Init%s\n\n", bold, gold, reset)
	fmt.Fprintf(w, "  This wizard will set up your Carto configuration.\n")
	fmt.Fprintf(w, "  Press Enter to accept the default value shown in [brackets].\n\n")

	// LLM provider
	providerDefault := cfg.LLMProvider
	if flagProvider != "" {
		providerDefault = flagProvider
	}
	provider := promptValue(cmd, "LLM provider (anthropic, openai, ollama)", providerDefault)

	// API key
	keyDefault := cfg.EffectiveAPIKey()
	if flagAPIKey != "" {
		keyDefault = flagAPIKey
	}
	displayDefault := ""
	if keyDefault != "" {
		displayDefault = config.MaskSecret(keyDefault)
	}
	apiKey := promptValue(cmd, "API key", displayDefault)
	// If user just pressed Enter on a masked default, keep the original key.
	if apiKey == displayDefault || apiKey == "" {
		apiKey = keyDefault
	}

	// Memories URL
	memURLDefault := cfg.MemoriesURL
	if flagMemURL != "" {
		memURLDefault = flagMemURL
	}
	memURL := promptValue(cmd, "Memories server URL", memURLDefault)

	// Memories key
	memKeyDefault := cfg.MemoriesKey
	if flagMemKey != "" {
		memKeyDefault = flagMemKey
	}
	memKeyDisplay := ""
	if memKeyDefault != "" {
		memKeyDisplay = config.MaskSecret(memKeyDefault)
	}
	memKey := promptValue(cmd, "Memories API key", memKeyDisplay)
	if memKey == memKeyDisplay || memKey == "" {
		memKey = memKeyDefault
	}

	// Projects directory
	projDirDefault := os.Getenv("PROJECTS_DIR")
	if flagProjDir != "" {
		projDirDefault = flagProjDir
	}
	projDir := promptValue(cmd, "Projects directory", projDirDefault)

	// Apply values.
	cfg.LLMProvider = provider
	if apiKey != "" {
		cfg.LLMApiKey = apiKey
	}
	cfg.MemoriesURL = memURL
	if memKey != "" {
		cfg.MemoriesKey = memKey
	}

	if err := ensureConfigDir(cfgPath); err != nil {
		return err
	}
	if err := config.Save(cfg); err != nil {
		return newConfigError("failed to write config: " + err.Error())
	}

	if projDir != "" {
		os.Setenv("PROJECTS_DIR", projDir)
	}

	fmt.Fprintln(w)

	result := initResult{
		ConfigPath:  cfgPath,
		Provider:    cfg.LLMProvider,
		MemoriesURL: cfg.MemoriesURL,
	}

	writeEnvelopeHuman(cmd, result, nil, func() {
		printInitSummary(cmd, cfg, cfgPath)
	})

	logAuditEvent(cmd, "ok", "", map[string]any{"mode": "interactive"})
	return nil
}

// ── Prompt helper ───────────────────────────────────────────────────────────

func promptValue(cmd *cobra.Command, label, defaultVal string) string {
	w := cmd.ErrOrStderr()
	if defaultVal != "" {
		fmt.Fprintf(w, "  %s%s%s [%s]: ", bold, label, reset, defaultVal)
	} else {
		fmt.Fprintf(w, "  %s%s%s: ", bold, label, reset)
	}
	scanner := bufio.NewScanner(cmd.InOrStdin())
	if scanner.Scan() {
		val := strings.TrimSpace(scanner.Text())
		if val != "" {
			return val
		}
	}
	return defaultVal
}

// ── Summary printer ─────────────────────────────────────────────────────────

func printInitSummary(cmd *cobra.Command, cfg config.Config, cfgPath string) {
	w := cmd.ErrOrStderr()
	fmt.Fprintf(w, "%s%sConfiguration written%s to %s\n\n", bold, gold, reset, cfgPath)
	fmt.Fprintf(w, "  %-20s %s\n", "LLM provider:", cfg.LLMProvider)
	fmt.Fprintf(w, "  %-20s %s\n", "API key:", config.MaskSecret(cfg.LLMApiKey))
	fmt.Fprintf(w, "  %-20s %s\n", "Memories URL:", cfg.MemoriesURL)
	if cfg.MemoriesKey != "" {
		fmt.Fprintf(w, "  %-20s %s\n", "Memories key:", config.MaskSecret(cfg.MemoriesKey))
	}
	fmt.Fprintf(w, "\n  Run %scarto doctor%s to verify your setup.\n", bold, reset)
}

// ── Filesystem helper ───────────────────────────────────────────────────────

func ensureConfigDir(cfgPath string) error {
	dir := filepath.Dir(cfgPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return newConfigError("cannot create config directory: " + err.Error())
	}
	return nil
}
