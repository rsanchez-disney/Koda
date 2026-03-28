package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/ops"
)

var autoUpdateCmd = &cobra.Command{
	Use:   "auto-update",
	Short: "Manage daily auto-update (enable/disable/status)",
}

var autoUpdateEnableCmd = &cobra.Command{
	Use:   "enable",
	Short: "Register daily sync at 9:00 AM",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ops.EnableAutoUpdate(); err != nil {
			return err
		}
		fmt.Println("\u2705 Auto-update enabled (daily at 9:00 AM)")
		return nil
	},
}

var autoUpdateDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Remove daily sync job",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ops.DisableAutoUpdate(); err != nil {
			return err
		}
		fmt.Println("\u2705 Auto-update disabled")
		return nil
	},
}

var autoUpdateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Check auto-update status",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Auto-update: %s\n", ops.AutoUpdateStatus())
	},
}

func init() {
	autoUpdateCmd.AddCommand(autoUpdateEnableCmd)
	autoUpdateCmd.AddCommand(autoUpdateDisableCmd)
	autoUpdateCmd.AddCommand(autoUpdateStatusCmd)
}
