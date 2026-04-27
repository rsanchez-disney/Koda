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

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Start an interactive chat with a Kiro agent (proxies to kiro-cli --tui)",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := chatAgent
		if agent == "" {
			agent = ops.SuggestDefaultAgent(steerRoot, config.TargetDir(projectDir))
		}

		trustAll := chatTrustAll
		if !chatTrustAll && !chatNoTrust {
			fmt.Print("Trust all tools? (Y/n): ")
			var answer string
			fmt.Scanln(&answer)
			trustAll = answer == "" || strings.HasPrefix(strings.ToLower(answer), "y")
		}

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
