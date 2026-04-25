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

// Quiet suppresses stdout output from ops functions. Set to true when running inside the TUI.
var Quiet bool

func logf(format string, a ...interface{}) {
	if !Quiet {
		fmt.Printf(format, a...)
	}
}

func logln(a ...interface{}) {
	if !Quiet {
		fmt.Println(a...)
	}
}

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
	// Global profiles
	var profiles []model.Profile
	globalAgents := map[string][]model.Agent{} // id -> agents (for inheritance)
	for _, d := range dirs {
		id := filepath.Base(d)
		agents, _ := discoverAgents(d)
		installed := isProfileInstalled(id, d, targetDir)
		profiles = append(profiles, model.Profile{
			ID:         id,
			SourceDir:  d,
			Agents:     agents,
			AgentCount: len(agents),
			Installed:  installed,
		})
		globalAgents[id] = agents
	}

	// Workspace profiles as separate entries with inherited agent count
	wsGlob := filepath.Join(steerRoot, config.WorkspacesDir, "*", "profiles", "*")
	wsDirs, _ := filepath.Glob(wsGlob)
	for _, d := range wsDirs {
		info, err := os.Stat(d)
		if err != nil || !info.IsDir() {
			continue
		}
		id := filepath.Base(d)
		wsName := filepath.Base(filepath.Dir(filepath.Dir(d)))
		wsAgents, _ := discoverAgents(d)

		// Merge global base + workspace agents for count
		merged := make(map[string]model.Agent)
		for _, a := range globalAgents[id] {
			merged[a.Name] = a
		}
		for _, a := range wsAgents {
			merged[a.Name] = a
		}
		var agents []model.Agent
		for _, a := range merged {
			agents = append(agents, a)
		}

		installed := isProfileInstalled(id, d, targetDir)
		profiles = append(profiles, model.Profile{
			ID:            id,
			SourceDir:     d,
			Agents:        agents,
			AgentCount:    len(agents),
			Installed:     installed,
			WorkspaceName: wsName,
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
		dstPath := filepath.Join(agentsDst, e.Name())
		// If agent already exists in target, merge instead of overwrite
		if existing, err := os.ReadFile(dstPath); err == nil {
			if merged, err := mergeAgentJSON(existing, []byte(expanded)); err == nil {
				expanded = string(merged)
			}
		}
		os.WriteFile(dstPath, []byte(expanded), 0644)
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

// EnrichWelcomeMessages patches installed agent JSONs that have a welcomeMessage
// with workspace context from the resolved workspace snapshot.
func EnrichWelcomeMessages(targetDir string) {
	home, _ := os.UserHomeDir()
	wsData, err := os.ReadFile(filepath.Join(home, ".kiro", "settings", "workspace.json"))
	if err != nil {
		return
	}
	var ws struct {
		Name       string   `json:"name"`
		Team       string   `json:"team"`
		JiraPrefix string   `json:"jira_prefix"`
		Profiles   []string `json:"profiles"`
		Projects   []struct {
			Name string `json:"name"`
			Repo string `json:"repo,omitempty"`
		} `json:"projects"`
		Services []string `json:"services,omitempty"`
		Channels []string `json:"channels,omitempty"`
	}
	if json.Unmarshal(wsData, &ws) != nil || ws.Name == "" {
		return
	}

	// Build workspace suffix
	var b strings.Builder
	b.WriteString("\n\n\U0001f4cb Workspace: " + ws.Name)
	if ws.Team != "" {
		b.WriteString(" (" + ws.Team + ")")
	}
	if ws.JiraPrefix != "" {
		b.WriteString("\n  Jira: " + ws.JiraPrefix + "-*")
	}
	if len(ws.Profiles) > 0 {
		b.WriteString("\n  Profiles: " + strings.Join(ws.Profiles, ", "))
	}
	if len(ws.Projects) > 0 {
		b.WriteString("\n  Projects:")
		for _, p := range ws.Projects {
			b.WriteString("\n    \u2022 " + p.Name)
			if p.Repo != "" {
				b.WriteString(" (" + p.Repo + ")")
			}
		}
	}
	if len(ws.Services) > 0 {
		b.WriteString("\n  Services: " + strings.Join(ws.Services, ", "))
	}
	if len(ws.Channels) > 0 {
		b.WriteString("\n  Channels: " + strings.Join(ws.Channels, ", "))
	}
	suffix := b.String()

	// Patch each agent JSON that has a welcomeMessage
	agentsDir := filepath.Join(targetDir, config.AgentsDir)
	entries, _ := os.ReadDir(agentsDir)
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		path := filepath.Join(agentsDir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var raw map[string]interface{}
		if json.Unmarshal(data, &raw) != nil {
			continue
		}
		msg, ok := raw["welcomeMessage"].(string)
		if !ok || msg == "" {
			continue
		}
		// Strip any previous workspace suffix (re-enrichment safe)
		if idx := strings.Index(msg, "\n\n\U0001f4cb Workspace:"); idx >= 0 {
			msg = msg[:idx]
		}
		raw["welcomeMessage"] = msg + suffix
		out, _ := json.MarshalIndent(raw, "", "  ")
		os.WriteFile(path, out, 0644)
	}
}

// mergeAgentJSON merges a workspace agent JSON on top of a global agent JSON.
// Arrays (tools, allowedTools, resources): append unique.
// Scalars (prompt, description, welcomeMessage): workspace wins if non-empty.
// Objects (hooks, toolsSettings): deep merge (workspace keys override).
func mergeAgentJSON(globalData, wsData []byte) ([]byte, error) {
	var global, ws map[string]json.RawMessage
	if err := json.Unmarshal(globalData, &global); err != nil {
		return wsData, nil // can't parse global, use workspace as-is
	}
	if err := json.Unmarshal(wsData, &ws); err != nil {
		return globalData, nil // can't parse workspace, use global as-is
	}

	// Merge each key from workspace into global
	for key, wsVal := range ws {
		globalVal, exists := global[key]
		if !exists {
			global[key] = wsVal
			continue
		}

		// Try array merge for known array fields
		switch key {
		case "tools", "allowedTools", "resources", "rules":
			merged := mergeJSONArrays(globalVal, wsVal)
			if merged != nil {
				global[key] = merged
				continue
			}
		case "hooks", "toolsSettings", "mcpServers":
			merged := mergeJSONObjects(globalVal, wsVal)
			if merged != nil {
				global[key] = merged
				continue
			}
		}

		// Scalar: workspace wins if non-empty
		wsStr := strings.Trim(string(wsVal), "\" \t\n")
		if wsStr != "" && wsStr != "null" {
			global[key] = wsVal
		}
	}

	return json.MarshalIndent(global, "", "  ")
}

func mergeJSONArrays(a, b json.RawMessage) json.RawMessage {
	var arrA, arrB []string
	if json.Unmarshal(a, &arrA) != nil || json.Unmarshal(b, &arrB) != nil {
		return nil
	}
	seen := map[string]bool{}
	for _, v := range arrA {
		seen[v] = true
	}
	for _, v := range arrB {
		if !seen[v] {
			arrA = append(arrA, v)
			seen[v] = true
		}
	}
	out, _ := json.Marshal(arrA)
	return out
}

func mergeJSONObjects(a, b json.RawMessage) json.RawMessage {
	var objA, objB map[string]json.RawMessage
	if json.Unmarshal(a, &objA) != nil || json.Unmarshal(b, &objB) != nil {
		return nil
	}
	for k, v := range objB {
		objA[k] = v
	}
	out, _ := json.Marshal(objA)
	return out
}
