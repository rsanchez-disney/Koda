package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/tray"
)

var trayCmd = &cobra.Command{
	Use:   "tray",
	Short: "Launch Koda menu bar app",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		fmt.Println("🐾 Koda tray running — check your menu bar")
		tray.Run(steerRoot)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(trayCmd)
}
