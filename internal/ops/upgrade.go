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

	// Clean up stale kiro processes to free memory
	CleanStaleProcesses()

	// Install yax if not present
	if !YaxInstalled() {
		if err := YaxInstall(); err != nil {
			fmt.Printf("  ⚠ yax: %v\n", err)
		}
	}

	// Install prompt-scorer if not present
	if FindScorerBin() == "" {
		if err := ScorerInstall(); err != nil {
			fmt.Printf("  ⚠ prompt-scorer: %v\n", err)
		}
	}

	return nil
}

// RestartTray kills any running "koda tray" process and restarts it with the new binary.
func RestartTray(exePath string) {
	KillTray()
	LaunchTray(exePath)
}

// LaunchTray starts the tray process in the background.
func LaunchTray(exePath string) {
	cmd := exec.Command(exePath, "tray")
	cmd.Stdout = nil
	cmd.Stderr = nil
	cmd.Stdin = nil
	if cmd.Start() == nil {
		cmd.Process.Release()
	}
}

// IsTrayRunning checks if a "koda tray" process is alive.
func IsTrayRunning() bool {
	if runtime.GOOS == "windows" {
		out, err := exec.Command("wmic", "process", "where",
			"commandline like '%koda%tray%' and not commandline like '%wmic%'",
			"get", "processid").Output()
		if err != nil {
			return false
		}
		for _, line := range strings.Split(string(out), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(line))
			if err == nil && pid != os.Getpid() {
				return true
			}
		}
		return false
	}
	out, err := exec.Command("pgrep", "-f", "koda tray").Output()
	if err != nil {
		return false
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		pid, err := strconv.Atoi(strings.TrimSpace(line))
		if err == nil && pid != os.Getpid() {
			return true
		}
	}
	return false
}

// KillTray stops any running "koda tray" process.
func KillTray() {
	if runtime.GOOS == "windows" {
		// Windows: taskkill /IM koda.exe /F filters by window title not possible,
		// so we use wmic to find "koda tray" command line
		out, err := exec.Command("wmic", "process", "where",
			"commandline like '%koda%tray%' and not commandline like '%wmic%'",
			"get", "processid").Output()
		if err != nil {
			return
		}
		for _, line := range strings.Split(string(out), "\n") {
			line = strings.TrimSpace(line)
			pid, err := strconv.Atoi(line)
			if err != nil || pid == os.Getpid() {
				continue
			}
			exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F").Run()
			fmt.Printf("  \u2713 Stopped old tray (pid %d)\n", pid)
		}
	} else {
		// macOS / Linux: pgrep + SIGTERM
		out, err := exec.Command("pgrep", "-f", "koda tray").Output()
		if err != nil {
			return
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			pid, err := strconv.Atoi(strings.TrimSpace(line))
			if err != nil || pid == os.Getpid() {
				continue
			}
			if p, err := os.FindProcess(pid); err == nil {
				p.Signal(syscall.SIGTERM)
				fmt.Printf("  \u2713 Stopped old tray (pid %d)\n", pid)
			}
		}
	}
}
