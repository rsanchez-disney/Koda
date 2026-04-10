package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
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

var mcpAssistant bool

var mcpInstallCmd = &cobra.Command{
	Use:   "mcp-install",
	Short: "Install MCP server dependencies and generate config",
	RunE: func(cmd *cobra.Command, args []string) error {
		target := config.TargetDir(projectDir)

		// Decide mode: interactive assistant vs quick reinstall.
		// Assistant runs when: no existing config, or --assistant flag, or non-TTY (legacy).
		hasConfig := ops.HasExistingMCPConfig()
		useAssistant := mcpAssistant || !hasConfig

		// 1. Discover and verify bundles
		fmt.Println("\n🔍 Verifying MCP server bundles...")
		available, verified := ops.DiscoverServers(target)
		for _, srv := range available {
			if verified[srv.Name] {
				fmt.Printf("  ✓ %s\n", srv.BundleDir)
			}
		}
		verifiedCount := 0
		for _, v := range verified {
			if v {
				verifiedCount++
			}
		}
		fmt.Printf("\n✅ %d MCP servers available\n", verifiedCount)

		var selected []ops.MCPServer
		tokens := ops.ReadTokens()
		envVars := ops.ReadEnvVars()
		var ghRemotes []model.GitHubRemote

		if !isTerminal() {
			// Non-TTY: select all verified servers (legacy behavior)
			for _, srv := range available {
				if verified[srv.Name] {
					selected = append(selected, srv)
				}
			}
			ghRemotes = ops.ReadGitHubRemotes()
		} else if useAssistant {
			// Interactive assistant: server selection + token prompts + GitHub remotes
			fmt.Print("\n🔧 Select MCP servers to install (toggle with number, Enter to confirm):\n\n")
			reader := bufio.NewReader(os.Stdin)
			selected = promptServerSelection(reader, available, verified)

			// Token configuration
			tokens = promptTokens(reader, selected, tokens)

			// GitHub remotes (only if github is selected)
			githubSelected := false
			for _, srv := range selected {
				if srv.Name == "github" {
					githubSelected = true
					break
				}
			}
			if githubSelected {
				ghRemotes = promptGitHubRemotes(reader)
			} else {
				ghRemotes = ops.ReadGitHubRemotes()
			}

			// Install context7 npm dependencies if selected
			for _, srv := range selected {
				if srv.IsNPM {
					fmt.Printf("\n📦 Installing %s from public registry...\n", srv.BundleDir)
					ctx7Dir := filepath.Join(target, config.ToolsDir, "mcp-servers", srv.BundleDir)
					npmCmd := exec.Command("npm", "install", "--registry", "https://registry.npmjs.org", "--silent")
					npmCmd.Dir = ctx7Dir
					if out, err := npmCmd.CombinedOutput(); err != nil {
						fmt.Printf("  ⚠ %s: %s\n", srv.Name, strings.TrimSpace(string(out)))
					} else {
						fmt.Printf("  ✓ %s\n", srv.Name)
					}
				}
			}
		} else {
			// Quick reinstall: existing config present, no --assistant flag.
			// Select all verified servers without prompting (same as non-TTY).
			for _, srv := range available {
				if verified[srv.Name] {
					selected = append(selected, srv)
				}
			}
			ghRemotes = ops.ReadGitHubRemotes()
		}

		// Generate mcp.json
		fmt.Println("\n🔧 Generating mcp.json...")
		mcpPath, err := ops.GenerateMCPConfig(selected, ghRemotes, tokens, envVars)
		if err != nil {
			return err
		}
		fmt.Printf("  ✓ %s\n", mcpPath)

		// Print summary
		fmt.Println("\n  Servers included:")
		for _, srv := range selected {
			if srv.Name == "github" {
				if len(ghRemotes) == 1 {
					fmt.Printf("    • github\n")
				} else {
					for _, r := range ghRemotes {
						fmt.Printf("    • github-%s\n", r.Name)
					}
				}
				continue
			}
			if srv.IsSSE && tokens["COMPASS_TOKEN"] == "" {
				continue // compass excluded when no token
			}
			fmt.Printf("    • %s\n", srv.Name)
		}

		serverCount := len(selected)
		// Adjust count for multi-remote github
		for _, srv := range selected {
			if srv.Name == "github" && len(ghRemotes) > 1 {
				serverCount += len(ghRemotes) - 1
			}
		}

		// Inject tokens into agent configs
		ops.InjectAgentTokens(target)

		fmt.Printf("\n✅ MCP servers ready (%d servers configured)\n", serverCount)
		return nil
	},
}

