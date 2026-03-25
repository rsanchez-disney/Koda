package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// DoctorResult holds a single check result.
type DoctorResult struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail"`
}

// RunDoctor performs deep health checks.
func RunDoctor(steerRoot, targetDir string) []DoctorResult {
	var results []DoctorResult

	// 1. kiro-cli installed
	if path, err := exec.LookPath("kiro-cli"); err == nil {
		out, _ := exec.Command("kiro-cli", "--version").Output()
		results = append(results, DoctorResult{"kiro-cli", true, strings.TrimSpace(string(out)) + " (" + path + ")"})
	} else {
		results = append(results, DoctorResult{"kiro-cli", false, "not found in PATH"})
	}

	// 2. node installed
	if path, err := exec.LookPath("node"); err == nil {
		out, _ := exec.Command("node", "--version").Output()
		results = append(results, DoctorResult{"node", true, strings.TrimSpace(string(out)) + " (" + path + ")"})
	} else {
		results = append(results, DoctorResult{"node", false, "not found — MCP servers need Node.js"})
	}

	// 3. git installed
	if _, err := exec.LookPath("git"); err == nil {
		results = append(results, DoctorResult{"git", true, "installed"})
	} else {
		results = append(results, DoctorResult{"git", false, "not found"})
	}

	// 4. steer-runtime found
	if steerRoot != "" {
		results = append(results, DoctorResult{"steer-runtime", true, steerRoot})
	} else {
		results = append(results, DoctorResult{"steer-runtime", false, "not found — use --steer-root"})
	}

	// 5. agents directory
	agentsDir := filepath.Join(targetDir, config.AgentsDir)
	if info, err := os.Stat(agentsDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(agentsDir)
		count := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".json") {
				count++
			}
		}
		results = append(results, DoctorResult{"agents", true, fmt.Sprintf("%d installed", count)})
	} else {
		results = append(results, DoctorResult{"agents", false, "no agents directory"})
	}

	// 6. MCP server bundles
	mcpDir := filepath.Join(targetDir, config.ToolsDir, "mcp-servers")
	if entries, err := os.ReadDir(mcpDir); err == nil {
		var ready, missing []string
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			bundle := filepath.Join(mcpDir, e.Name(), "dist", "index.cjs")
			if _, err := os.Stat(bundle); err == nil {
				ready = append(ready, e.Name())
			} else {
				missing = append(missing, e.Name())
			}
		}
		detail := fmt.Sprintf("%d ready", len(ready))
		if len(missing) > 0 {
			detail += fmt.Sprintf(", %d missing bundle: %s", len(missing), strings.Join(missing, ", "))
		}
		results = append(results, DoctorResult{"mcp-servers", len(missing) == 0, detail})
	} else {
		results = append(results, DoctorResult{"mcp-servers", false, "directory not found"})
	}

	// 7. tokens
	tokens := ReadTokens()
	set := 0
	for _, v := range tokens {
		if v != "" {
			set++
		}
	}
	results = append(results, DoctorResult{"tokens", set > 0, fmt.Sprintf("%d configured", set)})

	// 8. git status of steer-runtime
	if steerRoot != "" {
		out, err := exec.Command("git", "-C", steerRoot, "status", "--short").Output()
		if err == nil {
			lines := strings.TrimSpace(string(out))
			if lines == "" {
				results = append(results, DoctorResult{"steer-git", true, "clean"})
			} else {
				count := len(strings.Split(lines, "\n"))
				results = append(results, DoctorResult{"steer-git", false, fmt.Sprintf("%d uncommitted changes", count)})
			}
		}
	}

	return results
}

// PrintDoctor prints doctor results.
func PrintDoctor(results []DoctorResult) {
	fmt.Println("\U0001fa7a Koda Doctor")
	fmt.Println()
	for _, r := range results {
		icon := "\u2713"
		if !r.OK {
			icon = "\u2717"
		}
		fmt.Printf("  %s %-16s %s\n", icon, r.Name, r.Detail)
	}
}
