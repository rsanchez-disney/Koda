package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var kiroIDEProfiles []string

var kiroIDECmd = &cobra.Command{
	Use:   "kiro-ide",
	Short: "Manage Kiro IDE steering, skills, agents, and workspace config",
}

var kiroIDEInstallCmd = &cobra.Command{
	Use:   "install [workspace-dir]",
	Short: "Install steering + skills (user-level) and agents + context + rules + hooks (workspace-level)",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		var wsDir string
		if len(args) > 0 {
			wsDir = args[0]
		}

		resolved := ops.ExpandAliases(kiroIDEProfiles)
		if len(resolved) == 0 {
			fmt.Println("\U0001f527 Installing Kiro IDE config (profiles from workspace settings)")
		} else {
			fmt.Printf("\U0001f527 Installing Kiro IDE config for: %s\n", strings.Join(resolved, ", "))
		}

		r, err := ops.InstallKiroIDE(steerRoot, wsDir, kiroIDEProfiles)
		if err != nil {
			return err
		}
		fmt.Println("\nUser-level (~/.kiro/):")
		fmt.Printf("  \u2713 %d steering, %d skills\n", r.Steering, r.Skills)
		if wsDir != "" {
			fmt.Printf("\nWorkspace-level (%s/.kiro/):\n", wsDir)
			fmt.Printf("  \u2713 %d agents, %d prompts, %d context, %d rules, %d hooks\n",
				r.Agents, r.Prompts, r.Context, r.Rules, r.Hooks)
		}
		fmt.Printf("\nMCP: %d servers\n", r.MCP)
		fmt.Println("\n\u2705 Done")
		if wsDir == "" {
			fmt.Println("\n\U0001f4a1 To install workspace-level agents + hooks, pass a project directory:")
			fmt.Println("   koda kiro-ide install /path/to/project")
		}
		return nil
	},
}

var kiroIDESyncCmd = &cobra.Command{
	Use:   "sync [workspace-dir]",
	Short: "Update installed steering, skills, agents, and context from latest profiles",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		var wsDir string
		if len(args) > 0 {
			wsDir = args[0]
		}

		fmt.Println("\U0001f504 Syncing Kiro IDE config")
		r, err := ops.SyncKiroIDE(steerRoot, wsDir, kiroIDEProfiles)
		if err != nil {
			return err
		}
		fmt.Printf("\u2705 Synced: %d steering, %d skills", r.Steering, r.Skills)
		if wsDir != "" {
			fmt.Printf(", %d agents, %d prompts, %d context, %d rules",
				r.Agents, r.Prompts, r.Context, r.Rules)
		}
		fmt.Println()
		return nil
	},
}

var kiroIDEListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available profiles for Kiro IDE installation",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		target := config.TargetDir(projectDir)
		profiles, err := ops.ListProfiles(steerRoot, target)
		if err != nil {
			return err
		}

		// Show active workspace context
		s := config.ReadSteerSettings()
		if s.ActiveWorkspace != "" {
			if ws, err := ops.GetWorkspace(steerRoot, s.ActiveWorkspace); err == nil {
				fmt.Printf("\U0001f4cb Workspace: %s", ws.Name)
				if ws.Team != "" {
					fmt.Printf(" (%s)", ws.Team)
				}
				fmt.Println()
				if len(ws.Profiles) > 0 {
					fmt.Printf("  Profiles: %s\n", strings.Join(ws.Profiles, ", "))
				}
				fmt.Println()
			}
		}

		fmt.Println("\U0001f4e6 Available profiles:")
		if devAliases, ok := model.Aliases["dev"]; ok {
			fmt.Printf("  \u2022 dev (alias \u2192 %s)\n", strings.Join(devAliases, " + "))
		}
		for _, p := range profiles {
			status := " "
			if p.Installed {
				status = "\u2713"
			}
			label := p.ID
			if p.WorkspaceName != "" {
				label += " (" + p.WorkspaceName + ")"
			}
			fmt.Printf("  [%s] %-20s %2d agents\n", status, label, p.AgentCount)
		}
		return nil
	},
}

var kiroIDERemoveCmd = &cobra.Command{
	Use:   "remove <workspace-dir>",
	Short: "Remove generated .kiro content from a workspace",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		removed := ops.RemoveKiroIDE(args[0])
		if removed > 0 {
			fmt.Printf("\u2705 Removed %d directories from %s/.kiro/\n", removed, args[0])
		} else {
			fmt.Println("Nothing to remove")
		}
		return nil
	},
}

func init() {
	kiroIDEInstallCmd.Flags().StringSliceVarP(&kiroIDEProfiles, "profiles", "p", nil, "Profiles to install (default: from workspace settings)")
	kiroIDESyncCmd.Flags().StringSliceVarP(&kiroIDEProfiles, "profiles", "p", nil, "Profiles to sync")

	kiroIDECmd.AddCommand(kiroIDEInstallCmd)
	kiroIDECmd.AddCommand(kiroIDESyncCmd)
	kiroIDECmd.AddCommand(kiroIDEListCmd)
	kiroIDECmd.AddCommand(kiroIDERemoveCmd)
}
