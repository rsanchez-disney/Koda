package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/ops"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Check and install missing dependencies",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("\U0001f50d Checking dependencies...")
		fmt.Println()

		deps := ops.CheckDeps()
		var missing []ops.Dep

		for _, d := range deps {
			if d.Installed {
				ver := d.Version
				if len(ver) > 40 {
					ver = ver[:40]
				}
				fmt.Printf("  \u2713 %-14s %s\n", d.Name, ver)
			} else {
				fmt.Printf("  \u2717 %-14s not found\n", d.Name)
				missing = append(missing, d)
			}
		}

		if len(missing) == 0 {
			fmt.Println("\n\u2705 All dependencies installed")
			return nil
		}

		fmt.Printf("\n%d missing. Install them? [y/N]: ", len(missing))
		var answer string
		fmt.Scanln(&answer)
		if strings.ToLower(answer) != "y" {
			fmt.Println("Skipped.")
			return nil
		}

		fmt.Println()
		for _, d := range missing {
			fmt.Printf("\U0001f4e6 Installing %s...\n", d.Name)
			if err := ops.InstallDep(d); err != nil {
				fmt.Printf("  \u26a0 %s: %v\n", d.Name, err)
			} else {
				fmt.Printf("  \u2713 %s\n", d.Name)
			}
			fmt.Println()
		}

		fmt.Println("\u2705 Setup complete")
		return nil
	},
}
