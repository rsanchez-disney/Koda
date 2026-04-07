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

var initMemoryFrom string

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
		if err := ops.InitMemory(steerRoot, dir, initMemoryFrom); err != nil {
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

var amazonqSyncCmd = &cobra.Command{
	Use:   "sync [dir]",
	Short: "Update Amazon Q rules to latest",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		count, err := ops.InstallAmazonQRules(steerRoot, args[0])
		if err != nil {
			return err
		}
		fmt.Printf("\u2705 Synced %d Amazon Q rules\n", count)
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
	initMemoryCmd.Flags().StringVar(&initMemoryFrom, "from", "", "Copy memory bank from a known project")
	rulesInstallCmd.Flags().BoolVar(&rulesInstallAll, "all", false, "Install all rules")
	rulesCmd.AddCommand(rulesListCmd)
	rulesCmd.AddCommand(rulesInstallCmd)

	promptsInstallCmd.Flags().BoolVar(&promptsInstallAll, "all", false, "Install all prompts")
	promptsCmd.AddCommand(promptsListCmd)
	promptsCmd.AddCommand(promptsInstallCmd)

	amazonqCmd.AddCommand(amazonqInstallCmd)
	amazonqCmd.AddCommand(amazonqRemoveCmd)
	amazonqCmd.AddCommand(amazonqSyncCmd)
	amazonqCmd.AddCommand(amazonqSyncAllCmd)
	amazonqCmd.AddCommand(amazonqSyncMCPCmd)
	amazonqCmd.AddCommand(amazonqStatusCmd)
}

var amazonqSyncAllCmd = &cobra.Command{
	Use:   "sync-all [dir]",
	Short: "Full sync: templates + context + MCP to Amazon Q",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		dir = expandHome(dir)

		// 1. Templates
		tplCount, err := ops.InstallAmazonQRules(steerRoot, dir)
		if err != nil {
			return err
		}
		fmt.Printf("  ✓ %d template rules\n", tplCount)

		// 2. Context
		ctxCount := ops.SyncAmazonQContext(dir)
		fmt.Printf("  ✓ %d context rules\n", ctxCount)

		// 3. MCP
		mcpCount, err := ops.SyncAmazonQMCP()
		if err != nil {
			fmt.Printf("  ⚠ MCP: %v\n", err)
		} else {
			fmt.Printf("  ✓ %d MCP servers synced\n", mcpCount)
		}

		fmt.Printf("\n✅ Amazon Q sync complete (%d rules total)\n", tplCount+ctxCount)
		return nil
	},
}

var amazonqSyncMCPCmd = &cobra.Command{
	Use:   "sync-mcp",
	Short: "Sync MCP servers to Amazon Q",
	RunE: func(cmd *cobra.Command, args []string) error {
		count, err := ops.SyncAmazonQMCP()
		if err != nil {
			return err
		}
		fmt.Printf("✅ %d MCP servers synced to Amazon Q\n", count)
		return nil
	},
}

var amazonqStatusCmd = &cobra.Command{
	Use:   "status [dir]",
	Short: "Show Amazon Q sync status",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := "."
		if len(args) > 0 {
			dir = args[0]
		}
		dir = expandHome(dir)

		report := ops.AmazonQStatus(dir)
		fmt.Println("📋 Amazon Q Sync Status")
		fmt.Println()
		if report.RulesCount > 0 {
			fmt.Printf("  Rules:  %d files in %s\n", report.RulesCount, report.RulesDir)
		} else {
			fmt.Printf("  Rules:  ❌ Not configured (%s missing)\n", report.RulesDir)
		}
		if report.MCPCount > 0 {
			fmt.Printf("  MCP:    %d servers in %s\n", report.MCPCount, report.MCPPath)
		} else {
			fmt.Printf("  MCP:    ❌ Not configured (%s missing)\n", report.MCPPath)
		}
		if report.KiroMCP {
			fmt.Println("  Source: ✓ Kiro MCP config found")
		} else {
			fmt.Println("  Source: ❌ Run 'koda mcp-install' first")
		}
		return nil
	},
}

func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
