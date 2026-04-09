package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// KiroIDEResult holds install/sync counts.
type KiroIDEResult struct {
	Steering int
	Skills   int
	Hooks    int
	MCP      int
}

// InstallKiroIDE installs steering + skills (user-level) and hooks (workspace-level).
func InstallKiroIDE(steerRoot, workspaceDir string) (KiroIDEResult, error) {
	var r KiroIDEResult

	// Steering + skills → ~/.kiro/ (user-level)
	kiroRoot := config.KiroRoot()
	r.Steering = installSteering(steerRoot, kiroRoot)
	r.Skills = installSkills(steerRoot, kiroRoot)

	// Hooks → <workspace>/.kiro/hooks/ (workspace-level)
	if workspaceDir != "" {
		var err error
		r.Hooks, err = installKiroHooks(workspaceDir)
		if err != nil {
			return r, err
		}
	}

	// MCP bundles + mcp.json
	r.MCP = CopyMcpBundles(steerRoot)
	GenerateMcpJson(FindNodeExe())

	return r, nil
}

// SyncKiroIDE updates steering + skills from latest profiles.
func SyncKiroIDE(steerRoot string) KiroIDEResult {
	kiroRoot := config.KiroRoot()
	return KiroIDEResult{
		Steering: installSteering(steerRoot, kiroRoot),
		Skills:   installSkills(steerRoot, kiroRoot),
	}
}

// RemoveKiroIDE removes hooks from a workspace directory.
func RemoveKiroIDE(workspaceDir string) int {
	removed := 0
	for _, sub := range []string{"hooks"} {
		p := filepath.Join(workspaceDir, ".kiro", sub)
		if _, err := os.Stat(p); err == nil {
			os.RemoveAll(p)
			removed++
		}
	}
	return removed
}

func installSteering(steerRoot, targetRoot string) int {
	dst := filepath.Join(targetRoot, "steering")
	os.MkdirAll(dst, 0755)
	count := 0
	for _, profile := range []string{"dev-core", "dev-web"} {
		src := filepath.Join(steerRoot, "profiles", profile, "steering")
		entries, err := os.ReadDir(src)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
			count++
		}
	}
	return count
}

func installSkills(steerRoot, targetRoot string) int {
	dst := filepath.Join(targetRoot, "skills")
	os.MkdirAll(dst, 0755)
	count := 0
	// Common skills
	for _, dir := range []string{
		filepath.Join(steerRoot, "common", "skills"),
		filepath.Join(steerRoot, "profiles", "dev-web", "skills"),
	} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || e.Name() == "README.md" {
				continue
			}
			copyFile(filepath.Join(dir, e.Name()), filepath.Join(dst, e.Name()))
			count++
		}
	}
	return count
}

var kiroHooks = []struct {
	File    string
	Content string
}{
	{"guard-writes.kiro.hook", `{"name":"Guard Sensitive Paths","version":"1.0.0","description":"Blocks file writes to node_modules/, dist/, and .git/ directories.","when":{"type":"preToolUse","toolTypes":["write"]},"then":{"type":"askAgent","prompt":"Check if the file being written is inside node_modules/, dist/, or .git/ directories. If it is, REFUSE to proceed."}}`},
	{"secret-scan.kiro.hook", `{"name":"Secret Scan on Write","version":"1.0.0","description":"Scans for hardcoded secrets before writing files.","when":{"type":"preToolUse","toolTypes":["write"]},"then":{"type":"askAgent","prompt":"Scan the content being written for potential hardcoded secrets. If a real secret is detected, REFUSE the write and instruct to use environment variables instead."}}`},
	{"branch-guard.kiro.hook", `{"name":"Branch Guard","version":"1.0.0","description":"Blocks direct git commit, push, or merge on main/master branch.","when":{"type":"preToolUse","toolTypes":["shell"]},"then":{"type":"askAgent","prompt":"Check if the shell command involves git commit, git push, or git merge. If the current branch is main or master, REFUSE and instruct to use a feature branch. Read-only git commands are always allowed."}}`},
	{"warn-destructive.kiro.hook", `{"name":"Warn on Destructive Commands","version":"1.0.0","description":"Warns after destructive commands like rm -rf, DROP TABLE, or --force.","when":{"type":"postToolUse","toolTypes":["shell"]},"then":{"type":"askAgent","prompt":"If the command contained destructive patterns like rm -rf, DROP TABLE, DELETE FROM, or --force, warn the user. Otherwise do nothing."}}`},
}

