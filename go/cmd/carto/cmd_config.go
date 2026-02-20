package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
)

func configCmdGroup() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "View and update Carto configuration",
	}
	cmd.AddCommand(configGetCmd())
	cmd.AddCommand(configSetCmd())
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

	// Non-sensitive config fields for display.
	configMap := map[string]string{
		"memories_url":   cfg.MemoriesURL,
		"fast_model":     cfg.FastModel,
		"deep_model":     cfg.DeepModel,
		"max_concurrent": fmt.Sprintf("%d", cfg.MaxConcurrent),
		"llm_provider":   cfg.LLMProvider,
		"llm_base_url":   cfg.LLMBaseURL,
	}

	if len(args) == 1 {
		key := args[0]
		val, ok := configMap[key]
		if !ok {
			return fmt.Errorf("unknown config key: %q", key)
		}
		writeOutput(cmd, map[string]string{key: val}, func() {
			fmt.Printf("%s: %s\n", key, val)
		})
		return nil
	}

	writeOutput(cmd, configMap, func() {
		fmt.Printf("%s%sConfiguration%s\n\n", bold, cyan, reset)
		// Print keys in a stable order.
		orderedKeys := []string{
			"memories_url", "fast_model", "deep_model",
			"max_concurrent", "llm_provider", "llm_base_url",
		}
		for _, k := range orderedKeys {
			v := configMap[k]
			if v == "" {
				v = "(not set)"
			}
			fmt.Printf("  %-18s %s\n", k, v)
		}
	})
	return nil
}

func configSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "set <key> <value>",
		Short: "Set a configuration value",
		Args:  cobra.ExactArgs(2),
		RunE:  runConfigSet,
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
	case "llm_provider":
		cfg.LLMProvider = value
	case "llm_base_url":
		cfg.LLMBaseURL = value
	default:
		return fmt.Errorf("unknown or read-only config key: %q", key)
	}

	if err := config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	writeOutput(cmd, map[string]string{key: value, "status": "saved"}, func() {
		fmt.Printf("%s✓%s Set %s = %s\n", green, reset, key, value)
	})
	return nil
}
