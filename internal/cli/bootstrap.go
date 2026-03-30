package cli

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
)

func cloneSteerRuntime() error {
	settings := config.ReadSteerSettings()
	dir := config.DefaultSteerRoot()
	repo := settings.Repo
	branch := settings.Branch

	fmt.Printf("   Target: %s\n", dir)
	fmt.Printf("   Repo:   %s\n", repo)
	fmt.Printf("   Branch: %s\n\n", branch)

	// Step 1: Try gh repo clone
	fmt.Println("   [1/3] Trying: gh repo clone...")
	ghCmd := exec.Command("gh", "repo", "clone", repo, dir,
		"--", "--branch", branch, "--single-branch")
	ghCmd.Env = append(ghCmd.Environ(), "GH_HOST="+config.GHHost)
	if out, err := ghCmd.CombinedOutput(); err == nil {
		fmt.Printf("   ✅ Cloned via gh\n\n")
		saveCloneSuccess(settings)
		return nil
	} else {
		fmt.Printf("   ✗ gh clone failed: %s\n", strings.TrimSpace(string(out)))
	}

	// Step 2: Try git clone via HTTPS
	fmt.Println("   [2/3] Trying: git clone (HTTPS)...")
	gitURL := fmt.Sprintf("https://%s/%s.git", config.GHHost, repo)
	gitCmd := exec.Command("git", "clone", "--branch", branch, "--single-branch", gitURL, dir)
	if out, err := gitCmd.CombinedOutput(); err == nil {
		fmt.Printf("   ✅ Cloned via git\n\n")
		saveCloneSuccess(settings)
		return nil
	} else {
		fmt.Printf("   ✗ git clone failed: %s\n", strings.TrimSpace(string(out)))
	}

	// Step 3: Try git clone via SSH
	fmt.Println("   [3/3] Trying: git clone (SSH)...")
	sshURL := fmt.Sprintf("git@%s:%s.git", config.GHHost, repo)
	sshCmd := exec.Command("git", "clone", "--branch", branch, "--single-branch", sshURL, dir)
	if out, err := sshCmd.CombinedOutput(); err == nil {
		fmt.Printf("   ✅ Cloned via SSH\n\n")
		saveCloneSuccess(settings)
		return nil
	} else {
		fmt.Printf("   ✗ SSH clone failed: %s\n", strings.TrimSpace(string(out)))
	}

	// All methods failed
	return fmt.Errorf("all clone methods failed\n\n" +
		"To fix, authenticate GitHub CLI for Disney Enterprise:\n\n" +
		"  gh auth login --hostname " + config.GHHost + "\n\n" +
		"  Select: HTTPS, authenticate via browser.\n")
}

func saveCloneSuccess(settings config.SteerSettings) {
	settings.LastSync = time.Now().UTC().Format(time.RFC3339)
	config.SaveSteerSettings(settings)
}
