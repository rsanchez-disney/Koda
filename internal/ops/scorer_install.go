package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// FindScorerBin returns the prompt-scorer binary path (PATH or known location).
func FindScorerBin() string {
	if p, err := exec.LookPath("prompt-scorer"); err == nil {
		return p
	}
	p := scorerKnownPath()
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

func scorerKnownPath() string {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".local", "bin", "prompt-scorer")
	if runtime.GOOS == "windows" {
		p += ".exe"
	}
	return p
}

// ScorerInstall installs prompt-scorer from Koda releases on github.com.
func ScorerInstall() error {
	home, _ := os.UserHomeDir()
	installDir := filepath.Join(home, ".local", "bin")
	os.MkdirAll(installDir, 0755)

	asset := fmt.Sprintf("prompt-scorer-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	dest := filepath.Join(installDir, "prompt-scorer")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}

	fmt.Println("  📥 Installing prompt-scorer...")

	url := fmt.Sprintf("https://github.com/rsanchez-disney/Koda/releases/latest/download/%s", asset)
	curlBin := "curl"
	if runtime.GOOS == "windows" {
		curlBin = "curl.exe"
	}
	if out, err := exec.Command(curlBin, "-fsSL", "-o", dest, url).CombinedOutput(); err == nil {
		os.Chmod(dest, 0755)
		fmt.Println("  ✅ prompt-scorer installed")
		return nil
	} else {
		fmt.Fprintf(os.Stderr, "  curl: %s\n", strings.TrimSpace(string(out)))
	}

	// Fallback: gh release download
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
			fmt.Println("  ✅ prompt-scorer installed")
			return nil
		}
	}

	fmt.Println("  ⚠ prompt-scorer: download failed (skipping)")
	return nil
}
