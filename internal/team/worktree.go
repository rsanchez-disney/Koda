package team

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// WorktreeEntry tracks a single worktree.
type WorktreeEntry struct {
	WorkerID string `json:"worker_id"`
	TeamID   string `json:"team_id"`
	Path     string `json:"path"`
	Branch   string `json:"branch"`
}

// WorktreeRegistry persists worktree state to .koda/state.json.
type WorktreeRegistry struct {
	Worktrees []WorktreeEntry `json:"worktrees"`
}

// GitWorktreeManager handles worktree lifecycle.
type GitWorktreeManager struct {
	RepoRoot string
	BaseDir  string // .koda/worktrees/
	Registry WorktreeRegistry
	regPath  string
}

// NewWorktreeManager creates a manager for the given repo.
func NewWorktreeManager(repoRoot string) *GitWorktreeManager {
	baseDir := filepath.Join(repoRoot, ".koda", "worktrees")
	regPath := filepath.Join(repoRoot, ".koda", "state.json")
	os.MkdirAll(baseDir, 0755)

	m := &GitWorktreeManager{
		RepoRoot: repoRoot,
		BaseDir:  baseDir,
		regPath:  regPath,
	}
	m.loadRegistry()
	return m
}

// Create adds a new worktree for a worker.
func (m *GitWorktreeManager) Create(teamID, workerID, baseBranch string) (string, string, error) {
	branch := fmt.Sprintf("koda/team-%s/worker-%s", teamID, workerID)
	wtPath := filepath.Join(m.BaseDir, teamID, workerID)

	// Create worktree with new branch
	cmd := exec.Command("git", "worktree", "add", "-b", branch, wtPath, baseBranch)
	cmd.Dir = m.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", "", fmt.Errorf("git worktree add failed: %s", strings.TrimSpace(string(out)))
	}

	// Register
	m.Registry.Worktrees = append(m.Registry.Worktrees, WorktreeEntry{
		WorkerID: workerID,
		TeamID:   teamID,
		Path:     wtPath,
		Branch:   branch,
	})
	m.saveRegistry()

	return wtPath, branch, nil
}

// Remove cleans up a worktree.
func (m *GitWorktreeManager) Remove(teamID, workerID string) error {
	wtPath := filepath.Join(m.BaseDir, teamID, workerID)

	cmd := exec.Command("git", "worktree", "remove", "--force", wtPath)
	cmd.Dir = m.RepoRoot
	cmd.CombinedOutput()

	// Remove branch
	branch := fmt.Sprintf("koda/team-%s/worker-%s", teamID, workerID)
	exec.Command("git", "-C", m.RepoRoot, "branch", "-D", branch).Run()

	// Update registry
	var kept []WorktreeEntry
	for _, e := range m.Registry.Worktrees {
		if !(e.TeamID == teamID && e.WorkerID == workerID) {
			kept = append(kept, e)
		}
	}
	m.Registry.Worktrees = kept
	m.saveRegistry()
	return nil
}

// CleanupTeam removes all worktrees for a team.
func (m *GitWorktreeManager) CleanupTeam(teamID string) {
	var toRemove []WorktreeEntry
	for _, e := range m.Registry.Worktrees {
		if e.TeamID == teamID {
			toRemove = append(toRemove, e)
		}
	}
	for _, e := range toRemove {
		m.Remove(e.TeamID, e.WorkerID)
	}
}

// List returns all registered worktrees.
func (m *GitWorktreeManager) List() []WorktreeEntry {
	return m.Registry.Worktrees
}

func (m *GitWorktreeManager) loadRegistry() {
	data, err := os.ReadFile(m.regPath)
	if err != nil {
		return
	}
	json.Unmarshal(data, &m.Registry)
}

func (m *GitWorktreeManager) saveRegistry() {
	os.MkdirAll(filepath.Dir(m.regPath), 0755)
	data, _ := json.MarshalIndent(m.Registry, "", "  ")
	os.WriteFile(m.regPath, append(data, '\n'), 0644)
}
