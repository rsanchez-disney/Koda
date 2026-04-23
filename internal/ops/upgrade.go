package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"os/exec"
	"strings"
	"strconv"
	"syscall"
)

const releaseURL = "https://api.github.com/repos/rsanchez-disney/Koda/releases/latest"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// CheckForUpdate returns the latest version tag, or empty if already current.
func CheckForUpdate(currentVersion string) string {
	resp, err := http.Get(releaseURL)
	if err != nil || resp.StatusCode != 200 {
		return ""
	}
	defer resp.Body.Close()
	var rel ghRelease
	if json.NewDecoder(resp.Body).Decode(&rel) != nil {
		return ""
	}
	if rel.TagName == currentVersion || rel.TagName == "v"+currentVersion {
		return ""
	}
	return rel.TagName
}

// Upgrade downloads the latest release binary and replaces the current one.
func Upgrade(currentVersion string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find current binary: %w", err)
	}

	// Fetch latest release
	fmt.Println("\U0001f50d Checking for updates...")
	resp, err := http.Get(releaseURL)
	if err != nil {
		return fmt.Errorf("cannot reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return fmt.Errorf("cannot parse release: %w", err)
	}

	if rel.TagName == currentVersion || rel.TagName == "v"+currentVersion {
		fmt.Printf("\u2705 Already on latest: %s\n", currentVersion)
		return nil
	}

	// Find matching asset
	want := fmt.Sprintf("koda-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, a := range rel.Assets {
		if strings.Contains(a.Name, want) {
			downloadURL = a.URL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, rel.TagName)
	}

	// Download
	fmt.Printf("\U0001f4e5 Downloading %s...\n", rel.TagName)
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer dlResp.Body.Close()

	tmpFile := exePath + ".new"
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return err
	}
	f.Close()
	os.Chmod(tmpFile, 0755)

	// Atomic swap
	oldFile := exePath + ".old"
	os.Rename(exePath, oldFile)
	if err := os.Rename(tmpFile, exePath); err != nil {
		os.Rename(oldFile, exePath) // rollback
		return err
	}
	os.Remove(oldFile)

	fmt.Printf("\u2705 Upgraded: %s \u2192 %s\n", currentVersion, rel.TagName)

	// Restart tray process if running
	RestartTray(exePath)

	// Install yax if not present
	if !YaxInstalled() {
		if err := YaxInstall(); err != nil {
			fmt.Printf("  ⚠ yax: %v\n", err)
		}
	}

	return nil
}

// RestartTray kills any running "koda tray" process and restarts it with the new binary.
func RestartTray(exePath string) {
	// Find running tray process
	out, err := exec.Command("pgrep", "-f", "koda tray").Output()
	if err != nil {
		return // no tray running
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil || pid == os.Getpid() {
			continue
		}
		// Kill old tray
		if p, err := os.FindProcess(pid); err == nil {
			p.Signal(syscall.SIGTERM)
			fmt.Printf("  \u2713 Stopped old tray (pid %d)\n", pid)
		}
	}
	// Start new tray in background
	cmd := exec.Command(exePath, "tray")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if cmd.Start() == nil {
		fmt.Printf("  \u2713 Started new tray (pid %d)\n", cmd.Process.Pid)
		cmd.Process.Release()
	}
}
