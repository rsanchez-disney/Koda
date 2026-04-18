package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.disney.com/SANCR225/koda/internal/config"
)

var (
	kiroCLIPath string
	kiroCLIOnce sync.Once
)

// FindKiroCLI returns the absolute path to kiro-cli, checking PATH first
// then common Windows install locations. Result is cached.
func FindKiroCLI() string {
	kiroCLIOnce.Do(func() {
		if p, err := exec.LookPath("kiro-cli"); err == nil {
			kiroCLIPath = p
			return
		}
		if runtime.GOOS == "windows" {
			for _, base := range []string{
				filepath.Join(os.Getenv("LOCALAPPDATA"), "kiro-cli"),
				filepath.Join(os.Getenv("PROGRAMFILES"), "Kiro CLI"),
				filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "kiro-cli"),
			} {
				candidate := filepath.Join(base, "kiro-cli.exe")
				if _, err := os.Stat(candidate); err == nil {
					kiroCLIPath = candidate
					return
				}
			}
		}
		kiroCLIPath = "kiro-cli"
	})
	return kiroCLIPath
}

// DiagnoseKiroCLI checks kiro-cli health on Windows and returns a diagnostic message.
// Returns (ok, detail, fix).
func DiagnoseKiroCLI() (bool, string, string) {
	kiroPath := FindKiroCLI()

	// Try running --version
	cmd := exec.Command(kiroPath, "--version")
	out, err := cmd.CombinedOutput()
	version := strings.TrimSpace(string(out))

	if err == nil && version != "" {
		return true, version + " (" + kiroPath + ")", ""
	}

	// Binary found but silent/crash
	if _, statErr := os.Stat(kiroPath); statErr == nil {
		if runtime.GOOS == "windows" {
			// Check exit code for VC++ Redist missing (0xC0000135 = -1073741515)
			if exitErr, ok := err.(*exec.ExitError); ok {
				code := exitErr.ExitCode()
				if code == -1073741515 {
					return false,
						fmt.Sprintf("found at %s but missing Visual C++ Redistributable (exit 0xC0000135)", kiroPath),
						"Download from https://aka.ms/vs/17/release/vc_redist.x64.exe"
				}
				return false,
					fmt.Sprintf("found at %s but crashed (exit %d)", kiroPath, code),
					"Try reinstalling: irm https://kiro.dev/install.ps1 | iex"
			}
			if version == "" {
				return false,
					fmt.Sprintf("found at %s but --version returned empty", kiroPath),
					"Try reinstalling: irm https://kiro.dev/install.ps1 | iex"
			}
		}
		return false, fmt.Sprintf("found at %s but failed: %v", kiroPath, err), ""
	}

	return false, "not found in PATH or common install locations", "Install from https://kiro.dev"
}

// KiroSetting represents a kiro-cli setting Koda can manage.
type KiroSetting struct {
	Key         string
	Label       string
	Type        string // "bool" or "agent"
	Recommended bool
}

// ManagedKiroSettings defines the settings Koda manages.
var ManagedKiroSettings = []KiroSetting{
	{Key: "chat.defaultAgent", Label: "Default Agent", Type: "agent"},
	{Key: "chat.enableNotifications", Label: "Enable Notifications", Type: "bool", Recommended: true},
	{Key: "chat.enableThinking", Label: "Enable Thinking", Type: "bool", Recommended: true},
	{Key: "chat.enableTodoList", Label: "Enable Todo List", Type: "bool", Recommended: true},
	{Key: "chat.enableKnowledge", Label: "Enable Knowledge", Type: "bool", Recommended: true},
	{Key: "autocomplete.developerMode", Label: "Developer Mode", Type: "bool", Recommended: true},
	{Key: "autocomplete.immediatelyExecuteAfterSpace", Label: "Execute After Space", Type: "bool", Recommended: true},
}

// SetKiroSetting sets a kiro-cli setting.
func SetKiroSetting(key, value string) error {
	return exec.Command(FindKiroCLI(), "settings", key, value).Run()
}

// ReadKiroSettings reads current values from kiro-cli settings list.
func ReadKiroSettings() map[string]string {
	out, err := exec.Command(FindKiroCLI(), "settings", "list").Output()
	if err != nil {
		return map[string]string{}
	}
	result := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		parts := strings.SplitN(line, " = ", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			// Strip scope suffixes: (global), (workspace), (default)
			for _, suffix := range []string{" (global)", " (workspace)", " (default)"} {
				val = strings.TrimSuffix(val, suffix)
			}
			val = strings.Trim(val, "\"")
			result[key] = val
		}
	}
	return result
}

// SuggestDefaultAgent returns the best agent for chat.
// Priority: workspace default > auto-detect orchestrator from installed agents.
func SuggestDefaultAgent(steerRoot, targetDir string) string {
	// 1. Check workspace default
	s := config.ReadSteerSettings()
	if s.ActiveWorkspace != "" {
		ws, err := GetWorkspace(steerRoot, s.ActiveWorkspace)
		if err == nil && ws.DefaultAgent != "" {
			return ws.DefaultAgent
		}
	}
	agentsDir := filepath.Join(targetDir, config.AgentsDir)
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return ""
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	for _, n := range names {
		if n == "orchestrator" {
			return "orchestrator"
		}
	}
	for _, n := range names {
		if strings.HasSuffix(n, "_orchestrator_agent") {
			return n
		}
	}
	return ""
}

// ConfigureKiroSettings applies recommended settings + default agent.
func ConfigureKiroSettings(steerRoot, targetDir string) {
	current := ReadKiroSettings()
	if current["chat.defaultAgent"] == "" {
		if agent := SuggestDefaultAgent(steerRoot, targetDir); agent != "" {
			SetKiroSetting("chat.defaultAgent", agent)
			fmt.Printf("  ✓ kiro: chat.defaultAgent = %s\n", agent)
		}
	}
	for _, s := range ManagedKiroSettings {
		if s.Recommended && s.Type == "bool" {
			SetKiroSetting(s.Key, "true")
		}
	}
	fmt.Println("  ✓ kiro: recommended settings enabled")
}
