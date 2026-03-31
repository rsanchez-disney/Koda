package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const (
	DefaultSteerRepo   = "SANCR225/steer-runtime"
	DefaultSteerBranch = "main"
	GHHost             = "github.disney.com"
)

// DefaultSteerRoot returns ~/.kiro/steer-runtime
func DefaultSteerRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kiro", "steer-runtime")
}

// SharedSettingsPath returns ~/.kiro/settings/kite.json (shared with Kite)
func SharedSettingsPath() string {
	return filepath.Join(KiroRoot(), SettingsDir, "kite.json")
}

type SteerSettings struct {
	Repo            string `json:"repo"`
	Branch          string `json:"branch"`
	Source          string `json:"source"` // "tarball" (default) or "git"
	LastSync        string `json:"lastSync"`
	AutoSync        bool   `json:"autoSync"`
	ActiveWorkspace string `json:"activeWorkspace"`
}

func ReadSteerSettings() SteerSettings {
	data, err := os.ReadFile(SharedSettingsPath())
	if err != nil {
		return SteerSettings{Repo: DefaultSteerRepo, Branch: DefaultSteerBranch, AutoSync: true}
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return SteerSettings{Repo: DefaultSteerRepo, Branch: DefaultSteerBranch, AutoSync: true}
	}
	var s SteerSettings
	if sr, ok := raw["steerRuntime"]; ok {
		json.Unmarshal(sr, &s)
	}
	if s.Repo == "" {
		s.Repo = DefaultSteerRepo
	}
	if s.Branch == "" {
		s.Branch = DefaultSteerBranch
	}
	return s
}

func SaveSteerSettings(s SteerSettings) error {
	path := SharedSettingsPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	existing := map[string]interface{}{}
	if data, err := os.ReadFile(path); err == nil {
		json.Unmarshal(data, &existing)
	}
	existing["steerRuntime"] = s
	data, _ := json.MarshalIndent(existing, "", "  ")
	return os.WriteFile(path, data, 0644)
}

func MarkSynced() {
	s := ReadSteerSettings()
	s.LastSync = time.Now().UTC().Format(time.RFC3339)
	SaveSteerSettings(s)
}
