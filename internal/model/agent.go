package model

import "encoding/json"

// Agent represents a kiro agent JSON configuration.
type Agent struct {
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	Prompt         string              `json:"prompt,omitempty"`
	Tools          []string            `json:"tools,omitempty"`
	ToolsSettings  map[string]any      `json:"toolsSettings,omitempty"`
	Resources      []ResourceEntry     `json:"resources,omitempty"`
	Hooks          AgentHooks          `json:"hooks,omitempty"`
	WelcomeMessage string              `json:"welcomeMessage,omitempty"`
	AllowedTools   []string            `json:"allowedTools,omitempty"`
	IncludeMCPJson bool                `json:"includeMcpJson,omitempty"`
	MCPServers     map[string]MCPEntry `json:"mcpServers,omitempty"`
	ContextBudget  map[string]float64  `json:"contextBudget,omitempty"`
	Phases         map[string]PhaseConfig `json:"phases,omitempty"`
}

// PhaseConfig defines tool restrictions for an execution phase.
type PhaseConfig struct {
	Tools        []string `json:"tools"`
	AllowedTools []string `json:"allowedTools"`
}

// ResourceEntry supports both string and object resource declarations.
// String form: "file://.kiro/context/golden_rules.md" (always loaded, normal priority).
// Object form: {"path": "file://...", "when": "task_contains:estimate", "priority": "low"}.
type ResourceEntry struct {
	Path     string `json:"path"`
	When     string `json:"when,omitempty"`
	Priority string `json:"priority,omitempty"`
}

// UnmarshalJSON handles both "file://path" strings and {"path":...} objects.
func (r *ResourceEntry) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		r.Path = s
		r.When = "always"
		r.Priority = "normal"
		return nil
	}
	type alias ResourceEntry
	return json.Unmarshal(data, (*alias)(r))
}

// MarshalJSON always emits the path as a plain string.
// kiro-cli requires resources to be []string. The When/Priority metadata
// is preserved in-memory for Koda's use but not serialized to agent JSON.
func (r ResourceEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(r.Path)
}

// AgentHooks groups lifecycle hook definitions.
type AgentHooks struct {
	AgentSpawn    []HookDef `json:"agentSpawn,omitempty"`
	PreToolUse    []HookDef `json:"preToolUse,omitempty"`
	PostToolUse   []HookDef `json:"postToolUse,omitempty"`
	AgentComplete []HookDef `json:"agentComplete,omitempty"`
	AgentFailed   []HookDef `json:"agentFailed,omitempty"`
	AgentTimeout  []HookDef `json:"agentTimeout,omitempty"`
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
