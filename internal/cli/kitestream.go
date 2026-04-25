package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/kitestream"
	"github.disney.com/SANCR225/koda/internal/ops"
	"github.disney.com/SANCR225/koda/internal/pkg"
)

const (
	kitestreamRepo = "rsanchez-disney/kitestream"
	kitestreamName = "kitestream"
)

var kitestreamPort int

var kitestreamCmd = &cobra.Command{
	Use:   "kitestream [command]",
	Short: "Browser-based control surface for Kiro sessions",
	Long:  "Manage KiteStream — install, start, stop, restart, update, uninstall.",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default: start
		return kitestreamStartCmd.RunE(cmd, args)
	},
}

var kitestreamInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install KiteStream",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pkg.IsInstalled(kitestreamName) {
			fmt.Println("KiteStream is already installed.")
			fmt.Printf("  Path: %s\n", pkg.BundlePath(kitestreamName))
			return nil
		}
		key := ops.GetReleaseKey()
		if key == "" {
			return fmt.Errorf("release key not available in this build")
		}
		fmt.Println("⚡ Installing KiteStream...")
		rel, err := pkg.FetchLatestRelease(kitestreamRepo)
		if err != nil {
			return fmt.Errorf("fetch release: %w", err)
		}
		fmt.Printf("  Version: %s\n", rel.TagName)
		manifest := &pkg.PackageManifest{
			Platforms: []pkg.Platform{
				{OS: "darwin", Arch: "arm64", Artifact: "kitestream-darwin-arm64.tar.gz.enc"},
				{OS: "darwin", Arch: "amd64", Artifact: "kitestream-darwin-amd64.tar.gz.enc"},
				{OS: "linux", Arch: "amd64", Artifact: "kitestream-linux-amd64.tar.gz.enc"},
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
		if err := pkg.InstallBundle(kitestreamName, url, key); err != nil {
			return err
		}
		fmt.Printf("\n✅ KiteStream %s installed\n", rel.TagName)
		fmt.Printf("  Path: %s\n", pkg.BundlePath(kitestreamName))
		fmt.Println("  Run 'koda kitestream' to start")
		return nil
	},
}

var kitestreamStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start KiteStream server and open browser",
	RunE: func(cmd *cobra.Command, args []string) error {
		devMode := os.Getenv("KITESTREAM_DEV_DIST") != ""
		if !devMode && !pkg.IsInstalled(kitestreamName) {
			fmt.Println("KiteStream is not installed.")
			fmt.Println("Run 'koda kitestream install' to get started.")
			fmt.Println("Or set KITESTREAM_DEV_DIST to serve from a local build.")
			return nil
		}
		target := config.TargetDir(projectDir)
		sr := steerRoot
		if sr == "" {
			sr = config.DefaultSteerRoot()
		}
		return kitestream.Launch(sr, target, kitestreamPort)
	},
}

var kitestreamStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop running KiteStream server",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := kitestream.Stop(); err != nil {
			return err
		}
		fmt.Println("✅ KiteStream stopped")
		return nil
	},
}

var kitestreamRestartCmd = &cobra.Command{
	Use:   "restart",
	Short: "Restart KiteStream server",
	RunE: func(cmd *cobra.Command, args []string) error {
		kitestream.Stop() // ignore error if not running
		return kitestreamStartCmd.RunE(cmd, args)
	},
}

var kitestreamUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update KiteStream to latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		ifInstalled, _ := cmd.Flags().GetBool("if-installed")
		if !pkg.IsInstalled(kitestreamName) {
			if ifInstalled {
				return nil // silent skip
			}
			return fmt.Errorf("KiteStream is not installed")
		}
		key := ops.GetReleaseKey()
		if key == "" {
			return fmt.Errorf("release key not available in this build")
		}
		wasRunning := kitestream.IsRunning()
		if wasRunning {
			fmt.Println("Stopping KiteStream...")
			kitestream.Stop()
		}
		pkg.UninstallBundle(kitestreamName)
		if err := kitestreamInstallCmd.RunE(cmd, args); err != nil {
			return err
		}
		if wasRunning {
			fmt.Println("Restarting KiteStream...")
			return kitestreamStartCmd.RunE(cmd, args)
		}
		return nil
	},
}

var kitestreamUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall KiteStream",
	RunE: func(cmd *cobra.Command, args []string) error {
		if kitestream.IsRunning() {
			kitestream.Stop()
		}
		if err := pkg.UninstallBundle(kitestreamName); err != nil {
			return err
		}
		fmt.Println("✅ KiteStream uninstalled")
		return nil
	},
}

var kitestreamStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show KiteStream status",
	Run: func(cmd *cobra.Command, args []string) {
		installed := pkg.IsInstalled(kitestreamName)
		fmt.Printf("Installed: %v\n", installed)
		if installed {
			fmt.Printf("  Path: %s\n", pkg.BundlePath(kitestreamName))
		}
		fmt.Printf("Status: %s\n", kitestream.Status(kitestreamPort))
	},
}

func init() {
	kitestreamCmd.AddCommand(kitestreamInstallCmd)
	kitestreamCmd.AddCommand(kitestreamStartCmd)
	kitestreamCmd.AddCommand(kitestreamStopCmd)
	kitestreamCmd.AddCommand(kitestreamRestartCmd)
	kitestreamCmd.AddCommand(kitestreamUpdateCmd)
	kitestreamCmd.AddCommand(kitestreamUninstallCmd)
	kitestreamCmd.AddCommand(kitestreamStatusCmd)

	kitestreamStartCmd.Flags().IntVar(&kitestreamPort, "port", kitestream.DefaultPort, "HTTP port")
	kitestreamCmd.Flags().IntVar(&kitestreamPort, "port", kitestream.DefaultPort, "HTTP port")
	kitestreamUpdateCmd.Flags().Bool("if-installed", false, "Skip silently if not installed")
}
