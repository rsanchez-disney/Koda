package ops

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.disney.com/SANCR225/koda/internal/config"
)

// Settings represents shared preferences between Kite and Koda.
type Settings struct {
	SlackBotToken string        `json:"slackBotToken,omitempty"`
	SlackAppToken string        `json:"slackAppToken,omitempty"`
	ActiveProfile string        `json:"activeProfile,omitempty"`
	LastAgent     string        `json:"lastAgent,omitempty"`
	SteerRuntime  *SteerConfig  `json:"steerRuntime,omitempty"`
}

// SteerConfig holds steer-runtime preferences.
type SteerConfig struct {
	ActiveWorkspace string `json:"activeWorkspace,omitempty"`
	AutoSync        bool   `json:"autoSync,omitempty"`
	Branch          string `json:"branch,omitempty"`
	LastSync        string `json:"lastSync,omitempty"`
	Repo            string `json:"repo,omitempty"`
}

func settingsPath() string {
	return filepath.Join(config.KiroRoot(), config.SettingsDir, "kite.json")
}

// LoadSettings reads ~/.kiro/settings/kite.json (shared with Kite).
func LoadSettings() Settings {
	var s Settings
	data, err := os.ReadFile(settingsPath())
	if err != nil {
		return s
	}
	json.Unmarshal(data, &s)
	return s
}

// SaveSettings writes ~/.kiro/settings/kite.json.
func SaveSettings(s Settings) error {
	path := settingsPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	out, err := json.MarshalIndent(s, "", "    ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(out, '\n'), 0644)
}

// UpdateLastAgent saves the last used agent.
func UpdateLastAgent(agent string) {
	s := LoadSettings()
	s.LastAgent = agent
	SaveSettings(s)
}

// UpdateActiveProfile saves the active profile.
func UpdateActiveProfile(profile string) {
	s := LoadSettings()
	s.ActiveProfile = profile
	SaveSettings(s)
}
