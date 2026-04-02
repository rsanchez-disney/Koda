package model

// Workspace represents a team workspace configuration.
type Workspace struct {
	Name          string             `json:"name"`
	Extends       string             `json:"extends,omitempty"`
	Description   string             `json:"description"`
	Team          string             `json:"team"`
	Profiles      []string           `json:"profiles"`
	DefaultAgent  string             `json:"default_agent"`
	Projects      []WorkspaceProject `json:"projects"`
	Rules         []string           `json:"rules"`
	EnableTools   bool               `json:"enable_tools"`
	JiraPrefix    string             `json:"jira_prefix"`
	WorkspacePath string             `json:"workspace_path,omitempty"`
}

// WorkspaceProject is a repo entry inside a workspace.
type WorkspaceProject struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Repo       string `json:"repo,omitempty"`
	MemoryBank string `json:"memory_bank,omitempty"`
}

// Token represents a configured MCP token.
type Token struct {
	Key   string
	Label string
	Value string
	Hint  string
}

// KnownTokens defines the tokens Koda manages.
var KnownTokens = []Token{
	{Key: "JIRA_PAT", Label: "Jira PAT", Hint: "https://jira.disney.com/secure/ViewProfile.jspa → Personal Access Tokens"},
	{Key: "CONFLUENCE_PAT", Label: "Confluence PAT", Hint: "https://confluence.disney.com/plugins/personalaccesstokens/usertokens.action"},
	{Key: "GITHUB_TOKEN", Label: "GitHub Token", Hint: "https://github.disney.com/settings/tokens → Generate new token"},
	{Key: "SONARQUBE_TOKEN", Label: "SonarQube Token", Hint: "https://sonar.cicd.wdprapps.disney.com/account/security"},
	{Key: "MYWIKI_PAT", Label: "MyWiki PAT", Hint: "https://mywiki.disney.com/plugins/personalaccesstokens/usertokens.action"},
	{Key: "HARNESS_API_KEY", Label: "Harness API Key", Hint: "https://disney.harness.io/ → My Profile → API Key"},
}

// EnvVar represents a managed environment variable.
type EnvVar struct {
	Key         string
	Default     string
	Description string
}

// KnownEnvVars defines the env vars Koda manages with their defaults.
var KnownEnvVars = []EnvVar{
	{Key: "GITHUB_URL", Default: "https://github.disney.com", Description: "GitHub Enterprise URL"},
	{Key: "CONFLUENCE_URL", Default: "https://confluence.disney.com", Description: "Confluence URL"},
	{Key: "MYWIKI_URL", Default: "https://mywiki.disney.com", Description: "MyWiki Confluence URL"},
	{Key: "JIRA_URL", Default: "https://jira.disney.com", Description: "Jira URL"},
	{Key: "GITHUB_API_PATH", Default: "/api/v3", Description: "GitHub API path"},
}
