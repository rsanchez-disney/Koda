package model

// Agent represents a kiro agent JSON configuration.
type Agent struct {
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	Prompt         string              `json:"prompt,omitempty"`
	Tools          []string            `json:"tools,omitempty"`
	ToolsSettings  map[string]any      `json:"toolsSettings,omitempty"`
	Resources      []string            `json:"resources,omitempty"`
	Hooks          AgentHooks          `json:"hooks,omitempty"`
	WelcomeMessage string              `json:"welcomeMessage,omitempty"`
	AllowedTools   []string            `json:"allowedTools,omitempty"`
	IncludeMCPJson bool                `json:"includeMcpJson,omitempty"`
	MCPServers     map[string]MCPEntry `json:"mcpServers,omitempty"`
}

// AgentHooks groups lifecycle hook definitions.
type AgentHooks struct {
	AgentSpawn  []HookDef `json:"agentSpawn,omitempty"`
	PreToolUse  []HookDef `json:"preToolUse,omitempty"`
	PostToolUse []HookDef `json:"postToolUse,omitempty"`
}

// HookDef is a single hook entry.
type HookDef struct {
	Matcher     string `json:"matcher,omitempty"`
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
}

// MCPEntry represents an MCP server configuration inside an agent.
type MCPEntry struct {
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}
