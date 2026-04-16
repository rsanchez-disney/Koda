package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const yaxRepo = "QUINJ327/yax"

// YaxInstalled checks if yax binary is in PATH.
func YaxInstalled() bool {
	_, err := exec.LookPath("yax")
	return err == nil
}

// YaxInstall installs yax using gh release download (authenticated via gh CLI).
// Falls back gracefully if gh is not available or the repo is unreachable.
func YaxInstall() error {
	// Require gh CLI for authenticated GHE access
	ghPath, err := exec.LookPath("gh")
	if err != nil {
		fmt.Println("  ⚠ yax: gh CLI required for install (skipping)")
		return nil
	}

	home, _ := os.UserHomeDir()
	installDir := filepath.Join(home, ".local", "bin")
	os.MkdirAll(installDir, 0755)

	fmt.Println("  📥 Installing yax...")

	// Download latest release binary via gh
	cmd := exec.Command(ghPath, "release", "download", "--repo", yaxRepo,
		"--pattern", "yax-*", "--dir", installDir, "--clobber")
	cmd.Env = append(os.Environ(), "GH_HOST=github.disney.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		// Non-fatal — yax is optional
		fmt.Printf("  ⚠ yax: download failed (skipping): %s\n", string(out))
		return nil
	}

	// Make executable
	yaxBin := filepath.Join(installDir, "yax")
	os.Chmod(yaxBin, 0755)

	// Run setup
	if _, err := os.Stat(yaxBin); err == nil {
		s := exec.Command(yaxBin, "setup")
		if err := s.Run(); err != nil {
			fmt.Printf("  ⚠ yax setup: %v\n", err)
		}
	}

	fmt.Println("  ✅ yax installed")
	return nil
}
