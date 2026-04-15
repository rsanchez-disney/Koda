package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

const yaxRepo = "https://github.disney.com/QUINJ327/yax.git"

// YaxInstalled checks if yax binary is in PATH.
func YaxInstalled() bool {
	_, err := exec.LookPath("yax")
	return err == nil
}

// YaxInstall clones the yax repo, builds, and installs to ~/.local/bin.
func YaxInstall() error {
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("Go is required to build yax. Install from https://go.dev/dl/")
	}

	home, _ := os.UserHomeDir()
	installDir := filepath.Join(home, ".local", "bin")
	os.MkdirAll(installDir, 0755)

	tmpDir, err := os.MkdirTemp("", "yax-build-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	// Clone
	fmt.Println("  📥 Downloading yax...")
	cmd := exec.Command("git", "clone", "--depth", "1", yaxRepo, filepath.Join(tmpDir, "yax"))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("clone yax: %w", err)
	}

	// Build
	fmt.Println("  🔨 Building yax...")
	build := exec.Command("go", "build", "-o", filepath.Join(installDir, "yax"), "./cmd/yax")
	build.Dir = filepath.Join(tmpDir, "yax")
	build.Stdout = os.Stdout
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		return fmt.Errorf("build yax: %w", err)
	}

	// Setup MCP
	setup := exec.Command(filepath.Join(installDir, "yax"), "setup")
	setup.Stdout = os.Stdout
	setup.Stderr = os.Stderr
	setup.Run()

	fmt.Println("  ✅ yax installed")
	return nil
}
