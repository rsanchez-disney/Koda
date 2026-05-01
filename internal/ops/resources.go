package ops

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.disney.com/SANCR225/koda/internal/config"
)

// SystemProfile describes the machine's resource tier for delegation budgeting.
type SystemProfile struct {
	TotalRAMGB int    `json:"total_ram_gb"`
	MaxAgents  int    `json:"max_concurrent_agents"`
	Tier       string `json:"tier"` // "light", "standard", "power"
}

// DetectSystemProfile reads total physical memory and returns a resource tier.
func DetectSystemProfile() SystemProfile {
	totalBytes := totalPhysicalMemory()
	gb := int(totalBytes / (1024 * 1024 * 1024))
	if gb < 1 {
		gb = 1
	}

	var tier string
	var maxAgents int
	switch {
	case gb <= 16:
		tier, maxAgents = "light", 2
	case gb <= 32:
		tier, maxAgents = "standard", 4
	default:
		tier, maxAgents = "power", 6
	}

	return SystemProfile{
		TotalRAMGB: gb,
		MaxAgents:  maxAgents,
		Tier:       tier,
	}
}

// WriteSystemProfile writes the system profile to ~/.kiro/settings/system.json.
func WriteSystemProfile() SystemProfile {
	profile := DetectSystemProfile()
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".kiro", config.SettingsDir)
	os.MkdirAll(dir, 0755)
	data, _ := json.MarshalIndent(profile, "", "  ")
	os.WriteFile(filepath.Join(dir, "system.json"), append(data, '\n'), 0644)
	return profile
}

// ReadSystemProfile reads the cached system profile from ~/.kiro/settings/system.json.
func ReadSystemProfile() SystemProfile {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".kiro", config.SettingsDir, "system.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return DetectSystemProfile()
	}
	var profile SystemProfile
	if json.Unmarshal(data, &profile) != nil {
		return DetectSystemProfile()
	}
	return profile
}

// totalPhysicalMemory returns total physical RAM in bytes.
func totalPhysicalMemory() uint64 {
	// runtime doesn't expose total physical memory directly.
	// Use a platform-specific approach.
	return detectRAM()
}
