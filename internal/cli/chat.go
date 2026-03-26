package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/acp"
	"github.disney.com/SANCR225/koda/internal/tui"
)

var (
	chatAgent string
	chatDebug bool
)

var chatCmd = &cobra.Command{
	Use:   "chat",
	Short: "Start an interactive chat with a Kiro agent",
	RunE: func(cmd *cobra.Command, args []string) error {
		if chatDebug {
			logFile := "koda-debug.log"
			acp.EnableDebug(logFile)
			fmt.Printf("Debug log: %s\n", logFile)
		}
		return tui.RunChat(chatAgent)
	},
}

func init() {
	chatCmd.Flags().StringVar(&chatAgent, "agent", "", "Agent to chat with (e.g., orchestrator, backend)")
	chatCmd.Flags().BoolVar(&chatDebug, "debug", false, "Log ACP traffic to koda-debug.log")
}
