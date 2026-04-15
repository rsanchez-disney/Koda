package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// MCPServer describes an available MCP server and its requirements.
type MCPServer struct {
	Name      string   // display name (e.g., "jira", "confluence")
	BundleDir string   // directory name under mcp-servers/ (e.g., "jira-mcp")
	TokenKeys []string // required token keys from KnownTokens (e.g., ["JIRA_PAT"])
	EnvKeys   []string // required env var keys (e.g., ["CONFLUENCE_URL"])
	Command   string   // override command (default: "node"); set from mcp-meta.json
	IsNPM     bool     // true for context7 (npm install required)
	IsSSE     bool     // true for compass (SSE transport)
}

// knownServers defines all MCP servers Koda can install.
var knownServers = []MCPServer{
	{Name: "jira", BundleDir: "jira-mcp"},
	{Name: "confluence", BundleDir: "confluence-mcp"},
	{Name: "mermaid", BundleDir: "mermaid-diagram-mcp"},
	{Name: "bruno", BundleDir: "bruno-mcp"},
	{Name: "figma", BundleDir: "figma-mcp", TokenKeys: []string{"FIGMA_TOKEN"}},
	{Name: "github", BundleDir: "github-mcp"},
	{Name: "compass", BundleDir: "", TokenKeys: []string{"COMPASS_TOKEN"}, EnvKeys: []string{"COMPASS_URL"}, IsSSE: true},
}

// CopyMcpBundles copies pre-built MCP server bundles from steerRoot to ~/.kiro/tools/mcp-servers/.
// Returns the number of bundles copied.
func CopyMcpBundles(steerRoot string) int {
	home, _ := os.UserHomeDir()
	srcDir := filepath.Join(steerRoot, "shared", config.ToolsDir, "mcp-servers")
	dstBase := filepath.Join(home, ".kiro", config.ToolsDir, "mcp-servers")
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		bundle := filepath.Join(srcDir, e.Name(), "dist", "index.cjs")
		if _, err := os.Stat(bundle); err == nil {
			dst := filepath.Join(dstBase, e.Name(), "dist")
			os.MkdirAll(dst, 0755)
			copyFile(bundle, filepath.Join(dst, "index.cjs"))
			count++
		}
	}
	return count
}

