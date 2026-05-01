package ops

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"os"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// ListWorkspaces discovers workspace.json files under steerRoot/workspaces/ recursively.
func ListWorkspaces(steerRoot string) ([]model.Workspace, error) {
	wsDir := filepath.Join(steerRoot, config.WorkspacesDir)
	var workspaces []model.Workspace
	filepath.Walk(wsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "workspace.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var ws model.Workspace
		if json.Unmarshal(data, &ws) == nil {
			workspaces = append(workspaces, ws)
		}
		return nil
	})
	return workspaces, nil
}

// GetWorkspace loads a single workspace by name, searching recursively.
func GetWorkspace(steerRoot, name string) (model.Workspace, error) {
	// Try flat path first (fast path)
	wsFile := filepath.Join(steerRoot, config.WorkspacesDir, name, "workspace.json")
	if data, err := os.ReadFile(wsFile); err == nil {
		var ws model.Workspace
		if json.Unmarshal(data, &ws) == nil {
			return ws, nil
		}
	}
	// Search recursively
	wsDir := filepath.Join(steerRoot, config.WorkspacesDir)
	var found model.Workspace
	var foundErr error = fmt.Errorf("workspace not found: %s", name)
	filepath.Walk(wsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "workspace.json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var ws model.Workspace
		if json.Unmarshal(data, &ws) == nil && ws.Name == name {
			found = ws
			foundErr = nil
			return filepath.SkipAll
		}
		return nil
	})
	return found, foundErr
}

// findWorkspaceDir returns the directory containing a workspace.json for the given name.
func findWorkspaceDir(steerRoot, name string) string {
	// Try flat path first
	flat := filepath.Join(steerRoot, config.WorkspacesDir, name)
	if _, err := os.Stat(filepath.Join(flat, "workspace.json")); err == nil {
		return flat
	}
	// Search recursively
	wsDir := filepath.Join(steerRoot, config.WorkspacesDir)
	var result string
	filepath.Walk(wsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() != "workspace.json" {
			return nil
		}
		data, _ := os.ReadFile(path)
		var ws model.Workspace
		if json.Unmarshal(data, &ws) == nil && ws.Name == name {
			result = filepath.Dir(path)
			return filepath.SkipAll
		}
		return nil
	})
	if result != "" {
		return result
	}
	return flat // fallback
}

// ResolveWorkspace walks the extends chain and merges into a single workspace.
// Additive: profiles, rules. Child-wins: scalar fields. Context dirs are collected for copy.
func ResolveWorkspace(steerRoot string, ws model.Workspace) (model.Workspace, []string) {
	if ws.Extends == "" {
		return ws, []string{ws.Name}
	}

	// Walk chain bottom-up, collect ancestors
	chain := []model.Workspace{ws}
	seen := map[string]bool{ws.Name: true}
	cur := ws
	for cur.Extends != "" {
		if seen[cur.Extends] {
			break // cycle guard
		}
		parent, err := GetWorkspace(steerRoot, cur.Extends)
		if err != nil {
			break
		}
		seen[parent.Name] = true
		chain = append(chain, parent)
		cur = parent
	}

	// Merge root-first: start from oldest ancestor
	merged := chain[len(chain)-1]
	names := []string{merged.Name}
	for i := len(chain) - 2; i >= 0; i-- {
		child := chain[i]
		names = append(names, child.Name)
		// Additive
		merged.Profiles = appendUnique(merged.Profiles, child.Profiles)
		merged.Rules = appendUnique(merged.Rules, child.Rules)
		merged.Services = appendUnique(merged.Services, child.Services)
		merged.Channels = appendUnique(merged.Channels, child.Channels)
		// Child-wins scalars
		merged.Name = child.Name
		merged.Extends = child.Extends
		if child.Description != "" {
			merged.Description = child.Description
		}
		if child.Team != "" {
			merged.Team = child.Team
		}
		if child.DefaultAgent != "" {
			merged.DefaultAgent = child.DefaultAgent
		}
		if child.JiraPrefix != "" {
			merged.JiraPrefix = child.JiraPrefix
		}
		if len(child.JiraCustomFields) > 0 {
			if merged.JiraCustomFields == nil {
				merged.JiraCustomFields = map[string]string{}
			}
			for k, v := range child.JiraCustomFields {
				merged.JiraCustomFields[k] = v
			}
		}
		if child.WorkspacePath != "" {
			merged.WorkspacePath = child.WorkspacePath
		}
		if len(child.Projects) > 0 {
			merged.Projects = child.Projects
		}
		if child.EnableTools {
			merged.EnableTools = true
		}
	}
	return merged, names
}

