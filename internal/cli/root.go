package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/tui"
)

var (
	appVersion string
	steerRoot  string
	projectDir string
	jsonOutput bool
)

func Execute(version string) error {
	appVersion = version
	return rootCmd.Execute()
}

var rootCmd = &cobra.Command{
	Use:   "koda",
	Short: "Koda — interactive agent runtime manager for steer-runtime",
	Long: `Koda manages Kiro agent profiles, tokens, workspaces, and IDE integrations.
Run with no arguments to launch the interactive TUI.`,
	SilenceUsage: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip for version command
		if cmd.Name() == "version" {
			return nil
		}

		// 1. Explicit --steer-root flag
		if steerRoot != "" {
			return nil
		}

		// 2. Try CWD and parent
		cwd, _ := os.Getwd()
		steerRoot = config.SteerRoot(cwd)
		if steerRoot == "" {
			parent := filepath.Join(cwd, "..", "steer-runtime")
			if sr := config.SteerRoot(parent); sr != "" {
				steerRoot = sr
			}
		}

		// 3. Try default location (~/.kiro/steer-runtime)
		if steerRoot == "" {
			defaultDir := config.DefaultSteerRoot()
			if sr := config.SteerRoot(defaultDir); sr != "" {
				steerRoot = sr
			}
		}

		// 4. Auto-clone if not found anywhere
		if steerRoot == "" {
			fmt.Println("steer-runtime not found. Cloning...")
			if err := cloneSteerRuntime(); err != nil {
				return fmt.Errorf("auto-clone failed: %w", err)
			}
			steerRoot = config.DefaultSteerRoot()
		}

		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		launchChat, err := tui.Run(steerRoot, config.TargetDir(projectDir))
		if err != nil {
			return err
		}
		if launchChat {
			return tui.RunChat("")
		}
		return nil
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Koda version",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func init() {
	rootCmd.PersistentFlags().StringVar(&steerRoot, "steer-root", "", "Path to steer-runtime repo")
	rootCmd.PersistentFlags().StringVar(&projectDir, "project", "", "Target project directory (default: ~/.kiro)")
	rootCmd.PersistentFlags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	rootCmd.AddCommand(versionCmd)
	rootCmd.AddCommand(installCmd)
	rootCmd.AddCommand(removeCmd)
	rootCmd.AddCommand(syncCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(checkCmd)
	rootCmd.AddCommand(cleanCmd)
	rootCmd.AddCommand(workspaceCmd)
	rootCmd.AddCommand(rulesCmd)
	rootCmd.AddCommand(promptsCmd)
	rootCmd.AddCommand(initMemoryCmd)
	rootCmd.AddCommand(amazonqCmd)
	rootCmd.AddCommand(mcpInstallCmd)
	rootCmd.AddCommand(configureCmd)
	rootCmd.AddCommand(enableToolsCmd)
	rootCmd.AddCommand(doctorCmd)
	rootCmd.AddCommand(upgradeCmd)
	rootCmd.AddCommand(diffCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(chatCmd)
	rootCmd.AddCommand(setupCmd)
	rootCmd.AddCommand(statsCmd)
	rootCmd.AddCommand(autoUpdateCmd)
	rootCmd.AddCommand(slackCmd)
}
