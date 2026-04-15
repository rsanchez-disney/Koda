package cli

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

var memoryCmd = &cobra.Command{
	Use:        "memory",
	Short:      "Persistent memory powered by yax",
	Deprecated: "use 'yax' directly. Run 'yax help' for usage.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("yax"); err != nil {
			fmt.Println("yax is not installed. Install it with:")
			fmt.Println("  curl -sSL https://github.disney.com/QUINJ327/yax/raw/main/install.sh | sh")
			return nil
		}
		fmt.Println("Memory is powered by yax. Use 'yax' directly:")
		fmt.Println()
		fmt.Println("  yax save <title> <content>   Save a memory")
		fmt.Println("  yax search <query>           Search memories")
		fmt.Println("  yax context [project]        Recent context")
		fmt.Println("  yax stats                    Statistics")
		fmt.Println("  yax help                     Full help")
		return nil
	},
}
