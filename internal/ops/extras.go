package ops

import (
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
func InitMemory(steerRoot, projectDir string) error {
	projectName := filepath.Base(projectDir)
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
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".template") {
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
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
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
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") && e.Name() != "README.md" {
			names = append(names, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	return names
}
