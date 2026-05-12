package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// MaterializedMeta tracks workspace materialization state.
type MaterializedMeta struct {
	Name        string    `json:"name"`
	LastUsed    time.Time `json:"lastUsed"`
	LastSynced  time.Time `json:"lastSynced"`
	Profiles    []string  `json:"profiles"`
	SteerCommit string    `json:"steerCommit,omitempty"`
}

// MaterializedWorkspace represents a workspace with a runtime dir.
type MaterializedWorkspace struct {
	Name   string
	Dir    string
	Meta   MaterializedMeta
	Active bool
}

// MaterializeWorkspace installs a workspace's profiles into ~/.kiro/workspaces/<name>/.
func MaterializeWorkspace(steerRoot string, ws model.Workspace) error {
	targetDir := config.WorkspaceRuntimeDir(ws.Name)

	// Resolve inheritance
	resolved, wsNames := ResolveWorkspace(steerRoot, ws)
	profiles := ExpandAliases(resolved.Profiles)

	// Install shared + profiles into workspace-specific dir
	InstallShared(steerRoot, targetDir)

	wsOverrides := map[string]string{}
	for _, wsName := range wsNames {
		wsProfilesDir := filepath.Join(findWorkspaceDir(steerRoot, wsName), "profiles")
		for _, p := range profiles {
			wsProfile := filepath.Join(wsProfilesDir, p)
			if _, err := os.Stat(wsProfile); err == nil {
				wsOverrides[p] = wsProfile
			}
		}
	}

	for _, p := range profiles {
		if wsDir, ok := wsOverrides[p]; ok {
			InstallProfileFrom(wsDir, targetDir)
			TrackProfileInstall(p, wsDir, targetDir)
		} else {
			InstallProfile(steerRoot, p, targetDir)
			TrackProfileInstall(p, filepath.Join(steerRoot, config.ProfilePrefix+p), targetDir)
		}
	}

	// Copy rules and context from workspace chain
	for _, wsName := range wsNames {
		wsPath := findWorkspaceDir(steerRoot, wsName)
		copyDirContents(filepath.Join(wsPath, config.RulesDir), filepath.Join(targetDir, config.RulesDir))
		copyDirContents(filepath.Join(wsPath, config.ContextDir), filepath.Join(targetDir, config.ContextDir))
	}

	// Copy global mcp.json
	globalMCP := filepath.Join(config.KiroRoot(), config.SettingsDir, "mcp.json")
	localSettings := filepath.Join(targetDir, config.SettingsDir)
	os.MkdirAll(localSettings, 0755)
	if data, err := os.ReadFile(globalMCP); err == nil {
		os.WriteFile(filepath.Join(localSettings, "mcp.json"), data, 0600)
	}

	// Write workspace snapshot
	WriteWorkspaceSnapshot(targetDir, resolved)

	// Inject tokens
	InjectAgentTokens(targetDir)

	// Write meta
	meta := MaterializedMeta{
		Name:       ws.Name,
		LastUsed:   time.Now(),
		LastSynced: time.Now(),
		Profiles:   profiles,
	}
	metaData, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(filepath.Join(targetDir, ".meta.json"), metaData, 0644)

	return nil
}

// DematerializeWorkspace removes a workspace's runtime dir.
func DematerializeWorkspace(name string) error {
	dir := config.WorkspaceRuntimeDir(name)
	if _, err := os.Stat(dir); err != nil {
		return fmt.Errorf("workspace '%s' is not materialized", name)
	}
	return os.RemoveAll(dir)
}

// ListMaterialized returns all materialized workspaces.
func ListMaterialized() []MaterializedWorkspace {
	baseDir := config.WorkspacesRuntimeDir()
	entries, err := os.ReadDir(baseDir)
	if err != nil {
		return nil
	}

	s := config.ReadSteerSettings()
	activeSet := map[string]bool{}
	for _, name := range s.ActiveWorkspaces {
		activeSet[name] = true
	}
	// Backward compat
	if len(s.ActiveWorkspaces) == 0 && s.ActiveWorkspace != "" {
		activeSet[s.ActiveWorkspace] = true
	}

	var result []MaterializedWorkspace
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		metaPath := filepath.Join(baseDir, e.Name(), ".meta.json")
		var meta MaterializedMeta
		if data, err := os.ReadFile(metaPath); err == nil {
			json.Unmarshal(data, &meta)
		} else {
			meta.Name = e.Name()
		}
		result = append(result, MaterializedWorkspace{
			Name:   e.Name(),
			Dir:    filepath.Join(baseDir, e.Name()),
			Meta:   meta,
			Active: activeSet[e.Name()],
		})
	}
	return result
}

// TouchWorkspace updates the lastUsed timestamp for a materialized workspace.
func TouchWorkspace(name string) {
	metaPath := filepath.Join(config.WorkspaceRuntimeDir(name), ".meta.json")
	var meta MaterializedMeta
	if data, err := os.ReadFile(metaPath); err == nil {
		json.Unmarshal(data, &meta)
	}
	meta.LastUsed = time.Now()
	data, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(metaPath, data, 0644)
}

// PropagateMCPJson copies the global mcp.json to all materialized workspaces.
func PropagateMCPJson() {
	globalMCP := filepath.Join(config.KiroRoot(), config.SettingsDir, "mcp.json")
	data, err := os.ReadFile(globalMCP)
	if err != nil {
		return
	}
	for _, mw := range ListMaterialized() {
		dst := filepath.Join(mw.Dir, config.SettingsDir, "mcp.json")
		os.MkdirAll(filepath.Dir(dst), 0755)
		os.WriteFile(dst, data, 0600)
	}
}

// SyncMaterializedWorkspaces re-materializes the given workspaces.
func SyncMaterializedWorkspaces(steerRoot string, names []string) {
	for _, name := range names {
		ws, err := GetWorkspace(steerRoot, name)
		if err != nil {
			fmt.Printf("  ⚠ %s: %v\n", name, err)
			continue
		}
		if err := MaterializeWorkspace(steerRoot, ws); err != nil {
			fmt.Printf("  ⚠ %s: %v\n", name, err)
		} else {
			fmt.Printf("  ✓ %s synced\n", name)
		}
	}
}

// PruneMaterialized removes workspaces not used in the given duration.
func PruneMaterialized(maxAge time.Duration) int {
	pruned := 0
	for _, mw := range ListMaterialized() {
		if mw.Active {
			continue
		}
		if time.Since(mw.Meta.LastUsed) > maxAge {
			os.RemoveAll(mw.Dir)
			pruned++
		}
	}
	return pruned
}
