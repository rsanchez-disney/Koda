package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"os/exec"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

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
	return exec.Command("kiro-cli", "settings", key, value).Run()
}

// ReadKiroSettings reads current values from kiro-cli settings list.
func ReadKiroSettings() map[string]string {
	out, err := exec.Command("kiro-cli", "settings", "list").Output()
	if err != nil {
		return map[string]string{}
	}
	result := map[string]string{}
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.SplitN(line, " = ", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.TrimSuffix(val, " (global)")
			val = strings.Trim(val, "\"")
			result[key] = val
		}
	}
	return result
}

// SuggestDefaultAgent returns the best agent for chat.
// Priority: kiro-cli chat.defaultAgent > workspace default > auto-detect orchestrator.
func SuggestDefaultAgent(steerRoot, targetDir string) string {
	// 1. Check kiro-cli setting
	if settings := ReadKiroSettings(); settings["chat.defaultAgent"] != "" {
		return settings["chat.defaultAgent"]
	}
	// 2. Check workspace default
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
	if agent := SuggestDefaultAgent(steerRoot, targetDir); agent != "" {
		SetKiroSetting("chat.defaultAgent", agent)
		fmt.Printf("  ✓ kiro: chat.defaultAgent = %s\n", agent)
	}
	for _, s := range ManagedKiroSettings {
		if s.Recommended && s.Type == "bool" {
			SetKiroSetting(s.Key, "true")
		}
	}
	fmt.Println("  ✓ kiro: recommended settings enabled")
}
