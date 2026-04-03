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

// ReadGitHubRemotes discovers GitHub remotes from tokens.env by scanning for GITHUB_TOKEN_* keys.
// Falls back to single remote from GITHUB_TOKEN + GITHUB_URL if no suffixed keys found.
func ReadGitHubRemotes() []model.GitHubRemote {
	tokens := ReadTokens()
	var remotes []model.GitHubRemote
	seen := map[string]bool{}

	for k, v := range tokens {
		if !strings.HasPrefix(k, "GITHUB_TOKEN_") || v == "" {
			continue
		}
		name := strings.TrimPrefix(k, "GITHUB_TOKEN_")
		host := tokens["GITHUB_HOST_"+name]
		if host == "" {
			continue
		}
		seen[name] = true
		remotes = append(remotes, model.GitHubRemote{
			Name:    name,
			Host:    host,
			Token:   v,
			APIPath: tokens["GITHUB_API_PATH_"+name],
		})
	}

	// Backward compat: single GITHUB_TOKEN → remote "disney"
	if len(remotes) == 0 {
		if tok := tokens["GITHUB_TOKEN"]; tok != "" {
			host := tokens["GITHUB_URL"]
			if host == "" {
				host = "https://github.disney.com"
			}
			// Strip https:// for host
			host = strings.TrimPrefix(host, "https://")
			host = strings.TrimPrefix(host, "http://")
			remotes = append(remotes, model.GitHubRemote{
				Name:  "disney",
				Host:  host,
				Token: tok,
			})
		}
	}

	return remotes
}

// WriteGitHubRemote adds or updates a GitHub remote in tokens.env.
func WriteGitHubRemote(r model.GitHubRemote) {
	tokens := ReadTokens()
	tokens["GITHUB_TOKEN_"+r.Name] = r.Token
	tokens["GITHUB_HOST_"+r.Name] = r.Host
	if r.APIPath != "" {
		tokens["GITHUB_API_PATH_"+r.Name] = r.APIPath
	}
	WriteTokens(tokens)
}

// RemoveGitHubRemote removes a GitHub remote from tokens.env.
func RemoveGitHubRemote(name string) {
	tokens := ReadTokens()
	delete(tokens, "GITHUB_TOKEN_"+name)
	delete(tokens, "GITHUB_HOST_"+name)
	delete(tokens, "GITHUB_API_PATH_"+name)
	WriteTokens(tokens)
}

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
	written := map[string]bool{}
	for _, t := range model.KnownTokens {
		if v, ok := tokens[t.Key]; ok && v != "" {
			lines = append(lines, fmt.Sprintf("%s=%s", t.Key, v))
			written[t.Key] = true
		}
	}
	// Write GitHub remote keys and any other custom keys
	for k, v := range tokens {
		if !written[k] && v != "" {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
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
		"mywiki":     {"CONFLUENCE_PAT": tokens["MYWIKI_PAT"]},
		"figma":      {"FIGMA_TOKEN": tokens["FIGMA_TOKEN"]},
	}

	// GitHub: per-remote injections
	ghRemotes := ReadGitHubRemotes()
	if len(ghRemotes) == 1 {
		injections["github"] = map[string]string{"GITHUB_TOKEN": ghRemotes[0].Token}
	} else {
		for _, r := range ghRemotes {
			injections["github-"+r.Name] = map[string]string{"GITHUB_TOKEN": r.Token}
		}
	}

	agentsDir := filepath.Join(targetDir, config.AgentsDir)
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		return nil
	}

	// Build tool expansion map for multi-remote (e.g., @github/* → @github-disney/*, @github-public/*)
	var toolExpansions map[string][]string
	if len(ghRemotes) > 1 {
		toolExpansions = map[string][]string{}
		for _, r := range ghRemotes {
			toolExpansions["@github/*"] = append(toolExpansions["@github/*"], "@github-"+r.Name+"/*")
		}
	}

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), "._") {
			continue
		}
		path := filepath.Join(agentsDir, e.Name())
		injectTokensInFile(path, injections)
		if len(toolExpansions) > 0 {
			expandToolRefs(path, toolExpansions)
		}
	}
	return nil
}

// expandToolRefs replaces tool references in an agent's tools array.
// e.g., @github/* → @github-disney/*, @github-public/*
func expandToolRefs(path string, expansions map[string][]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var raw map[string]json.RawMessage
	if json.Unmarshal(data, &raw) != nil {
		return
	}
	toolsRaw, ok := raw["tools"]
	if !ok {
		return
	}
	var tools []string
	if json.Unmarshal(toolsRaw, &tools) != nil {
		return
	}
	var expanded []string
	changed := false
	for _, t := range tools {
		if replacements, ok := expansions[t]; ok {
			expanded = append(expanded, replacements...)
			changed = true
		} else {
			expanded = append(expanded, t)
		}
	}
	if changed {
		if b, err := json.Marshal(expanded); err == nil {
			raw["tools"] = b
		}
		if out, err := json.MarshalIndent(raw, "", "  "); err == nil {
			os.WriteFile(path, append(out, '\n'), 0644)
		}
	}
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
