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
	chatAgent    string
	chatTrustAll bool
	chatNoTrust  bool
)

// resolveTrust returns whether to trust all tools, consulting saved preference
// and prompting the user if needed. Flags override everything.
func resolveTrust(flagTrustAll, flagNoTrust bool) bool {
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
		fmt.Println("  ✓ Saved: always trust tools (reset with: koda chat --no-trust)")
		return true
	case "never":
		s.TrustTools = "none"
		config.SaveSteerSettings(s)
		fmt.Println("  ✓ Saved: never trust tools (reset with: koda chat --trust-all)")
		return false
	default:
		return answer == "" || strings.HasPrefix(answer, "y")
	}
}

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Start an interactive chat with a Kiro agent (proxies to kiro-cli --tui)",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := chatAgent
		if agent == "" {
			agent = ops.SuggestDefaultAgent(steerRoot, config.TargetDir(projectDir))
		}

		trustAll := resolveTrust(chatTrustAll, chatNoTrust)
		return launchKiroCLIChat(agent, trustAll, args...)
	},
}

func launchKiroCLIChat(agent string, trustAll bool, extra ...string) error {
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
	chatCmd.Flags().BoolVar(&chatTrustAll, "trust-all", false, "Trust all tools without prompting")
	chatCmd.Flags().BoolVar(&chatNoTrust, "no-trust", false, "Don't trust any tools (kiro-cli will prompt per tool)")
	promptCmd.Flags().StringVar(&chatAgent, "agent", "", "Agent to chat with (e.g., orchestrator, backend)")
}
