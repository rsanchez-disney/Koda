package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/model"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var envCmd = &cobra.Command{
	Use:   "env",
	Short: "Manage environment variables",
}

var envListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all env vars",
	RunE: func(cmd *cobra.Command, args []string) error {
		vars := ops.ReadEnvVars()
		for _, e := range model.KnownEnvVars {
			fmt.Printf("  %-20s %s\n", e.Key, vars[e.Key])
		}
		// Custom vars
		known := map[string]bool{}
		for _, e := range model.KnownEnvVars {
			known[e.Key] = true
		}
		for k, v := range vars {
			if !known[k] {
				fmt.Printf("  %-20s %s\n", k, v)
			}
		}
		return nil
	},
}

var envGetCmd = &cobra.Command{
	Use:   "get KEY",
	Short: "Get an env var value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(ops.GetEnvVar(args[0]))
		return nil
	},
}

var envSetCmd = &cobra.Command{
	Use:   "set KEY=VALUE",
	Short: "Set an env var",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		k, v, ok := strings.Cut(args[0], "=")
		if !ok {
			return fmt.Errorf("format: KEY=VALUE")
		}
		vars := ops.ReadEnvVars()
		vars[k] = v
		if err := ops.WriteEnvVars(vars); err != nil {
			return err
		}
		fmt.Printf("✓ %s=%s\n", k, v)
		return nil
	},
}

func init() {
	envCmd.AddCommand(envListCmd)
	envCmd.AddCommand(envGetCmd)
	envCmd.AddCommand(envSetCmd)
}
