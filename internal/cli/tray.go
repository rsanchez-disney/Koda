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
		// Auto-register on first run
		if !tray.AutoStartEnabled() {
			if err := tray.EnableAutoStart(); err == nil {
				fmt.Println("✓ Registered to start on login")
			}
		}
		fmt.Println("🐾 Koda tray running — check your menu bar")
		tray.Run(steerRoot, appVersion)
		return nil
	},
}

var trayDisableCmd = &cobra.Command{
	Use:   "disable",
	Short: "Disable tray auto-start on login",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := tray.DisableAutoStart(); err != nil {
			return fmt.Errorf("not registered")
		}
		fmt.Println("✓ Tray auto-start disabled")
		return nil
	},
}

func init() {
	trayCmd.AddCommand(trayDisableCmd)
	rootCmd.AddCommand(trayCmd)
}