// GenerateMcpJson writes ~/.kiro/settings/mcp.json.
// If nodeExe is empty, "node" is used. Pass FindNodeExe() for absolute path resolution.
// This is the legacy function used by kiroide.go for non-interactive config generation.
func GenerateMcpJson(nodeExe string) error {
	if nodeExe == "" {
		nodeExe = "node"
	}
	tokens := ReadTokens()
	envVars := ReadEnvVars()
	home, _ := os.UserHomeDir()

	type mcpServer struct {
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
		Type    string            `json:"type,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
	}

	bundleDir := filepath.Join(home, ".kiro", "tools", "mcp-servers")

	servers := map[string]mcpServer{
		"mermaid": {
			Command: nodeExe,
			Args:    []string{filepath.Join(bundleDir, "mermaid-diagram-mcp", "dist", "index.cjs")},
		},
		"bruno": {
			Command: nodeExe,
			Args:    []string{filepath.Join(bundleDir, "bruno-mcp", "dist", "index.cjs")},
		},
		"figma": {
			Command: nodeExe,
			Args:    []string{filepath.Join(bundleDir, "figma-mcp", "dist", "index.cjs")},
			Env:     map[string]string{"FIGMA_TOKEN": tokens["FIGMA_TOKEN"]},
		},
	}

	// fetch via uvx (optional)
	if uvx := FindUvxExe(); uvx != "" {
		servers["fetch"] = mcpServer{Command: uvx, Args: []string{"mcp-server-fetch"}}
	}

	// Jira: per-instance entries
	jiraInstances := ReadJiraInstances()
	jiraBundle := filepath.Join(bundleDir, "jira-mcp", "dist", "index.cjs")
	if len(jiraInstances) == 1 {
		servers["jira"] = mcpServer{Command: nodeExe, Args: []string{jiraBundle},
			Env: map[string]string{"JIRA_PAT": jiraInstances[0].Token, "JIRA_URL": jiraInstances[0].URL}}
	} else {
		for _, inst := range jiraInstances {
			servers["jira-"+inst.Name] = mcpServer{Command: nodeExe, Args: []string{jiraBundle},
				Env: map[string]string{"JIRA_PAT": inst.Token, "JIRA_URL": inst.URL}}
		}
	}

	// Confluence: per-instance entries
	confInstances := ReadConfluenceInstances()
	confBundle := filepath.Join(bundleDir, "confluence-mcp", "dist", "index.cjs")
	if len(confInstances) == 1 {
		servers["confluence"] = mcpServer{Command: nodeExe, Args: []string{confBundle},
			Env: map[string]string{"CONFLUENCE_PAT": confInstances[0].Token, "CONFLUENCE_URL": confInstances[0].URL}}
	} else {
		for _, inst := range confInstances {
			servers["confluence-"+inst.Name] = mcpServer{Command: nodeExe, Args: []string{confBundle},
				Env: map[string]string{"CONFLUENCE_PAT": inst.Token, "CONFLUENCE_URL": inst.URL}}
		}
	}

	// GitHub: per-remote entries
	ghRemotes := ReadGitHubRemotes()
	ghBundle := filepath.Join(bundleDir, "github-mcp", "dist", "index.cjs")
	if len(ghRemotes) == 1 {
		env := map[string]string{"GITHUB_REMOTE": ghRemotes[0].Name, "GITHUB_HOST": ghRemotes[0].Host, "GITHUB_TOKEN": ghRemotes[0].Token}
		if ghRemotes[0].APIPath != "" {
			env["GITHUB_API_PATH"] = ghRemotes[0].APIPath
		}
		servers["github"] = mcpServer{Command: nodeExe, Args: []string{ghBundle}, Env: env}
	} else {
		for _, r := range ghRemotes {
			env := map[string]string{"GITHUB_REMOTE": r.Name, "GITHUB_HOST": r.Host, "GITHUB_TOKEN": r.Token}
			if r.APIPath != "" {
				env["GITHUB_API_PATH"] = r.APIPath
			}
			servers["github-"+r.Name] = mcpServer{Command: nodeExe, Args: []string{ghBundle}, Env: env}
		}
	}

	// Compass: remote SSE MCP
	if ct := tokens["COMPASS_TOKEN"]; ct != "" {
		servers["compass"] = mcpServer{
			URL:     envVars["COMPASS_URL"],
			Type:    "sse",
			Headers: map[string]string{"Authorization": "Bearer " + ct},
		}
	}

	// Memory: docker-type MCP (HTTP endpoint when running)
	memStatus := MemoryStatus(config.TargetDir(""))
	if memStatus.Running {
		servers["memory"] = mcpServer{
			URL:  fmt.Sprintf("http://localhost:%d/mcp", memStatus.Port),
			Type: "sse",
		}
		fmt.Println("  ✓ memory-mcp (running)")
	} else if memStatus.Installed {
		fmt.Println("  ⚠ memory-mcp (installed but not running — use koda memory start)")
	}

	// Workspace MCP servers (from steer-runtime mcp-meta.json)
	steerRoot := filepath.Join(home, ".kiro", "steer-runtime")
	for _, wm := range walkWorkspaceMCPMetas(steerRoot) {
		bundle := filepath.Join(bundleDir, wm.DirName, "dist", "index.cjs")
		if _, err := os.Stat(bundle); err != nil {
			continue
		}
		cmd := nodeExe
		if wm.Meta.Command != "" {
			cmd = wm.Meta.Command
		}
		entry := mcpServer{Command: cmd, Args: []string{bundle}}
		if len(wm.Meta.Env) > 0 {
			env := make(map[string]string, len(wm.Meta.Env))
			hasValue := false
			for k := range wm.Meta.Env {
				if v := tokens[k]; v != "" {
					env[k] = v
					hasValue = true
				}
			}
			if !hasValue {
				continue // skip server when no tokens are configured
			}
			entry.Env = env
		}
		servers[wm.Meta.Name] = entry
	}

	mcpConfig := map[string]any{"mcpServers": servers}
	settingsDir := filepath.Join(home, ".kiro", config.SettingsDir)
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return fmt.Errorf("cannot create settings directory: %w", err)
	}
	mcpPath := filepath.Join(settingsDir, "mcp.json")
	out, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("cannot marshal config: %w", err)
	}
	return os.WriteFile(mcpPath, append(out, '\n'), 0644)
}

// DiscoverServers scans targetDir/tools/mcp-servers/ and returns all
// available servers with their bundle verification status.
func DiscoverServers(targetDir string) (available []MCPServer, verified map[string]bool) {
	verified = make(map[string]bool)
	mcpDir := filepath.Join(targetDir, config.ToolsDir, "mcp-servers")

	// Build a set of directories present on disk.
	dirSet := make(map[string]bool)
	entries, err := os.ReadDir(mcpDir)
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				dirSet[e.Name()] = true
			}
		}
	}

	for _, srv := range knownServers {
		if srv.IsSSE {
			// SSE servers (compass) are always available and verified.
			available = append(available, srv)
			verified[srv.Name] = true
			continue
		}

		if srv.BundleDir == "" {
			continue
		}

		if !dirSet[srv.BundleDir] {
			continue
		}

		available = append(available, srv)

		// Verify the bundle.
		if srv.IsNPM {
			pkgPath := filepath.Join(mcpDir, srv.BundleDir, "package.json")
			if _, err := os.Stat(pkgPath); err == nil {
				verified[srv.Name] = true
			}
		} else {
			cjsPath := filepath.Join(mcpDir, srv.BundleDir, "dist", "index.cjs")
			if _, err := os.Stat(cjsPath); err == nil {
				verified[srv.Name] = true
			}
		}
	}

	// Workspace MCP servers (discovered via mcp-meta.json)
	knownDirs := knownBundleDirs()
	if entries, err := os.ReadDir(mcpDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() || knownDirs[e.Name()] {
				continue
			}
			meta, err := ReadWorkspaceMCPMeta(filepath.Join(mcpDir, e.Name()))
			if err != nil {
				continue
			}
			var tokenKeys []string
			for k := range meta.Env {
				tokenKeys = append(tokenKeys, k)
			}
			cjsPath := filepath.Join(mcpDir, e.Name(), "dist", "index.cjs")
			srv := MCPServer{Name: meta.Name, BundleDir: e.Name(), TokenKeys: tokenKeys, Command: meta.Command}
			available = append(available, srv)
			if _, err := os.Stat(cjsPath); err == nil {
				verified[meta.Name] = true
			}
		}
	}
	return available, verified
}

// WorkspaceMCPTokens returns token definitions for workspace MCP servers
// discovered via mcp-meta.json, suitable for the configure command.
func WorkspaceMCPTokens(targetDir string) []model.Token {
	mcpDir := filepath.Join(targetDir, config.ToolsDir, "mcp-servers")
	entries, err := os.ReadDir(mcpDir)
	if err != nil {
		return nil
	}

	knownDirs := knownBundleDirs()

	var tokens []model.Token
	for _, e := range entries {
		if !e.IsDir() || knownDirs[e.Name()] {
			continue
		}
		meta, err := ReadWorkspaceMCPMeta(filepath.Join(mcpDir, e.Name()))
		if err != nil {
			continue
		}
		envKeys := make([]string, 0, len(meta.Env))
		for k := range meta.Env {
			envKeys = append(envKeys, k)
		}
		sort.Strings(envKeys)
		for _, k := range envKeys {
			tokens = append(tokens, model.Token{
				Key:   k,
				Label: fmt.Sprintf("%s (%s)", k, meta.Name),
			})
		}
	}
	return tokens
}

// workspaceMCPMeta pairs a parsed mcp-meta.json with its directory name.
type workspaceMCPMeta struct {
	DirName string
	Meta    *WorkspaceMCPMeta
}

// walkWorkspaceMCPMetas returns all workspace MCP metas from steer-runtime.
// Skips built-in server dirs. Returns nil if the workspaces dir doesn't exist.
func walkWorkspaceMCPMetas(steerRoot string) []workspaceMCPMeta {
	wsDir := filepath.Join(steerRoot, config.WorkspacesDir)
	if _, err := os.Stat(wsDir); err != nil {
		return nil
	}
	knDirs := knownBundleDirs()
	var results []workspaceMCPMeta
	filepath.WalkDir(wsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "mcp-meta.json" {
			return nil
		}
		dir := filepath.Dir(path)
		dirName := filepath.Base(dir)
		if knDirs[dirName] {
			return nil
		}
		meta, err := ReadWorkspaceMCPMeta(dir)
		if err != nil {
			return nil
		}
		results = append(results, workspaceMCPMeta{DirName: dirName, Meta: meta})
		return nil
	})
	return results
}

// WorkspaceMCPEnvVarKeys returns env var keys defined by workspace MCP servers.
// Reads mcp-meta.json from steer-runtime workspace dirs (not ~/.kiro/tools/).
func WorkspaceMCPEnvVarKeys(steerRoot string) []string {
	seen := make(map[string]bool)
	var keys []string
	for _, wm := range walkWorkspaceMCPMetas(steerRoot) {
		for k := range wm.Meta.Env {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

// knownBundleDirs returns a set of bundle directory names from the built-in server registry.
func knownBundleDirs() map[string]bool {
	dirs := make(map[string]bool, len(knownServers))
	for _, srv := range knownServers {
		if srv.BundleDir != "" {
			dirs[srv.BundleDir] = true
		}
	}
	return dirs
}

// RequiredTokens returns the deduplicated list of tokens required by the
// selected servers, preserving first-appearance order.
func RequiredTokens(selected []MCPServer) []model.Token {
	knownMap := make(map[string]model.Token, len(model.KnownTokens))
	for _, t := range model.KnownTokens {
		knownMap[t.Key] = t
	}

	seen := make(map[string]bool)
	var result []model.Token
	for _, srv := range selected {
		for _, k := range srv.TokenKeys {
			if seen[k] {
				continue
			}
			seen[k] = true
			if tok, ok := knownMap[k]; ok {
				result = append(result, tok)
			}
		}
	}
	return result
}

// HasExistingMCPConfig returns true if ~/.kiro/settings/mcp.json already exists.
func HasExistingMCPConfig() bool {
	home, _ := os.UserHomeDir()
	mcpPath := filepath.Join(home, ".kiro", config.SettingsDir, "mcp.json")
	_, err := os.Stat(mcpPath)
	return err == nil
}

// GenerateMCPConfig builds and writes mcp.json for the given selected servers.
// ghRemotes provides GitHub remote configs; tokens and envVars supply credentials.
// Returns the path to the written file.
func GenerateMCPConfig(selected []MCPServer, ghRemotes []model.GitHubRemote,
	jiraInstances []model.JiraInstance, confInstances []model.ConfluenceInstance,
	tokens map[string]string, envVars map[string]string) (string, error) {

	home, _ := os.UserHomeDir()
	bundleDir := filepath.Join(home, ".kiro", "tools", "mcp-servers")

	type mcpServer struct {
		Command string            `json:"command,omitempty"`
		Args    []string          `json:"args,omitempty"`
		Env     map[string]string `json:"env,omitempty"`
		URL     string            `json:"url,omitempty"`
		Type    string            `json:"type,omitempty"`
		Headers map[string]string `json:"headers,omitempty"`
	}

	servers := make(map[string]mcpServer)

	for _, srv := range selected {
		switch {
		case srv.IsSSE:
			// Compass: only include if token is non-empty.
			ct := tokens["COMPASS_TOKEN"]
			if ct == "" {
				continue
			}
			servers[srv.Name] = mcpServer{
				URL:     envVars["COMPASS_URL"],
				Type:    "sse",
				Headers: map[string]string{"Authorization": "Bearer " + ct},
			}

		case srv.IsNPM:
			// context7: npx-based.
			servers[srv.Name] = mcpServer{
				Command: "npx",
				Args:    []string{"-y", "@upstash/context7-mcp"},
			}

		case srv.Name == "jira":
			// Jira: per-instance entries (same pattern as GitHub).
			jiraBundle := filepath.Join(bundleDir, "jira-mcp", "dist", "index.cjs")
			if len(jiraInstances) == 1 {
				servers["jira"] = mcpServer{Command: "node", Args: []string{jiraBundle},
					Env: map[string]string{"JIRA_PAT": jiraInstances[0].Token, "JIRA_URL": jiraInstances[0].URL}}
			} else {
				for _, inst := range jiraInstances {
					servers["jira-"+inst.Name] = mcpServer{Command: "node", Args: []string{jiraBundle},
						Env: map[string]string{"JIRA_PAT": inst.Token, "JIRA_URL": inst.URL}}
				}
			}

		case srv.Name == "confluence":
			// Confluence: per-instance entries (same pattern as GitHub).
			confBundle := filepath.Join(bundleDir, "confluence-mcp", "dist", "index.cjs")
			if len(confInstances) == 1 {
				servers["confluence"] = mcpServer{Command: "node", Args: []string{confBundle},
					Env: map[string]string{"CONFLUENCE_PAT": confInstances[0].Token, "CONFLUENCE_URL": confInstances[0].URL}}
			} else {
				for _, inst := range confInstances {
					servers["confluence-"+inst.Name] = mcpServer{Command: "node", Args: []string{confBundle},
						Env: map[string]string{"CONFLUENCE_PAT": inst.Token, "CONFLUENCE_URL": inst.URL}}
				}
			}

		case srv.Name == "github":
			// GitHub: handled separately based on remote count.
			ghBundle := filepath.Join(bundleDir, "github-mcp", "dist", "index.cjs")
			if len(ghRemotes) == 1 {
				env := map[string]string{
					"GITHUB_REMOTE": ghRemotes[0].Name,
					"GITHUB_HOST":   ghRemotes[0].Host,
					"GITHUB_TOKEN":  ghRemotes[0].Token,
				}
				if ghRemotes[0].APIPath != "" {
					env["GITHUB_API_PATH"] = ghRemotes[0].APIPath
				}
				servers["github"] = mcpServer{Command: "node", Args: []string{ghBundle}, Env: env}
			} else {
				for _, r := range ghRemotes {
					env := map[string]string{
						"GITHUB_REMOTE": r.Name,
						"GITHUB_HOST":   r.Host,
						"GITHUB_TOKEN":  r.Token,
					}
					if r.APIPath != "" {
						env["GITHUB_API_PATH"] = r.APIPath
					}
					servers["github-"+r.Name] = mcpServer{Command: "node", Args: []string{ghBundle}, Env: env}
				}
			}

		default:
			// Regular node-based server.
			cmd := "node"
			if srv.Command != "" {
				cmd = srv.Command
			}
			entry := mcpServer{
				Command: cmd,
				Args:    []string{filepath.Join(bundleDir, srv.BundleDir, "dist", "index.cjs")},
			}
			// Build env from TokenKeys and EnvKeys.
			if len(srv.TokenKeys) > 0 || len(srv.EnvKeys) > 0 {
				env := make(map[string]string)
				for _, tk := range srv.TokenKeys {
					env[tk] = tokens[tk]
				}
				for _, ek := range srv.EnvKeys {
					env[ek] = envVars[ek]
				}
				entry.Env = env
			}
			servers[srv.Name] = entry
		}
	}

	// Use sorted keys for deterministic output (idempotence).
	sortedKeys := make([]string, 0, len(servers))
	for k := range servers {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	orderedServers := make([]orderedEntry, 0, len(servers))
	for _, k := range sortedKeys {
		orderedServers = append(orderedServers, orderedEntry{Key: k, Value: servers[k]})
	}

	mcpConfig := orderedMCPConfig{MCPServers: orderedServers}

	settingsDir := filepath.Join(home, ".kiro", config.SettingsDir)
	if err := os.MkdirAll(settingsDir, 0755); err != nil {
		return "", fmt.Errorf("cannot create settings directory: %w", err)
	}
	mcpPath := filepath.Join(settingsDir, "mcp.json")
	out, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return "", fmt.Errorf("cannot marshal config: %w", err)
	}
	if err := os.WriteFile(mcpPath, append(out, '\n'), 0644); err != nil {
		return "", fmt.Errorf("cannot write mcp.json: %w", err)
	}
	return mcpPath, nil
}

// orderedEntry holds a key-value pair for deterministic JSON output.
type orderedEntry struct {
	Key   string
	Value interface{}
}

// orderedMCPConfig wraps the mcpServers map for deterministic JSON marshalling.
type orderedMCPConfig struct {
	MCPServers []orderedEntry
}

// MarshalJSON produces {"mcpServers": {...}} with keys in sorted order.
func (c orderedMCPConfig) MarshalJSON() ([]byte, error) {
	inner := make([]byte, 0, 256)
	inner = append(inner, '{')
	for i, e := range c.MCPServers {
		if i > 0 {
			inner = append(inner, ',')
		}
		keyBytes, _ := json.Marshal(e.Key)
		valBytes, err := json.Marshal(e.Value)
		if err != nil {
			return nil, err
		}
		inner = append(inner, keyBytes...)
		inner = append(inner, ':')
		inner = append(inner, valBytes...)
	}
	inner = append(inner, '}')

	wrapper := map[string]json.RawMessage{"mcpServers": inner}
	return json.Marshal(wrapper)
}

// WriteProfilesManifest writes settings/profiles.json to targetDir.
func WriteProfilesManifest(steerRoot, targetDir string) error {
	profiles, err := ListProfiles(steerRoot, targetDir)
	if err != nil {
		return err
	}

	type manifestProfile struct {
		ID         string   `json:"id"`
		Agents     []string `json:"agents"`
		AgentCount int      `json:"agent_count"`
		Installed  bool     `json:"installed"`
	}

	var mProfiles []manifestProfile
	for _, p := range profiles {
		var agentNames []string
		for _, a := range p.Agents {
			agentNames = append(agentNames, a.Name)
		}
		mProfiles = append(mProfiles, manifestProfile{
			ID:         p.ID,
			Agents:     agentNames,
			AgentCount: p.AgentCount,
			Installed:  p.Installed,
		})
	}

	manifest := map[string]any{
		"steer_root": steerRoot,
		"profiles":   mProfiles,
	}

	settingsDir := filepath.Join(targetDir, config.SettingsDir)
	os.MkdirAll(settingsDir, 0755)

	out, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(settingsDir, "profiles.json"), append(out, '\n'), 0644)
}