func init() {
	mcpInstallCmd.Flags().BoolVar(&mcpAssistant, "assistant", false, "Force interactive assistant for server selection and token configuration")
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

// parseToggleInput parses comma-separated numbers from user input.
// Returns the 0-indexed positions to toggle. Invalid entries are silently ignored.
func parseToggleInput(input string, max int) []int {
	var indices []int
	for _, part := range strings.Split(input, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > max {
			continue
		}
		indices = append(indices, n-1)
	}
	return indices
}

// promptServerSelection displays a numbered toggle list and returns
// the user's selected servers. All servers default to enabled.
// Uses bufio.NewReader for line input.
func promptServerSelection(reader *bufio.Reader, servers []ops.MCPServer, verified map[string]bool) []ops.MCPServer {
	// Initialize selection: verified servers default to enabled, unverified to disabled.
	selected := make([]bool, len(servers))
	for i, srv := range servers {
		selected[i] = verified[srv.Name]
	}

	for {
		// Display the numbered toggle list.
		for i, srv := range servers {
			check := " "
			if selected[i] {
				check = "✓"
			}
			suffix := ""
			if !verified[srv.Name] {
				suffix = "  (bundle missing)"
				check = " "
			}
			fmt.Printf("  [%d] %s %s%s\n", i+1, check, srv.Name, suffix)
		}
		fmt.Printf("\nToggle (1-%d, a=all, n=none, Enter=confirm): ", len(servers))

		line, _ := reader.ReadString('\n')
		input := strings.TrimSpace(line)

		// Enter = confirm
		if input == "" {
			break
		}

		switch input {
		case "a":
			for i, srv := range servers {
				if verified[srv.Name] {
					selected[i] = true
				}
			}
		case "n":
			for i := range selected {
				selected[i] = false
			}
		default:
			for _, idx := range parseToggleInput(input, len(servers)) {
				// Only allow toggling verified servers
				if verified[servers[idx].Name] {
					selected[idx] = !selected[idx]
				}
			}
		}

		fmt.Println() // blank line before redisplay
	}

	// Return only servers that are both selected AND verified.
	var result []ops.MCPServer
	for i, srv := range servers {
		if selected[i] && verified[srv.Name] {
			result = append(result, srv)
		}
	}
	return result
}

// promptTokens prompts for tokens required by the selected servers.
// Shows current masked value, accepts new value or Enter to keep.
// Uses term.ReadPassword for masked input in TTY mode.
func promptTokens(reader *bufio.Reader, selected []ops.MCPServer, tokens map[string]string) map[string]string {
	required := ops.RequiredTokens(selected)
	if len(required) == 0 {
		return tokens
	}

	fmt.Println("\n🔑 Configure tokens for selected servers")

	for _, tk := range required {
		current := tokens[tk.Key]
		masked := ops.MaskToken(current)

		fmt.Printf("\n  %s [%s]:\n", tk.Label, masked)
		fmt.Printf("    Hint: %s\n", tk.Hint)

		if current != "" {
			fmt.Print("    New value (Enter to keep): ")
		} else {
			fmt.Print("    New value: ")
		}

		var input string
		if isTerminal() {
			raw, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err == nil {
				input = strings.TrimSpace(string(raw))
			}
		} else {
			line, _ := reader.ReadString('\n')
			input = strings.TrimSpace(line)
		}

		if input == "" {
			fmt.Println("    ⏭ Kept")
		} else {
			tokens[tk.Key] = input
			fmt.Println("    ✓ Updated")
		}
	}

	ops.WriteTokens(tokens)
	return tokens
}

// promptGitHubRemotes displays existing remotes and offers to add/update.
// Returns the final list of remotes after user interaction.
func promptGitHubRemotes(reader *bufio.Reader) []model.GitHubRemote {
	fmt.Println("\n🐙 GitHub Remotes")

	remotes := ops.ReadGitHubRemotes()
	if len(remotes) > 0 {
		fmt.Println("\n  Current remotes:")
		for _, r := range remotes {
			fmt.Printf("    • %s (%s) %s\n", r.Name, r.Host, ops.MaskToken(r.Token))
		}
	}

	for {
		fmt.Print("\n  Add GitHub remote? (name or Enter to skip): ")
		line, _ := reader.ReadString('\n')
		name := strings.TrimSpace(line)
		if name == "" {
			break
		}

		fmt.Printf("    Host for '%s' (e.g., github.com): ", name)
		line, _ = reader.ReadString('\n')
		host := strings.TrimSpace(line)
		if host == "" {
			continue
		}

		fmt.Printf("    Token for '%s': ", name)
		var tok string
		if isTerminal() {
			raw, err := term.ReadPassword(int(syscall.Stdin))
			fmt.Println()
			if err == nil {
				tok = strings.TrimSpace(string(raw))
			}
		} else {
			line, _ = reader.ReadString('\n')
			tok = strings.TrimSpace(line)
		}
		if tok == "" {
			continue
		}

		fmt.Printf("    API path for '%s' (Enter to skip): ", name)
		line, _ = reader.ReadString('\n')
		apiPath := strings.TrimSpace(line)

		ops.WriteGitHubRemote(model.GitHubRemote{
			Name:    name,
			Host:    host,
			Token:   tok,
			APIPath: apiPath,
		})
		fmt.Printf("    ✓ Added remote '%s'\n", name)
	}

	return ops.ReadGitHubRemotes()
}

func expandHome(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[1:])
	}
	return path
}
