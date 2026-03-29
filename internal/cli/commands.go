package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var installCmd = &cobra.Command{
	Use:   "install [profiles...]",
	Short: "Install one or more agent profiles",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		target := config.TargetDir(projectDir)
		fmt.Printf("\U0001f3af Target: %s\n", target)
		ops.InstallShared(steerRoot, target)
		profiles := ops.ExpandAliases(args)
		for _, p := range profiles {
			fmt.Printf("\U0001f4e6 Installing %s...\n", p)
			count, err := ops.InstallProfile(steerRoot, p, target)
			if err != nil {
				fmt.Printf("  \u2717 %s: %v\n", p, err)
				continue
			}
			fmt.Printf("  \u2713 %s (%d agents)\n", p, count)
		}
		ops.InjectAgentTokens(target)
		ops.WriteProfilesManifest(steerRoot, target)
		fmt.Printf("\n\u2705 Installation complete (%d agents total)\n", countAgents(target))
		return nil
	},
}

var removeCmd = &cobra.Command{
	Use:   "remove [profiles...]",
	Short: "Remove specific profiles",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		target := config.TargetDir(projectDir)
		profiles := ops.ExpandAliases(args)
		for _, p := range profiles {
			removed, err := ops.RemoveProfile(steerRoot, p, target)
			if err != nil {
				fmt.Printf("  \u2717 %s: %v\n", p, err)
				continue
			}
			fmt.Printf("  \u2713 Removed %s (%d agents)\n", p, removed)
		}
		fmt.Printf("\n\u2705 Removal complete (%d agents remaining)\n", countAgents(target))
		return nil
	},
}

var syncUpdate bool

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Update installed profiles to latest",
	RunE: func(cmd *cobra.Command, args []string) error {
		// If --update or no steer-runtime found, download latest release
		if syncUpdate || steerRoot == "" {
			if err := cloneSteerRuntime(); err != nil {
				return err
			}
			steerRoot = config.DefaultSteerRoot()
		}
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		target := config.TargetDir(projectDir)
		installed := ops.DetectInstalled(steerRoot, target)
		if len(installed) == 0 {
			fmt.Println("\u26a0 No profiles detected. Use 'koda install' first.")
			return nil
		}
		fmt.Printf("\U0001f504 Syncing: %s\n", strings.Join(installed, ", "))
		ops.InstallShared(steerRoot, target)
		for _, p := range installed {
			count, err := ops.InstallProfile(steerRoot, p, target)
			if err != nil {
				fmt.Printf("  \u2717 %s: %v\n", p, err)
				continue
			}
			fmt.Printf("  \u2713 %s (%d agents)\n", p, count)
		}
		ops.InjectAgentTokens(target)
		fmt.Printf("\n\u2705 Sync complete (%d agents total)\n", countAgents(target))
		ops.WriteProfilesManifest(steerRoot, target)
		return nil
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List available profiles",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		target := config.TargetDir(projectDir)
		profiles, err := ops.ListProfiles(steerRoot, target)
		if err != nil {
			return err
		}
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(profiles)
		}
		fmt.Println("\U0001f4cb Available profiles:")
		fmt.Println()
		fmt.Println("  \u2022 dev (alias \u2192 dev-core + dev-web + dev-mobile)")
		for _, p := range profiles {
			status := " "
			if p.Installed {
				status = "\u2713"
			}
			fmt.Printf("  [%s] %-14s %2d agents\n", status, p.ID, p.AgentCount)
		}
		return nil
	},
}

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Verify installation health",
	RunE: func(cmd *cobra.Command, args []string) error {
		target := config.TargetDir(projectDir)
		report := ops.CheckInstallation(steerRoot, target)
		if jsonOutput {
			return json.NewEncoder(os.Stdout).Encode(report)
		}
		ops.PrintReport(report)
		return nil
	},
}

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove ALL profiles and agents",
	RunE: func(cmd *cobra.Command, args []string) error {
		target := config.TargetDir(projectDir)
		total := countAgents(target)
		fmt.Printf("\U0001f9f9 Cleaning %s (%d agents)...\n", target, total)
		for _, sub := range []string{"agents", "prompts", "context", "powers", "skills", "steering"} {
			os.RemoveAll(filepath.Join(target, sub))
		}
		fmt.Println("\u2705 Clean complete")
		return nil
	},
}

func countAgents(targetDir string) int {
	entries, _ := os.ReadDir(filepath.Join(targetDir, config.AgentsDir))
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			count++
		}
	}
	return count
}

func init() {
	syncCmd.Flags().BoolVar(&syncUpdate, "update", false, "Download latest release before syncing")
}
