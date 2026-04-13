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
	fmt.Println("  Syncing steer-runtime...")
	s := config.ReadSteerSettings()
	if s.Source == "git" {
		syncGit(steerRoot)
	}
	config.MarkSynced()

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

	// Install profiles: workspace override replaces global entirely
	InstallShared(steerRoot, targetDir)
	for _, p := range profiles {
		if wsDir, ok := wsOverrides[p]; ok {
			InstallProfileFrom(wsDir, targetDir)
		} else {
			InstallProfile(steerRoot, p, targetDir)
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

	InjectAgentTokens(targetDir)
	GenerateMcpJson(FindNodeExe())

	// Install service and channel banks
	if len(resolved.Services) > 0 || len(resolved.Channels) > 0 {
		svcN, chN := InstallBanks(steerRoot, targetDir, resolved.Services, resolved.Channels)
		if svcN > 0 || chN > 0 {
			fmt.Printf("  \u2713 %d service banks, %d channel banks\n", svcN, chN)
		}
	}

	// For each project: clone if missing, then ensure memory bank exists
	for _, p := range resolved.Projects {
		projPath := resolveProjectPath(resolved.WorkspacePath, p.Path, steerRoot)
		if _, err := os.Stat(filepath.Join(projPath, ".git")); err != nil {
			if p.Repo != "" && resolved.WorkspacePath != "" {
				fmt.Printf("  Cloning %s...\n", p.Name)
				url := GitCloneURL(p.Repo)
				if err := exec.Command("git", "clone", url, projPath).Run(); err != nil {
					fmt.Printf("  ✗ %s (clone failed: %v)\n", p.Name, err)
					continue
				}
				fmt.Printf("  ✓ %s cloned\n", p.Name)
			} else {
				fmt.Printf("  ⏭ %s (not cloned)\n", p.Name)
				continue
			}
		}
		mbPath := filepath.Join(projPath, ".kiro", config.RulesDir, "memory-bank")
		if entries, _ := os.ReadDir(mbPath); len(entries) == 0 {
			fmt.Printf("  Initializing memory bank for %s...\n", p.Name)
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

	// Update default agent for this workspace
	if agent := SuggestDefaultAgent(steerRoot, targetDir); agent != "" {
		SetKiroSetting("chat.defaultAgent", agent)
		fmt.Printf("  ✓ kiro: chat.defaultAgent = %s\n", agent)
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
