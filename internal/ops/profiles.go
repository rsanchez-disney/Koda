package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	// Index global profiles by ID
	globalByID := map[string]model.Profile{}
	var globalOrder []string
	for _, d := range dirs {
		id := filepath.Base(d)
		agents, _ := discoverAgents(d)
		installed := isProfileInstalled(id, d, targetDir)
		globalByID[id] = model.Profile{
			ID:         id,
			SourceDir:  d,
			Agents:     agents,
			AgentCount: len(agents),
			Installed:  installed,
		}
		globalOrder = append(globalOrder, id)
	}

	// Workspace profiles merge into globals with same ID (specialization)
	wsGlob := filepath.Join(steerRoot, config.WorkspacesDir, "*", "profiles", "*")
	wsDirs, _ := filepath.Glob(wsGlob)
	var extraProfiles []model.Profile
	for _, d := range wsDirs {
		info, err := os.Stat(d)
		if err != nil || !info.IsDir() {
			continue
		}
		id := filepath.Base(d)
		wsName := filepath.Base(filepath.Dir(filepath.Dir(d)))
		wsAgents, _ := discoverAgents(d)

		if global, ok := globalByID[id]; ok {
			// Merge: global agents + workspace agents (workspace wins on conflict)
			merged := make(map[string]model.Agent)
			for _, a := range global.Agents {
				merged[a.Name] = a
			}
			for _, a := range wsAgents {
				merged[a.Name] = a
			}
			var agents []model.Agent
			for _, a := range merged {
				agents = append(agents, a)
			}
			installed := isProfileInstalled(id, d, targetDir) || global.Installed
			globalByID[id] = model.Profile{
				ID:            id,
				SourceDir:     d,
				Agents:        agents,
				AgentCount:    len(agents),
				Installed:     installed,
				WorkspaceName: wsName,
			}
		} else {
			// Workspace-only profile (no global parent)
			installed := isProfileInstalled(id, d, targetDir)
			extraProfiles = append(extraProfiles, model.Profile{
				ID:            id,
				SourceDir:     d,
				Agents:        wsAgents,
				AgentCount:    len(wsAgents),
				Installed:     installed,
				WorkspaceName: wsName,
			})
		}
	}

	var profiles []model.Profile
	for _, id := range globalOrder {
		profiles = append(profiles, globalByID[id])
	}
	profiles = append(profiles, extraProfiles...)
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
	// Use forward slashes in JSON — backslashes are escape characters
	jsonHome := strings.ReplaceAll(home, "\\", "/")
	count := 0
	entries, _ := os.ReadDir(agentsSrc)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), "._") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(agentsSrc, e.Name()))
		if err != nil {
			continue
		}
		expanded := strings.ReplaceAll(string(data), "$HOME", jsonHome)
		if runtime.GOOS == "windows" {
			expanded = strings.ReplaceAll(expanded, ".sh\"", ".ps1\"")
		}
		os.WriteFile(filepath.Join(agentsDst, e.Name()), []byte(expanded), 0644)
		count++
	}

	// Copy supporting directories
	for _, sub := range []string{config.PromptsDir, config.ContextDir, config.PowersDir, config.SkillsDir, config.SteeringDir} {
		copyDirContents(filepath.Join(srcDir, sub), filepath.Join(targetDir, sub))
	}

	return count, nil
}

// InstallProfileFrom installs a profile from an arbitrary source directory (e.g. workspace profile).
func InstallProfileFrom(srcDir, targetDir string) (int, error) {
	if _, err := os.Stat(srcDir); err != nil {
		return 0, fmt.Errorf("profile source not found: %s", srcDir)
	}

	agentsSrc := filepath.Join(srcDir, config.AgentsDir)
	agentsDst := filepath.Join(targetDir, config.AgentsDir)
	os.MkdirAll(agentsDst, 0755)

	home, _ := os.UserHomeDir()
	jsonHome := strings.ReplaceAll(home, "\\", "/")
	count := 0
	entries, _ := os.ReadDir(agentsSrc)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), "._") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(agentsSrc, e.Name()))
		if err != nil {
			continue
		}
		expanded := strings.ReplaceAll(string(data), "$HOME", jsonHome)
		if runtime.GOOS == "windows" {
			expanded = strings.ReplaceAll(expanded, ".sh\"", ".ps1\"")
		}
		os.WriteFile(filepath.Join(agentsDst, e.Name()), []byte(expanded), 0644)
		count++
	}

	for _, sub := range []string{config.PromptsDir, config.ContextDir, config.RulesDir, config.PowersDir, config.SkillsDir, config.SteeringDir} {
		copyDirContents(filepath.Join(srcDir, sub), filepath.Join(targetDir, sub))
	}

	return count, nil
}

// ResolveProfileSource returns the workspace-specific profile directory if the active
// workspace overrides profileID, otherwise returns the global profile directory.
// The second return value is the workspace name if an override was found, empty otherwise.
func ResolveProfileSource(steerRoot, profileID string) (string, string) {
	s := config.ReadSteerSettings()
	if s.ActiveWorkspace != "" {
		wsDir := filepath.Join(findWorkspaceDir(steerRoot, s.ActiveWorkspace), "profiles", profileID)
		if _, err := os.Stat(wsDir); err == nil {
			return wsDir, s.ActiveWorkspace
		}
	}
	return filepath.Join(steerRoot, config.ProfilePrefix+profileID), ""
}

