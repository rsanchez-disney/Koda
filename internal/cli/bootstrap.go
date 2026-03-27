package cli

import (
	"fmt"
	"os/exec"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
)

func cloneSteerRuntime() error {
	settings := config.ReadSteerSettings()
	dir := config.DefaultSteerRoot()

	fmt.Printf("   Cloning %s (%s) to %s...\n", settings.Repo, settings.Branch, dir)

	// Try gh CLI first (respects GHE auth)
	cmd := exec.Command("gh", "repo", "clone",
		settings.Repo, dir,
		"--", "--branch", settings.Branch, "--single-branch")
	cmd.Env = append(cmd.Environ(), "GH_HOST="+config.GHHost)

	if output, err := cmd.CombinedOutput(); err != nil {
		// Fallback to git clone
		gitURL := fmt.Sprintf("git@%s:%s.git", config.GHHost, settings.Repo)
		cmd2 := exec.Command("git", "clone",
			"--branch", settings.Branch, "--single-branch",
			gitURL, dir)
		if output2, err2 := cmd2.CombinedOutput(); err2 != nil {
			return fmt.Errorf("clone failed\n\n" +
				"gh error: %s\n" +
				"git error: %s\n\n" +
				"To fix, authenticate GitHub CLI for Disney Enterprise:\n\n" +
				"  gh auth login --hostname github.disney.com\n\n" +
				"  Select: HTTPS, authenticate via browser.\n\n" +
				string(output), string(output2), settings.Repo, dir)
		}
	}

	fmt.Printf("   ✅ Cloned to %s\n\n", dir)

	// Save settings (shared with Kite)
	settings.LastSync = time.Now().UTC().Format(time.RFC3339)
	config.SaveSteerSettings(settings)
	return nil
}
