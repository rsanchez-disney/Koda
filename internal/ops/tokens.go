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
	} else {
		delete(tokens, "GITHUB_API_PATH_"+r.Name)
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

// ReadJiraInstances discovers Jira instances from tokens.env by scanning for JIRA_PAT_* keys.
// Falls back to single instance from JIRA_PAT + JIRA_URL if no suffixed keys found.
func ReadJiraInstances() []model.JiraInstance {
	tokens := ReadTokens()
	envVars := ReadEnvVars()
	var instances []model.JiraInstance

	for k, v := range tokens {
		if !strings.HasPrefix(k, "JIRA_PAT_") || v == "" {
			continue
		}
		name := strings.TrimPrefix(k, "JIRA_PAT_")
		url := tokens["JIRA_URL_"+name]
		if url == "" {
			url = envVars["JIRA_URL_"+name]
		}
		instances = append(instances, model.JiraInstance{Name: name, URL: url, Token: v})
	}

	// Backward compat: single JIRA_PAT
	if len(instances) == 0 {
		if tok := tokens["JIRA_PAT"]; tok != "" {
			url := envVars["JIRA_URL"]
			if url == "" {
				url = "https://myjira.disney.com"
			}
			instances = append(instances, model.JiraInstance{Name: "jira", URL: url, Token: tok})
		}
	}
	return instances
}

// WriteJiraInstance adds or updates a Jira instance in tokens.env.
func WriteJiraInstance(inst model.JiraInstance) {
	tokens := ReadTokens()
	tokens["JIRA_PAT_"+inst.Name] = inst.Token
	tokens["JIRA_URL_"+inst.Name] = inst.URL
	WriteTokens(tokens)
}

// RemoveJiraInstance removes a Jira instance from tokens.env.
func RemoveJiraInstance(name string) {
	tokens := ReadTokens()
	delete(tokens, "JIRA_PAT_"+name)
	delete(tokens, "JIRA_URL_"+name)
	WriteTokens(tokens)
}

// ReadConfluenceInstances discovers Confluence instances from tokens.env by scanning for CONFLUENCE_PAT_* keys.
// Falls back to single instance from CONFLUENCE_PAT + CONFLUENCE_URL, and MYWIKI_PAT + MYWIKI_URL.
func ReadConfluenceInstances() []model.ConfluenceInstance {
	tokens := ReadTokens()
	envVars := ReadEnvVars()
	var instances []model.ConfluenceInstance

	for k, v := range tokens {
		if !strings.HasPrefix(k, "CONFLUENCE_PAT_") || v == "" {
			continue
		}
		name := strings.TrimPrefix(k, "CONFLUENCE_PAT_")
		url := tokens["CONFLUENCE_URL_"+name]
		if url == "" {
			url = envVars["CONFLUENCE_URL_"+name]
		}
		instances = append(instances, model.ConfluenceInstance{Name: name, URL: url, Token: v})
	}

	// Backward compat: single CONFLUENCE_PAT + MYWIKI_PAT
	if len(instances) == 0 {
		if tok := tokens["CONFLUENCE_PAT"]; tok != "" {
			url := envVars["CONFLUENCE_URL"]
			if url == "" {
				url = "https://confluence.disney.com"
			}
			instances = append(instances, model.ConfluenceInstance{Name: "confluence", URL: url, Token: tok})
		}
		if tok := tokens["MYWIKI_PAT"]; tok != "" {
			url := envVars["MYWIKI_URL"]
			if url == "" {
				url = "https://mywiki.disney.com"
			}
			instances = append(instances, model.ConfluenceInstance{Name: "mywiki", URL: url, Token: tok})
		}
	}
	return instances
}

// WriteConfluenceInstance adds or updates a Confluence instance in tokens.env.
func WriteConfluenceInstance(inst model.ConfluenceInstance) {
	tokens := ReadTokens()
	tokens["CONFLUENCE_PAT_"+inst.Name] = inst.Token
	tokens["CONFLUENCE_URL_"+inst.Name] = inst.URL
	WriteTokens(tokens)
}

// RemoveConfluenceInstance removes a Confluence instance from tokens.env.
func RemoveConfluenceInstance(name string) {
	tokens := ReadTokens()
	delete(tokens, "CONFLUENCE_PAT_"+name)
	delete(tokens, "CONFLUENCE_URL_"+name)
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
		"figma": {"FIGMA_TOKEN": tokens["FIGMA_TOKEN"]},
	}

	// Jira: per-instance injections
	jiraInstances := ReadJiraInstances()
	if len(jiraInstances) == 1 {
		injections["jira"] = map[string]string{"JIRA_PAT": jiraInstances[0].Token, "JIRA_URL": jiraInstances[0].URL}
	} else {
		for _, inst := range jiraInstances {
			injections["jira-"+inst.Name] = map[string]string{"JIRA_PAT": inst.Token, "JIRA_URL": inst.URL}
		}
	}

	// Confluence: per-instance injections
	confInstances := ReadConfluenceInstances()
	if len(confInstances) == 1 {
		injections["confluence"] = map[string]string{"CONFLUENCE_PAT": confInstances[0].Token, "CONFLUENCE_URL": confInstances[0].URL}
	} else {
		for _, inst := range confInstances {
			injections["confluence-"+inst.Name] = map[string]string{"CONFLUENCE_PAT": inst.Token, "CONFLUENCE_URL": inst.URL}
		}
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

	// Build tool expansion map for multi-remote/instance
	toolExpansions := map[string][]string{}
	if len(ghRemotes) > 1 {
		for _, r := range ghRemotes {
			toolExpansions["@github/*"] = append(toolExpansions["@github/*"], "@github-"+r.Name+"/*")
		}
	}
	if len(jiraInstances) > 1 {
		for _, inst := range jiraInstances {
			toolExpansions["@jira/*"] = append(toolExpansions["@jira/*"], "@jira-"+inst.Name+"/*")
		}
	}
	if len(confInstances) > 1 {
		for _, inst := range confInstances {
			toolExpansions["@confluence/*"] = append(toolExpansions["@confluence/*"], "@confluence-"+inst.Name+"/*")
			toolExpansions["@mywiki/*"] = append(toolExpansions["@mywiki/*"], "@confluence-"+inst.Name+"/*")
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
