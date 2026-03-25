package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// HealthReport summarizes the installation state.
type HealthReport struct {
	AgentsDir     bool     `json:"agents_dir"`
	TotalAgents   int      `json:"total_agents"`
	Profiles      []string `json:"installed_profiles"`
	TokensSet     []string `json:"tokens_set"`
	TokensMissing []string `json:"tokens_missing"`
	InvalidAgents []string `json:"invalid_agents,omitempty"`
}

// CheckInstallation builds a health report for the given target directory.
func CheckInstallation(steerRoot, targetDir string) HealthReport {
	r := HealthReport{}

	agentsDir := filepath.Join(targetDir, config.AgentsDir)
	if info, err := os.Stat(agentsDir); err == nil && info.IsDir() {
		r.AgentsDir = true
	}

	// Count agents and validate JSON
	entries, _ := os.ReadDir(agentsDir)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		r.TotalAgents++
		data, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
		if err != nil {
			r.InvalidAgents = append(r.InvalidAgents, e.Name())
			continue
		}
		var a model.Agent
		if json.Unmarshal(data, &a) != nil || a.Name == "" {
			r.InvalidAgents = append(r.InvalidAgents, e.Name())
		}
	}

	// Detect installed profiles
	r.Profiles = DetectInstalled(steerRoot, targetDir)

	// Check tokens
	tokens := ReadTokens()
	for _, t := range model.KnownTokens {
		if v, ok := tokens[t.Key]; ok && v != "" {
			r.TokensSet = append(r.TokensSet, t.Key)
		} else {
			r.TokensMissing = append(r.TokensMissing, t.Key)
		}
	}

	return r
}

// PrintReport prints a human-readable health report.
func PrintReport(r HealthReport) {
	fmt.Println("🔍 Installation status:")
	fmt.Println()

	if r.AgentsDir {
		fmt.Println("  ✓ Agents directory exists")
	} else {
		fmt.Println("  ✗ No agents directory")
	}

	fmt.Printf("  ✓ Total agents: %d\n", r.TotalAgents)

	if len(r.Profiles) > 0 {
		fmt.Printf("  ✓ Installed profiles: %s\n", strings.Join(r.Profiles, ", "))
	} else {
		fmt.Println("  ⚠ No profiles detected")
	}

	if len(r.InvalidAgents) > 0 {
		fmt.Printf("  ✗ Invalid agents: %s\n", strings.Join(r.InvalidAgents, ", "))
	} else if r.TotalAgents > 0 {
		fmt.Println("  ✓ All agent configs valid")
	}

	fmt.Println()
	fmt.Println("  Tokens:")
	for _, k := range r.TokensSet {
		fmt.Printf("    ✓ %s\n", k)
	}
	for _, k := range r.TokensMissing {
		fmt.Printf("    ✗ %s\n", k)
	}
}
