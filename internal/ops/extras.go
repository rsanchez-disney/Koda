package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// TargetDirFromProject delegates to config.TargetDir.
func TargetDirFromProject(projectDir string) string {
	return config.TargetDir(projectDir)
}

// ListRules returns available rule names from common/rules/.
func ListRules(steerRoot string) []string {
	return listMDFiles(filepath.Join(steerRoot, "common", config.RulesDir))
}

// InstallRules copies rules to targetDir/rules/.
func InstallRules(steerRoot, targetDir string, names []string) int {
	srcDir := filepath.Join(steerRoot, "common", config.RulesDir)
	dstDir := filepath.Join(targetDir, config.RulesDir)
	os.MkdirAll(dstDir, 0755)
	count := 0
	for _, name := range names {
		src := filepath.Join(srcDir, name+".md")
		if _, err := os.Stat(src); err == nil {
			copyFile(src, filepath.Join(dstDir, name+".md"))
			count++
		}
	}
	return count
}

// ListPrompts returns available prompt names from common/prompts/.
func ListPrompts(steerRoot string) []string {
	return listMDFiles(filepath.Join(steerRoot, "common", "prompts"))
}

// InstallPrompts copies prompts to ~/.kiro/prompts/.
func InstallPrompts(steerRoot string, names []string) int {
	srcDir := filepath.Join(steerRoot, "common", "prompts")
	dstDir := filepath.Join(config.KiroRoot(), config.PromptsDir)
	os.MkdirAll(dstDir, 0755)
	count := 0
	for _, name := range names {
		src := filepath.Join(srcDir, name+".md")
		if _, err := os.Stat(src); err == nil {
			copyFile(src, filepath.Join(dstDir, name+".md"))
			count++
		}
	}
	return count
}

// InitMemory creates a memory bank in projectDir/.kiro/rules/memory-bank/.
func InitMemory(steerRoot, projectDir, fromProject string) error {
	projectName := filepath.Base(projectDir)
	if fromProject != "" {
		projectName = fromProject
	}
	targetMB := filepath.Join(projectDir, ".kiro", config.RulesDir, "memory-bank")
	os.MkdirAll(targetMB, 0755)

	knownMB := filepath.Join(steerRoot, config.WorkspacesDir, "default", "projects", projectName, ".kiro", config.RulesDir, "memory-bank")
	if entries, err := os.ReadDir(knownMB); err == nil && len(entries) > 0 {
		fmt.Printf("  Found known project: %s\n", projectName)
		copyDirContents(knownMB, targetMB)
		return nil
	}

	tmplDir := filepath.Join(steerRoot, "common", "memory-bank-templates")
	entries, err := os.ReadDir(tmplDir)
	if err != nil {
		return fmt.Errorf("no templates found at %s", tmplDir)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".template") || strings.HasPrefix(e.Name(), "._") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(tmplDir, e.Name()))
		if err != nil {
			continue
		}
		outName := strings.TrimSuffix(e.Name(), ".template")
		expanded := strings.ReplaceAll(string(data), "{{PROJECT_NAME}}", projectName)
		os.WriteFile(filepath.Join(targetMB, outName), []byte(expanded), 0644)
		fmt.Printf("  \u2713 %s\n", outName)
	}
	return nil
}

// InstallAmazonQRules copies .amazonq-templates/*.md to dir/.amazonq/rules/.
func InstallAmazonQRules(steerRoot, dir string) (int, error) {
	srcDir := filepath.Join(steerRoot, ".amazonq-templates")
	dstDir := filepath.Join(dir, ".amazonq", "rules")
	os.MkdirAll(dstDir, 0755)
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return 0, fmt.Errorf("no amazonq templates found")
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.HasPrefix(e.Name(), "._") || e.Name() == "README.md" {
			continue
		}
		copyFile(filepath.Join(srcDir, e.Name()), filepath.Join(dstDir, e.Name()))
		count++
	}
	return count, nil
}

// RemoveDir removes a directory tree.
func RemoveDir(path string) error {
	return os.RemoveAll(path)
}

// AgentInfo holds agent metadata across profiles.
type AgentInfo struct {
	ProfileID   string
	Name        string
	Description string
	Tools       []string
	MCPServers  []string
}

