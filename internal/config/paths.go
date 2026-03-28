package config

import (
	"os"
	"path/filepath"
)

const (
	ProfilePrefix = "profiles/"
	AgentsDir     = "agents"
	PromptsDir    = "prompts"
	ContextDir    = "context"
	HooksDir      = "hooks"
	ToolsDir      = "tools"
	SettingsDir   = "settings"
	RulesDir      = "rules"
	WorkspacesDir = "workspaces"
	TokensFile    = "tokens.env"
)

// KiroRoot returns ~/.kiro
func KiroRoot() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".kiro")
}

// SteerRoot finds the steer-runtime repo root by looking for profiles/dev-core/
// starting from dir and walking up. Returns empty string if not found.
func SteerRoot(dir string) string {
	for {
		if _, err := os.Stat(filepath.Join(dir, ProfilePrefix+"dev-core")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// ProfileDirs returns all profiles/* directories under steerRoot.
func ProfileDirs(steerRoot string) ([]string, error) {
	matches, err := filepath.Glob(filepath.Join(steerRoot, ProfilePrefix+"*"))
	if err != nil {
		return nil, err
	}
	var dirs []string
	for _, m := range matches {
		info, err := os.Stat(m)
		if err == nil && info.IsDir() {
			dirs = append(dirs, m)
		}
	}
	return dirs, nil
}

// TargetDir resolves the installation target.
// If projectDir is set, returns projectDir/.kiro; otherwise ~/.kiro.
func TargetDir(projectDir string) string {
	if projectDir != "" {
		return filepath.Join(projectDir, ".kiro")
	}
	return KiroRoot()
}
