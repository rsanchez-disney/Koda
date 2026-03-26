package cli

import (
	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/tui"
)

var chatAgent string

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat with a Kiro agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.RunChat(chatAgent)
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatAgent, "agent", "", "Agent to chat with (e.g., orchestrator, backend)")
}
