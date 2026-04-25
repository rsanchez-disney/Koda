package kitestream

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// PersistSessionName creates a named symlink or copy of the Kiro CLI session file
// so it appears as a top-level named session in ~/.kiro/sessions/.
func PersistSessionName(sessionID, name string) error {
	if sessionID == "" || name == "" {
		return nil
	}

	sessDir := filepath.Join(config.KiroRoot(), "sessions", "cli")
	targetDir := filepath.Join(config.KiroRoot(), "sessions")

	// Find the session file by ID
	srcFile := filepath.Join(sessDir, sessionID+".json")
	if _, err := os.Stat(srcFile); err != nil {
		return nil // Session file not created yet by Kiro CLI
	}

	// Clean the name for filesystem
	safeName := strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == ':' || r == '*' || r == '?' || r == '"' || r == '<' || r == '>' || r == '|' {
			return '_'
		}
		return r
	}, name)
	if safeName == "" {
		return nil
	}

	destFile := filepath.Join(targetDir, "KS-"+safeName+".json")

	// Read source, update title in metadata, write to dest
	data, err := os.ReadFile(srcFile)
	if err != nil {
		return nil
	}

	var raw map[string]interface{}
	if json.Unmarshal(data, &raw) != nil {
		// Not valid JSON yet — just copy as-is
		return os.WriteFile(destFile, data, 0o644)
	}

	// Update title in metadata
	if meta, ok := raw["metadata"].(map[string]interface{}); ok {
		if meta["title"] == nil || meta["title"] == "" {
			meta["title"] = name
		}
	}

	out, _ := json.Marshal(raw)
	return os.WriteFile(destFile, out, 0o644)
}
