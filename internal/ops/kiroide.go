package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// steeringMap represents a steering-map.json file.
type steeringMap struct {
	Mappings []steeringMapping `json:"mappings"`
}

// steeringMapping maps a context file to a Kiro IDE steering file.
type steeringMapping struct {
	Context          string   `json:"context"`
	Steering         string   `json:"steering"`
	Inclusion        string   `json:"inclusion"`
	FileMatchPattern []string `json:"fileMatchPattern,omitempty"`
}

// KiroIDEResult holds install/sync counts.
type KiroIDEResult struct {
	Steering int
	Skills   int
	Hooks    int
	MCP      int
	Agents   int
	Prompts  int
	Context  int
	Rules    int
}

// resolveKiroProfiles returns the profile IDs to use for kiro-ide operations.
// Priority: explicit args > active workspace profiles > all discovered profiles.
// Returns an error if any explicit profile name doesn't match a discovered profile.
func resolveKiroProfiles(steerRoot string, explicit []string) ([]string, error) {
	available, _ := config.ProfileDirs(steerRoot)
	valid := make(map[string]bool, len(available))
	for _, d := range available {
		valid[filepath.Base(d)] = true
	}

	if len(explicit) > 0 {
		expanded := ExpandAliases(explicit)
		for _, p := range expanded {
			if !valid[p] {
				return nil, fmt.Errorf("unknown profile: %s", p)
			}
		}
		return expanded, nil
	}
	s := config.ReadSteerSettings()
	if s.ActiveWorkspace != "" {
		if ws, err := GetWorkspace(steerRoot, s.ActiveWorkspace); err == nil && len(ws.Profiles) > 0 {
			return ExpandAliases(ws.Profiles), nil
		}
	}
	// Fallback: all discovered profiles
	var all []string
	for _, d := range available {
		all = append(all, filepath.Base(d))
	}
	return all, nil
}

// InstallKiroIDE installs steering + skills (user-level), agents + prompts +
// context + rules + hooks (workspace-level), and MCP config.
func InstallKiroIDE(steerRoot, workspaceDir string, profiles []string) (KiroIDEResult, error) {
	var r KiroIDEResult
	selected, err := resolveKiroProfiles(steerRoot, profiles)
	if err != nil {
		return r, err
	}

	// Steering + skills → ~/.kiro/ (user-level)
	kiroRoot := config.KiroRoot()
	r.Steering = installSteering(steerRoot, kiroRoot, selected)
	r.Skills = installSkills(steerRoot, kiroRoot, selected)

	// Workspace-level → <project>/.kiro/
	if workspaceDir != "" {
		wsDotKiro := filepath.Join(workspaceDir, ".kiro")
		r.Agents, r.Prompts = installKiroAgents(steerRoot, wsDotKiro, selected)
		r.Context = installKiroContext(steerRoot, wsDotKiro, selected)
		r.Rules = installKiroRules(steerRoot, wsDotKiro)
		var err error
		r.Hooks, err = installKiroHooks(workspaceDir)
		if err != nil {
			return r, err
		}
		addToGitignore(workspaceDir, ".kiro/agents/")
		addToGitignore(workspaceDir, ".kiro/prompts/")
		addToGitignore(workspaceDir, ".kiro/context/")
	}

	// MCP bundles + mcp.json
	r.MCP = CopyMcpBundles(steerRoot)
	GenerateMcpJson(FindNodeExe())

	return r, nil
}

// SyncKiroIDE updates steering + skills (user-level) and agents + prompts +
// context + rules (workspace-level) from latest profiles.
func SyncKiroIDE(steerRoot, workspaceDir string, profiles []string) (KiroIDEResult, error) {
	selected, err := resolveKiroProfiles(steerRoot, profiles)
	if err != nil {
		return KiroIDEResult{}, err
	}
	kiroRoot := config.KiroRoot()
	r := KiroIDEResult{
		Steering: installSteering(steerRoot, kiroRoot, selected),
		Skills:   installSkills(steerRoot, kiroRoot, selected),
	}
	if workspaceDir != "" {
		wsDotKiro := filepath.Join(workspaceDir, ".kiro")
		r.Agents, r.Prompts = installKiroAgents(steerRoot, wsDotKiro, selected)
		r.Context = installKiroContext(steerRoot, wsDotKiro, selected)
		r.Rules = installKiroRules(steerRoot, wsDotKiro)
	}
	return r, nil
}

