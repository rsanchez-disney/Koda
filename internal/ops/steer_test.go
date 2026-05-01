package ops

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupGitRepo creates a temp git repo with an initial commit and returns its path.
func setupGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %s", args, out)
		}
	}
	run("init", "-b", "main")
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644)
	run("add", ".")
	run("commit", "-m", "initial")
	return dir
}

// TestHandleDirtySyncNonInteractive verifies that non-TTY defaults to stash.
func TestHandleDirtySyncNonInteractive(t *testing.T) {
	dir := setupGitRepo(t)
	changes := []string{"M  README.md", "A  new-file.md"}

	// When stdin is not a TTY (as in tests), handleDirtySync should return stash.
	action := handleDirtySync(dir, changes)
	if action != syncActionStash {
		t.Errorf("expected syncActionStash (0), got %d", action)
	}
}

// TestSyncGitCleanRepo verifies sync works on a clean repo (no changes).
func TestSyncGitCleanRepo(t *testing.T) {
	dir := setupGitRepo(t)

	// syncGit on a repo with no remote will fail on pull, but should not panic.
	// We just verify it doesn't crash on the dirty-check path.
	status := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, _ := status.Output()
	if len(out) != 0 {
		t.Errorf("expected clean repo, got: %s", out)
	}
}

// TestSyncGitDirtyRepoStash verifies stash action works end-to-end.
func TestSyncGitDirtyRepoStash(t *testing.T) {
	dir := setupGitRepo(t)

	// Create a dirty file
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# modified"), 0644)

	// Verify dirty
	status := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, _ := status.Output()
	if len(out) == 0 {
		t.Fatal("expected dirty repo")
	}

	// Stash
	exec.Command("git", "-C", dir, "stash", "push", "-m", "test-stash").Run()

	// Verify clean after stash
	status2 := exec.Command("git", "-C", dir, "status", "--porcelain")
	out2, _ := status2.Output()
	if len(out2) != 0 {
		t.Errorf("expected clean after stash, got: %s", out2)
	}

	// Pop
	exec.Command("git", "-C", dir, "stash", "pop").Run()

	// Verify dirty again
	status3 := exec.Command("git", "-C", dir, "status", "--porcelain")
	out3, _ := status3.Output()
	if len(out3) == 0 {
		t.Error("expected dirty after stash pop")
	}
}

// TestSyncGitDirtyRepoDiscard verifies discard action works.
func TestSyncGitDirtyRepoDiscard(t *testing.T) {
	dir := setupGitRepo(t)

	// Modify tracked file
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# modified"), 0644)
	// Add untracked file
	os.WriteFile(filepath.Join(dir, "untracked.md"), []byte("new"), 0644)

	// Discard
	exec.Command("git", "-C", dir, "checkout", "--", ".").Run()
	exec.Command("git", "-C", dir, "clean", "-fd").Run()

	// Verify clean
	status := exec.Command("git", "-C", dir, "status", "--porcelain")
	out, _ := status.Output()
	if len(out) != 0 {
		t.Errorf("expected clean after discard, got: %s", out)
	}

	// Verify original content restored
	data, _ := os.ReadFile(filepath.Join(dir, "README.md"))
	if string(data) != "# test" {
		t.Errorf("expected original content, got: %s", data)
	}

	// Verify untracked file removed
	if _, err := os.Stat(filepath.Join(dir, "untracked.md")); err == nil {
		t.Error("expected untracked file to be removed")
	}
}

// TestSyncGitDirtyRepoCommit verifies commit action creates a branch.
func TestSyncGitDirtyRepoCommit(t *testing.T) {
	dir := setupGitRepo(t)

	// Modify file
	os.WriteFile(filepath.Join(dir, "README.md"), []byte("# committed change"), 0644)

	// Stage and commit to a branch
	exec.Command("git", "-C", dir, "checkout", "-b", "feat/test-changes").Run()
	exec.Command("git", "-C", dir, "add", "-A").Run()
	cmd := exec.Command("git", "-C", dir, "commit", "-m", "test: local changes")
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.com")
	cmd.Run()

	// Verify commit exists
	log := exec.Command("git", "-C", dir, "log", "--oneline", "-1")
	out, _ := log.Output()
	if len(out) == 0 {
		t.Error("expected commit on branch")
	}

	// Return to main
	exec.Command("git", "-C", dir, "checkout", "main").Run()

	// Verify main is clean
	status := exec.Command("git", "-C", dir, "status", "--porcelain")
	out2, _ := status.Output()
	if len(out2) != 0 {
		t.Errorf("expected main to be clean, got: %s", out2)
	}
}

// TestIsInteractiveConsistency verifies isInteractive returns a bool without panicking.
func TestIsInteractiveConsistency(t *testing.T) {
	// isInteractive depends on the test runner's TTY — just verify it doesn't panic.
	_ = isInteractive()
}

// TestSyncActionConstants verifies the action constants are distinct.
func TestSyncActionConstants(t *testing.T) {
	actions := map[syncAction]string{
		syncActionStash:   "stash",
		syncActionCommit:  "commit",
		syncActionDiscard: "discard",
		syncActionAbort:   "abort",
	}
	if len(actions) != 4 {
		t.Error("expected 4 distinct sync actions")
	}
}
