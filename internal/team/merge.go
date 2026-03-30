package team

import (
	"fmt"
	"os/exec"
	"strings"
)

// ConflictReport lists files modified by multiple workers.
type ConflictReport struct {
	Overlaps []FileOverlap
}

// FileOverlap is a file touched by more than one worker.
type FileOverlap struct {
	Path    string
	Workers []string
}

// DetectConflicts compares changed files across workers.
func DetectConflicts(t *Team) ConflictReport {
	fileWorkers := map[string][]string{}

	for _, id := range t.WorkerOrder {
		w := t.Workers[id]
		if w.GetState() != StateCompleted || w.WorktreePath == "" {
			continue
		}
		// Get changed files in worktree
		cmd := exec.Command("git", "diff", "--name-only", t.Spec.BaseBranch+"..HEAD")
		cmd.Dir = w.WorktreePath
		out, err := cmd.Output()
		if err != nil {
			continue
		}
		for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				fileWorkers[line] = append(fileWorkers[line], id)
			}
		}
	}

	var overlaps []FileOverlap
	for path, workers := range fileWorkers {
		if len(workers) > 1 {
			overlaps = append(overlaps, FileOverlap{Path: path, Workers: workers})
		}
	}
	return ConflictReport{Overlaps: overlaps}
}

// Merge executes the configured merge strategy.
func Merge(t *Team) error {
	switch t.Spec.MergeStrategy {
	case "rebase-chain":
		return mergeRebaseChain(t)
	case "parallel-merge":
		return mergeParallel(t)
	case "pr-per-worker":
		return mergePRPerWorker(t)
	default:
		return mergeRebaseChain(t)
	}
}

func mergeRebaseChain(t *Team) error {
	for _, id := range t.WorkerOrder {
		w := t.Workers[id]
		if w.GetState() != StateCompleted {
			continue
		}
		fmt.Printf("  Rebasing %s onto %s...\n", w.Branch, t.Spec.BaseBranch)
		// Checkout base and merge worker branch
		cmd := exec.Command("git", "checkout", t.Spec.BaseBranch)
		cmd.Dir = t.RepoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("checkout %s: %s", t.Spec.BaseBranch, string(out))
		}

		cmd = exec.Command("git", "merge", "--no-ff", w.Branch, "-m", fmt.Sprintf("merge: %s (%s)", w.Role, w.ID))
		cmd.Dir = t.RepoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("merge %s: %s", w.Branch, string(out))
		}
		fmt.Printf("  ✓ %s merged\n", w.Role)
	}
	return nil
}

func mergeParallel(t *Team) error {
	cmd := exec.Command("git", "checkout", t.Spec.BaseBranch)
	cmd.Dir = t.RepoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("checkout: %s", string(out))
	}

	for _, id := range t.WorkerOrder {
		w := t.Workers[id]
		if w.GetState() != StateCompleted {
			continue
		}
		cmd := exec.Command("git", "merge", w.Branch, "-m", fmt.Sprintf("merge: %s", w.Role))
		cmd.Dir = t.RepoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("merge %s: %s", w.Branch, string(out))
		}
		fmt.Printf("  ✓ %s merged\n", w.Role)
	}
	return nil
}

func mergePRPerWorker(t *Team) error {
	for _, id := range t.WorkerOrder {
		w := t.Workers[id]
		if w.GetState() != StateCompleted {
			continue
		}
		// Push branch
		cmd := exec.Command("git", "push", "origin", w.Branch)
		cmd.Dir = w.WorktreePath
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("  ⚠ push %s: %s\n", w.Branch, string(out))
			continue
		}

		// Create PR via gh
		title := fmt.Sprintf("%s: %s", w.Role, t.Goal)
		body := ExtractResult(w.Result)
		cmd = exec.Command("gh", "pr", "create",
			"--base", t.Spec.BaseBranch,
			"--head", w.Branch,
			"--title", title,
			"--body", body)
		cmd.Dir = t.RepoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("  ⚠ PR for %s: %s\n", w.Role, string(out))
		} else {
			fmt.Printf("  ✓ PR created for %s: %s\n", w.Role, strings.TrimSpace(string(out)))
		}
	}
	return nil
}

// CleanupAfterMerge removes worktrees and branches.
func CleanupAfterMerge(t *Team) {
	if t.Worktrees != nil {
		t.Worktrees.CleanupTeam(t.ID)
	}
	fmt.Println("  ✓ Worktrees cleaned up")
}
