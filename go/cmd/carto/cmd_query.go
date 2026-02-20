package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/divyekant/carto/internal/config"
	"github.com/divyekant/carto/internal/storage"
)

func queryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "query <question>",
		Short: "Query the indexed codebase",
		Args:  cobra.ExactArgs(1),
		RunE:  runQuery,
	}
	cmd.Flags().String("project", "", "Project name to search within")
	cmd.Flags().String("tier", "standard", "Context tier: mini, standard, full")
	cmd.Flags().IntP("count", "k", 10, "Number of results")
	return cmd
}

func runQuery(cmd *cobra.Command, args []string) error {
	query := args[0]

	project, _ := cmd.Flags().GetString("project")
	tier, _ := cmd.Flags().GetString("tier")
	count, _ := cmd.Flags().GetInt("count")

	cfg := config.Load()
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// If a project is provided, try tier-based retrieval.
	if project != "" {
		store := storage.NewStore(memoriesClient, project)

		storageTier := storage.Tier(tier)
		results, err := store.RetrieveByTier(query, storageTier)
		if err != nil {
			return fmt.Errorf("retrieve by tier: %w", err)
		}

		writeOutput(cmd, results, func() {
			fmt.Printf("%s%sResults for project %q (tier: %s)%s\n\n", bold, cyan, project, tier, reset)

			for layer, entries := range results {
				if len(entries) == 0 {
					continue
				}
				fmt.Printf("%s%s[%s]%s\n", bold, yellow, layer, reset)
				for _, entry := range entries {
					snippet := truncateText(entry.Text, 200)
					fmt.Printf("  %ssource:%s %s\n", cyan, reset, entry.Source)
					fmt.Printf("  %sscore:%s  %.4f\n", cyan, reset, entry.Score)
					fmt.Printf("  %s\n\n", snippet)
				}
			}
		})
		return nil
	}

	// Free-form search across all projects.
	results, err := memoriesClient.Search(query, storage.SearchOptions{
		K:      count,
		Hybrid: true,
	})
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	writeOutput(cmd, results, func() {
		fmt.Printf("%s%sSearch results for: %q%s (k=%d)\n\n", bold, cyan, query, reset, count)

		if len(results) == 0 {
			fmt.Println("  No results found.")
			return
		}

		for i, r := range results {
			snippet := truncateText(r.Text, 200)
			fmt.Printf("%s%d.%s %ssource:%s %s  %sscore:%s %.4f\n", bold, i+1, reset, cyan, reset, r.Source, cyan, reset, r.Score)
			fmt.Printf("   %s\n\n", snippet)
		}
	})

	return nil
}
