package cli

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/pkg"
)

const (
	autopilotRepo = "rsanchez-disney/steer-autopilot"
	autopilotName = "autopilot"
)

// autopilotCmd is the parent command: koda autopilot [subcommand | proxy to binary]
var autopilotCmd = &cobra.Command{
	Use:   "autopilot [command]",
	Short: "AI-SDLC pipeline orchestrator",
	Long:  "Run autopilot commands. If autopilot is installed, delegates to the autopilot binary.",
	DisableFlagParsing: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !pkg.IsInstalled(autopilotName) {
			fmt.Println("autopilot is not installed.")
			fmt.Println("Run 'koda autopilot install' to get started.")
			return nil
		}
		// Proxy all args to the autopilot binary
		return proxyToAutopilot(args)
	},
}

var autopilotInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install autopilot",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pkg.IsInstalled(autopilotName) {
			fmt.Println("autopilot is already installed.")
			fmt.Printf("  Path: %s\n", pkg.BinPath(autopilotName))
			return nil
		}

		key := os.Getenv("STEER_RELEASE_KEY")
		if key == "" {
			return fmt.Errorf("STEER_RELEASE_KEY env var required")
		}

		fmt.Println("⚡ Installing autopilot...")

		// Fetch latest release
		rel, err := pkg.FetchLatestRelease(autopilotRepo)
		if err != nil {
			return fmt.Errorf("fetch release: %w", err)
		}
		fmt.Printf("  Version: %s\n", rel.TagName)

		// Resolve platform artifact
		manifest := &pkg.PackageManifest{
			Platforms: []pkg.Platform{
				{OS: "darwin", Arch: "arm64", Artifact: "autopilot-darwin-arm64.tar.gz.enc"},
				{OS: "darwin", Arch: "amd64", Artifact: "autopilot-darwin-amd64.tar.gz.enc"},
				{OS: "linux", Arch: "amd64", Artifact: "autopilot-linux-amd64.tar.gz.enc"},
			},
		}
		platform, err := pkg.ResolveArtifact(manifest)
		if err != nil {
			return err
		}

		url, err := pkg.FindAssetURL(rel, platform.Artifact)
		if err != nil {
			return err
		}

		if err := pkg.Install(autopilotName, url, key); err != nil {
			return err
		}

		fmt.Printf("\n✅ autopilot %s installed\n", rel.TagName)
		fmt.Printf("  Path: %s\n", pkg.BinPath(autopilotName))

		// Run post-install verification
		out, err := exec.Command(pkg.BinPath(autopilotName), "version").Output()
		if err == nil {
			fmt.Printf("  %s", string(out))
		}
		return nil
	},
}

var autopilotUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update autopilot to latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		key := os.Getenv("STEER_RELEASE_KEY")
		if key == "" {
			return fmt.Errorf("STEER_RELEASE_KEY env var required")
		}
		// Uninstall + reinstall
		pkg.Uninstall(autopilotName)
		return autopilotInstallCmd.RunE(cmd, args)
	},
}

var autopilotUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall autopilot",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := pkg.Uninstall(autopilotName); err != nil {
			return err
		}
		fmt.Println("✅ autopilot uninstalled")
		return nil
	},
}

// proxyToAutopilot delegates to the autopilot binary with context injection.
func proxyToAutopilot(args []string) error {
	binPath := pkg.BinPath(autopilotName)
	cmd := exec.Command(binPath, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Context injection: pass Koda workspace info as env vars
	cmd.Env = append(os.Environ(),
		"KODA_HOME="+config.KiroRoot(),
		"KODA_STEER_ROOT="+steerRoot,
		"KODA_PROJECT_DIR="+projectDir,
	)

	return cmd.Run()
}

func init() {
	// Sub-commands that are handled by Koda (not proxied)
	autopilotCmd.AddCommand(autopilotInstallCmd)
	autopilotCmd.AddCommand(autopilotUpdateCmd)
	autopilotCmd.AddCommand(autopilotUninstallCmd)
}
