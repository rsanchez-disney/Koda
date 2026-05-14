package apps

import (
	"fmt"
	"os/exec"
	"runtime"

	"github.disney.com/SANCR225/koda/internal/ops"
	"github.disney.com/SANCR225/koda/internal/pkg"
)

// Install downloads and installs an app from the catalog.
func Install(app *App) error {
	if pkg.IsInstalled(app.Name) {
		return fmt.Errorf("%s is already installed at %s", app.Name, pkg.BundlePath(app.Name))
	}
	key := ops.GetReleaseKey()
	if key == "" {
		return fmt.Errorf("release key not available in this build")
	}
	fmt.Printf("⚡ Installing %s...\n", app.Name)
	rel, err := pkg.FetchLatestRelease(app.Repo)
	if err != nil {
		return fmt.Errorf("fetch release: %w", err)
	}
	fmt.Printf("  Version: %s\n", rel.TagName)
	artifact := app.ResolveArtifact()
	if artifact == "" {
		return fmt.Errorf("no artifact for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	url, err := pkg.FindAssetURL(rel, artifact)
	if err != nil {
		return err
	}
	if err := pkg.InstallBundle(app.Name, url, key); err != nil {
		return err
	}
	fmt.Printf("\n✅ %s %s installed\n", app.Name, rel.TagName)
	fmt.Printf("  Path: %s\n", pkg.BundlePath(app.Name))
	return nil
}

// Update fetches the latest release and reinstalls.
func Update(app *App) error {
	if !pkg.IsInstalled(app.Name) {
		return fmt.Errorf("%s is not installed", app.Name)
	}
	key := ops.GetReleaseKey()
	if key == "" {
		return fmt.Errorf("release key not available in this build")
	}
	rel, err := pkg.FetchLatestRelease(app.Repo)
	if err != nil {
		return fmt.Errorf("fetch release: %w", err)
	}
	artifact := app.ResolveArtifact()
	if artifact == "" {
		return fmt.Errorf("no artifact for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	url, err := pkg.FindAssetURL(rel, artifact)
	if err != nil {
		return err
	}
	fmt.Printf("⚡ Updating %s to %s...\n", app.Name, rel.TagName)
	if err := pkg.InstallBundle(app.Name, url, key); err != nil {
		return err
	}
	fmt.Printf("✅ %s updated to %s\n", app.Name, rel.TagName)
	return nil
}

// Uninstall removes an installed app.
func Uninstall(app *App) error {
	if err := pkg.UninstallBundle(app.Name); err != nil {
		return err
	}
	fmt.Printf("✅ %s uninstalled\n", app.Name)
	return nil
}

// Start launches an installed app.
func Start(app *App) error {
	if !pkg.IsInstalled(app.Name) {
		return fmt.Errorf("%s is not installed. Run 'koda apps install %s'", app.Name, app.Name)
	}
	base := pkg.BundlePath(app.Name)
	target := app.LaunchCommand()
	if target == "" {
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", base+"/"+target).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", "", base+`\`+target).Start()
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
}
