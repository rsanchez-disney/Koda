package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
	"github.disney.com/SANCR225/koda/internal/tui"
)

var (
	chatAgent      string
	chatWs         string
	chatTrustAll   bool
	chatNoTrust    bool
	chatResetTrust bool
)

// resolveTrust returns whether to trust all tools, consulting saved preference
// and prompting the user if needed. Flags override everything.
func resolveTrust(flagTrustAll, flagNoTrust, flagResetTrust bool) bool {
	if flagResetTrust {
		s := config.ReadSteerSettings()
		s.TrustTools = ""
		config.SaveSteerSettings(s)
		fmt.Println("  ✓ Trust preference reset — will prompt next time")
	}
	if flagTrustAll {
		return true
	}
	if flagNoTrust {
		return false
	}

	s := config.ReadSteerSettings()
	switch s.TrustTools {
	case "all":
		return true
	case "none":
		return false
	}

	// No saved preference — prompt
	fmt.Print("Trust all tools? (Y/n/always/never): ")
	var answer string
	fmt.Scanln(&answer)
	answer = strings.TrimSpace(strings.ToLower(answer))

	switch answer {
	case "always":
		s.TrustTools = "all"
		config.SaveSteerSettings(s)
		fmt.Println("  ✓ Saved: always trust tools (reset with: koda chat --reset-trust)")
		return true
	case "never":
		s.TrustTools = "none"
		config.SaveSteerSettings(s)
		fmt.Println("  ✓ Saved: never trust tools (reset with: koda chat --reset-trust)")
		return false
	default:
		return answer == "" || strings.HasPrefix(answer, "y")
	}
}

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Start an interactive chat with a Kiro agent (proxies to kiro-cli --tui)",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve workspace session
		var wsDir string
		if chatWs != "" {
			wsDir = config.WorkspaceRuntimeDir(chatWs)
			// Auto-materialize if not exists
			if _, err := os.Stat(wsDir); err != nil {
				if steerRoot == "" {
					return fmt.Errorf("steer-runtime not found — cannot materialize workspace")
				}
				ws, err := ops.GetWorkspace(steerRoot, chatWs)
				if err != nil {
					return fmt.Errorf("workspace '%s' not found: %w", chatWs, err)
				}
				fmt.Printf("⏳ Materializing workspace '%s'...\n", chatWs)
				if err := ops.MaterializeWorkspace(steerRoot, ws); err != nil {
					return err
				}
				// Add to active workspaces
				s := config.ReadSteerSettings()
				found := false
				for _, n := range s.ActiveWorkspaces {
					if n == chatWs {
						found = true
						break
					}
				}
				if !found {
					s.ActiveWorkspaces = append(s.ActiveWorkspaces, chatWs)
				}
				if s.PrimaryWorkspace == "" {
					s.PrimaryWorkspace = chatWs
				}
				config.SaveSteerSettings(s)
			}
			ops.TouchWorkspace(chatWs)
		}

		agent := chatAgent
		if agent == "" {
			targetDir := config.TargetDir(projectDir)
			if wsDir != "" {
				targetDir = wsDir
			}
			agent = ops.SuggestDefaultAgent(steerRoot, targetDir)
		}

		trustAll := resolveTrust(chatTrustAll, chatNoTrust, chatResetTrust)
		return launchKiroCLIChatWithWs(agent, trustAll, wsDir, args...)
	},
}

func launchKiroCLIChat(agent string, trustAll bool, extra ...string) error {
	return launchKiroCLIChatWithWs(agent, trustAll, "", extra...)
}

func launchKiroCLIChatWithWs(agent string, trustAll bool, wsDir string, extra ...string) error {
	cliArgs := []string{"chat", "--tui"}
	if trustAll {
		cliArgs = append(cliArgs, "--trust-all-tools")
	}
	if agent != "" {
		cliArgs = append(cliArgs, "--agent", agent)
	}
	cliArgs = append(cliArgs, extra...)
	c := exec.Command(ops.FindKiroCLI(), cliArgs...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	if wsDir != "" {
		c.Env = append(os.Environ(), "KIRO_HOME="+wsDir)
	}
	return c.Run()
}

var promptCmd = &cobra.Command{
	Use:   "prompt [message]",
	Short: "Start a chat using Koda's built-in ACP TUI (with live indicators)",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := chatAgent
		if agent == "" {
			agent = ops.SuggestDefaultAgent(steerRoot, config.TargetDir(projectDir))
		}
		return tui.RunChat(agent)
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatAgent, "agent", "", "Agent to chat with (e.g., orchestrator, backend)")
	chatCmd.Flags().StringVar(&chatWs, "ws", "", "Workspace session to use (materializes on first use)")
	chatCmd.Flags().BoolVar(&chatTrustAll, "trust-all", false, "Trust all tools without prompting")
	chatCmd.Flags().BoolVar(&chatNoTrust, "no-trust", false, "Don't trust any tools (kiro-cli will prompt per tool)")
	chatCmd.Flags().BoolVar(&chatResetTrust, "reset-trust", false, "Clear saved trust preference (will prompt again)")
	promptCmd.Flags().StringVar(&chatAgent, "agent", "", "Agent to chat with (e.g., orchestrator, backend)")
}