func appendUnique(base, add []string) []string {
	seen := map[string]bool{}
	for _, s := range base {
		seen[s] = true
	}
	for _, s := range add {
		if !seen[s] {
			base = append(base, s)
			seen[s] = true
		}
	}
	return base
}

// ApplyWorkspace installs a workspace's profiles, rules, and context.
func ApplyWorkspace(steerRoot, targetDir string, ws model.Workspace) error {
	// Resolve inheritance
	resolved, wsNames := ResolveWorkspace(steerRoot, ws)

	// Fetch latest steer-runtime before installing so profiles use updated source
	logln("  Syncing steer-runtime...")
	s := config.ReadSteerSettings()
	if s.Source == "git" {
		syncGit(steerRoot)
	}
	config.MarkSynced()

	// Snapshot files present before workspace install so we can track what we add
	before := snapshotFiles(targetDir)

	// If switching from a different workspace, clean up its files first
	if s.ActiveWorkspace != "" && s.ActiveWorkspace != ws.Name {
		logf("  Deactivating workspace '%s'...\n", s.ActiveWorkspace)
		DeactivateWorkspace(targetDir)
	}

	// Build workspace override map: last workspace in chain wins for each profile ID
	profiles := ExpandAliases(resolved.Profiles)
	wsOverrides := map[string]string{} // profileID -> wsProfileDir
	for _, wsName := range wsNames {
		wsProfilesDir := filepath.Join(findWorkspaceDir(steerRoot, wsName), "profiles")
		for _, p := range profiles {
			wsProfile := filepath.Join(wsProfilesDir, p)
			if _, err := os.Stat(wsProfile); err == nil {
				wsOverrides[p] = wsProfile
			}
		}
	}

	// Install profiles: core (always), then selected profiles with workspace overlays
	InstallShared(steerRoot, targetDir)
	InstallProfile(steerRoot, "core", targetDir)
	for _, p := range profiles {
		InstallProfile(steerRoot, p, targetDir)
		if wsDir, ok := wsOverrides[p]; ok {
			InstallProfileFrom(wsDir, targetDir)
		}
	}

	// Install common rules from all resolved
	for _, rule := range resolved.Rules {
		src := filepath.Join(steerRoot, "common", config.RulesDir, rule+".md")
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(targetDir, config.RulesDir)
			os.MkdirAll(dst, 0755)
			copyFile(src, filepath.Join(dst, rule+".md"))
		}
	}

	// Copy rules and context from each workspace in the chain (root first)
	for _, wsName := range wsNames {
		wsPath := findWorkspaceDir(steerRoot, wsName)
		copyDirContents(filepath.Join(wsPath, config.RulesDir), filepath.Join(targetDir, config.RulesDir))
		copyDirContents(filepath.Join(wsPath, config.ContextDir), filepath.Join(targetDir, config.ContextDir))
	}


	// Copy workspace steering and MCP server bundles (chain order: parent first)
	InstallWorkspaceSteering(steerRoot, targetDir, wsNames)
	InstallWorkspaceMCPBundles(steerRoot, targetDir, wsNames)

	// Install workspace-level common/ (transversal to all profiles in this workspace)
	InstallWorkspaceCommon(steerRoot, targetDir, wsNames)

	InjectAgentTokens(targetDir)
	EnrichWelcomeMessages(targetDir)
	GenerateMcpJson(FindNodeExe())

	// Install service and channel banks
	if len(resolved.Services) > 0 || len(resolved.Channels) > 0 {
		svcN, chN := InstallBanks(steerRoot, targetDir, resolved.Services, resolved.Channels)
		if svcN > 0 || chN > 0 {
			logf("  \u2713 %d service banks, %d channel banks\n", svcN, chN)
		}
	}

	// For each project: clone if missing, then ensure memory bank exists
	for _, p := range resolved.Projects {
		projPath := resolveProjectPath(resolved.WorkspacePath, p.Path, steerRoot)
		if _, err := os.Stat(filepath.Join(projPath, ".git")); err != nil {
			if p.Repo != "" && resolved.WorkspacePath != "" {
				logf("  Cloning %s...\n", p.Name)
				url := GitCloneURL(p.Repo)
				if err := exec.Command("git", "clone", url, projPath).Run(); err != nil {
					logf("  ✗ %s (clone failed: %v)\n", p.Name, err)
					continue
				}
				logf("  ✓ %s cloned\n", p.Name)
			} else {
				logf("  ⏭ %s (not cloned)\n", p.Name)
				continue
			}
		}
		mbPath := filepath.Join(projPath, ".kiro", config.RulesDir, "memory-bank")
		if entries, _ := os.ReadDir(mbPath); len(entries) == 0 {
			logf("  Initializing memory bank for %s...\n", p.Name)
			from := p.MemoryBank
			if from == "" {
				from = p.Name
			}
			InitMemory(steerRoot, projPath, from)
		}
	}

	// Save active workspace
	s.ActiveWorkspace = ws.Name
	config.SaveSteerSettings(s)

	// Write manifest of files added by this workspace apply (used by DeactivateWorkspace)
	after := snapshotFiles(targetDir)
	WriteWorkspaceManifest(targetDir, before, after)

	WriteProfilesManifest(steerRoot, targetDir)

	// Persist system resource profile for agent/hook consumption
	WriteSystemProfile()

	// Persist resolved workspace snapshot for agent/hook consumption
	WriteWorkspaceSnapshot(targetDir, resolved)

	// Update default agent for this workspace
	if agent := SuggestDefaultAgent(steerRoot, targetDir); agent != "" {
		SetKiroSetting("chat.defaultAgent", agent)
		logf("  ✓ kiro: chat.defaultAgent = %s\n", agent)
	}

	return nil
}

