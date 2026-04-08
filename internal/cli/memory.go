package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Manage memory-mcp persistent memory server",
	RunE: func(cmd *cobra.Command, args []string) error {
		return memoryStatusCmd.RunE(cmd, args)
	},
}

var memoryStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the memory-mcp Docker stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("🧠 Starting memory-mcp...")
		if err := ops.MemoryStart(config.TargetDir(projectDir)); err != nil {
			return err
		}
		fmt.Println("✅ memory-mcp running on http://localhost:9377")
		fmt.Println("   Run 'koda mcp-install' to add it to mcp.json")
		return nil
	},
}

var memoryStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the memory-mcp Docker stack",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Stopping memory-mcp...")
		return ops.MemoryStop(config.TargetDir(projectDir))
	},
}

var memoryStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show memory-mcp status",
	RunE: func(cmd *cobra.Command, args []string) error {
		status := ops.MemoryStatus(config.TargetDir(projectDir))
		if jsonOutput {
			out, _ := json.MarshalIndent(status, "", "  ")
			fmt.Println(string(out))
			return nil
		}
		fmt.Println("🧠 Memory MCP Status")
		fmt.Println()
		if !status.Installed {
			fmt.Println("  ✗ Not installed (install a profile with memory-mcp first)")
			return nil
		}
		fmt.Println("  ✓ Installed")
		if status.Runtime == "" {
			fmt.Println("  ✗ No container runtime found (need docker, nerdctl, or podman)")
			return nil
		}
		fmt.Printf("  ✓ Runtime: %s\n", status.Runtime)
		if status.Running {
			fmt.Printf("  ✓ Running on port %d\n", status.Port)
			fmt.Printf("  ✓ Health: %s\n", status.Health)
		} else {
			fmt.Println("  ✗ Not running — use 'koda memory start'")
		}
		return nil
	},
}

func init() {
	memoryCmd.AddCommand(memoryStartCmd)
	memoryCmd.AddCommand(memoryStopCmd)
	memoryCmd.AddCommand(memoryStatusCmd)
}
