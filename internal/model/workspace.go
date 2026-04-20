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
	Services      []string           `json:"services,omitempty"`
	Channels      []string           `json:"channels,omitempty"`
	WorkspacePath string             `json:"workspace_path,omitempty"`
	Teams         []TeamEntry        `json:"teams,omitempty"`
}

// WorkspaceProject is a repo entry inside a workspace.
type WorkspaceProject struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	Repo       string `json:"repo,omitempty"`
	MemoryBank string `json:"memory_bank,omitempty"`
}

// TeamEntry represents a team within a leadership/vertical workspace.
type TeamEntry struct {
	Name         string   `json:"name"`
	Workspace    string   `json:"workspace,omitempty"`
	JiraProjects []string `json:"jira_projects,omitempty"`
	BoardIDs     []int    `json:"board_ids,omitempty"`
	Studio       string   `json:"studio,omitempty"`
	StudioID     int      `json:"studio_id,omitempty"`
	TeamID       int      `json:"team_id,omitempty"`
}

// Token represents a configured MCP token.
type Token struct {
	Key   string
	Label string
	Value string
	Hint  string
}

// KnownTokens defines the tokens Koda manages.
// Jira and Confluence tokens are managed via instances (like GitHub remotes).
var KnownTokens = []Token{
	{Key: "SONARQUBE_TOKEN", Label: "SonarQube Token", Hint: "https://sonar.cicd.wdprapps.disney.com/account/security"},
	{Key: "HARNESS_API_KEY", Label: "Harness API Key", Hint: "https://disney.harness.io/ → My Profile → API Key"},
	{Key: "FIGMA_TOKEN", Label: "Figma Token", Hint: "https://www.figma.com/developers/api#access-tokens"},
	{Key: "COMPASS_TOKEN", Label: "Compass Token", Hint: "https://compass.wdprapps.disney.com — contact your team lead"},
	{Key: "QTEST_BEARER_TOKEN", Label: "qTest Token", Hint: "https://qtest.disney.com — Settings → API Keys"},
}

// EnvVar represents a managed environment variable.
type EnvVar struct {
	Key         string
	Default     string
	Description string
}

// GitHubRemote represents a configured GitHub instance.
type GitHubRemote struct {
	Name    string
	Host    string
	Token   string
	APIPath string
}

// JiraInstance represents a configured Jira instance.
type JiraInstance struct {
	Name  string // e.g., "myjira", "jira"
	URL   string // e.g., "https://myjira.disney.com"
	Token string // JIRA PAT
}

// ConfluenceInstance represents a configured Confluence instance.
type ConfluenceInstance struct {
	Name  string // e.g., "confluence", "mywiki"
	URL   string // e.g., "https://confluence.disney.com"
	Token string // Confluence PAT
}

// DefaultGitHubRemotes defines the pre-populated GitHub instances.
var DefaultGitHubRemotes = []GitHubRemote{
	{Name: "disney", Host: "github.disney.com"},
	{Name: "public", Host: "github.com"},
}

// DefaultJiraInstances defines the pre-populated Jira instances.
var DefaultJiraInstances = []JiraInstance{
	{Name: "myjira", URL: "https://myjira.disney.com"},
	{Name: "jira", URL: "https://jira.disney.com"},
}

// DefaultConfluenceInstances defines the pre-populated Confluence instances.
var DefaultConfluenceInstances = []ConfluenceInstance{
	{Name: "confluence", URL: "https://confluence.disney.com"},
	{Name: "mywiki", URL: "https://mywiki.disney.com"},
}

// KnownEnvVars defines the env vars Koda manages with their defaults.
var KnownEnvVars = []EnvVar{
	{Key: "COMPASS_URL", Default: "", Description: "Compass MCP endpoint URL"},
	{Key: "QTEST_BASE_URL", Default: "https://qtest.disney.com", Description: "qTest Manager base URL"},
	{Key: "QTEST_PROJECT_ID", Default: "", Description: "Default qTest project ID (optional)"},
	{Key: "CONTAINER_RUNTIME", Default: "", Description: "Container runtime (docker, nerdctl, podman) — auto-detected if empty"},
}
