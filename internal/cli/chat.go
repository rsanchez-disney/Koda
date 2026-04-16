package cli

import (
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
	"github.disney.com/SANCR225/koda/internal/tui"
)

var (
	chatAgent string
)

var chatCmd = &cobra.Command{
	Use:   "chat [message]",
	Short: "Start an interactive chat with a Kiro agent (proxies to kiro-cli --tui)",
	RunE: func(cmd *cobra.Command, args []string) error {
		agent := chatAgent
		if agent == "" {
			agent = ops.SuggestDefaultAgent(steerRoot, config.TargetDir(projectDir))
		}

		var cliArgs []string
		cliArgs = append(cliArgs, "chat", "--tui")
		if agent != "" {
			cliArgs = append(cliArgs, "--agent", agent)
		}
		cliArgs = append(cliArgs, args...)

		c := exec.Command(ops.FindKiroCLI(), cliArgs...)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
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
	promptCmd.Flags().StringVar(&chatAgent, "agent", "", "Agent to chat with (e.g., orchestrator, backend)")
}
