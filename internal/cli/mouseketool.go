package cli

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.disney.com/SANCR225/koda/internal/ops"
	"github.disney.com/SANCR225/koda/internal/pkg"
)

const (
	mouseketoolRepo   = "rsanchez-disney/mouseketool"
	mouseketoolName   = "mouseketool"
	mouseketoolApp    = "Mouseketool.app"
	mouseketoolExe    = "Mouseketool.exe"
)

var mouseketoolCmd = &cobra.Command{
	Use:   "mouseketool [command]",
	Short: "Local AWS companion for backend developers",
	Long:  "Manage Mouseketool — install, start, update, uninstall, status.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return mouseketoolStartCmd.RunE(cmd, args)
	},
}

var mouseketoolInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Install Mouseketool",
	RunE: func(cmd *cobra.Command, args []string) error {
		if pkg.IsInstalled(mouseketoolName) {
			fmt.Println("Mouseketool is already installed.")
			fmt.Printf("  Path: %s\n", pkg.BundlePath(mouseketoolName))
			return nil
		}
		key := ops.GetReleaseKey()
		if key == "" {
			return fmt.Errorf("release key not available in this build")
		}
		fmt.Println("⚡ Installing Mouseketool...")
		rel, err := pkg.FetchLatestRelease(mouseketoolRepo)
		if err != nil {
			return fmt.Errorf("fetch release: %w", err)
		}
		fmt.Printf("  Version: %s\n", rel.TagName)
		manifest := &pkg.PackageManifest{
			Platforms: []pkg.Platform{
				{OS: "darwin", Arch: "arm64", Artifact: "mouseketool-darwin-arm64.tar.gz.enc"},
				{OS: "darwin", Arch: "amd64", Artifact: "mouseketool-darwin-amd64.tar.gz.enc"},
				{OS: "windows", Arch: "amd64", Artifact: "mouseketool-windows-amd64.tar.gz.enc"},
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
		if err := pkg.InstallBundle(mouseketoolName, url, key); err != nil {
			return err
		}
		fmt.Printf("\n✅ Mouseketool %s installed\n", rel.TagName)
		fmt.Printf("  Path: %s\n", pkg.BundlePath(mouseketoolName))
		fmt.Println("  Run 'koda mouseketool' to launch")
		return nil
	},
}

var mouseketoolStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Launch Mouseketool",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !pkg.IsInstalled(mouseketoolName) {
			return fmt.Errorf("Mouseketool is not installed. Run 'koda mouseketool install' to get started")
		}
		return launchMouseketool()
	},
}

var mouseketoolUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update Mouseketool to latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !pkg.IsInstalled(mouseketoolName) {
			return fmt.Errorf("Mouseketool is not installed")
		}
		key := ops.GetReleaseKey()
		if key == "" {
			return fmt.Errorf("release key not available in this build")
		}
		// Fetch release info before uninstalling to fail early
		rel, err := pkg.FetchLatestRelease(mouseketoolRepo)
		if err != nil {
			return fmt.Errorf("fetch release: %w", err)
		}
		manifest := &pkg.PackageManifest{
			Platforms: []pkg.Platform{
				{OS: "darwin", Arch: "arm64", Artifact: "mouseketool-darwin-arm64.tar.gz.enc"},
				{OS: "darwin", Arch: "amd64", Artifact: "mouseketool-darwin-amd64.tar.gz.enc"},
				{OS: "windows", Arch: "amd64", Artifact: "mouseketool-windows-amd64.tar.gz.enc"},
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
		fmt.Printf("⚡ Updating Mouseketool to %s...\n", rel.TagName)
		if err := pkg.InstallBundle(mouseketoolName, url, key); err != nil {
			return err
		}
		fmt.Printf("✅ Mouseketool updated to %s\n", rel.TagName)
		return nil
	},
}

var mouseketoolUninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall Mouseketool",
	RunE: func(cmd *cobra.Command, args []string) error {
		if err := pkg.UninstallBundle(mouseketoolName); err != nil {
			return err
		}
		fmt.Println("✅ Mouseketool uninstalled")
		return nil
	},
}

var mouseketoolStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Mouseketool status",
	Run: func(cmd *cobra.Command, args []string) {
		installed := pkg.IsInstalled(mouseketoolName)
		fmt.Printf("Installed: %v\n", installed)
		if installed {
			fmt.Printf("  Path: %s\n", pkg.BundlePath(mouseketoolName))
		}
	},
}

func launchMouseketool() error {
	base := pkg.BundlePath(mouseketoolName)
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", base+"/"+mouseketoolApp).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", "", base+`\`+mouseketoolExe).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}

func init() {
	mouseketoolCmd.AddCommand(mouseketoolInstallCmd)
	mouseketoolCmd.AddCommand(mouseketoolStartCmd)
	mouseketoolCmd.AddCommand(mouseketoolUpdateCmd)
	mouseketoolCmd.AddCommand(mouseketoolUninstallCmd)
	mouseketoolCmd.AddCommand(mouseketoolStatusCmd)
}