// resolveProjectPath resolves a project path using workspace_path as base when set.
func resolveProjectPath(workspacePath, projPath, steerRoot string) string {
	if workspacePath != "" && !filepath.IsAbs(projPath) && !strings.HasPrefix(projPath, "~/") && !strings.HasPrefix(projPath, "../") {
		base := expandHome(workspacePath)
		return filepath.Join(base, projPath)
	}
	if strings.HasPrefix(projPath, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, projPath[2:])
	}
	if strings.HasPrefix(projPath, "../") && steerRoot != "" {
		return filepath.Join(filepath.Dir(steerRoot), projPath[3:])
	}
	return projPath
}


// installDirs are the only directories tracked by the workspace manifest.
// This avoids capturing settings, sessions, MCP bundles, or files written
// by background processes during apply.
var installDirs = []string{
	config.AgentsDir, config.PromptsDir, config.ContextDir,
	config.SteeringDir, config.RulesDir, config.SkillsDir, config.HooksDir,
}

// snapshotFiles returns the set of relative paths (relative to targetDir) for
// all regular files under the known install dirs.
func snapshotFiles(targetDir string) map[string]struct{} {
	files := map[string]struct{}{}
	for _, dir := range installDirs {
		filepath.Walk(filepath.Join(targetDir, dir), func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				if rel, err := filepath.Rel(targetDir, path); err == nil {
					files[rel] = struct{}{}
				}
			}
			return nil
		})
	}
	return files
}

// WriteWorkspaceManifest persists relative paths of files added by workspace apply
// (i.e. present after but not before) to WorkspaceManifestFile.
func WriteWorkspaceManifest(targetDir string, before, after map[string]struct{}) {
	var added []string
	for f := range after {
		if _, existed := before[f]; !existed {
			added = append(added, f)
		}
	}
	data, _ := json.MarshalIndent(added, "", "  ")
	os.WriteFile(filepath.Join(targetDir, config.WorkspaceManifestFile), append(data, '\n'), 0644)
}

// RemoveWorkspaceFiles reads the manifest and deletes every listed file.
// Paths in the manifest are relative to targetDir.
func RemoveWorkspaceFiles(targetDir string) {
	manifestPath := filepath.Join(targetDir, config.WorkspaceManifestFile)
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return
	}
	var files []string
	if json.Unmarshal(data, &files) != nil {
		return
	}
	for _, f := range files {
		os.Remove(filepath.Join(targetDir, f))
	}
	os.Remove(manifestPath)
}

// DeactivateWorkspace removes files installed by the active workspace and removes
// the workspace snapshot. ActiveWorkspace is intentionally NOT cleared here —
// ApplyWorkspace overwrites it immediately after calling this function.
func DeactivateWorkspace(targetDir string) {
	RemoveWorkspaceFiles(targetDir)
	os.Remove(filepath.Join(targetDir, config.SettingsDir, "workspace.json"))
}

