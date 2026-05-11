package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// MCPServer describes an available MCP server and its requirements.
type MCPServer struct {
	Name       string   // display name (e.g., "jira", "confluence")
	BundleDir  string   // directory name under mcp-servers/ (e.g., "jira-mcp")
	TokenKeys  []string // required token keys from KnownTokens (e.g., ["JIRA_PAT"])
	EnvKeys    []string // required env var keys (e.g., ["CONFLUENCE_URL"])
	Command    string   // override command (default: "node"); set from mcp-meta.json
	IsNPM      bool     // true for context7 (npm install required)
	IsSSE      bool     // true for compass (SSE transport)
	IsNPX      bool     // true for npx-based servers (no bundle, no npm install)
	NPXPackage string   // npm package spec for npx (e.g., "@anthropic-ai/chrome-devtools-mcp@latest")
	DisabledByDefault bool // true for servers that need external processes (e.g., chrome-devtools needs Chrome running)
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
	{Name: "qtest", BundleDir: "qtest-mcp", TokenKeys: []string{"QTEST_BEARER_TOKEN"}, EnvKeys: []string{"QTEST_BASE_URL", "QTEST_PROJECT_ID"}},
	{Name: "splunk-mcp", BundleDir: "splunk-mcp", TokenKeys: []string{"SPLUNK_API_USERNAME", "SPLUNK_API_PASSWORD"}, EnvKeys: []string{"SPLUNK_BASE_URL"}},
	{Name: "appdynamics-mcp", BundleDir: "appdynamics-mcp", TokenKeys: []string{"APPD_CLIENT_ID", "APPD_CLIENT_SECRET"}, EnvKeys: []string{"APPD_CONTROLLER_URL"}},
{Name: "servicenow-mcp", BundleDir: "servicenow-mcp", TokenKeys: []string{"SNOW_API_USERNAME", "SNOW_API_PASSWORD"}, EnvKeys: []string{"SNOW_INSTANCE"}},
	{Name: "chrome", BundleDir: "chrome-mcp"},
	{Name: "chrome-devtools", IsNPX: true, NPXPackage: "@anthropic-ai/chrome-devtools-mcp@latest", DisabledByDefault: true},
	{Name: "sharepoint", BundleDir: "sharepoint-mcp", TokenKeys: []string{"SHAREPOINT_CLIENT_ID", "SHAREPOINT_CLIENT_SECRET"}, EnvKeys: []string{"SHAREPOINT_TENANT_ID", "SHAREPOINT_SITE_URL"}},
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
	// Build set of source bundles
	srcSet := make(map[string]bool)
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		srcSet[e.Name()] = true
		bundle := filepath.Join(srcDir, e.Name(), "dist", "index.cjs")
		if _, err := os.Stat(bundle); err == nil {
			dstDir := filepath.Join(dstBase, e.Name())
			os.RemoveAll(dstDir) // clean stale files from previous installs
			dst := filepath.Join(dstDir, "dist")
			os.MkdirAll(dst, 0755)
			copyFile(bundle, filepath.Join(dst, "index.cjs"))
			count++
		}
	}

	// Clean up stale bundles not in source
	if dstEntries, err := os.ReadDir(dstBase); err == nil {
		for _, e := range dstEntries {
			if e.IsDir() && !srcSet[e.Name()] {
				os.RemoveAll(filepath.Join(dstBase, e.Name()))
			}
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

	// Read workspace-level Jira custom fields (if active workspace defines them)
	wsCustomFields := map[string]string{}
	steerRoot := filepath.Join(home, ".kiro", "steer-runtime")
	if s := config.ReadSteerSettings(); s.ActiveWorkspace != "" {
		if ws, err := GetWorkspace(steerRoot, s.ActiveWorkspace); err == nil {
			wsCustomFields = ws.JiraCustomFields
		}
	}

	type mcpServer struct {
		Command  string            `json:"command,omitempty"`
		Args     []string          `json:"args,omitempty"`
		Env      map[string]string `json:"env,omitempty"`
		URL      string            `json:"url,omitempty"`
		Type     string            `json:"type,omitempty"`
		Headers  map[string]string `json:"headers,omitempty"`
		Disabled bool              `json:"disabled,omitempty"`
		Source   string            `json:"_source,omitempty"`
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
		"chrome": {
			Command: nodeExe,
			Args:    []string{filepath.Join(bundleDir, "chrome-mcp", "dist", "index.cjs")},
		},
		"chrome-devtools": {
			Command:  filepath.Join(home, ".kiro", "hooks", "chrome-devtools-mcp.sh"),
			Disabled: true,
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
		env := map[string]string{"JIRA_PAT": jiraInstances[0].Token, "JIRA_URL": jiraInstances[0].URL}
		if jiraInstances[0].Email != "" {
			env["JIRA_EMAIL"] = jiraInstances[0].Email
		}
		if cf := jiraInstances[0].CustomFields; cf != "" {
			env["JIRA_CUSTOM_FIELDS"] = cf
		} else if cf := wsCustomFields[jiraInstances[0].Name]; cf != "" {
			env["JIRA_CUSTOM_FIELDS"] = cf
		} else if cf := tokens["JIRA_CUSTOM_FIELDS_"+jiraInstances[0].Name]; cf != "" {
			env["JIRA_CUSTOM_FIELDS"] = cf
		} else if cf := envVars["JIRA_CUSTOM_FIELDS"]; cf != "" {
			env["JIRA_CUSTOM_FIELDS"] = cf
		}
		servers["jira"] = mcpServer{Command: nodeExe, Args: []string{jiraBundle}, Env: env}
	} else {
		for _, inst := range jiraInstances {
			env := map[string]string{"JIRA_PAT": inst.Token, "JIRA_URL": inst.URL, "JIRA_INSTANCE_PREFIX": inst.Name + "_"}
			if inst.Email != "" {
				env["JIRA_EMAIL"] = inst.Email
			}
			if cf := inst.CustomFields; cf != "" {
				env["JIRA_CUSTOM_FIELDS"] = cf
			} else if cf := wsCustomFields[inst.Name]; cf != "" {
				env["JIRA_CUSTOM_FIELDS"] = cf
			} else if cf := tokens["JIRA_CUSTOM_FIELDS_"+inst.Name]; cf != "" {
				env["JIRA_CUSTOM_FIELDS"] = cf
			} else if cf := envVars["JIRA_CUSTOM_FIELDS"]; cf != "" {
				env["JIRA_CUSTOM_FIELDS"] = cf
			}
			servers["jira-"+inst.Name] = mcpServer{Command: nodeExe, Args: []string{jiraBundle}, Env: env}
		}
	}

	// Confluence: per-instance entries
	confInstances := ReadConfluenceInstances()
	confBundle := filepath.Join(bundleDir, "confluence-mcp", "dist", "index.cjs")
	if len(confInstances) == 1 {
		servers["confluence"] = mcpServer{Command: nodeExe, Args: []string{confBundle},
			Env: map[string]string{"CONFLUENCE_INSTANCE_PREFIX": confInstances[0].Name + "_", "CONFLUENCE_PAT": confInstances[0].Token, "CONFLUENCE_URL": confInstances[0].URL}}
	} else {
		for _, inst := range confInstances {
			servers["confluence-"+inst.Name] = mcpServer{Command: nodeExe, Args: []string{confBundle},
				Env: map[string]string{"CONFLUENCE_INSTANCE_PREFIX": inst.Name + "_", "CONFLUENCE_PAT": inst.Token, "CONFLUENCE_URL": inst.URL}}

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

	// qTest: test management MCP
	if qt := tokens["QTEST_BEARER_TOKEN"]; qt != "" {
		qtestBundle := filepath.Join(bundleDir, "qtest-mcp", "dist", "index.cjs")
		if _, err := os.Stat(qtestBundle); err == nil {
			env := map[string]string{"QTEST_BEARER_TOKEN": qt}
			if u := envVars["QTEST_BASE_URL"]; u != "" {
				env["QTEST_BASE_URL"] = u
			}
			if p := envVars["QTEST_PROJECT_ID"]; p != "" {
				env["QTEST_PROJECT_ID"] = p
			}
			servers["qtest"] = mcpServer{Command: nodeExe, Args: []string{qtestBundle}, Env: env}
		}
	}

	// SharePoint: conditional on Azure AD credentials
	if spClient := tokens["SHAREPOINT_CLIENT_ID"]; spClient != "" {
		spBundle := filepath.Join(bundleDir, "sharepoint-mcp", "dist", "index.cjs")
		if _, err := os.Stat(spBundle); err == nil {
			env := map[string]string{"SHAREPOINT_CLIENT_ID": spClient}
			if v := tokens["SHAREPOINT_CLIENT_SECRET"]; v != "" {
				env["SHAREPOINT_CLIENT_SECRET"] = v
			}
			if v := envVars["SHAREPOINT_TENANT_ID"]; v != "" {
				env["SHAREPOINT_TENANT_ID"] = v
			}
			if v := envVars["SHAREPOINT_SITE_URL"]; v != "" {
				env["SHAREPOINT_SITE_URL"] = v
			}
			servers["sharepoint"] = mcpServer{Command: nodeExe, Args: []string{spBundle}, Env: env}
		}
	}

	// yax: persistent memory via stdio MCP
	if yaxBin := findYax(); yaxBin != "" {
		servers["yax"] = mcpServer{Command: yaxBin, Args: []string{"mcp", "--tools=agent"}}
		fmt.Println("  ✓ yax (persistent memory)")
	}

	// Workspace MCP servers (from steer-runtime mcp-meta.json)
	steerRoot = filepath.Join(home, ".kiro", "steer-runtime")
	for _, wm := range walkWorkspaceMCPMetas(steerRoot) {
		bundle := filepath.Join(bundleDir, wm.DirName, "dist", "index.cjs")
		if _, err := os.Stat(bundle); err != nil {
			continue
		}
		cmd := nodeExe
		if wm.Meta.Command != "" {
			cmd = wm.Meta.Command
		}
		entry := mcpServer{Command: cmd, Args: []string{bundle}, Source: "fork"}
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

	// Workspace-level MCPs (from workspaces/<active>/mcp/mcp.json)
	if s := config.ReadSteerSettings(); s.ActiveWorkspace != "" {
		if wsCfg, err := readWorkspaceMcpConfig(steerRoot, s.ActiveWorkspace); err == nil && wsCfg != nil {
			wsDefaults := readWorkspaceDefaultsEnv(steerRoot, s.ActiveWorkspace)
			pathVars := map[string]string{
				"KIRO_MCP_DIR":      filepath.Join(home, ".kiro", "tools", "mcp-servers"),
				"WORKSPACE_MCP_DIR": filepath.Join(steerRoot, config.WorkspacesDir, s.ActiveWorkspace, "mcp"),
				"KIRO_ROOT":         filepath.Join(home, ".kiro"),
				"WORKSPACE_NAME":    s.ActiveWorkspace,
			}
			for srvName, srvDef := range wsCfg.McpServers {
				// Handle _overrides: remove the global server being replaced
				if srvDef.Overrides != "" {
					delete(servers, srvDef.Overrides)
				}
				// Resolve variables in args
				resolvedArgs := make([]string, len(srvDef.Args))
				for i, arg := range srvDef.Args {
					resolvedArgs[i] = resolveVariables(arg, tokens, wsDefaults, wsCfg.Variables, pathVars)
				}
				// Resolve variables in env
				resolvedEnv := make(map[string]string, len(srvDef.Env))
				for k, v := range srvDef.Env {
					resolvedEnv[k] = resolveVariables(v, tokens, wsDefaults, wsCfg.Variables, pathVars)
				}
				cmd := srvDef.Command
				if cmd == "" {
					cmd = nodeExe
				}
				servers[srvName] = mcpServer{
					Command: cmd,
					Args:    resolvedArgs,
					Env:     resolvedEnv,
					URL:     srvDef.URL,
					Type:    srvDef.Type,
					Source:  "workspace:" + s.ActiveWorkspace,
				}
			}
		}
	}

	// Tag all servers built above as "global" if not already tagged
	for name, srv := range servers {
		if srv.Source == "" {
			srv.Source = "global"
			servers[name] = srv
		}
	}

	// Snapshot user customizations (disabled, autoApprove) before overwriting.
	priorState := readExistingMCPUserState()
	// Read user-added servers to preserve after writing
	userServers := readUserAddedServers()

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
	if err := os.WriteFile(mcpPath, append(out, '\n'), 0644); err != nil {
		return err
	}
	// Merge user-added servers back into the written mcp.json
	if len(userServers) > 0 {
		mergeUserServersIntoJSON(mcpPath, userServers)
	}
	// Restore user customizations for servers that still exist.
	if err := mergeUserStateIntoJSON(mcpPath, priorState); err != nil {
		return err
	}
	return applyOverridesToMCPJson()
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

		if srv.IsNPX {
			// NPX servers (chrome-devtools) need no local bundle.
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

// --- Workspace-level MCP config (workspaces/<name>/mcp/mcp.json) ---

// WorkspaceMcpConfig represents workspaces/<name>/mcp/mcp.json
type WorkspaceMcpConfig struct {
	McpServers map[string]WorkspaceMcpServerDef `json:"mcpServers"`
	Variables  map[string]VariableDecl          `json:"variables"`
}

// WorkspaceMcpServerDef is a server definition with ${VAR} placeholders.
type WorkspaceMcpServerDef struct {
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	URL       string            `json:"url,omitempty"`
	Type      string            `json:"type,omitempty"`
	Headers   map[string]string `json:"headers,omitempty"`
	Overrides string            `json:"_overrides,omitempty"`
}

// VariableDecl declares a variable needed by a workspace MCP.
type VariableDecl struct {
	Description string `json:"description"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
	Default     string `json:"default,omitempty"`
}

// readWorkspaceMcpConfig reads workspaces/<wsName>/mcp/mcp.json if it exists.
func readWorkspaceMcpConfig(steerRoot, wsName string) (*WorkspaceMcpConfig, error) {
	mcpPath := filepath.Join(steerRoot, config.WorkspacesDir, wsName, "mcp", "mcp.json")
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		return nil, err
	}
	var cfg WorkspaceMcpConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// readWorkspaceDefaultsEnv reads workspaces/<wsName>/mcp/defaults.env.
// Returns a map of KEY=VALUE pairs.
func readWorkspaceDefaultsEnv(steerRoot, wsName string) map[string]string {
	envPath := filepath.Join(steerRoot, config.WorkspacesDir, wsName, "mcp", "defaults.env")
	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil
	}
	result := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result
}

// resolveVariables substitutes ${VAR} references in a string using 3-tier lookup:
// 1. tokens (from tokens.env)
// 2. defaults (from defaults.env)
// 3. declarations (variable.default)
// Also resolves built-in path variables.
func resolveVariables(value string, tokens, defaults map[string]string, declarations map[string]VariableDecl, pathVars map[string]string) string {
	// Resolve built-in path variables first
	for k, v := range pathVars {
		value = strings.ReplaceAll(value, "${"+k+"}", v)
	}
	// Resolve declared variables
	for varName := range declarations {
		placeholder := "${" + varName + "}"
		if !strings.Contains(value, placeholder) {
			continue
		}
		resolved := ""
		if v, ok := tokens[varName]; ok && v != "" {
			resolved = v
		} else if v, ok := defaults[varName]; ok && v != "" {
			resolved = v
		} else if declarations[varName].Default != "" {
			resolved = declarations[varName].Default
		}
		value = strings.ReplaceAll(value, placeholder, resolved)
	}
	// Resolve any remaining ${VAR} from tokens directly
	for strings.Contains(value, "${") {
		start := strings.Index(value, "${")
		end := strings.Index(value[start:], "}")
		if end < 0 {
			break
		}
		varName := value[start+2 : start+end]
		resolved := ""
		if v, ok := tokens[varName]; ok {
			resolved = v
		} else if v, ok := defaults[varName]; ok {
			resolved = v
		}
		value = strings.ReplaceAll(value, "${"+varName+"}", resolved)
	}
	return value
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
		Command  string            `json:"command,omitempty"`
		Args     []string          `json:"args,omitempty"`
		Env      map[string]string `json:"env,omitempty"`
		URL      string            `json:"url,omitempty"`
		Type     string            `json:"type,omitempty"`
		Headers  map[string]string `json:"headers,omitempty"`
		Disabled bool              `json:"disabled,omitempty"`
		Source   string            `json:"_source,omitempty"`
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

		case srv.IsNPX:
			// NPX-based servers (no bundle, no npm install).
			if srv.DisabledByDefault {
				// Server needs an external process — use wrapper script that launches it first.
				servers[srv.Name] = mcpServer{
					Command:  filepath.Join(home, ".kiro", "hooks", srv.Name+"-mcp.sh"),
					Disabled: true,
				}
			} else {
				servers[srv.Name] = mcpServer{
					Command: "npx",
					Args:    []string{"-y", srv.NPXPackage},
				}
			}

		case srv.Name == "jira":
			jiraBundle := filepath.Join(bundleDir, "jira-mcp", "dist", "index.cjs")
			if len(jiraInstances) == 1 {
				servers["jira"] = mcpServer{Command: "node", Args: []string{jiraBundle},
					Env: map[string]string{"JIRA_PAT": jiraInstances[0].Token, "JIRA_URL": jiraInstances[0].URL}}
			} else {
				for _, inst := range jiraInstances {
					entry := mcpServer{Command: "node", Args: []string{jiraBundle},
						Env: map[string]string{"JIRA_PAT": inst.Token, "JIRA_URL": inst.URL, "JIRA_INSTANCE_PREFIX": inst.Name + "_"}}
					servers["jira-"+inst.Name] = entry
				}
			}

		case srv.Name == "confluence":
			confBundle := filepath.Join(bundleDir, "confluence-mcp", "dist", "index.cjs")
			if len(confInstances) == 1 {
				servers["confluence"] = mcpServer{Command: "node", Args: []string{confBundle},
					Env: map[string]string{"CONFLUENCE_INSTANCE_PREFIX": confInstances[0].Name + "_", "CONFLUENCE_PAT": confInstances[0].Token, "CONFLUENCE_URL": confInstances[0].URL}}
			} else {
				for _, inst := range confInstances {
					entry := mcpServer{Command: "node", Args: []string{confBundle},
						Env: map[string]string{"CONFLUENCE_INSTANCE_PREFIX": inst.Name + "_", "CONFLUENCE_PAT": inst.Token, "CONFLUENCE_URL": inst.URL}}
					servers["confluence-"+inst.Name] = entry
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
			// Regular node-based server — skip if no required tokens are set.
			if len(srv.TokenKeys) > 0 {
				hasToken := false
				for _, tk := range srv.TokenKeys {
					if tokens[tk] != "" { hasToken = true; break }
				}
				if !hasToken { continue }
			}
			cmd := "node"
			if srv.Command != "" {
				cmd = srv.Command
			}
			entry := mcpServer{
				Command: cmd,
				Args:    []string{filepath.Join(bundleDir, srv.BundleDir, "dist", "index.cjs")},
			}
			if len(srv.TokenKeys) > 0 || len(srv.EnvKeys) > 0 {
				env := make(map[string]string)
				for _, tk := range srv.TokenKeys {
					if v := tokens[tk]; v != "" { env[tk] = v }
				}
				for _, ek := range srv.EnvKeys {
					if v := envVars[ek]; v != "" { env[ek] = v }
				}
				entry.Env = env
			}
			servers[srv.Name] = entry
		}
	}

	// yax: persistent memory via stdio MCP
	if yaxBin := findYax(); yaxBin != "" {
		servers["yax"] = mcpServer{Command: yaxBin, Args: []string{"mcp", "--tools=agent"}}
	}

	// Snapshot user customizations (disabled, autoApprove) before overwriting.
	priorState := readExistingMCPUserState()

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
	// Restore user customizations for servers that still exist.
	mergeUserStateIntoJSON(mcpPath, priorState)
	applyOverridesToMCPJson()
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
		Workspace  string   `json:"workspace,omitempty"`
		SourceDir  string   `json:"source_dir,omitempty"`
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
			Workspace:  p.WorkspaceName,
			SourceDir:  p.SourceDir,
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

// --- Preserve user customizations across regeneration ---

// existingServerState holds user-customizable fields from an existing mcp.json entry.
type existingServerState struct {
	Disabled    bool
	AutoApprove []string
}

// readExistingMCPUserState reads the current mcp.json (if any) and returns
// a map of server name → user-customizable state (disabled, autoApprove).
// Returns an empty map if the file doesn't exist or can't be parsed.
func readExistingMCPUserState() map[string]existingServerState {
	home, _ := os.UserHomeDir()
	mcpPath := filepath.Join(home, ".kiro", config.SettingsDir, "mcp.json")
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		return nil
	}
	var parsed struct {
		MCPServers map[string]struct {
			Disabled    bool     `json:"disabled"`
			AutoApprove []string `json:"autoApprove"`
		} `json:"mcpServers"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return nil
	}
	result := make(map[string]existingServerState, len(parsed.MCPServers))
	for name, srv := range parsed.MCPServers {
		if srv.Disabled || len(srv.AutoApprove) > 0 {
			result[name] = existingServerState{
				Disabled:    srv.Disabled,
				AutoApprove: srv.AutoApprove,
			}
		}
	}
	return result
}

// readUserAddedServers reads the existing mcp.json and returns servers that
// were added by the user (not managed by Koda). These are identified by having
// _source: "user" or no _source field and not matching any known server name.
// Returns raw JSON entries to avoid depending on the local mcpServer struct.
func readUserAddedServers() map[string]json.RawMessage {
	home, _ := os.UserHomeDir()
	mcpPath := filepath.Join(home, ".kiro", config.SettingsDir, "mcp.json")
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		return nil
	}
	var parsed struct {
		MCPServers map[string]json.RawMessage `json:"mcpServers"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return nil
	}

	// Build set of known server name prefixes
	knownPrefixes := []string{"jira", "confluence", "github", "mermaid", "bruno",
		"splunk-mcp", "appdynamics-mcp", "servicenow-mcp", "qtest", "compass",
		"sharepoint", "chrome", "chrome-devtools", "fetch", "yax", "figma", "memory"}

	isManaged := func(name string) bool {
		for _, prefix := range knownPrefixes {
			if name == prefix || (len(name) > len(prefix)+1 && name[:len(prefix)+1] == prefix+"-") {
				return true
			}
		}
		return false
	}

	result := make(map[string]json.RawMessage)
	for name, raw := range parsed.MCPServers {
		var srv struct {
			Source string `json:"_source"`
		}
		json.Unmarshal(raw, &srv)

		// User server: explicitly tagged as "user" OR no source and not a known prefix
		if srv.Source == "user" || (srv.Source == "" && !isManaged(name)) {
			result[name] = raw
		}
	}
	return result
}

// mergeUserServersIntoJSON reads the written mcp.json and adds user-added
// servers that were preserved from the previous config.
func mergeUserServersIntoJSON(mcpPath string, userServers map[string]json.RawMessage) {
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		return
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return
	}
	var servers map[string]json.RawMessage
	if json.Unmarshal(raw["mcpServers"], &servers) != nil {
		return
	}
	for name, srv := range userServers {
		if _, exists := servers[name]; !exists {
			servers[name] = srv
		}
	}
	serversJSON, _ := json.Marshal(servers)
	raw["mcpServers"] = serversJSON
	out, _ := json.MarshalIndent(raw, "", "  ")
	os.WriteFile(mcpPath, append(out, '\n'), 0644)
}

// mergeUserStateIntoJSON reads the written mcp.json, re-applies preserved
// disabled and autoApprove fields for servers that still exist, and writes back.
// Servers not present in prior are left untouched (new servers default to enabled).
func mergeUserStateIntoJSON(mcpPath string, prior map[string]existingServerState) error {
	if len(prior) == 0 {
		return nil
	}
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return fmt.Errorf("cannot parse mcp.json for merge")
	}
	var servers map[string]map[string]any
	if json.Unmarshal(raw["mcpServers"], &servers) != nil {
		return fmt.Errorf("cannot parse mcpServers for merge")
	}
	changed := false
	for name, state := range prior {
		srv, ok := servers[name]
		if !ok {
			continue // server was removed, don't resurrect it
		}
		if state.Disabled {
			srv["disabled"] = true
			changed = true
		}
		if len(state.AutoApprove) > 0 {
			srv["autoApprove"] = state.AutoApprove
			changed = true
		}
	}
	if !changed {
		return nil
	}
	serversJSON, _ := json.Marshal(servers)
	raw["mcpServers"] = serversJSON
	out, _ := json.MarshalIndent(raw, "", "  ")
	return os.WriteFile(mcpPath, append(out, '\n'), 0644)
}

// --- MCP Overrides (user-level enable/disable) ---

const mcpOverridesFile = "mcp-overrides.json"

// MCPOverride holds a per-server user override.
type MCPOverride struct {
	Disabled bool `json:"disabled"`
}

// ReadMCPOverrides reads ~/.kiro/settings/mcp-overrides.json.
func ReadMCPOverrides() map[string]MCPOverride {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".kiro", config.SettingsDir, mcpOverridesFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return map[string]MCPOverride{}
	}
	var overrides map[string]MCPOverride
	if json.Unmarshal(data, &overrides) != nil {
		return map[string]MCPOverride{}
	}
	return overrides
}

// WriteMCPOverrides writes ~/.kiro/settings/mcp-overrides.json.
func WriteMCPOverrides(overrides map[string]MCPOverride) error {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".kiro", config.SettingsDir)
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(overrides, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, mcpOverridesFile), append(data, '\n'), 0644)
}

// ToggleMCPServer sets or clears the disabled flag for a server.
func ToggleMCPServer(name string, disabled bool) error {
	overrides := ReadMCPOverrides()
	if disabled {
		overrides[name] = MCPOverride{Disabled: true}
	} else {
		delete(overrides, name)
	}
	if err := WriteMCPOverrides(overrides); err != nil {
		return err
	}
	return applyOverridesToMCPJson()
}

// applyOverridesToMCPJson reads mcp.json, applies overrides, writes back.
func applyOverridesToMCPJson() error {
	home, _ := os.UserHomeDir()
	mcpPath := filepath.Join(home, ".kiro", config.SettingsDir, "mcp.json")
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return fmt.Errorf("cannot parse mcp.json")
	}
	var servers map[string]map[string]any
	if json.Unmarshal(raw["mcpServers"], &servers) != nil {
		return fmt.Errorf("cannot parse mcpServers")
	}
	overrides := ReadMCPOverrides()
	for name, ov := range overrides {
		if srv, ok := servers[name]; ok {
			if ov.Disabled {
				srv["disabled"] = true
			} else {
				delete(srv, "disabled")
			}
		}
	}
	serversJSON, _ := json.Marshal(servers)
	raw["mcpServers"] = serversJSON
	out, _ := json.MarshalIndent(raw, "", "  ")
	return os.WriteFile(mcpPath, append(out, '\n'), 0644)
}

// ListMCPServers returns server names and their disabled status from mcp.json.
func ListMCPServers() ([]MCPServerStatus, error) {
	home, _ := os.UserHomeDir()
	mcpPath := filepath.Join(home, ".kiro", config.SettingsDir, "mcp.json")
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		MCPServers map[string]struct {
			Disabled bool `json:"disabled"`
		} `json:"mcpServers"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return nil, fmt.Errorf("cannot parse mcp.json")
	}
	var result []MCPServerStatus
	for name, srv := range parsed.MCPServers {
		result = append(result, MCPServerStatus{Name: name, Disabled: srv.Disabled})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// MCPServerStatus holds the name and disabled state of an MCP server.
type MCPServerStatus struct {
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
}

// MCPServerSourceStatus extends MCPServerStatus with source information.
type MCPServerSourceStatus struct {
	Name     string
	Source   string
	Disabled bool
}

// ListMCPServersBySource reads mcp.json and returns servers grouped by _source.
func ListMCPServersBySource() ([]MCPServerSourceStatus, error) {
	home, _ := os.UserHomeDir()
	mcpPath := filepath.Join(home, ".kiro", config.SettingsDir, "mcp.json")
	data, err := os.ReadFile(mcpPath)
	if err != nil {
		return nil, err
	}
	var parsed struct {
		MCPServers map[string]struct {
			Disabled bool   `json:"disabled"`
			Source   string `json:"_source"`
		} `json:"mcpServers"`
	}
	if json.Unmarshal(data, &parsed) != nil {
		return nil, fmt.Errorf("cannot parse mcp.json")
	}
	var result []MCPServerSourceStatus
	for name, srv := range parsed.MCPServers {
		result = append(result, MCPServerSourceStatus{Name: name, Source: srv.Source, Disabled: srv.Disabled})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Source != result[j].Source {
			return result[i].Source < result[j].Source
		}
		return result[i].Name < result[j].Name
	})
	return result, nil
}
