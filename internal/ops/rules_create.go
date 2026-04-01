package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

const ruleTemplate = `# %s

## Purpose
Describe what this rule enforces or guides.

## Guidelines
- Guideline 1
- Guideline 2

## Examples

### Good
` + "```" + `
// good example
` + "```" + `

### Bad
` + "```" + `
// bad example
` + "```" + `
`

// CreateRule scaffolds a rule markdown file with a template.
func CreateRule(steerRoot, name string) (string, error) {
	path := filepath.Join(steerRoot, "common", config.RulesDir, name+".md")
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("rule already exists: %s", name)
	}
	os.MkdirAll(filepath.Dir(path), 0755)
	content := fmt.Sprintf(ruleTemplate, name)
	return path, os.WriteFile(path, []byte(content), 0644)
}

// EditorCmd returns an exec.Cmd for the user's preferred editor.
func EditorCmd(filePath string) *exec.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi"
	}
	return exec.Command(editor, filePath)
}

// PublishRule publishes a rule via branch + PR.
func PublishRule(steerRoot, name string) (string, error) {
	branch := "rule/" + name
	msg := "feat: add " + name + " rule"
	rulePath := filepath.Join("common", config.RulesDir, name+".md")

	if err := exec.Command("git", "-C", steerRoot, "checkout", "-b", branch).Run(); err != nil {
		return "", fmt.Errorf("branch create failed: %w", err)
	}

	exec.Command("git", "-C", steerRoot, "add", rulePath).Run()
	if err := exec.Command("git", "-C", steerRoot, "commit", "-m", msg).Run(); err != nil {
		exec.Command("git", "-C", steerRoot, "checkout", "main").Run()
		return "", fmt.Errorf("commit failed: %w", err)
	}

	if err := exec.Command("git", "-C", steerRoot, "push", "-u", "origin", branch).Run(); err != nil {
		exec.Command("git", "-C", steerRoot, "checkout", "main").Run()
		return "", fmt.Errorf("push failed: %w", err)
	}

	cmd := exec.Command("gh", "pr", "create",
		"--title", msg,
		"--body", fmt.Sprintf("Adds rule `%s`.", name),
		"--base", "main",
	)
	cmd.Dir = steerRoot
	out, err := cmd.Output()
	prURL := strings.TrimSpace(string(out))

	exec.Command("git", "-C", steerRoot, "checkout", "main").Run()

	if err != nil {
		return "", fmt.Errorf("PR create failed: %w — branch pushed, create PR manually", err)
	}
	return prURL, nil
}

// PublishRuleToUpstream publishes a rule from tarball mode via temp git init.
func PublishRuleToUpstream(steerRoot, name string) (string, error) {
	gitDir := filepath.Join(steerRoot, ".git")
	hadGit := false
	if _, err := os.Stat(gitDir); err == nil {
		hadGit = true
	}

	if !hadGit {
		upstreamURL := fmt.Sprintf("https://%s/%s.git", config.GHHost, config.DefaultSteerRepo)
		exec.Command("git", "-C", steerRoot, "init").Run()
		exec.Command("git", "-C", steerRoot, "remote", "add", "origin", upstreamURL).Run()
		exec.Command("git", "-C", steerRoot, "add", ".").Run()
		exec.Command("git", "-C", steerRoot, "commit", "-m", "tarball baseline").Run()
	}

	prURL, err := PublishRule(steerRoot, name)

	if !hadGit {
		os.RemoveAll(gitDir)
	}

	return prURL, err
}
