package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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

	// Fallback: direct curl download from GitHub releases
	fmt.Println("  ⚠ yax: gh not available, trying curl...")
	url := fmt.Sprintf("https://github.com/rsanchez-disney/Koda/releases/latest/download/%s", asset)
	curlBin := "curl"
	if runtime.GOOS == "windows" {
		curlBin = "curl.exe"
	}
	cmd := exec.Command(curlBin, "-fsSL", "-o", dest, url)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("  ⚠ yax: download failed (skipping): %s\n", string(out))
		return nil
	}
	os.Chmod(dest, 0755)
	fmt.Println("  ✅ yax installed")
	return nil
}

// YaxStatus holds yax installation and usage info.
type YaxStatus struct {
	Installed    bool
	Version      string
	Path         string
	Observations int
	Sessions     int
	Edges        int
	Prompts      int
}

// GetYaxStatus checks yax installation and stats.
func GetYaxStatus() YaxStatus {
	yaxBin, err := exec.LookPath("yax")
	if err != nil {
		return YaxStatus{}
	}
	s := YaxStatus{Installed: true, Path: yaxBin}
	if out, err := exec.Command(yaxBin, "version").Output(); err == nil {
		s.Version = strings.TrimSpace(string(out))
	}
	if out, err := exec.Command(yaxBin, "stats").Output(); err == nil {
		var stats struct {
			Observations int `json:"total_observations"`
			Sessions     int `json:"total_sessions"`
			Edges        int `json:"total_edges"`
			Prompts      int `json:"total_prompts"`
		}
		if json.Unmarshal(out, &stats) == nil {
			s.Observations = stats.Observations
			s.Sessions = stats.Sessions
			s.Edges = stats.Edges
			s.Prompts = stats.Prompts
		}
	}
	return s
}
