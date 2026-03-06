package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
)

func configCmdGroup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and update Carto configuration",
		Long: `Manage Carto's runtime configuration.

Settings are loaded from environment variables first, then overlaid with
values from the persisted config file (~/.config/carto/config.json or the
file set by the serve --projects-dir flag).`,
	}
	cmd.AddCommand(configGetCmd())
	cmd.AddCommand(configSetCmd())
	cmd.AddCommand(configValidateCmd())
	cmd.AddCommand(configPathCmd())
	return cmd
}

func configGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [key]",
		Short: "Show configuration values",
		Args:  cobra.MaximumNArgs(1),
		RunE:  runConfigGet,
	}
}

func runConfigGet(cmd *cobra.Command, args []string) error {
	cfg := config.Load()
	profile := resolveProfile(cmd)

	verboseLog(cmd, "loading config for profile %q, file: %q", profile, config.ConfigPath)

	// Non-sensitive config fields for display.
	// Sensitive fields (keys/tokens) are never printed in plain text.
	configMap := map[string]string{
		"memories_url":     cfg.MemoriesURL,
		"fast_model":       cfg.FastModel,
		"deep_model":       cfg.DeepModel,
		"max_concurrent":   fmt.Sprintf("%d", cfg.MaxConcurrent),
		"fast_max_tokens":  fmt.Sprintf("%d", cfg.FastMaxTokens),
		"deep_max_tokens":  fmt.Sprintf("%d", cfg.DeepMaxTokens),
		"llm_provider":     cfg.LLMProvider,
		"llm_base_url":     cfg.LLMBaseURL,
		"profile":          profile,
		"audit_log":        cfg.AuditLogFile,
		// Show credential presence (masked, not the actual values).
		"anthropic_key":    maskPresence(cfg.AnthropicKey),
		"llm_api_key":      maskPresence(cfg.LLMApiKey),
		"memories_key":     maskPresence(cfg.MemoriesKey),
		"github_token":     maskPresence(cfg.GitHubToken),
		"jira_token":       maskPresence(cfg.JiraToken),
		"linear_token":     maskPresence(cfg.LinearToken),
		"notion_token":     maskPresence(cfg.NotionToken),
		"slack_token":      maskPresence(cfg.SlackToken),
		"server_token":     maskPresence(cfg.ServerToken),
	}

	if len(args) == 1 {
		key := args[0]
		val, ok := configMap[key]
		if !ok {
			return fmt.Errorf("unknown config key: %q (run 'carto config get' for a full list)", key)
		}
		writeOutput(cmd, map[string]string{key: val}, func() {
			fmt.Printf("%s: %s\n", key, val)
		})
		return nil
	}

	writeOutput(cmd, configMap, func() {
		fmt.Printf("%s%sConfiguration%s  profile: %s\n\n", bold, gold, reset, profile)
		if config.ConfigPath != "" {
			fmt.Printf("  %sfile:%s %s\n\n", gold, reset, config.ConfigPath)
		}

		// ── Non-sensitive settings ────────────────────────────────────────
		fmt.Printf("  %s%sSettings%s\n", bold, gold, reset)
		settingKeys := []string{
			"llm_provider", "fast_model", "deep_model",
			"max_concurrent", "fast_max_tokens", "deep_max_tokens",
			"llm_base_url", "memories_url", "profile", "audit_log",
		}
		for _, k := range settingKeys {
			v := configMap[k]
			if v == "" {
				v = dimmed("(not set)")
			}
			fmt.Printf("  %-18s %s\n", k, v)
		}

		// ── Credential presence ───────────────────────────────────────────
		fmt.Printf("\n  %s%sCredentials%s  (masked — use 'carto auth status' for details)\n", bold, gold, reset)
		credKeys := []string{
			"anthropic_key", "llm_api_key", "memories_key",
			"github_token", "jira_token", "linear_token",
			"notion_token", "slack_token", "server_token",
		}
		for _, k := range credKeys {
			v := configMap[k]
			fmt.Printf("  %-18s %s\n", k, v)
		}
	})

	logAuditEvent(cmd, "ok", "", map[string]any{"profile": profile})
	return nil
}

// maskPresence returns a masked form of the key if set, or a dimmed placeholder.
func maskPresence(val string) string {
	if val == "" {
		return dimmed("(not set)")
	}
	return config.MaskSecret(val)
}

// dimmed returns the string wrapped in the warm-neutral stone colour for de-emphasis.
func dimmed(s string) string { return stone + s + reset }

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Long: `Set a non-secret configuration value in the persisted config file.

Writable keys:
  memories_url      URL for the Memories vector store
  fast_model        LLM model used for high-volume fast operations
  deep_model        LLM model used for low-volume deep analysis
  max_concurrent    Maximum concurrent LLM calls (integer ≥ 1)
  fast_max_tokens   Max output tokens for fast model calls (integer)
  deep_max_tokens   Max output tokens for deep model calls (integer)
  llm_provider      LLM provider: anthropic | openai | ollama
  llm_base_url      Base URL for OpenAI-compatible providers

Use 'carto auth set-key' to store API keys and tokens securely.`,
		Args: cobra.ExactArgs(2),
		RunE: runConfigSet,
	}
}

