package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var statsDays int

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show prompt scoring and token usage stats",
	Run: func(cmd *cobra.Command, args []string) {
		if !config.IsTUIEnabled("scorer") {
			fmt.Println("⚠ stats is not available yet. Enable 'scorer' in internal/config/features.json")
			return
		}
		ops.PrintStats(statsDays)
	},
}

func init() {
	statsCmd.Flags().IntVar(&statsDays, "days", 7, "Number of days to include")
}