// RemoveKiroIDE removes generated .kiro content from a workspace directory.
func RemoveKiroIDE(workspaceDir string) int {
	removed := 0
	for _, sub := range []string{"hooks", "agents", "prompts", "context"} {
		p := filepath.Join(workspaceDir, ".kiro", sub)
		if _, err := os.Stat(p); err == nil {
			os.RemoveAll(p)
			removed++
		}
	}
	return removed
}

// installKiroAgents copies profile agents + prompts into <project>/.kiro/ with
// resource paths rewritten from file://~/.kiro/ to file://.kiro/ for Kiro IDE.
// Orphaned agent/prompt files from previously installed profiles are removed.
func installKiroAgents(steerRoot, dotKiro string, profiles []string) (int, int) {
	agentsDst := filepath.Join(dotKiro, config.AgentsDir)
	promptsDst := filepath.Join(dotKiro, config.PromptsDir)
	os.MkdirAll(agentsDst, 0755)
	os.MkdirAll(promptsDst, 0755)

	home, _ := os.UserHomeDir()
	jsonHome := strings.ReplaceAll(home, "\\", "/")
	agents, prompts := 0, 0

	expectedAgents := make(map[string]bool)
	expectedPrompts := make(map[string]bool)

	for _, p := range profiles {
		srcDir, _ := ResolveProfileSource(steerRoot, p)

		// Agents: copy JSON with path rewriting
		entries, _ := os.ReadDir(filepath.Join(srcDir, config.AgentsDir))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") || strings.HasPrefix(e.Name(), "._") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(srcDir, config.AgentsDir, e.Name()))
			if err != nil {
				continue
			}
			content := strings.ReplaceAll(string(data), "$HOME", jsonHome)
			// Rewrite resource paths for Kiro IDE (relative to project)
			content = strings.ReplaceAll(content, "file://~/.kiro/", "file://.kiro/")
			content = strings.ReplaceAll(content, "file://"+home+"/.kiro/", "file://.kiro/")
			// Rewrite prompt paths to relative
			content = strings.ReplaceAll(content, jsonHome+"/.kiro/prompts/", "../prompts/")
			if runtime.GOOS == "windows" {
				content = strings.ReplaceAll(content, ".sh\"", ".ps1\"")
			}
			os.WriteFile(filepath.Join(agentsDst, e.Name()), []byte(content), 0644)
			expectedAgents[e.Name()] = true
			agents++
		}

		// Prompts
		entries, _ = os.ReadDir(filepath.Join(srcDir, config.PromptsDir))
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.HasPrefix(e.Name(), "._") {
				continue
			}
			copyFile(filepath.Join(srcDir, config.PromptsDir, e.Name()), filepath.Join(promptsDst, e.Name()))
			expectedPrompts[e.Name()] = true
			prompts++
		}
	}

	// Remove orphaned agents and prompts from previously installed profiles.
	if removed := cleanOrphans(agentsDst, expectedAgents, ".json"); removed > 0 {
		logf("  cleaned %d orphaned agent(s)\n", removed)
	}
	if removed := cleanOrphans(promptsDst, expectedPrompts, ".md"); removed > 0 {
		logf("  cleaned %d orphaned prompt(s)\n", removed)
	}

	return agents, prompts
}

