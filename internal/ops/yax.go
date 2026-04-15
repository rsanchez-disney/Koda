package ops

import (
	"fmt"
	"os/exec"
)

const yaxInstallURL = "https://github.disney.com/QUINJ327/yax/raw/main/install.sh"

// YaxInstalled checks if yax binary is in PATH.
func YaxInstalled() bool {
	_, err := exec.LookPath("yax")
	return err == nil
}

// YaxInstall installs yax via the official install script.
func YaxInstall() error {
	fmt.Println("  📥 Installing yax...")
	cmd := exec.Command("sh", "-c", "curl -sSL "+yaxInstallURL+" | sh")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("yax install failed: %w", err)
	}
	// Run setup to initialize MCP config
	if setup, err := exec.LookPath("yax"); err == nil {
		s := exec.Command(setup, "setup")
		if err := s.Run(); err != nil {
			fmt.Printf("  ⚠ yax setup warning: %v\n", err)
		}
	}
	fmt.Println("  ✅ yax installed")
	return nil
}
