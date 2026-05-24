package main

import (
	"fmt"
	"strings"

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
	cmd.Flags().Float64("graph-weight", 0.1, "Graph traversal weight (0-1)")
	cmd.Flags().Float64("confidence-weight", 0.0, "Confidence decay weight (0-1)")
	cmd.Flags().Float64("feedback-weight", 0.1, "Feedback signal weight (0-1)")
	cmd.Flags().String("since", "", "Filter atoms after date (ISO 8601)")
	cmd.Flags().String("until", "", "Filter atoms before date (ISO 8601)")
	return cmd
}

func runQuery(cmd *cobra.Command, args []string) error {
	query := args[0]

	project, _ := cmd.Flags().GetString("project")
	tier, _ := cmd.Flags().GetString("tier")
	count, _ := cmd.Flags().GetInt("count")
	graphWeight, _ := cmd.Flags().GetFloat64("graph-weight")
	confidenceWeight, _ := cmd.Flags().GetFloat64("confidence-weight")
	feedbackWeight, _ := cmd.Flags().GetFloat64("feedback-weight")
	since, _ := cmd.Flags().GetString("since")
	until, _ := cmd.Flags().GetString("until")

	cfg := config.Load()
	memoriesClient := storage.NewMemoriesClient(cfg.MemoriesURL, cfg.MemoriesKey)

	// If a project is provided, run semantic search scoped to that project's
	// Carto source prefix. The tier narrows preferred layers but falls back to
	// any project result when those layers are partial or missing.
	if project != "" {
		storageTier := storage.Tier(tier)
		allowedLayers, err := queryLayersForTier(storageTier)
		if err != nil {
			return err
		}

		searchK := count * 3
		if searchK < count {
			searchK = count
		}
		results, err := memoriesClient.SearchAdvanced(query, storage.SearchOptions{
			K:                searchK,
			Hybrid:           true,
			SourcePrefix:     fmt.Sprintf("carto/%s/", project),
			GraphWeight:      graphWeight,
			ConfidenceWeight: confidenceWeight,
			FeedbackWeight:   feedbackWeight,
			Since:            since,
			Until:            until,
		})
		if err != nil {
			return fmt.Errorf("search project: %w", err)
		}
		filtered := filterQueryResultsByLayer(results, allowedLayers)
		if len(filtered) == 0 {
			filtered = results
		}
		if len(filtered) > count {
			filtered = filtered[:count]
		}

		writeEnvelopeHuman(cmd, filtered, nil, func() {
			fmt.Printf("%s%sResults for project %q (tier: %s)%s\n\n", bold, gold, project, tier, reset)

			if len(filtered) == 0 {
				fmt.Println("  No results found.")
				return
			}
			for i, entry := range filtered {
				snippet := truncateText(entry.Text, 200)
				fmt.Printf("%s%d.%s %ssource:%s %s  %sscore:%s %.4f\n", bold, i+1, reset, gold, reset, entry.Source, gold, reset, entry.Score)
				fmt.Printf("   %s\n\n", snippet)
			}
		})
		return nil
	}

	// Free-form search across all projects.
	results, err := memoriesClient.SearchAdvanced(query, storage.SearchOptions{
		K:                count,
		Hybrid:           true,
		GraphWeight:      graphWeight,
		ConfidenceWeight: confidenceWeight,
		FeedbackWeight:   feedbackWeight,
		Since:            since,
		Until:            until,
	})
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	writeEnvelopeHuman(cmd, results, nil, func() {
		fmt.Printf("%s%sSearch results for: %q%s (k=%d)\n\n", bold, gold, query, reset, count)

		if len(results) == 0 {
			fmt.Println("  No results found.")
			return
		}

		for i, r := range results {
			snippet := truncateText(r.Text, 200)
			graphTag := ""
			if r.MatchType == "graph" {
				graphTag = fmt.Sprintf(" %s[graph]%s", amber, reset)
			}
			fmt.Printf("%s%d.%s %ssource:%s %s  %sscore:%s %.4f%s\n", bold, i+1, reset, gold, reset, r.Source, gold, reset, r.Score, graphTag)
			fmt.Printf("   %s\n\n", snippet)
		}
	})

	return nil
}

func queryLayersForTier(tier storage.Tier) (map[string]bool, error) {
	switch tier {
	case storage.TierMini:
		return map[string]bool{
			storage.LayerZones:     true,
			storage.LayerBlueprint: true,
			storage.LayerPatterns:  true,
		}, nil
	case storage.TierStandard:
		return map[string]bool{
			storage.LayerZones:     true,
			storage.LayerBlueprint: true,
			storage.LayerPatterns:  true,
			storage.LayerAtoms:     true,
			storage.LayerWiring:    true,
		}, nil
	case storage.TierFull:
		return map[string]bool{
			storage.LayerZones:     true,
			storage.LayerBlueprint: true,
			storage.LayerPatterns:  true,
			storage.LayerAtoms:     true,
			storage.LayerWiring:    true,
			storage.LayerHistory:   true,
			storage.LayerSignals:   true,
		}, nil
	default:
		return nil, fmt.Errorf("unknown tier: %s", tier)
	}
}

func filterQueryResultsByLayer(results []storage.SearchResult, allowed map[string]bool) []storage.SearchResult {
	filtered := make([]storage.SearchResult, 0, len(results))
	for _, r := range results {
		layer := layerFromSource(r.Source)
		if allowed[layer] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

func layerFromSource(source string) string {
	idx := strings.LastIndex(source, "/layer:")
	if idx == -1 {
		return ""
	}
	return source[idx+len("/layer:"):]
}
