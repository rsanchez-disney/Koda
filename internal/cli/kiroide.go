package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/ops"
)

var kiroIDECmd = &cobra.Command{
	Use:   "kiro-ide",
	Short: "Manage Kiro IDE steering, skills, hooks, and MCP",
}

var kiroIDEInstallCmd = &cobra.Command{
	Use:   "install [workspace-dir]",
	Short: "Install steering + skills (user-level) and hooks (workspace-level)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		var wsDir string
		if len(args) > 0 {
			wsDir = args[0]
		}
		fmt.Println("\U0001f527 Installing Kiro IDE config")
		r, err := ops.InstallKiroIDE(steerRoot, wsDir)
		if err != nil {
			return err
		}
		fmt.Printf("\n\u2705 Installed: %d steering, %d skills, %d hooks\n", r.Steering, r.Skills, r.Hooks)
		if wsDir == "" {
			fmt.Println("\n\U0001f4a1 To install hooks, pass a workspace directory:")
			fmt.Println("   koda kiro-ide install /path/to/project")
		}
		return nil
	},
}

var kiroIDESyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Update steering + skills from latest profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		fmt.Println("\U0001f504 Syncing Kiro IDE config")
		r := ops.SyncKiroIDE(steerRoot)
		fmt.Printf("\u2705 Synced: %d steering, %d skills\n", r.Steering, r.Skills)
		return nil
	},
}

var kiroIDERemoveCmd = &cobra.Command{
	Use:   "remove <workspace-dir>",
	Short: "Remove hooks from a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		removed := ops.RemoveKiroIDE(args[0])
		if removed > 0 {
			fmt.Printf("\u2705 Removed hooks from %s\n", args[0])
		} else {
			fmt.Println("Nothing to remove")
		}
		return nil
	},
}

func init() {
	kiroIDECmd.AddCommand(kiroIDEInstallCmd)
	kiroIDECmd.AddCommand(kiroIDESyncCmd)
	kiroIDECmd.AddCommand(kiroIDERemoveCmd)
}
