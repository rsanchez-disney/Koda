package model

// Workspace represents a team workspace configuration.
type Workspace struct {
	Name          string             `json:"name"`
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
}

// KnownTokens defines the tokens Koda manages.
var KnownTokens = []Token{
	{Key: "JIRA_PAT", Label: "Jira PAT"},
	{Key: "CONFLUENCE_PAT", Label: "Confluence PAT"},
	{Key: "GITHUB_TOKEN_disney", Label: "GitHub Token"},
	{Key: "SONARQUBE_TOKEN", Label: "SonarQube Token"},
	{Key: "MYWIKI_PAT", Label: "MyWiki PAT"},
	{Key: "HARNESS_API_KEY", Label: "Harness API Key"},
}
