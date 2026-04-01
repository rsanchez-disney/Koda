package ops

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// ReadTokens reads key=value pairs from ~/.kiro/tokens.env.
func ReadTokens() map[string]string {
	tokens := map[string]string{}
	path := filepath.Join(config.KiroRoot(), config.TokensFile)
	f, err := os.Open(path)
	if err != nil {
		return tokens
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			tokens[k] = v
		}
	}
	return tokens
}

// WriteTokens writes tokens to ~/.kiro/tokens.env.
func WriteTokens(tokens map[string]string) error {
	path := filepath.Join(config.KiroRoot(), config.TokensFile)
	os.MkdirAll(filepath.Dir(path), 0755)

	var lines []string
	lines = append(lines, "# Koda Agent Tokens")
	for _, t := range model.KnownTokens {
		if v, ok := tokens[t.Key]; ok && v != "" {
			lines = append(lines, fmt.Sprintf("%s=%s", t.Key, v))
		}
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0600)
}

// InjectAgentTokens reads tokens.env and injects values into agent JSON mcpServers.
func InjectAgentTokens(targetDir string) error {
	tokens := ReadTokens()
	if len(tokens) == 0 {
		return nil
	}

	// Map MCP server name → env key
	injections := map[string]map[string]string{
		"jira":       {"JIRA_PAT": tokens["JIRA_PAT"]},
		"confluence": {"CONFLUENCE_PAT": tokens["CONFLUENCE_PAT"]},
		"github":     {"GITHUB_TOKEN_disney": tokens["GITHUB_TOKEN_disney"]},
		"mywiki":     {"CONFLUENCE_PAT": tokens["MYWIKI_PAT"]},
	}

	agentsDir := filepath.Join(targetDir, config.AgentsDir)
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), "._") {
			continue
		}
		path := filepath.Join(agentsDir, e.Name())
		injectTokensInFile(path, injections)
	}
	return nil
}

func injectTokensInFile(path string, injections map[string]map[string]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return
	}

	mcpRaw, ok := raw["mcpServers"]
	if !ok {
		return
	}

	var servers map[string]map[string]json.RawMessage
	if json.Unmarshal(mcpRaw, &servers) != nil {
		return
	}

	changed := false
	for mcpName, envMap := range injections {
		srv, ok := servers[mcpName]
		if !ok {
			continue
		}
		envRaw, ok := srv["env"]
		if !ok {
			continue
		}
		var env map[string]string
		if json.Unmarshal(envRaw, &env) != nil {
			continue
		}
		for k, v := range envMap {
			if _, exists := env[k]; exists && v != "" {
				env[k] = v
				changed = true
			}
		}
		if b, err := json.Marshal(env); err == nil {
			srv["env"] = b
		}
	}

	if changed {
		if b, err := json.Marshal(servers); err == nil {
			raw["mcpServers"] = b
		}
		if out, err := json.MarshalIndent(raw, "", "  "); err == nil {
			os.WriteFile(path, append(out, '\n'), 0644)
		}
	}
}

// MaskToken returns a masked version of a token for display.
func MaskToken(val string) string {
	if val == "" {
		return "not set"
	}
	if len(val) <= 10 {
		return "****"
	}
	return val[:6] + "..." + val[len(val)-4:]
}