// AllAgents returns all agents with their profile ID.
func AllAgents(steerRoot, targetDir string) []AgentInfo {
	profiles, _ := ListProfiles(steerRoot, targetDir)
	var all []AgentInfo
	for _, p := range profiles {
		for _, a := range p.Agents {
			var mcps []string
			for k := range a.MCPServers {
				mcps = append(mcps, k)
			}
			all = append(all, AgentInfo{
				ProfileID:   p.ID,
				Name:        a.Name,
				Description: a.Description,
				Tools:       a.Tools,
				MCPServers:  mcps,
			})
		}
	}
	return all
}

func listMDFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && !strings.HasPrefix(e.Name(), "._") && e.Name() != "README.md" {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return names
}

// SyncAmazonQContext copies ~/.kiro/context/*.md to projectDir/.amazonq/rules/
// as 60-ctx-<name> files, skipping any already covered by templates.
func SyncAmazonQContext(projectDir string) int {
	ctxDir := filepath.Join(config.KiroRoot(), config.ContextDir)
	dstDir := filepath.Join(projectDir, ".amazonq", "rules")
	os.MkdirAll(dstDir, 0755)

	entries, err := os.ReadDir(ctxDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.HasPrefix(e.Name(), "._") {
			continue
		}
		// Skip if a template already covers this name
		matches, _ := filepath.Glob(filepath.Join(dstDir, "*"+e.Name()+"*"))
		if len(matches) > 0 {
			continue
		}
		copyFile(filepath.Join(ctxDir, e.Name()), filepath.Join(dstDir, "60-ctx-"+e.Name()))
		count++
	}
	return count
}

// SyncAmazonQMCP merges ~/.kiro/settings/mcp.json into ~/.aws/amazonq/mcp.json,
// preserving any user-added servers.
func SyncAmazonQMCP() (int, error) {
	home, _ := os.UserHomeDir()
	kiroMCP := filepath.Join(config.KiroRoot(), config.SettingsDir, "mcp.json")
	aqMCP := filepath.Join(home, ".aws", "amazonq", "mcp.json")

	kiroData, err := os.ReadFile(kiroMCP)
	if err != nil {
		return 0, fmt.Errorf("no Kiro MCP config at %s — run 'koda mcp-install' first", kiroMCP)
	}

	// Parse Kiro config
	var kiro map[string]interface{}
	if err := json.Unmarshal(kiroData, &kiro); err != nil {
		return 0, fmt.Errorf("invalid JSON in %s: %w", kiroMCP, err)
	}
	kiroServers, _ := kiro["mcpServers"].(map[string]interface{})

	// Parse existing Amazon Q config (if any)
	merged := map[string]interface{}{}
	if aqData, err := os.ReadFile(aqMCP); err == nil {
		var existing map[string]interface{}
		if json.Unmarshal(aqData, &existing) == nil {
			if s, ok := existing["mcpServers"].(map[string]interface{}); ok {
				for k, v := range s {
					merged[k] = v
				}
			}
		}
	}

	// Kiro servers override existing
	for k, v := range kiroServers {
		merged[k] = v
	}

	os.MkdirAll(filepath.Dir(aqMCP), 0755)
	result, _ := json.MarshalIndent(map[string]interface{}{"mcpServers": merged}, "", "  ")
	if err := os.WriteFile(aqMCP, append(result, '\n'), 0644); err != nil {
		return 0, fmt.Errorf("failed to write %s: %w", aqMCP, err)
	}
	return len(kiroServers), nil
}

// AmazonQStatus returns a status report of Amazon Q sync state.
type AmazonQStatusReport struct {
	RulesDir   string
	RulesCount int
	MCPPath    string
	MCPCount   int
	KiroMCP    bool
}

func AmazonQStatus(projectDir string) AmazonQStatusReport {
	home, _ := os.UserHomeDir()
	report := AmazonQStatusReport{
		RulesDir: filepath.Join(projectDir, ".amazonq", "rules"),
		MCPPath:  filepath.Join(home, ".aws", "amazonq", "mcp.json"),
	}

	if entries, err := os.ReadDir(report.RulesDir); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".md") && !strings.HasPrefix(e.Name(), "._") {
				report.RulesCount++
			}
		}
	}

	if data, err := os.ReadFile(report.MCPPath); err == nil {
		var cfg map[string]interface{}
		if json.Unmarshal(data, &cfg) == nil {
			if s, ok := cfg["mcpServers"].(map[string]interface{}); ok {
				report.MCPCount = len(s)
			}
		}
	}

	kiroMCP := filepath.Join(config.KiroRoot(), config.SettingsDir, "mcp.json")
	_, err := os.Stat(kiroMCP)
	report.KiroMCP = err == nil

	return report
}
