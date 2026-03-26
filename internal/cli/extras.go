package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var rulesCmd = &cobra.Command{
	Use:   "rules",
	Short: "Manage coding rules",
}

var rulesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		rules := ops.ListRules(steerRoot)
		fmt.Println("\U0001f4cb Available rules:")
		for _, r := range rules {
			fmt.Printf("  \u2022 %s\n", r)
		}
		return nil
	},
}

var rulesInstallAll bool

var rulesInstallCmd = &cobra.Command{
	Use:   "install [rules...]",
	Short: "Install rules",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		names := args
		if rulesInstallAll {
			names = ops.ListRules(steerRoot)
		}
		if len(names) == 0 {
			return fmt.Errorf("specify rule names or use --all")
		}
		target := config.TargetDir(projectDir)
		count := ops.InstallRules(steerRoot, target, names)
		fmt.Printf("\u2705 Installed %d rules\n", count)
		return nil
	},
}

var promptsCmd = &cobra.Command{
	Use:   "prompts",
	Short: "Manage standalone prompts",
}

var promptsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List available prompts",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		prompts := ops.ListPrompts(steerRoot)
		fmt.Println("\U0001f4cb Available prompts:")
		for _, p := range prompts {
			fmt.Printf("  \u2022 %s\n", p)
		}
		return nil
	},
}

var promptsInstallAll bool

var promptsInstallCmd = &cobra.Command{
	Use:   "install [prompts...]",
	Short: "Install prompts",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		names := args
		if promptsInstallAll {
			names = ops.ListPrompts(steerRoot)
		}
		if len(names) == 0 {
			return fmt.Errorf("specify prompt names or use --all")
		}
		count := ops.InstallPrompts(steerRoot, names)
		fmt.Printf("\u2705 Installed %d prompts\n", count)
		return nil
	},
}

var initMemoryCmd = &cobra.Command{
	Use:   "init-memory [dir]",
	Short: "Initialize project memory bank",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		dir := args[0]
		if len(dir) > 0 && dir[0] == '~' {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, dir[1:])
		}
		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("directory not found: %s", dir)
		}
		fmt.Printf("\U0001f9e0 Initializing memory bank for %s...\n", filepath.Base(dir))
		if err := ops.InitMemory(steerRoot, dir); err != nil {
			return err
		}
		fmt.Println("\u2705 Memory bank initialized")
		return nil
	},
}

var amazonqCmd = &cobra.Command{
	Use:   "amazonq",
	Short: "Manage Amazon Q Developer rules",
}

var amazonqInstallCmd = &cobra.Command{
	Use:   "install [dir]",
	Short: "Install Amazon Q rules",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		count, err := ops.InstallAmazonQRules(steerRoot, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("\u2705 Installed %d Amazon Q rules to %s/.amazonq/rules/\n", count, args[0])
		return nil
	},
}

var amazonqRemoveCmd = &cobra.Command{
	Use:   "remove [dir]",
	Short: "Remove .amazonq/ directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ops.RemoveDir(filepath.Join(args[0], ".amazonq"))
		fmt.Println("\u2705 Removed .amazonq/")
		return nil
	},
}

var mcpInstallCmd = &cobra.Command{
	Use:   "mcp-install",
	Short: "Install MCP server dependencies and generate config",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		target := config.TargetDir(projectDir)
		return ops.MCPInstall(steerRoot, target)
	},
}

func init() {
	rulesInstallCmd.Flags().BoolVar(&rulesInstallAll, "all", false, "Install all rules")
	rulesCmd.AddCommand(rulesListCmd)
	rulesCmd.AddCommand(rulesInstallCmd)

	promptsInstallCmd.Flags().BoolVar(&promptsInstallAll, "all", false, "Install all prompts")
	promptsCmd.AddCommand(promptsListCmd)
	promptsCmd.AddCommand(promptsInstallCmd)

	amazonqCmd.AddCommand(amazonqInstallCmd)
	amazonqCmd.AddCommand(amazonqRemoveCmd)
}
