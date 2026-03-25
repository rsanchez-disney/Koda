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

// ExpandAliases expands profile aliases (e.g., "dev" → dev-core, dev-web, dev-mobile)
// and deduplicates the result.
func ExpandAliases(names []string) []string {
	var expanded []string
	seen := map[string]bool{}
	for _, n := range names {
		if aliases, ok := model.Aliases[n]; ok {
			for _, a := range aliases {
				if !seen[a] {
					seen[a] = true
					expanded = append(expanded, a)
				}
			}
		} else if !seen[n] {
			seen[n] = true
			expanded = append(expanded, n)
		}
	}
	return expanded
}

// ListProfiles discovers all available profiles under steerRoot.
func ListProfiles(steerRoot, targetDir string) ([]model.Profile, error) {
	dirs, err := config.ProfileDirs(steerRoot)
	if err != nil {
		return nil, err
	}
	var profiles []model.Profile
	for _, d := range dirs {
		id := strings.TrimPrefix(filepath.Base(d), config.ProfilePrefix)
		agents, _ := discoverAgents(d)
		installed := isProfileInstalled(id, d, targetDir)
		profiles = append(profiles, model.Profile{
			ID:         id,
			SourceDir:  d,
			Agents:     agents,
			AgentCount: len(agents),
			Installed:  installed,
		})
	}
	return profiles, nil
}

// InstallProfile copies a profile's agents, prompts, and supporting dirs to targetDir.
func InstallProfile(steerRoot, profileID, targetDir string) (int, error) {
	srcDir := filepath.Join(steerRoot, config.ProfilePrefix+profileID)
	if _, err := os.Stat(srcDir); err != nil {
		return 0, fmt.Errorf("profile not found: %s", profileID)
	}

	// Copy agents with $HOME expansion
	agentsSrc := filepath.Join(srcDir, config.AgentsDir)
	agentsDst := filepath.Join(targetDir, config.AgentsDir)
	os.MkdirAll(agentsDst, 0755)

	home, _ := os.UserHomeDir()
	count := 0
	entries, _ := os.ReadDir(agentsSrc)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(agentsSrc, e.Name()))
		if err != nil {
			continue
		}
		expanded := strings.ReplaceAll(string(data), "$HOME", home)
		os.WriteFile(filepath.Join(agentsDst, e.Name()), []byte(expanded), 0644)
		count++
	}

	// Copy supporting directories
	for _, sub := range []string{config.PromptsDir, config.ContextDir, "powers", "skills", "steering"} {
		copyDirContents(filepath.Join(srcDir, sub), filepath.Join(targetDir, sub))
	}

	return count, nil
}

// RemoveProfile removes a profile's agents and prompts from targetDir.
func RemoveProfile(steerRoot, profileID, targetDir string) (int, error) {
	srcDir := filepath.Join(steerRoot, config.ProfilePrefix+profileID)
	agentNames, err := agentNames(srcDir)
	if err != nil {
		return 0, fmt.Errorf("no agents found for profile: %s", profileID)
	}

	removed := 0
	for _, name := range agentNames {
		agentPath := filepath.Join(targetDir, config.AgentsDir, name+".json")
		if err := os.Remove(agentPath); err == nil {
			removed++
		}
		os.Remove(filepath.Join(targetDir, config.PromptsDir, name+".md"))
	}
	return removed, nil
}

// DetectInstalled returns profile IDs that are currently installed in targetDir.
func DetectInstalled(steerRoot, targetDir string) []string {
	profiles, _ := ListProfiles(steerRoot, targetDir)
	var installed []string
	for _, p := range profiles {
		if p.Installed {
			installed = append(installed, p.ID)
		}
	}
	return installed
}

// InstallShared copies hooks, MCP bundles, and shared context to targetDir.
func InstallShared(steerRoot, targetDir string) error {
	// Hooks
	copyDirContents(filepath.Join(steerRoot, ".kiro", config.HooksDir), filepath.Join(targetDir, config.HooksDir))
	chmodExec(filepath.Join(targetDir, config.HooksDir))

	// Shared context
	copyDirContents(filepath.Join(steerRoot, ".kiro", config.ContextDir), filepath.Join(targetDir, config.ContextDir))

	// MCP server bundles
	mcpSrc := filepath.Join(steerRoot, ".kiro", config.ToolsDir, "mcp-servers")
	if entries, err := os.ReadDir(mcpSrc); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			bundle := filepath.Join(mcpSrc, e.Name(), "dist", "index.cjs")
			if _, err := os.Stat(bundle); err == nil {
				dst := filepath.Join(targetDir, config.ToolsDir, "mcp-servers", e.Name(), "dist")
				os.MkdirAll(dst, 0755)
				copyFile(bundle, filepath.Join(dst, "index.cjs"))
			}
		}
	}
	return nil
}

// --- helpers ---

func discoverAgents(profileDir string) ([]model.Agent, error) {
	agentsDir := filepath.Join(profileDir, config.AgentsDir)
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, err
	}
	var agents []model.Agent
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(agentsDir, e.Name()))
		if err != nil {
			continue
		}
		var a model.Agent
		if json.Unmarshal(data, &a) == nil {
			agents = append(agents, a)
		}
	}
	return agents, nil
}

func agentNames(profileDir string) ([]string, error) {
	agentsDir := filepath.Join(profileDir, config.AgentsDir)
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names, nil
}

func isProfileInstalled(id, sourceDir, targetDir string) bool {
	names, err := agentNames(sourceDir)
	if err != nil || len(names) == 0 {
		return false
	}
	_, err = os.Stat(filepath.Join(targetDir, config.AgentsDir, names[0]+".json"))
	return err == nil
}

func copyDirContents(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	os.MkdirAll(dst, 0755)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
	}
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func chmodExec(dir string) {
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if !e.IsDir() {
			os.Chmod(filepath.Join(dir, e.Name()), 0755)
		}
	}
}