// installKiroContext copies context files from shared + profiles into <project>/.kiro/context/.
// Orphaned context files from previously installed profiles are removed.
func installKiroContext(steerRoot, dotKiro string, profiles []string) int {
	dst := filepath.Join(dotKiro, config.ContextDir)
	os.MkdirAll(dst, 0755)

	// Track expected files for orphan cleanup.
	expected := make(map[string]bool)

	// Shared context first
	sharedSrc := filepath.Join(steerRoot, "shared", config.ContextDir)
	if entries, err := os.ReadDir(sharedSrc); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				expected[e.Name()] = true
			}
		}
	}
	copyDirContents(sharedSrc, dst)

	// Profile-specific context (profile wins over shared on conflict)
	for _, p := range profiles {
		srcDir, _ := ResolveProfileSource(steerRoot, p)
		src := filepath.Join(srcDir, config.ContextDir)
		entries, err := os.ReadDir(src)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || strings.HasPrefix(e.Name(), "._") {
				continue
			}
			copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
			expected[e.Name()] = true
		}
	}

	// Workspace service/channel banks
	s := config.ReadSteerSettings()
	if s.ActiveWorkspace != "" {
		if ws, err := GetWorkspace(steerRoot, s.ActiveWorkspace); err == nil {
			InstallBanks(steerRoot, dotKiro, ws.Services, ws.Channels)
			// Add bank-generated filenames to expected set directly.
			for _, svc := range ws.Services {
				expected["svc-"+svc+".md"] = true
			}
			for _, ch := range ws.Channels {
				expected["ch-"+ch+".md"] = true
			}
		}
	}

	// Remove orphaned context files (local- prefixed files are preserved).
	if removed := cleanOrphans(dst, expected, ""); removed > 0 {
		logf("  cleaned %d orphaned context file(s)\n", removed)
	}

	// Count actual files on disk (avoids double-counting overwrites)
	entries, _ := os.ReadDir(dst)
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			count++
		}
	}
	return count
}

// installKiroRules copies workspace-configured rules into <project>/.kiro/rules/.
func installKiroRules(steerRoot, dotKiro string) int {
	s := config.ReadSteerSettings()
	if s.ActiveWorkspace == "" {
		return 0
	}
	ws, err := GetWorkspace(steerRoot, s.ActiveWorkspace)
	if err != nil || len(ws.Rules) == 0 {
		return 0
	}
	return InstallRules(steerRoot, dotKiro, ws.Rules)
}

// installSteering installs steering files from selected profiles into targetRoot/steering/.
// Orphaned files from previously installed profiles are removed.
// Files prefixed with "local-" are preserved (user-managed).
func installSteering(steerRoot, targetRoot string, profiles []string) int {
	dst := filepath.Join(targetRoot, "steering")
	os.MkdirAll(dst, 0755)

	// Collect the set of steering files that SHOULD exist after this install.
	expected := make(map[string]bool)
	count := 0
	allowed := make(map[string]bool, len(profiles))
	for _, p := range profiles {
		allowed[p] = true
	}
	profileDirs, err := config.ProfileDirs(steerRoot)
	if err != nil {
		return 0
	}
	for _, profileDir := range profileDirs {
		if !allowed[filepath.Base(profileDir)] {
			continue
		}
		// Check for steering-map.json first
		mapFile := filepath.Join(profileDir, "steering-map.json")
		if _, err := os.Stat(mapFile); err == nil {
			generated, names := generateSteeringFromMap(profileDir, mapFile, dst)
			count += generated
			for _, n := range names {
				expected[n] = true
			}
			continue
		}
		// Fall back to copying steering/ directory
		src := filepath.Join(profileDir, config.SteeringDir)
		entries, err := os.ReadDir(src)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
				continue
			}
			copyFile(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name()))
			expected[e.Name()] = true
			count++
		}
	}

	// Remove orphaned steering files that no longer belong to any selected profile.
	if removed := cleanOrphans(dst, expected, ".md"); removed > 0 {
		logf("  cleaned %d orphaned steering file(s)\n", removed)
	}

	return count
}

// cleanOrphans removes files from dir that are not in the expected set.
// Only files matching the given suffix are considered (e.g. ".md", ".json").
// Pass an empty suffix to consider all files.
// Files prefixed with "local-" are always preserved (user-managed).
func cleanOrphans(dir string, expected map[string]bool, suffix string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if suffix != "" && !strings.HasSuffix(e.Name(), suffix) {
			continue
		}
		if strings.HasPrefix(e.Name(), "local-") {
			continue
		}
		if !expected[e.Name()] {
			logf("  removing orphan: %s\n", e.Name())
			os.Remove(filepath.Join(dir, e.Name()))
			removed++
		}
	}
	return removed
}

