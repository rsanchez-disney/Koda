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

// YaxInstalled checks if yax binary is in PATH or known install location.
func YaxInstalled() bool {
	if _, err := exec.LookPath("yax"); err == nil {
		return true
	}
	_, err := os.Stat(yaxKnownPath())
	return err == nil
}

// yaxKnownPath returns the expected yax binary path.
func yaxKnownPath() string {
	home, _ := os.UserHomeDir()
	p := filepath.Join(home, ".local", "bin", "yax")
	if runtime.GOOS == "windows" {
		p += ".exe"
	}
	return p
}

// findYax returns the yax binary path (PATH or known location).
func findYax() string {
	if p, err := exec.LookPath("yax"); err == nil {
		return p
	}
	p := yaxKnownPath()
	if _, err := os.Stat(p); err == nil {
		return p
	}
	return ""
}

// YaxInstall installs yax from Koda releases on github.com.
func YaxInstall() error {
	home, _ := os.UserHomeDir()
	installDir := filepath.Join(home, ".local", "bin")
	os.MkdirAll(installDir, 0755)

	asset := fmt.Sprintf("yax-%s-%s", runtime.GOOS, runtime.GOARCH)
	if runtime.GOOS == "windows" {
		asset += ".exe"
	}
	dest := filepath.Join(installDir, "yax")
	if runtime.GOOS == "windows" {
		dest += ".exe"
	}

	fmt.Println("  📥 Installing yax...")

	// Primary: curl from github.com (public, no auth needed)
	url := fmt.Sprintf("https://github.com/rsanchez-disney/Koda/releases/latest/download/%s", asset)
	curlBin := "curl"
	if runtime.GOOS == "windows" {
		curlBin = "curl.exe"
	}
	if out, err := exec.Command(curlBin, "-fsSL", "-o", dest, url).CombinedOutput(); err == nil {
		os.Chmod(dest, 0755)
		fmt.Println("  ✅ yax installed")
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
			fmt.Println("  ✅ yax installed")
			return nil
		}
	}

	fmt.Println("  ⚠ yax: download failed (skipping)")
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
	yaxBin := findYax()
	if yaxBin == "" {
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

// YaxProject holds project name and count from yax projects.
type YaxProject struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// YaxObservation holds a single observation from yax.
type YaxObservation struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	Project   string `json:"project"`
	CreatedAt string `json:"created_at"`
}

// YaxProjects returns project list with counts.
func YaxProjects() []YaxProject {
	yaxBin, err := exec.LookPath("yax")
	if err != nil {
		return nil
	}
	out, err := exec.Command(yaxBin, "stats").Output()
	if err != nil {
		return nil
	}
	var stats struct {
		Projects []string `json:"projects"`
	}
	json.Unmarshal(out, &stats)
	// stats only has project names, get counts via context per project
	var projects []YaxProject
	for _, p := range stats.Projects {
		projects = append(projects, YaxProject{Name: p})
	}
	return projects
}

// YaxRecent returns recent observations, optionally filtered by project.
func YaxRecent(project string, limit int) []YaxObservation {
	yaxBin, err := exec.LookPath("yax")
	if err != nil {
		return nil
	}
	args := []string{"context"}
	if project != "" {
		args = append(args, project)
	}
	out, err := exec.Command(yaxBin, args...).Output()
	if err != nil || len(out) == 0 {
		return nil
	}
	// yax context outputs text lines, not JSON — parse them
	var obs []YaxObservation
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" || line == "No recent memories." {
			continue
		}
		o := YaxObservation{Title: line}
		obs = append(obs, o)
		if limit > 0 && len(obs) >= limit {
			break
		}
	}
	return obs
}

// YaxSearch runs a search query and returns text results.
func YaxSearch(query string) []string {
	yaxBin, err := exec.LookPath("yax")
	if err != nil {
		return nil
	}
	out, _ := exec.Command(yaxBin, "search", query).Output()
	if len(out) == 0 {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// YaxPrune runs yax prune with given days.
func YaxPrune(days int, hard bool) (string, error) {
	yaxBin, err := exec.LookPath("yax")
	if err != nil {
		return "", fmt.Errorf("yax not installed")
	}
	args := []string{"prune", "--older-than", fmt.Sprint(days)}
	if hard {
		args = append(args, "--hard")
	}
	out, err := exec.Command(yaxBin, args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}
