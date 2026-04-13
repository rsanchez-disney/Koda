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
	chatLite  bool
)

var chatCmd = &cobra.Command{
	Use:                "chat [message]",
	Short:              "Start an interactive chat with a Kiro agent",
	DisableFlagParsing: false,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Resolve agent: flag > workspace default > auto-detect
		agent := chatAgent
		if agent == "" {
			agent = ops.SuggestDefaultAgent(steerRoot, config.TargetDir(projectDir))
		}

		// Lite mode: proxy to kiro-cli
		if chatLite {
			var cliArgs []string
			cliArgs = append(cliArgs, "chat")
			if agent != "" {
				cliArgs = append(cliArgs, "--agent", agent)
			}
			cliArgs = append(cliArgs, args...)

			c := exec.Command("kiro-cli", cliArgs...)
			c.Stdin = os.Stdin
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			return c.Run()
		}

		// Default: rich ACP TUI with live indicators
		return tui.RunChat(agent)
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatAgent, "agent", "", "Agent to chat with (e.g., orchestrator, backend)")
	chatCmd.Flags().BoolVar(&chatLite, "lite", false, "Lite mode — proxy to kiro-cli chat (no live indicators)")
}
