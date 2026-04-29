package cli

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
)

var featuresCmd = &cobra.Command{
	Use:   "features",
	Short: "List TUI feature flags and their status",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("TUI features (internal/config/features.json):")
		features := config.TUIFeatures()
		keys := make([]string, 0, len(features))
		for k := range features {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			status := "enabled"
			if !features[k] {
				status = "disabled (alpha)"
			}
			fmt.Printf("  %-12s %s\n", k, status)
		}
	},
}
