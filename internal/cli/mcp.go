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
	mcpCmd.AddCommand(mcpStatusCmd)
	mcpCmd.AddCommand(mcpAddCmd)
	mcpAddCmd.Flags().BoolVar(&mcpAddFork, "fork", false, "Create at fork level (shared/tools/mcp-servers/) instead of workspace level")
}

var mcpAddFork bool

var mcpAddCmd = &cobra.Command{
	Use:   "add <name>",
	Short: "Scaffold a new custom MCP server (workspace or fork level)",
	Long: `Scaffold a new MCP server in the active workspace or fork.

Examples:
  koda mcp add team-db              # workspace-level (workspaces/<active>/mcp/)
  koda mcp add splunkweb --fork     # fork-level (shared/tools/mcp-servers/)`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		return ops.ScaffoldMCP(name, mcpAddFork)
	},
}

var mcpStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show MCP servers grouped by source (global, fork, workspace, user)",
	RunE: func(cmd *cobra.Command, args []string) error {
		servers, err := ops.ListMCPServersBySource()
		if err != nil {
			return err
		}
		groups := map[string][]ops.MCPServerSourceStatus{
			"global":    {},
			"fork":      {},
			"workspace": {},
			"user":      {},
		}
		for _, srv := range servers {
			source := srv.Source
			if source == "" {
				source = "global"
			}
			// Group workspace:X under "workspace"
			if len(source) > 10 && source[:10] == "workspace:" {
				groups["workspace"] = append(groups["workspace"], srv)
			} else if _, ok := groups[source]; ok {
				groups[source] = append(groups[source], srv)
			} else {
				groups["global"] = append(groups["global"], srv)
			}
		}

		fmt.Println("📋 MCP Servers:")
		for _, group := range []string{"global", "fork", "workspace", "user"} {
			srvs := groups[group]
			if len(srvs) == 0 {
				continue
			}
			label := group
			if group == "workspace" && len(srvs) > 0 {
				label = srvs[0].Source // "workspace:app-team"
			}
			fmt.Printf("\n  %s (%d):\n", label, len(srvs))
			for _, srv := range srvs {
				icon := "✅"
				if srv.Disabled {
					icon = "⏸️"
				}
				fmt.Printf("    %s %-24s\n", icon, srv.Name)
			}
		}
		fmt.Println()
		return nil
	},
}