func runConfigSet(cmd *cobra.Command, args []string) error {
	key := args[0]
	value := args[1]

	cfg := config.Load()

	switch key {
	case "memories_url":
		cfg.MemoriesURL = value
	case "fast_model":
		cfg.FastModel = value
	case "deep_model":
		cfg.DeepModel = value
	case "max_concurrent":
		n, err := fmt.Sscanf(value, "%d", &cfg.MaxConcurrent)
		if n != 1 || err != nil {
			return fmt.Errorf("max_concurrent must be an integer")
		}
		if cfg.MaxConcurrent < 1 {
			return fmt.Errorf("max_concurrent must be ≥ 1")
		}
	case "fast_max_tokens":
		n, err := fmt.Sscanf(value, "%d", &cfg.FastMaxTokens)
		if n != 1 || err != nil {
			return fmt.Errorf("fast_max_tokens must be an integer")
		}
		if cfg.FastMaxTokens < 1 {
			return fmt.Errorf("fast_max_tokens must be ≥ 1")
		}
	case "deep_max_tokens":
		n, err := fmt.Sscanf(value, "%d", &cfg.DeepMaxTokens)
		if n != 1 || err != nil {
			return fmt.Errorf("deep_max_tokens must be an integer")
		}
		if cfg.DeepMaxTokens < 1 {
			return fmt.Errorf("deep_max_tokens must be ≥ 1")
		}
	case "llm_provider":
		cfg.LLMProvider = value
	case "llm_base_url":
		cfg.LLMBaseURL = value
	default:
		return fmt.Errorf("unknown or read-only config key: %q — run 'carto config get' for all keys, 'carto auth set-key' for credentials", key)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	writeOutput(cmd, map[string]string{key: value, "status": "saved"}, func() {
		fmt.Printf("%s✓%s Set %s = %s\n", green, reset, key, value)
	})
	logAuditEvent(cmd, "ok", "", map[string]any{"key": key})
	return nil
}

// ─── config validate ─────────────────────────────────────────────────────

func configValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the current configuration",
		Long: `Checks that all required settings are present and internally consistent.
Exit code is non-zero when validation fails.`,
		RunE: runConfigValidate,
	}
}

func runConfigValidate(cmd *cobra.Command, _ []string) error {
	cfg := config.Load()
	profile := resolveProfile(cmd)

	verboseLog(cmd, "validating config for profile %q", profile)

	err := cfg.Validate()

	type result struct {
		Profile string   `json:"profile"`
		Valid   bool     `json:"valid"`
		Errors  []string `json:"errors,omitempty"`
	}

	if err == nil {
		writeOutput(cmd, result{Profile: profile, Valid: true}, func() {
			fmt.Printf("%s✓%s Configuration is valid (profile: %s)\n", green, reset, profile)
		})
		logAuditEvent(cmd, "ok", "", map[string]any{"profile": profile})
		return nil
	}

	var errs []string
	if ve, ok := err.(*config.ValidationError); ok {
		errs = ve.Fields
	} else {
		errs = []string{err.Error()}
	}

	writeOutput(cmd, result{Profile: profile, Valid: false, Errors: errs}, func() {
		fmt.Printf("%s✗%s Configuration invalid (profile: %s)\n\n", red, reset, profile)
		for _, e := range errs {
			fmt.Printf("  • %s\n", e)
		}
		fmt.Printf("\nRun %scarto doctor%s for more actionable hints.\n", bold, reset)
	})

	logAuditEvent(cmd, "error", strings.Join(errs, "; "), nil)
	return fmt.Errorf("config validation failed: %d issue(s)", len(errs))
}

// ─── config path ─────────────────────────────────────────────────────────

func configPathCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Show the config file and directory paths",
		Long: `Prints the effective config directory (XDG) and config file path.
Useful for debugging which config file Carto is reading.`,
		RunE: runConfigPath,
	}
}

func runConfigPath(cmd *cobra.Command, _ []string) error {
	cfgDir := config.ConfigDir()
	cfgFile := config.DefaultConfigFilePath()
	active := config.ConfigPath

	type paths struct {
		ConfigDir     string `json:"config_dir"`
		DefaultFile   string `json:"default_file"`
		ActiveFile    string `json:"active_file,omitempty"`
		DirExists     bool   `json:"dir_exists"`
		FileExists    bool   `json:"file_exists"`
	}

	_, dirErr := os.Stat(cfgDir)
	_, fileErr := os.Stat(cfgFile)

	out := paths{
		ConfigDir:   cfgDir,
		DefaultFile: cfgFile,
		ActiveFile:  active,
		DirExists:   dirErr == nil,
		FileExists:  fileErr == nil,
	}

	writeOutput(cmd, out, func() {
		fmt.Printf("%s%sConfig Paths%s\n\n", bold, gold, reset)
		dirStatus := checkMark(out.DirExists)
		fileStatus := checkMark(out.FileExists)
		fmt.Printf("  %s Config dir:    %s\n", dirStatus, cfgDir)
		fmt.Printf("  %s Default file:  %s\n", fileStatus, cfgFile)
		if active != "" {
			fmt.Printf("  %s Active file:   %s\n", checkMark(true), active)
		} else {
			fmt.Printf("    Active file:   %s\n", dimmed("(none — using environment variables only)"))
		}
	})
	return nil
}
