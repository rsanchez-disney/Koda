package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// MCPInstall verifies MCP bundles, installs context7, and generates mcp.json.
func MCPInstall(steerRoot, targetDir string) error {
	// 1. Verify pre-built bundles
	fmt.Println("\U0001f50d Verifying MCP server bundles...")
	mcpDir := filepath.Join(targetDir, config.ToolsDir, "mcp-servers")
	entries, err := os.ReadDir(mcpDir)
	if err != nil {
		return fmt.Errorf("no MCP servers found at %s", mcpDir)
	}
	var ready []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		bundle := filepath.Join(mcpDir, e.Name(), "dist", "index.cjs")
		if _, err := os.Stat(bundle); err == nil {
			ready = append(ready, e.Name())
			fmt.Printf("  \u2713 %s\n", e.Name())
		}
	}
	fmt.Printf("\n\u2705 %d MCP servers ready\n", len(ready))

	// 2. Install context7-mcp from public npm
	ctx7Dir := filepath.Join(mcpDir, "context7-mcp")
	if _, err := os.Stat(filepath.Join(ctx7Dir, "package.json")); err == nil {
		fmt.Println("\n\U0001f4e6 Installing context7-mcp from public registry...")
		cmd := exec.Command("npm", "install", "--registry", "https://registry.npmjs.org", "--silent")
		cmd.Dir = ctx7Dir
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("  \u26a0 context7: %s\n", strings.TrimSpace(string(out)))
		} else {
			fmt.Println("  \u2713 context7")
		}
	}

	// 3. Generate ~/.kiro/settings/mcp.json
	fmt.Println("\n\U0001f527 Generating mcp.json...")
	tokens := ReadTokens()
	envVars := ReadEnvVars()
	home, _ := os.UserHomeDir()

	type mcpServer struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}

	servers := map[string]mcpServer{
		"jira": {
			Command: "node",
			Args:    []string{filepath.Join(home, ".kiro", "tools", "mcp-servers", "jira-mcp", "dist", "index.cjs")},
			Env:     map[string]string{"JIRA_PAT": tokens["JIRA_PAT"]},
		},
		"confluence": {
			Command: "node",
			Args:    []string{filepath.Join(home, ".kiro", "tools", "mcp-servers", "confluence-mcp", "dist", "index.cjs")},
			Env:     map[string]string{"CONFLUENCE_URL": envVars["CONFLUENCE_URL"], "CONFLUENCE_PAT": tokens["CONFLUENCE_PAT"]},
		},
		"mermaid": {
			Command: "node",
			Args:    []string{filepath.Join(home, ".kiro", "tools", "mcp-servers", "mermaid-diagram-mcp", "dist", "index.cjs")},
		},
		"bruno": {
			Command: "node",
			Args:    []string{filepath.Join(home, ".kiro", "tools", "mcp-servers", "bruno-mcp", "dist", "index.cjs")},
		},
		"mywiki": {
			Command: "node",
			Args:    []string{filepath.Join(home, ".kiro", "tools", "mcp-servers", "mywiki-mcp", "dist", "index.cjs")},
			Env:     map[string]string{"CONFLUENCE_URL": envVars["MYWIKI_URL"], "CONFLUENCE_PAT": tokens["MYWIKI_PAT"]},
		},
		"figma": {
			Command: "node",
			Args:    []string{filepath.Join(home, ".kiro", "tools", "mcp-servers", "figma-mcp", "dist", "index.cjs")},
			Env:     map[string]string{"FIGMA_TOKEN": tokens["FIGMA_TOKEN"]},
		},
		"context7": {
			Command: "npx",
			Args:    []string{"-y", "@upstash/context7-mcp"},
		},
	}

	// GitHub: per-remote entries (single remote keeps "github" name for compat)
	ghRemotes := ReadGitHubRemotes()
	ghBundle := filepath.Join(home, ".kiro", "tools", "mcp-servers", "github-mcp", "dist", "index.cjs")
	if len(ghRemotes) == 1 {
		env := map[string]string{"GITHUB_REMOTE": ghRemotes[0].Name, "GITHUB_HOST": ghRemotes[0].Host, "GITHUB_TOKEN": ghRemotes[0].Token}
		if ghRemotes[0].APIPath != "" {
			env["GITHUB_API_PATH"] = ghRemotes[0].APIPath
		}
		servers["github"] = mcpServer{Command: "node", Args: []string{ghBundle}, Env: env}
	} else {
		for _, r := range ghRemotes {
			env := map[string]string{"GITHUB_REMOTE": r.Name, "GITHUB_HOST": r.Host, "GITHUB_TOKEN": r.Token}
			if r.APIPath != "" {
				env["GITHUB_API_PATH"] = r.APIPath
			}
			servers["github-"+r.Name] = mcpServer{Command: "node", Args: []string{ghBundle}, Env: env}
		}
	}

	mcpConfig := map[string]any{"mcpServers": servers}

	settingsDir := filepath.Join(home, ".kiro", config.SettingsDir)
	os.MkdirAll(settingsDir, 0755)
	mcpPath := filepath.Join(settingsDir, "mcp.json")

	out, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(mcpPath, append(out, '\n'), 0644); err != nil {
		return err
	}
	fmt.Printf("  \u2713 %s\n", mcpPath)

	// 4. Inject tokens into installed agents
	InjectAgentTokens(targetDir)

	fmt.Println("\n\u2705 MCP servers ready")
	return nil
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