// cleanOrphanDirs removes subdirectories from dir that are not in the expected set.
func cleanOrphanDirs(dir string, expected map[string]bool) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	removed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if !expected[e.Name()] {
			logf("  removing orphan dir: %s\n", e.Name())
			os.RemoveAll(filepath.Join(dir, e.Name()))
			removed++
		}
	}
	return removed
}

// generateSteeringFromMap reads a steering-map.json and generates steering files
// from context files with Kiro IDE frontmatter prepended.
// Returns the count of files generated and the list of destination filenames.
func generateSteeringFromMap(profileDir, mapFile, dstDir string) (int, []string) {
	data, err := os.ReadFile(mapFile)
	if err != nil {
		return 0, nil
	}
	var sm steeringMap
	if err := json.Unmarshal(data, &sm); err != nil {
		return 0, nil
	}
	count := 0
	names := make([]string, 0, len(sm.Mappings))
	for _, m := range sm.Mappings {
		content, err := os.ReadFile(filepath.Join(profileDir, config.ContextDir, m.Context))
		if err != nil {
			continue
		}
		var frontmatter string
		if m.Inclusion == "fileMatch" && len(m.FileMatchPattern) > 0 {
			patternsJSON, _ := json.Marshal(m.FileMatchPattern)
			frontmatter = fmt.Sprintf("---\ninclusion: fileMatch\nfileMatchPattern: %s\n---\n\n", patternsJSON)
		} else {
			frontmatter = "---\ninclusion: always\n---\n\n"
		}
		os.WriteFile(filepath.Join(dstDir, m.Steering), append([]byte(frontmatter), content...), 0644)
		names = append(names, m.Steering)
		count++
	}
	return count, names
}

// installSkills installs skill files from common + selected profiles into targetRoot/skills/.
// Orphaned skill files and directories from previously installed profiles are removed.
// Files prefixed with "local-" are preserved (user-managed).
func installSkills(steerRoot, targetRoot string, profiles []string) int {
	dst := filepath.Join(targetRoot, "skills")
	os.MkdirAll(dst, 0755)
	count := 0
	allowed := make(map[string]bool, len(profiles))
	for _, p := range profiles {
		allowed[p] = true
	}

	// Track expected files and directories for orphan cleanup.
	expectedFiles := make(map[string]bool)
	expectedDirs := make(map[string]bool)

	// Collect skill source directories: common/skills + selected profiles' skills/
	skillDirs := []string{filepath.Join(steerRoot, "common", "skills")}
	if profileDirs, err := config.ProfileDirs(steerRoot); err == nil {
		for _, pd := range profileDirs {
			if allowed[filepath.Base(pd)] {
				skillDirs = append(skillDirs, filepath.Join(pd, config.SkillsDir))
			}
		}
	}

	for _, dir := range skillDirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			src := filepath.Join(dir, e.Name())
			if e.IsDir() {
				// Skill directory (SKILL.md + references/ + assets/)
				copySkillDir(src, filepath.Join(dst, e.Name()))
				expectedDirs[e.Name()] = true
				count++
			} else if strings.HasSuffix(e.Name(), ".md") && e.Name() != "README.md" {
				copyFile(src, filepath.Join(dst, e.Name()))
				expectedFiles[e.Name()] = true
				count++
			}
		}
	}

	// Remove orphaned skill files and directories.
	if removed := cleanOrphans(dst, expectedFiles, ".md"); removed > 0 {
		logf("  cleaned %d orphaned skill file(s)\n", removed)
	}
	if removed := cleanOrphanDirs(dst, expectedDirs); removed > 0 {
		logf("  cleaned %d orphaned skill dir(s)\n", removed)
	}

	return count
}

// copySkillDir recursively copies a skill directory (SKILL.md, references/, assets/).
func copySkillDir(src, dst string) {
	os.MkdirAll(dst, 0755)
	entries, err := os.ReadDir(src)
	if err != nil {
		return
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			copySkillDir(srcPath, dstPath)
		} else if !strings.HasPrefix(e.Name(), "._") {
			copyFile(srcPath, dstPath)
		}
	}
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
