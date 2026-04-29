package config

import (
	_ "embed"
	"encoding/json"
)

//go:embed features.json
var featuresData []byte

// featuresConfig holds the parsed features.json structure.
var featuresConfig struct {
	TUI map[string]bool `json:"tui"`
}

func init() {
	json.Unmarshal(featuresData, &featuresConfig)
	if featuresConfig.TUI == nil {
		featuresConfig.TUI = map[string]bool{}
	}
}

// IsTUIEnabled returns true if a TUI command/screen is enabled in features.json.
func IsTUIEnabled(name string) bool {
	enabled, ok := featuresConfig.TUI[name]
	return !ok || enabled // default to true if not listed
}

// TUIFeatures returns a copy of all TUI feature flags.
func TUIFeatures() map[string]bool {
	m := make(map[string]bool, len(featuresConfig.TUI))
	for k, v := range featuresConfig.TUI {
		m[k] = v
	}
	return m
}
