package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/apps"
	"github.disney.com/SANCR225/koda/internal/pkg"
)

var appsCmd = &cobra.Command{
	Use:   "apps [command]",
	Short: "Manage Koda apps (install, update, start, uninstall)",
	Long:  "Browse and manage desktop apps distributed through Koda.",
	Run: func(cmd *cobra.Command, args []string) {
		appsListCmd.Run(cmd, args)
	},
}

var appsListCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	Short:   "List available apps",
	Run: func(cmd *cobra.Command, args []string) {
		catalog := apps.Catalog()
		fmt.Println("Available apps:")
		fmt.Println()
		for _, a := range catalog {
			installed := "  "
			if pkg.IsInstalled(a.Name) {
				installed = "✓ "
			}
			fmt.Printf("  %s%-14s %s\n", installed, a.Name, a.Description)
		}
		fmt.Println("\nUse 'koda apps install <name>' to install an app.")
	},
}

var appsInstallCmd = &cobra.Command{
	Use:   "install <app>",
	Short: "Install an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := apps.Find(args[0])
		if app == nil {
			return fmt.Errorf("unknown app: %s. Run 'koda apps list' to see available apps", args[0])
		}
		return apps.Install(app)
	},
}

var appsUpdateCmd = &cobra.Command{
	Use:   "update <app>",
	Short: "Update an app to latest version",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := apps.Find(args[0])
		if app == nil {
			return fmt.Errorf("unknown app: %s", args[0])
		}
		return apps.Update(app)
	},
}

var appsUninstallCmd = &cobra.Command{
	Use:   "uninstall <app>",
	Short: "Uninstall an app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := apps.Find(args[0])
		if app == nil {
			return fmt.Errorf("unknown app: %s", args[0])
		}
		return apps.Uninstall(app)
	},
}

var appsStartCmd = &cobra.Command{
	Use:   "start <app>",
	Short: "Launch an installed app",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := apps.Find(args[0])
		if app == nil {
			return fmt.Errorf("unknown app: %s", args[0])
		}
		return apps.Start(app)
	},
}

var appsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show installed apps and their paths",
	Run: func(cmd *cobra.Command, args []string) {
		catalog := apps.Catalog()
		any := false
		for _, a := range catalog {
			if pkg.IsInstalled(a.Name) {
				any = true
				fmt.Printf("  ✓ %-14s %s\n", a.Name, pkg.BundlePath(a.Name))
			}
		}
		if !any {
			fmt.Println("No apps installed. Run 'koda apps list' to see available apps.")
		}
	},
}

var appsSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Search available apps",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		query := ""
		if len(args) > 0 {
			query = strings.ToLower(args[0])
		}
		catalog := apps.Catalog()
		var matches []string
		for _, a := range catalog {
			if query == "" || strings.Contains(strings.ToLower(a.Name), query) || strings.Contains(strings.ToLower(a.Description), query) {
				installed := "  "
				if pkg.IsInstalled(a.Name) {
					installed = "✓ "
				}
				fmt.Printf("  %s%-14s %s\n", installed, a.Name, a.Description)
				matches = append(matches, a.Name)
			}
		}
		if len(matches) == 0 && query != "" {
			fmt.Printf("No apps matching '%s'.\n", query)
		} else if len(matches) == 0 {
			fmt.Println("No apps available.")
		} else {
			fmt.Printf("\nTo install: koda apps install <name>\n")
		}
	},
}

func init() {
	appsCmd.AddCommand(appsListCmd)
	appsCmd.AddCommand(appsInstallCmd)
	appsCmd.AddCommand(appsUpdateCmd)
	appsCmd.AddCommand(appsUninstallCmd)
	appsCmd.AddCommand(appsStartCmd)
	appsCmd.AddCommand(appsStatusCmd)
	appsCmd.AddCommand(appsSearchCmd)
}
