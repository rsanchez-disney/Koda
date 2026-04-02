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

// ListWorkspaces discovers workspace.json files under steerRoot/workspaces/.
func ListWorkspaces(steerRoot string) ([]model.Workspace, error) {
	wsDir := filepath.Join(steerRoot, config.WorkspacesDir)
	entries, err := os.ReadDir(wsDir)
	if err != nil {
		return nil, nil
	}
	var workspaces []model.Workspace
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		wsFile := filepath.Join(wsDir, e.Name(), "workspace.json")
		data, err := os.ReadFile(wsFile)
		if err != nil {
			continue
		}
		var ws model.Workspace
		if json.Unmarshal(data, &ws) == nil {
			workspaces = append(workspaces, ws)
		}
	}
	return workspaces, nil
}

// GetWorkspace loads a single workspace by name.
func GetWorkspace(steerRoot, name string) (model.Workspace, error) {
	wsFile := filepath.Join(steerRoot, config.WorkspacesDir, name, "workspace.json")
	data, err := os.ReadFile(wsFile)
	if err != nil {
		return model.Workspace{}, fmt.Errorf("workspace not found: %s", name)
	}
	var ws model.Workspace
	if err := json.Unmarshal(data, &ws); err != nil {
		return model.Workspace{}, err
	}
	return ws, nil
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

	// Install profiles (global first, then workspace overrides win)
	profiles := ExpandAliases(resolved.Profiles)
	InstallShared(steerRoot, targetDir)
	for _, p := range profiles {
		InstallProfile(steerRoot, p, targetDir)
	}
	for _, wsName := range wsNames {
		wsProfilesDir := filepath.Join(steerRoot, config.WorkspacesDir, wsName, "profiles")
		for _, p := range profiles {
			wsProfile := filepath.Join(wsProfilesDir, p)
			if _, err := os.Stat(wsProfile); err == nil {
				InstallProfileFrom(wsProfile, targetDir)
			}
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
		wsPath := filepath.Join(steerRoot, config.WorkspacesDir, wsName)
		copyDirContents(filepath.Join(wsPath, config.RulesDir), filepath.Join(targetDir, config.RulesDir))
		copyDirContents(filepath.Join(wsPath, config.ContextDir), filepath.Join(targetDir, config.ContextDir))
	}

	InjectAgentTokens(targetDir)

	// Clone missing repos
	if ws.WorkspacePath != "" {
		CloneWorkspaceRepos(ws)
	}

	// Ensure memory banks exist for each project
	for _, p := range resolved.Projects {
		projPath := p.Path
		if strings.HasPrefix(projPath, "~/") {
			home, _ := os.UserHomeDir()
			projPath = filepath.Join(home, projPath[2:])
		} else if strings.HasPrefix(projPath, "../") && steerRoot != "" {
			projPath = filepath.Join(filepath.Dir(steerRoot), projPath[3:])
		}
		if _, err := os.Stat(filepath.Join(projPath, ".git")); err != nil {
			continue // not cloned locally
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
	s := config.ReadSteerSettings()
	s.ActiveWorkspace = ws.Name
	config.SaveSteerSettings(s)

	return nil
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
