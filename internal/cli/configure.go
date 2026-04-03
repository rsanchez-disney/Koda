package cli

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
	"github.disney.com/SANCR225/koda/internal/ops"

	"golang.org/x/term"
)

var configureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure MCP tokens interactively",
	RunE: func(cmd *cobra.Command, args []string) error {
		tokens := ops.ReadTokens()
		reader := bufio.NewReader(os.Stdin)

		fmt.Println("\U0001f527 Configure MCP tokens")
		fmt.Printf("  Tokens file: %s/%s\n\n", config.KiroRoot(), config.TokensFile)

		for _, tk := range model.KnownTokens {
			current := tokens[tk.Key]
			status := ops.MaskToken(current)

			fmt.Printf("%s [%s]: ", tk.Label, status)

			// Try masked input, fall back to plain
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

			if input != "" {
				tokens[tk.Key] = input
				fmt.Println("  \u2713 Updated")
			} else {
				fmt.Println("  \u23ed Kept")
			}
		}

		if err := ops.WriteTokens(tokens); err != nil {
			return err
		}

		// GitHub remotes
		fmt.Println("\n\U0001f527 GitHub Remotes")
		remotes := ops.ReadGitHubRemotes()
		if len(remotes) > 0 {
			fmt.Println("  Current remotes:")
			for _, r := range remotes {
				fmt.Printf("    • %s (%s) %s\n", r.Name, r.Host, ops.MaskToken(r.Token))
			}
		}
		for {
			fmt.Print("\nAdd GitHub remote? (name or Enter to skip): ")
			line, _ := reader.ReadString('\n')
			name := strings.TrimSpace(line)
			if name == "" {
				break
			}
			fmt.Printf("  Host for '%s' (e.g., github.disney.com): ", name)
			line, _ = reader.ReadString('\n')
			host := strings.TrimSpace(line)
			if host == "" {
				continue
			}
			fmt.Printf("  Token for '%s': ", name)
			var tok string
			if isTerminal() {
				raw, _ := term.ReadPassword(int(syscall.Stdin))
				fmt.Println()
				tok = strings.TrimSpace(string(raw))
			} else {
				line, _ = reader.ReadString('\n')
				tok = strings.TrimSpace(line)
			}
			if tok == "" {
				continue
			}
			ops.WriteGitHubRemote(model.GitHubRemote{Name: name, Host: host, Token: tok})
			fmt.Printf("  \u2713 Added remote '%s'\n", name)
		}

		ops.InjectAgentTokens(config.TargetDir(projectDir))
		fmt.Println("\n\u2705 Tokens saved")
		return nil
	},
}

var enableToolsCmd = &cobra.Command{
	Use:   "enable-tools",
	Short: "Enable advanced kiro-cli tool settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("\U0001f527 Enabling advanced kiro-cli tool settings...")
		fmt.Println()

		settings := []string{
			"chat.enableThinking",
			"chat.enableTodoList",
			"chat.enableKnowledge",
			"chat.enableDelegate",
		}

		for _, s := range settings {
			out, err := exec.Command("kiro-cli", "settings", s, "true").CombinedOutput()
			if err != nil {
				fmt.Printf("  \u26a0 %s \u2014 %s\n", s, strings.TrimSpace(string(out)))
			} else {
				fmt.Printf("  \u2713 %s = true\n", s)
			}
		}

		fmt.Println("\n\u2705 Advanced tools enabled")
		return nil
	},
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Deep health check of the entire setup",
	Run: func(cmd *cobra.Command, args []string) {
		target := config.TargetDir(projectDir)
		results := ops.RunDoctor(steerRoot, target)
		ops.PrintDoctor(results)
	},
}

func isTerminal() bool {
	return term.IsTerminal(int(syscall.Stdin))
}

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Self-update to the latest release",
	RunE: func(cmd *cobra.Command, args []string) error {
		return ops.Upgrade(appVersion)
	},
}

var diffCmd = &cobra.Command{
	Use:   "diff",
	Short: "Show what would change on next sync (dry-run)",
	RunE: func(cmd *cobra.Command, args []string) error {
		if steerRoot == "" {
			return fmt.Errorf("steer-runtime not found")
		}
		target := config.TargetDir(projectDir)
		entries := ops.DiffSync(steerRoot, target)
		ops.PrintDiff(entries)
		return nil
	},
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show agent setup status (like git status)",
	Run: func(cmd *cobra.Command, args []string) {
		ops.PrintStatus(steerRoot, config.TargetDir(projectDir))
	},
}