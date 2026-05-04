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
	AutoUpgrade     bool   `json:"autoUpgrade"`
	ActiveWorkspace     string `json:"activeWorkspace"`
	KiroSettingsApplied bool   `json:"kiroSettingsApplied,omitempty"`
	TrustTools          string `json:"trustTools,omitempty"` // "all", "none", or "" (prompt)
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
	if s.Source == "" {
		s.Source = "tarball"
	}

	// Reconcile: if workspace.json exists with a different name, trust it
	// over kite.json — workspace.json is the ground truth of what's installed.
	s.ActiveWorkspace = reconcileWorkspace(s.ActiveWorkspace)

	return s
}

// reconcileWorkspace checks the installed workspace snapshot and returns
// the actual active workspace name. This prevents drift when kite.json
// has a stale value (e.g., after upgrade or manual edit).
func reconcileWorkspace(fromSettings string) string {
	snapshotPath := filepath.Join(KiroRoot(), SettingsDir, "workspace.json")
	data, err := os.ReadFile(snapshotPath)
	if err != nil {
		return fromSettings // no snapshot — trust settings
	}
	var snap struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(data, &snap) != nil || snap.Name == "" {
		return fromSettings
	}
	if fromSettings != "" && fromSettings != snap.Name {
		// Drift detected — workspace.json wins, fix kite.json silently
		return snap.Name
	}
	if fromSettings == "" && snap.Name != "" {
		return snap.Name
	}
	return fromSettings
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
