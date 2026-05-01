package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/ops"
)

var psKill bool

var psCmd = &cobra.Command{
	Use:   "ps",
	Short: "List running kiro processes and memory usage",
	Run: func(cmd *cobra.Command, args []string) {
		if psKill {
			killed := ops.KillOrphanProcesses()
			if killed == 0 {
				fmt.Println("  No orphan processes found.")
			} else {
				fmt.Printf("\n  Freed %d process(es). Restart kiro-cli chat for a fresh session.\n", killed)
			}
			return
		}
		procs := ops.ListKiroProcesses()
		ops.PrintProcesses(procs)
	},
}

func init() {
	psCmd.Flags().BoolVar(&psKill, "kill", false, "Kill orphaned sub-agent and stale session processes")
}