// RemoveProfile removes a profile's agents, prompts, and all profile-owned files
// (powers, context, rules, skills, steering) from targetDir.
// Files installed by InstallShared (from steer-runtime/shared/) are not touched.
// NOTE: Agent resolution uses the currently active workspace at the time of removal.
// If the active workspace changed since installation (or is no longer active), the resolved
// agent list may differ from what was installed, leaving orphaned agent files in targetDir.
// The recommended path for profile removal is the TUI (koda → p), which always operates
// with the correct workspace context.
// TODO: Persist the install source (global vs workspace) in the profiles manifest so that
// RemoveProfile can always resolve the correct agent list regardless of active workspace state.
// This requires a manifest schema change and is considered a major breaking change.
func RemoveProfile(steerRoot, profileID, targetDir string) (int, error) {
	srcDir, wsName := ResolveProfileSource(steerRoot, profileID)
	if wsName != "" {
		fmt.Printf("  ℹ Resolving %s from workspace '%s'\n", profileID, wsName)
	}
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

	// Remove all files that were installed by this profile from supporting dirs.
	// Files present in steer-runtime/shared/ or steer-runtime/common/ are preserved.
	for _, sub := range []string{config.ContextDir, config.RulesDir, config.PowersDir, config.SkillsDir, config.SteeringDir} {
		entries, err := os.ReadDir(filepath.Join(srcDir, sub))
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			// Preserve files that come from shared/ or common/
			if _, err := os.Stat(filepath.Join(steerRoot, "shared", sub, e.Name())); err == nil {
				continue
			}
			if _, err := os.Stat(filepath.Join(steerRoot, "common", sub, e.Name())); err == nil {
				continue
			}
			os.Remove(filepath.Join(targetDir, sub, e.Name()))
		}
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
	// Clean macOS resource fork files from previous installs
	filepath.Walk(targetDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && strings.HasPrefix(info.Name(), "._") {
			os.Remove(path)
		}
		return nil
	})

	// Hooks
	copyDirContents(filepath.Join(steerRoot, "shared", config.HooksDir), filepath.Join(targetDir, config.HooksDir))
	chmodExec(filepath.Join(targetDir, config.HooksDir))

	// Shared context
	copyDirContents(filepath.Join(steerRoot, "shared", config.ContextDir), filepath.Join(targetDir, config.ContextDir))

	// MCP server bundles
	mcpSrc := filepath.Join(steerRoot, "shared", config.ToolsDir, "mcp-servers")
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

// removeGlobalOrphans removes files installed by the global profile that are NOT
// present in the workspace override, preventing stale files from leaking through.
func removeGlobalOrphans(steerRoot, profileID, wsDir, targetDir string) {
	globalSrc := filepath.Join(steerRoot, config.ProfilePrefix+profileID)

	// Agents (keyed by name, stored as <name>.json)
	if globalNames, err := agentNames(globalSrc); err == nil {
		wsAgents, _ := agentNames(wsDir)
		wsSet := make(map[string]bool, len(wsAgents))
		for _, n := range wsAgents {
			wsSet[n] = true
		}
		for _, n := range globalNames {
			if !wsSet[n] {
				os.Remove(filepath.Join(targetDir, config.AgentsDir, n+".json"))
				os.Remove(filepath.Join(targetDir, config.PromptsDir, n+".md"))
			}
		}
	}

	// Support dirs: remove global files absent from workspace override
	for _, sub := range []string{config.ContextDir, config.RulesDir, config.PowersDir, config.SkillsDir, config.SteeringDir} {
		globalEntries, err := os.ReadDir(filepath.Join(globalSrc, sub))
		if err != nil {
			continue
		}
		for _, e := range globalEntries {
			if e.IsDir() {
				continue
			}
			// Keep if workspace override also ships this file
			if _, err := os.Stat(filepath.Join(wsDir, sub, e.Name())); err == nil {
				continue
			}
			os.Remove(filepath.Join(targetDir, sub, e.Name()))
		}
	}
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
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), "._") {
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
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), "._") {
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
	for _, name := range names {
		if _, err := os.Stat(filepath.Join(targetDir, config.AgentsDir, name+".json")); err != nil {
			return false
		}
	}
	return true
}

func copyDirContents(src, dst string) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	os.MkdirAll(dst, 0755)
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), "._") {
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

// InstallBanks copies service and channel bank markdown files into the target context directory.
// Each service's .md files are merged into a single svc-{name}.md file.
// Each channel's .md files are merged into a single ch-{name}.md file.
func InstallBanks(steerRoot, targetDir string, services, channels []string) (int, int) {
	ctxDir := filepath.Join(targetDir, config.ContextDir)
	os.MkdirAll(ctxDir, 0755)

	svcCount := 0
	for _, svc := range services {
		srcDir := filepath.Join(steerRoot, "shared", "services", svc)
		if merged := mergeBank(srcDir, filepath.Join(ctxDir, "svc-"+svc+".md")); merged {
			svcCount++
		}
	}

	chCount := 0
	for _, ch := range channels {
		srcDir := filepath.Join(steerRoot, "channels", ch)
		if merged := mergeBank(srcDir, filepath.Join(ctxDir, "ch-"+ch+".md")); merged {
			chCount++
		}
	}

	return svcCount, chCount
}

// mergeBank reads all .md files from srcDir and writes them concatenated into dstFile.
func mergeBank(srcDir, dstFile string) bool {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return false
	}
	var buf strings.Builder
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(srcDir, e.Name()))
		if err != nil {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString("\n\n---\n\n")
		}
		buf.Write(data)
	}
	if buf.Len() == 0 {
		return false
	}
	os.WriteFile(dstFile, []byte(buf.String()), 0644)
	return true
}