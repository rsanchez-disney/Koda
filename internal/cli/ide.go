package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/ops"
)

var ideCmd = &cobra.Command{
	Use:   "ide",
	Short: "Manage IDE plugins (VS Code, Cursor, IntelliJ, etc.)",
	RunE: func(cmd *cobra.Command, args []string) error {
		return ideStatusCmd.RunE(cmd, args)
	},
}

var ideStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show detected IDEs and plugin install status",
	Run: func(cmd *cobra.Command, args []string) {
		ides := ops.DetectIDEs()
		fmt.Println("🔌 IDE Plugin Status")
		fmt.Println()
		for _, ide := range ides {
			if !ide.Installed {
				continue
			}
			plugin := "✗ not installed"
			if ide.Plugin {
				plugin = "✓ installed"
			}
			fmt.Printf("  %-18s %s\n", ide.Name, plugin)
		}
		fmt.Println()
		fmt.Println("Install with: koda ide install <id>")
		fmt.Println("IDs: vscode, cursor, intellij, webstorm, pycharm, rider, goland")
	},
}

var ideInstallCmd = &cobra.Command{
	Use:   "install <ide>",
	Short: "Install the steer plugin for an IDE",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ide := strings.ToLower(args[0])
		fmt.Printf("🔌 Installing steer plugin for %s...\n", ide)
		if err := ops.InstallIDEPlugin(ide); err != nil {
			return err
		}
		fmt.Println("✅ Plugin installed. Restart your IDE to activate.")
		return nil
	},
}

var ideUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update steer plugins to latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		ides := ops.DetectIDEs()
		updated := 0
		for _, ide := range ides {
			if ide.Plugin {
				fmt.Printf("  Updating %s...\n", ide.Name)
				if err := ops.InstallIDEPlugin(ide.ID); err != nil {
					fmt.Printf("  ⚠ %s: %v\n", ide.Name, err)
				} else {
					updated++
				}
			}
		}
		if updated == 0 {
			fmt.Println("No plugins installed to update. Use: koda ide install <id>")
		} else {
			fmt.Printf("✅ Updated %d plugin(s)\n", updated)
		}
		return nil
	},
}

func init() {
	ideCmd.AddCommand(ideStatusCmd)
	ideCmd.AddCommand(ideInstallCmd)
	ideCmd.AddCommand(ideUpdateCmd)
}
