package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

// YaxInstalled checks if yax binary is in PATH.
func YaxInstalled() bool {
	_, err := exec.LookPath("yax")
	return err == nil
}

// YaxInstall installs yax.
// Priority: 1) binary from Koda releases, 2) build from source via GHE install script.
func YaxInstall() error {
	home, _ := os.UserHomeDir()
	installDir := filepath.Join(home, ".local", "bin")
	os.MkdirAll(installDir, 0755)

	fmt.Println("  📥 Installing yax...")

	// Try binary download from Koda releases
	asset := fmt.Sprintf("yax-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	dest := filepath.Join(installDir, "yax")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}

	if ghPath, err := exec.LookPath("gh"); err == nil {
		cmd := exec.Command(ghPath, "release", "download", "--repo", "rsanchez-disney/Koda",
			"--pattern", asset, "--dir", installDir, "--clobber")
		cmd.Env = append(os.Environ(), "GH_HOST=github.com")
		if _, err := cmd.CombinedOutput(); err == nil {
			downloaded := filepath.Join(installDir, asset)
			if downloaded != dest {
				os.Rename(downloaded, dest)
			}
			os.Chmod(dest, 0755)
			fmt.Println("  ✅ yax installed")
			return nil
		}
	}

	// Fallback: build from source (requires Go + GHE access)
	fmt.Println("  ⚠ yax binary not found in release, trying build from source...")
	url := "https://github.disney.com/raw/QUINJ327/yax/main/install.sh"
	cmd := exec.Command("bash", "-c", fmt.Sprintf("curl -fsSL %s | sh", url))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("  ⚠ yax: install failed (skipping): %v\n", err)
		return nil
	}

	fmt.Println("  ✅ yax installed from source")
	return nil
}
