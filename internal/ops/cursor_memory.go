package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// InitCursorMemory generates a 60-project-context.mdc file for Cursor.
func InitCursorMemory(steerRoot, projectDir string) error {
	projectName := filepath.Base(projectDir)
	rulesDir := filepath.Join(projectDir, ".cursor", "rules")
	os.MkdirAll(rulesDir, 0755)

	var content strings.Builder
	content.WriteString("---\n")
	content.WriteString(fmt.Sprintf("description: Project context for %s\n", projectName))
	content.WriteString("alwaysApply: true\n")
	content.WriteString("---\n\n")
	content.WriteString(fmt.Sprintf("# Project Context: %s\n", projectName))

	// Try Kiro memory bank first
	kiroMB := filepath.Join(projectDir, ".kiro", config.RulesDir, "memory-bank")
	knownMB := filepath.Join(steerRoot, config.WorkspacesDir, "default", "projects", projectName, ".kiro", config.RulesDir, "memory-bank")

	var sourceDir string
	if entries, err := os.ReadDir(kiroMB); err == nil && len(entries) > 0 {
		sourceDir = kiroMB
	} else if entries, err := os.ReadDir(knownMB); err == nil && len(entries) > 0 {
		sourceDir = knownMB
	}

	if sourceDir != "" {
		entries, _ := os.ReadDir(sourceDir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(sourceDir, e.Name()))
			if err == nil {
				content.WriteString("\n")
				content.Write(data)
				content.WriteString("\n")
			}
		}
	} else {
		// Fall back to templates
		tmplDir := filepath.Join(steerRoot, "common", "memory-bank-templates")
		entries, _ := os.ReadDir(tmplDir)
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".template") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(tmplDir, e.Name()))
			if err == nil {
				expanded := strings.ReplaceAll(string(data), "{{PROJECT_NAME}}", projectName)
				content.WriteString("\n")
				content.WriteString(expanded)
				content.WriteString("\n")
			}
		}
	}

	outPath := filepath.Join(rulesDir, "60-project-context.mdc")
	return os.WriteFile(outPath, []byte(content.String()), 0644)
}
