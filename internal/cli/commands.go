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
		fmt.Printf("🎯 Target: %s\n", target)
		ops.InstallShared(steerRoot, target)
		profiles := ops.ExpandAliases(args)
		for _, p := range profiles {
			srcDir, wsName := ops.ResolveProfileSource(steerRoot, p)
			if wsName != "" {
				fmt.Printf("📦 Installing %s... (workspace: %s)\n", p, wsName)
				// Install global base first, then workspace specialization
				globalDir := filepath.Join(steerRoot, config.ProfilePrefix+p)
				ops.InstallProfileFrom(globalDir, target)
			} else {
				fmt.Printf("📦 Installing %s...\n", p)
			}
			count, err := ops.InstallProfileFrom(srcDir, target)
			if err != nil {
				fmt.Printf("  ✗ %s: %v\n", p, err)
				continue
			}
			fmt.Printf("  ✓ %s (%d agents)\n", p, count)
		}
		ops.InjectAgentTokens(target)
		ops.EnrichWelcomeMessages(target)
		ops.WriteProfilesManifest(steerRoot, target)
		ops.GenerateMcpJson(ops.FindNodeExe())
		ops.WriteSystemProfile()
		// Kiro settings: full configure on first run, just default agent after
		s := config.ReadSteerSettings()
		if !s.KiroSettingsApplied {
			ops.ConfigureKiroSettings(steerRoot, target)
			s.KiroSettingsApplied = true
			config.SaveSteerSettings(s)
		} else if agent := ops.SuggestDefaultAgent(steerRoot, target); agent != "" {
			ops.SetKiroSetting("chat.defaultAgent", agent)
			fmt.Printf("  ✓ kiro: chat.defaultAgent = %s\n", agent)
		}
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
		// If --update or no steer-runtime found, fetch latest
		if syncUpdate || steerRoot == "" {
			// If steerRoot is a git repo (fork or clone), use git pull instead of tarball
			if steerRoot != "" {
				if _, err := os.Stat(filepath.Join(steerRoot, ".git")); err == nil {
					fmt.Println("📦 Git repo detected — pulling latest...")
					if err := ops.SyncSteerRuntime(steerRoot, config.TargetDir(projectDir)); err != nil {
						return err
					}
					// SyncSteerRuntime already re-installs profiles, so we're done
					fmt.Println("✅ Sync complete")
					return nil
				}
			}
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
		fmt.Printf("🔄 Syncing: %s\n", strings.Join(installed, ", "))
		ops.InstallShared(steerRoot, target)
		for _, p := range installed {
			srcDir, wsName := ops.ResolveProfileSource(steerRoot, p)
			label := p
			if wsName != "" {
				label = fmt.Sprintf("%s (workspace: %s)", p, wsName)
				globalDir := filepath.Join(steerRoot, config.ProfilePrefix+p)
				ops.InstallProfileFrom(globalDir, target)
			}
			count, err := ops.InstallProfileFrom(srcDir, target)
			if err != nil {
				fmt.Printf("  ✗ %s: %v\n", label, err)
				continue
			}
			fmt.Printf("  ✓ %s (%d agents)\n", label, count)
		}
		ops.InjectAgentTokens(target)
		ops.EnrichWelcomeMessages(target)

		// Sync workspace steering and MCP bundles if a workspace is active
		if s := config.ReadSteerSettings(); s.ActiveWorkspace != "" {
			ws, err := ops.GetWorkspace(steerRoot, s.ActiveWorkspace)
			if err == nil {
				_, wsNames := ops.ResolveWorkspace(steerRoot, ws)
				ops.InstallWorkspaceSteering(steerRoot, target, wsNames)
				ops.InstallWorkspaceMCPBundles(steerRoot, target, wsNames)
			}
		}
		fmt.Printf("\n\u2705 Sync complete (%d agents total)\n", countAgents(target))
		ops.WriteProfilesManifest(steerRoot, target)
		ops.GenerateMcpJson(ops.FindNodeExe())
		ops.WriteSystemProfile()
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
		fmt.Println("📋 Available profiles:")
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

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Backup ~/.kiro and reinstall fresh (preserves tokens)",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🔄 Resetting Koda installation...")
		return ops.Reset(steerRoot)
	},
}

func countAgents(targetDir string) int {
	entries, _ := os.ReadDir(filepath.Join(targetDir, config.AgentsDir))
	count := 0
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), "._") {
			count++
		}
	}
	return count
}

func init() {
	syncCmd.Flags().BoolVar(&syncUpdate, "update", false, "Download latest release before syncing")
}