func installKiroHooks(workspaceDir string) (int, error) {
	hooksDir := filepath.Join(workspaceDir, ".kiro", "hooks")
	os.MkdirAll(hooksDir, 0755)
	for _, h := range kiroHooks {
		os.WriteFile(filepath.Join(hooksDir, h.File), []byte(h.Content), 0644)
	}
	// Auto-add to .gitignore
	addToGitignore(workspaceDir, ".kiro/hooks/")
	return len(kiroHooks), nil
}

func addToGitignore(dir, pattern string) {
	gi := filepath.Join(dir, ".gitignore")
	data, _ := os.ReadFile(gi)
	if strings.Contains(string(data), pattern) {
		return
	}
	f, err := os.OpenFile(gi, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	if len(data) > 0 && data[len(data)-1] != '\n' {
		f.WriteString("\n")
	}
	f.WriteString("\n# Kiro IDE hooks\n" + pattern + "\n")
}

// FindNodeExe returns the absolute path to node, resolving fnm/nvm stable paths on Windows.
func FindNodeExe() string {
	if runtime.GOOS == "windows" {
		// fnm stable path (not ephemeral multishell)
		appdata := os.Getenv("APPDATA")
		if appdata != "" {
			versions := filepath.Join(appdata, "fnm", "node-versions")
			if entries, err := os.ReadDir(versions); err == nil {
				for i := len(entries) - 1; i >= 0; i-- {
				if !strings.HasPrefix(entries[i].Name(), "v") {
					continue
				}
					candidate := filepath.Join(versions, entries[i].Name(), "installation", "node.exe")
					if _, err := os.Stat(candidate); err == nil {
						return candidate
					}
				}
			}
		}
		// PATH (skip fnm multishell)
		if p, err := exec.LookPath("node"); err == nil && !strings.Contains(p, "fnm_multishells") {
			return p
		}
		// Program Files
		pf := `C:\Program Files\nodejs\node.exe`
		if _, err := os.Stat(pf); err == nil {
			return pf
		}
		return "node"
	}
	// Unix: just use PATH
	if p, err := exec.LookPath("node"); err == nil {
		return p
	}
	return "node"
}

// FindUvxExe returns the absolute path to uvx, or empty string if not found.
func FindUvxExe() string {
	if p, err := exec.LookPath("uvx"); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	local := filepath.Join(home, ".local", "bin", "uvx")
	if runtime.GOOS == "windows" {
		local += ".exe"
	}
	if _, err := os.Stat(local); err == nil {
		return local
	}
	return ""
}

// KiroIDEStatus checks if Kiro IDE files are installed.
type KiroIDEStatus struct {
	SteeringCount  int
	SkillsCount    int
	HooksInstalled bool
	WorkspaceDir   string
}

// CheckKiroIDE returns the current Kiro IDE installation status.
func CheckKiroIDE(workspaceDir string) KiroIDEStatus {
	kiroRoot := config.KiroRoot()
	var s KiroIDEStatus
	s.WorkspaceDir = workspaceDir
	if entries, err := os.ReadDir(filepath.Join(kiroRoot, "steering")); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".md") {
				s.SteeringCount++
			}
		}
	}
	if entries, err := os.ReadDir(filepath.Join(kiroRoot, "skills")); err == nil {
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".md") {
				s.SkillsCount++
			}
		}
	}
	if workspaceDir != "" {
		hooksDir := filepath.Join(workspaceDir, ".kiro", "hooks")
		if entries, err := os.ReadDir(hooksDir); err == nil {
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".kiro.hook") {
					s.HooksInstalled = true
					break
				}
			}
		}
	}
	return s
}
