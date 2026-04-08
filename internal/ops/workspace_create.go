package ops

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/model"
)

// RepoInfo represents a discovered or manually added repository.
type RepoInfo struct {
	Repo  string // org/name
	Name  string // derived repo name
	Local bool   // exists on disk
}

// ScanRepos discovers git repos under a directory and extracts their org/name.
func ScanRepos(dir string) []RepoInfo {
	dir = expandHome(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var repos []RepoInfo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		gitDir := filepath.Join(dir, e.Name(), ".git")
		if _, err := os.Stat(gitDir); err != nil {
			continue
		}
		repo := repoFromRemote(filepath.Join(dir, e.Name()))
		if repo == "" {
			repo = e.Name()
		}
		repos = append(repos, RepoInfo{Repo: repo, Name: e.Name(), Local: true})
	}
	return repos
}

// CreateWorkspace scaffolds a workspace directory and writes workspace.json.
func CreateWorkspace(steerRoot string, ws model.Workspace) error {
	wsDir := filepath.Join(steerRoot, config.WorkspacesDir, ws.Name)
	for _, sub := range []string{"rules", "context"} {
		os.MkdirAll(filepath.Join(wsDir, sub), 0755)
	}
	data, _ := json.MarshalIndent(ws, "", "  ")
	return os.WriteFile(filepath.Join(wsDir, "workspace.json"), data, 0644)
}

// CloneWorkspaceRepos clones repos that don't exist locally yet.
func CloneWorkspaceRepos(ws model.Workspace) (cloned int, errors []string) {
	basePath := expandHome(ws.WorkspacePath)
	if basePath == "" {
		return 0, nil
	}
	os.MkdirAll(basePath, 0755)
	for _, p := range ws.Projects {
		if p.Repo == "" {
			continue
		}
		dest := filepath.Join(basePath, p.Name)
		if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
			continue // already cloned
		}
		url := fmt.Sprintf("git@%s:%s.git", config.GHHost, p.Repo)
		cmd := exec.Command("git", "clone", url, dest)
		if err := cmd.Run(); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", p.Name, err))
		} else {
			cloned++
		}
	}
	return
}

func repoFromRemote(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return parseRepoFromURL(strings.TrimSpace(string(out)))
}

func parseRepoFromURL(url string) string {
	// Handle SSH: git@github.disney.com:ORG/REPO.git
	if i := strings.Index(url, ":"); i > 0 && strings.Contains(url[:i], "@") {
		url = url[i+1:]
	} else {
		// Handle HTTPS: https://github.disney.com/ORG/REPO.git
		url = strings.TrimPrefix(url, "https://")
		url = strings.TrimPrefix(url, "http://")
		// Remove host
		if i := strings.Index(url, "/"); i >= 0 {
			url = url[i+1:]
		}
	}
	url = strings.TrimSuffix(url, ".git")
	return url
}

// PublishWorkspaceToUpstream publishes a workspace from a tarball install by
// temporarily initializing git, creating a branch + PR against upstream, then cleaning up.
func PublishWorkspaceToUpstream(steerRoot, wsName string, isEdit bool) (string, error) {
	gitDir := filepath.Join(steerRoot, ".git")
	hadGit := false
	if _, err := os.Stat(gitDir); err == nil {
		hadGit = true
	}

	// Init git + add upstream remote
	if !hadGit {
		upstreamURL := fmt.Sprintf("https://%s/%s.git", config.GHHost, config.DefaultSteerRepo)
		exec.Command("git", "-C", steerRoot, "init").Run()
		exec.Command("git", "-C", steerRoot, "remote", "add", "origin", upstreamURL).Run()
		exec.Command("git", "-C", steerRoot, "add", ".").Run()
		exec.Command("git", "-C", steerRoot, "commit", "-m", "tarball baseline").Run()
	}

	prURL, err := PublishWorkspace(steerRoot, wsName, isEdit)

	// Clean up .git if we created it
	if !hadGit {
		os.RemoveAll(gitDir)
	}

	return prURL, err
}

func expandHome(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// PublishWorkspace creates a branch, commits workspace files, pushes, and opens a PR.
// Returns the PR URL on success.
func PublishWorkspace(steerRoot, wsName string, isEdit bool) (string, error) {
	branch := "workspace/" + wsName
	verb := "add"
	if isEdit {
		verb = "update"
	}
	msg := "feat: " + verb + " " + wsName + " workspace"
	wsPath := "workspaces/" + wsName + "/"

	// Create branch from current HEAD
	if err := exec.Command("git", "-C", steerRoot, "checkout", "-b", branch).Run(); err != nil {
		return "", fmt.Errorf("branch create failed: %w", err)
	}

	// Commit workspace files
	exec.Command("git", "-C", steerRoot, "add", wsPath).Run()
	if err := exec.Command("git", "-C", steerRoot, "commit", "-m", msg).Run(); err != nil {
		exec.Command("git", "-C", steerRoot, "checkout", "main").Run()
		return "", fmt.Errorf("commit failed: %w", err)
	}

	// Push branch
	if err := exec.Command("git", "-C", steerRoot, "push", "-u", "origin", branch).Run(); err != nil {
		exec.Command("git", "-C", steerRoot, "checkout", "main").Run()
		return "", fmt.Errorf("push failed: %w", err)
	}

	// Create PR
	cmd := exec.Command("gh", "pr", "create",
		"--title", msg,
		"--body", fmt.Sprintf("Updates team workspace `%s` with profiles, rules, and repo configuration.", wsName),
		"--base", "main",
	)
	cmd.Dir = steerRoot
	out, err := cmd.Output()
	prURL := strings.TrimSpace(string(out))

	// Return to main
	exec.Command("git", "-C", steerRoot, "checkout", "main").Run()

	if err != nil {
		return "", fmt.Errorf("PR create failed: %w — branch pushed, create PR manually", err)
	}
	return prURL, nil
}
