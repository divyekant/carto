package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func completionsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "completions <bash|zsh|fish|powershell>",
		Short: "Generate shell completion scripts",
		Long: `Generate autocompletion scripts for your shell.

To load completions:

  bash:  source <(carto completions bash)
  zsh:   carto completions zsh > "${fpath[1]}/_carto"
  fish:  carto completions fish > ~/.config/fish/completions/carto.fish`,
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "zsh", "fish", "powershell"},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			switch args[0] {
			case "bash":
				return root.GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell: %s (use bash, zsh, fish, or powershell)", args[0])
			}
		},
	}
}
