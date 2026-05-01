package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/ops"
)

var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Manage MCP servers (list, enable, disable)",
}

var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List MCP servers and their status",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := ops.ListMCPServers()
		if err != nil {
			return err
		}
		for _, srv := range servers {
			status := "✓ enabled"
			if srv.Disabled {
				status = "✕ disabled"
			}
			fmt.Printf("  %-24s %s\n", srv.Name, status)
		}
		return nil
	},
}

var mcpEnableCmd = &cobra.Command{
	Use:   "enable <server>",
	Short: "Enable an MCP server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ops.ToggleMCPServer(args[0], false); err != nil {
			return err
		}
		fmt.Printf("✓ %s enabled — restart Kiro to apply\n", args[0])
		return nil
	},
}

var mcpDisableCmd = &cobra.Command{
	Use:   "disable <server>",
	Short: "Disable an MCP server",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := ops.ToggleMCPServer(args[0], true); err != nil {
			return err
		}
		fmt.Printf("✓ %s disabled — restart Kiro to apply\n", args[0])
		return nil
	},
}

func init() {
	mcpCmd.AddCommand(mcpListCmd)
	mcpCmd.AddCommand(mcpEnableCmd)
	mcpCmd.AddCommand(mcpDisableCmd)
}
