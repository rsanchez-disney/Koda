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

// ApplyWorkspace installs a workspace's profiles, rules, and context.
func ApplyWorkspace(steerRoot, targetDir string, ws model.Workspace) error {
	// Install profiles
	profiles := ExpandAliases(ws.Profiles)
	InstallShared(steerRoot, targetDir)
	for _, p := range profiles {
		InstallProfile(steerRoot, p, targetDir)
	}

	// Install common rules
	for _, rule := range ws.Rules {
		src := filepath.Join(steerRoot, "common", config.RulesDir, rule+".md")
		if _, err := os.Stat(src); err == nil {
			dst := filepath.Join(targetDir, config.RulesDir)
			os.MkdirAll(dst, 0755)
			copyFile(src, filepath.Join(dst, rule+".md"))
		}
	}

	// Copy workspace-specific rules and context
	wsPath := filepath.Join(steerRoot, config.WorkspacesDir, ws.Name)
	copyDirContents(filepath.Join(wsPath, config.RulesDir), filepath.Join(targetDir, config.RulesDir))
	copyDirContents(filepath.Join(wsPath, config.ContextDir), filepath.Join(targetDir, config.ContextDir))

	InjectAgentTokens(targetDir)

	// Save active workspace
	s := config.ReadSteerSettings()
	s.ActiveWorkspace = ws.Name
	config.SaveSteerSettings(s)

	return nil
}

// PrintWorkspace prints workspace details.
func PrintWorkspace(ws model.Workspace) {
	fmt.Printf("\n  Name:        %s\n", ws.Name)
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
