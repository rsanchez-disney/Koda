package cli

import (
	"fmt"
	"os"

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
		if steerRoot != "" {
			return nil
		}
		cwd, _ := os.Getwd()
		steerRoot = config.SteerRoot(cwd)
		if steerRoot == "" {
			parent := cwd + "/../steer-runtime"
			if sr := config.SteerRoot(parent); sr != "" {
				steerRoot = sr
			}
		}
		return nil
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found. Run from inside the repo or set --steer-root")
		}
		return tui.Run(steerRoot, config.TargetDir(projectDir))
	},
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print Koda version",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("koda %s\n", appVersion)
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
}
