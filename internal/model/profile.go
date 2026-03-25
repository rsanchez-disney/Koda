package model

// Profile represents an installable agent profile (e.g., dev-core, qa, ops).
type Profile struct {
	ID         string  `json:"id"`
	SourceDir  string  `json:"-"`
	Agents     []Agent `json:"agents,omitempty"`
	AgentCount int     `json:"agent_count"`
	Installed  bool    `json:"installed"`
}

// Aliases maps shorthand names to their expanded profile lists.
var Aliases = map[string][]string{
	"dev": {"dev-core", "dev-web", "dev-mobile"},
}