// InstallWorkspaceCommon copies workspace-level common/ files into targetDir,
// mirroring the behaviour of the global steer-runtime common/.
// Files are suffixed with the workspace name to avoid collisions with global common.
// e.g. bugfix.md → bugfix-opsheet-team.md
// Walks the inheritance chain (parent first, child wins).
// Note: only one level deep is supported (common/{subdir}/{file}).
// Nested subdirectories inside common/ are not copied.
func InstallWorkspaceCommon(steerRoot, targetDir string, wsNames []string) {
	for _, wsName := range wsNames {
		wsPath := findWorkspaceDir(steerRoot, wsName)
		commonSrc := filepath.Join(wsPath, "common")
		if _, err := os.Stat(commonSrc); err != nil {
			continue
		}
		entries, err := os.ReadDir(commonSrc)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			subSrc := filepath.Join(commonSrc, e.Name())
			subDst := filepath.Join(targetDir, e.Name())
			os.MkdirAll(subDst, 0755)
			files, err := os.ReadDir(subSrc)
			if err != nil {
				continue
			}
			for _, f := range files {
				if f.IsDir() || strings.HasPrefix(f.Name(), "._") {
					continue
				}
				ext := filepath.Ext(f.Name())
				base := strings.TrimSuffix(f.Name(), ext)
				dstName := base + "-" + wsName + ext
				copyFile(filepath.Join(subSrc, f.Name()), filepath.Join(subDst, dstName))
			}
		}
	}
}

// InstallWorkspaceSteering copies workspace-level steering files into targetDir.
// Walks the inheritance chain (parent first, child wins).
func InstallWorkspaceSteering(steerRoot, targetDir string, wsNames []string) {
	for _, wsName := range wsNames {
		wsPath := findWorkspaceDir(steerRoot, wsName)
		copyDirContents(
			filepath.Join(wsPath, config.SteeringDir),
			filepath.Join(targetDir, config.SteeringDir),
		)
	}
}

// InstallWorkspaceMCPBundles copies workspace MCP server bundles and their
// mcp-meta.json descriptors into targetDir. Walks the inheritance chain.
func InstallWorkspaceMCPBundles(steerRoot, targetDir string, wsNames []string) {
	for _, wsName := range wsNames {
		wsPath := findWorkspaceDir(steerRoot, wsName)
		mcpSrc := filepath.Join(wsPath, config.ToolsDir, "mcp-servers")
		entries, err := os.ReadDir(mcpSrc)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			srcDir := filepath.Join(mcpSrc, e.Name())
			dstDir := filepath.Join(targetDir, config.ToolsDir, "mcp-servers", e.Name())

			bundle := filepath.Join(srcDir, "dist", "index.cjs")
			if _, err := os.Stat(bundle); err == nil {
				os.MkdirAll(filepath.Join(dstDir, "dist"), 0755)
				copyFile(bundle, filepath.Join(dstDir, "dist", "index.cjs"))
			}

			meta := filepath.Join(srcDir, "mcp-meta.json")
			if _, err := os.Stat(meta); err == nil {
				os.MkdirAll(dstDir, 0755)
				copyFile(meta, filepath.Join(dstDir, "mcp-meta.json"))
			}
		}
	}
}
// PrintWorkspace prints workspace details.
func PrintWorkspace(ws model.Workspace) {
	fmt.Printf("\n  Name:        %s\n", ws.Name)
	if ws.Extends != "" {
		fmt.Printf("  Extends:     %s\n", ws.Extends)
	}
	if ws.Description != "" {
		fmt.Printf("  Description: %s\n", ws.Description)
	}
	if ws.Team != "" {
		fmt.Printf("  Team:        %s\n", ws.Team)
	}
	if ws.JiraPrefix != "" {
		fmt.Printf("  Jira Prefix: %s\n", ws.JiraPrefix)
	}
	fmt.Printf("  Profiles:    %s\n", strings.Join(ws.Profiles, ", "))
	if ws.DefaultAgent != "" {
		fmt.Printf("  Agent:       %s\n", ws.DefaultAgent)
	}
	if len(ws.Projects) > 0 {
		fmt.Println("  Projects:")
		for _, p := range ws.Projects {
			fmt.Printf("    • %s (%s)\n", p.Name, p.Path)
		}
	}
	if len(ws.Rules) > 0 {
		fmt.Printf("  Rules:       %s\n", strings.Join(ws.Rules, ", "))
	}
	fmt.Println()
}

// WriteWorkspaceSnapshot persists the resolved workspace config to
// ~/.kiro/settings/workspace.json so hooks and agents can read it directly.
func WriteWorkspaceSnapshot(targetDir string, ws model.Workspace) {
	settingsDir := filepath.Join(targetDir, config.SettingsDir)
	os.MkdirAll(settingsDir, 0755)
	data, _ := json.MarshalIndent(ws, "", "  ")
	os.WriteFile(filepath.Join(settingsDir, "workspace.json"), append(data, '\n'), 0644)
}
