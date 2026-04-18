package ops

import (
	"encoding/json"
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
	Fix    string `json:"fix,omitempty"`
}

// RunDoctor performs deep health checks.
func RunDoctor(steerRoot, targetDir string) []DoctorResult {
	var results []DoctorResult

	// 1. kiro-cli installed
	kiroOK, kiroDetail, kiroFix := DiagnoseKiroCLI()
	results = append(results, DoctorResult{Name: "kiro-cli", OK: kiroOK, Detail: kiroDetail, Fix: kiroFix})

	// 2. node installed
	if path, err := exec.LookPath("node"); err == nil {
		out, _ := exec.Command("node", "--version").Output()
		results = append(results, DoctorResult{Name: "node", OK: true, Detail: strings.TrimSpace(string(out)) + " (" + path + ")"})
	} else {
		results = append(results, DoctorResult{Name: "node", OK: false, Detail: "not found — MCP servers need Node.js"})
	}

	// 3. git installed
	if _, err := exec.LookPath("git"); err == nil {
		results = append(results, DoctorResult{Name: "git", OK: true, Detail: "installed"})
	} else {
		results = append(results, DoctorResult{Name: "git", OK: false, Detail: "not found"})
	}

	// 4. steer-runtime found
	if steerRoot != "" {
		results = append(results, DoctorResult{Name: "steer-runtime", OK: true, Detail: steerRoot})
	} else {
		results = append(results, DoctorResult{Name: "steer-runtime", OK: false, Detail: "not found — use --steer-root"})
	}

	// 5. agents directory
	agentsDir := filepath.Join(targetDir, config.AgentsDir)
	if info, err := os.Stat(agentsDir); err == nil && info.IsDir() {
		entries, _ := os.ReadDir(agentsDir)
		count := 0
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), "._") {
				count++
			}
		}
		results = append(results, DoctorResult{Name: "agents", OK: true, Detail: fmt.Sprintf("%d installed", count)})
	} else {
		results = append(results, DoctorResult{Name: "agents", OK: false, Detail: "no agents directory"})
	}

	// 6. MCP server bundles + diagnostics
	mcpDir := filepath.Join(targetDir, config.ToolsDir, "mcp-servers")
	if entries, err := os.ReadDir(mcpDir); err == nil {
		var ready, missing []string
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			bundle := filepath.Join(mcpDir, name, "dist", "index.cjs")
			npmBin := filepath.Join(mcpDir, name, "node_modules", ".bin")
			if _, err := os.Stat(bundle); err == nil {
				ready = append(ready, name)
			} else if _, err := os.Stat(npmBin); err == nil {
				ready = append(ready, name)
			} else {
				missing = append(missing, name)
			}
		}
		detail := fmt.Sprintf("%d ready", len(ready))
		if len(missing) > 0 {
			detail += fmt.Sprintf(", %d missing: %s", len(missing), strings.Join(missing, ", "))
		}
		results = append(results, DoctorResult{Name: "mcp-servers", OK: len(missing) == 0, Detail: detail})

		// Per-server diagnostics: try to start each and capture errors.
		// Skip servers that require runtime env vars (e.g. github-mcp needs GITHUB_HOST).
		skipDiag := map[string]bool{"github-mcp": true}
		for _, name := range ready {
			if skipDiag[name] {
				results = append(results, DoctorResult{Name: "  " + name, OK: true, Detail: "bundle ok (env checked separately)"})
				continue
			}
			var cmd *exec.Cmd
			bundle := filepath.Join(mcpDir, name, "dist", "index.cjs")
			npmBin := filepath.Join(mcpDir, name, "node_modules", ".bin", name)
			if _, err := os.Stat(bundle); err == nil {
				cmd = exec.Command("node", bundle, "--help")
			} else if _, err := os.Stat(npmBin); err == nil {
				cmd = exec.Command(npmBin, "--help")
			}
			if cmd == nil {
				continue
			}
			out, err := cmd.CombinedOutput()
			if err != nil {
				errDetail := strings.TrimSpace(string(out))
				if len(errDetail) > 120 {
					errDetail = errDetail[:120] + "…"
				}
				if errDetail == "" {
					errDetail = err.Error()
				}
				results = append(results, DoctorResult{Name: "  " + name, OK: false, Detail: errDetail})
			} else {
				results = append(results, DoctorResult{Name: "  " + name, OK: true, Detail: "ok"})
			}
		}
		for _, name := range missing {
			fix := "cd " + filepath.Join(mcpDir, name) + " && npm install"
			results = append(results, DoctorResult{Name: "  " + name, OK: false, Detail: "no bundle or node_modules", Fix: fix})
		}
	} else {
		results = append(results, DoctorResult{Name: "mcp-servers", OK: false, Detail: "directory not found"})
	}

	// 6a. SSE/remote servers from mcp.json (e.g., compass)
	mcpJSON := filepath.Join(targetDir, config.SettingsDir, "mcp.json")
	if data, err := os.ReadFile(mcpJSON); err == nil {
		var cfg struct {
			Servers map[string]struct {
				URL  string `json:"url"`
				Type string `json:"type"`
			} `json:"mcpServers"`
		}
		if json.Unmarshal(data, &cfg) == nil {
			for name, srv := range cfg.Servers {
				if srv.Type == "sse" {
					results = append(results, DoctorResult{Name: "  " + name + " (sse)", OK: true, Detail: srv.URL})
				}
			}
		}
	}

	// 6b. Service/channel bank staleness
	if steerRoot != "" {
		ctxDir := filepath.Join(targetDir, config.ContextDir)
		var stale []string
		// Check svc-*.md files
		entries, _ := os.ReadDir(ctxDir)
		for _, e := range entries {
			name := e.Name()
			var srcDir string
			if strings.HasPrefix(name, "svc-") && strings.HasSuffix(name, ".md") {
				svc := strings.TrimSuffix(strings.TrimPrefix(name, "svc-"), ".md")
				srcDir = filepath.Join(steerRoot, "shared", "services", svc)
			} else if strings.HasPrefix(name, "ch-") && strings.HasSuffix(name, ".md") {
				ch := strings.TrimSuffix(strings.TrimPrefix(name, "ch-"), ".md")
				srcDir = filepath.Join(steerRoot, "channels", ch)
			} else {
				continue
			}
			installed, _ := e.Info()
			if installed == nil {
				continue
			}
			// Check if any source file is newer than the installed merged file
			srcEntries, err := os.ReadDir(srcDir)
			if err != nil {
				stale = append(stale, name+" (source missing)")
				continue
			}
			for _, se := range srcEntries {
				if se.IsDir() || !strings.HasSuffix(se.Name(), ".md") {
					continue
				}
				si, _ := se.Info()
				if si != nil && si.ModTime().After(installed.ModTime()) {
					stale = append(stale, name)
					break
				}
			}
		}
		if len(stale) > 0 {
			results = append(results, DoctorResult{
				Name:   "banks",
				OK:     false,
				Detail: fmt.Sprintf("%d stale: %s", len(stale), strings.Join(stale, ", ")),
				Fix:    "koda workspace apply <name>",
			})
		} else if len(entries) > 0 {
			// Count how many bank files exist
			bankCount := 0
			for _, e := range entries {
				n := e.Name()
				if (strings.HasPrefix(n, "svc-") || strings.HasPrefix(n, "ch-")) && strings.HasSuffix(n, ".md") {
					bankCount++
				}
			}
			if bankCount > 0 {
				results = append(results, DoctorResult{Name: "banks", OK: true, Detail: fmt.Sprintf("%d banks up to date", bankCount)})
			}
		}
	}

	// 6c. Kiro IDE status
	kideStatus := CheckKiroIDE("")
	if kideStatus.SteeringCount > 0 || kideStatus.SkillsCount > 0 {
		results = append(results, DoctorResult{
			Name:   "kiro-ide",
			OK:     true,
			Detail: fmt.Sprintf("%d steering, %d skills", kideStatus.SteeringCount, kideStatus.SkillsCount),
		})
	}

	// 7. tokens
	tokens := ReadTokens()
	set := 0
	for _, v := range tokens {
		if v != "" {
			set++
		}
	}
	results = append(results, DoctorResult{Name: "tokens", OK: set > 0, Detail: fmt.Sprintf("%d configured", set)})

	// 8. GitHub remotes configuration
	ghRemotes := ReadGitHubRemotes()
	if len(ghRemotes) == 0 {
		results = append(results, DoctorResult{
			Name:   "github-mcp",
			OK:     false,
			Detail: "no GitHub remotes configured",
			Fix:    "koda mcp-install --assistant",
		})
	} else {
		var names []string
		for _, r := range ghRemotes {
			names = append(names, r.Name+"("+r.Host+")")
		}
		results = append(results, DoctorResult{
			Name:   "github-mcp",
			OK:     true,
			Detail: fmt.Sprintf("%d remote(s): %s", len(ghRemotes), strings.Join(names, ", ")),
		})
	}

	// 9. git status of steer-runtime
	if steerRoot != "" {
		out, err := exec.Command("git", "-C", steerRoot, "status", "--short").Output()
		if err == nil {
			lines := strings.TrimSpace(string(out))
			if lines == "" {
				results = append(results, DoctorResult{Name: "steer-git", OK: true, Detail: "clean"})
			} else {
				count := len(strings.Split(lines, "\n"))
				results = append(results, DoctorResult{Name: "steer-git", OK: false, Detail: fmt.Sprintf("%d uncommitted changes", count)})
			}
		}
	}

	// 10. gh auth status — check each configured remote host
	if len(ghRemotes) == 0 {
		// Fallback: check default host when no remotes configured
		ghCmd := exec.Command("gh", "auth", "status", "--hostname", config.GHHost)
		if out, err := ghCmd.CombinedOutput(); err != nil {
			results = append(results, DoctorResult{
				Name:   "gh-auth",
				OK:     false,
				Detail: "not authenticated to " + config.GHHost,
				Fix:    "gh auth login --hostname " + config.GHHost,
			})
		} else {
			detail := "authenticated"
			for _, line := range strings.Split(string(out), "\n") {
				if strings.Contains(line, "Logged in to") {
					detail = strings.TrimSpace(line)
					break
				}
			}
			results = append(results, DoctorResult{Name: "gh-auth", OK: true, Detail: detail})
		}
	} else {
		for _, r := range ghRemotes {
			name := "gh-auth-" + r.Name
			ghCmd := exec.Command("gh", "auth", "status", "--hostname", r.Host)
			if out, err := ghCmd.CombinedOutput(); err != nil {
				results = append(results, DoctorResult{
					Name:   name,
					OK:     false,
					Detail: "not authenticated to " + r.Host,
					Fix:    "gh auth login --hostname " + r.Host,
				})
			} else {
				detail := "authenticated to " + r.Host
				for _, line := range strings.Split(string(out), "\n") {
					if strings.Contains(line, "Logged in to") {
						detail = strings.TrimSpace(line)
						break
					}
				}
				results = append(results, DoctorResult{Name: name, OK: true, Detail: detail})
			}
		}
	}

	// 10. memory-mcp
	memStatus := MemoryStatus(targetDir)
	if memStatus.Installed {
		if memStatus.Running {
			results = append(results, DoctorResult{Name: "memory-mcp", OK: true, Detail: fmt.Sprintf("running (port %d, health: %s)", memStatus.Port, memStatus.Health)})
		} else {
			results = append(results, DoctorResult{Name: "memory-mcp", OK: false, Detail: "installed but not running", Fix: "koda memory start"})
		}
	}

	// 11. container runtime
	if rt := ContainerRuntime(); rt != "" {
		results = append(results, DoctorResult{Name: "container-runtime", OK: true, Detail: rt})
	} else if memStatus.Installed {
		results = append(results, DoctorResult{Name: "container-runtime", OK: false, Detail: "not found (needed for memory-mcp)", Fix: "Install docker, podman, or nerdctl"})
	}

	// 12. yax persistent memory
	if yaxBin := findYax(); yaxBin != "" {
		verOut, _ := exec.Command(yaxBin, "version").Output()
		ver := strings.TrimSpace(string(verOut))
		statsOut, _ := exec.Command(yaxBin, "stats").Output()
		detail := ver
		if len(statsOut) > 0 {
			var stats struct {
				Observations int `json:"total_observations"`
				Sessions     int `json:"total_sessions"`
				Edges        int `json:"total_edges"`
				Prompts      int `json:"total_prompts"`
			}
			if json.Unmarshal(statsOut, &stats) == nil {
				detail += fmt.Sprintf(" — %d observations, %d sessions, %d edges, %d prompts",
					stats.Observations, stats.Sessions, stats.Edges, stats.Prompts)
			}
		}
		results = append(results, DoctorResult{Name: "yax", OK: true, Detail: detail + " (" + yaxBin + ")"})
	} else {
		results = append(results, DoctorResult{Name: "yax", OK: false, Detail: "not installed (persistent memory disabled)", Fix: "koda upgrade"})
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
