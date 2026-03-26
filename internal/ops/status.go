package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// PrintStatus prints a one-liner status like git status.
func PrintStatus(steerRoot, targetDir string) {
	// Profiles
	installed := DetectInstalled(steerRoot, targetDir)
	var profileStr string
	if len(installed) > 0 {
		profileStr = strings.Join(installed, ", ")
	} else {
		profileStr = "(none)"
	}

	// Agent count
	agentCount := 0
	if entries, err := readJSONDir(filepath.Join(targetDir, config.AgentsDir)); err == nil {
		agentCount = len(entries)
	}

	// Tokens
	tokens := ReadTokens()
	tokSet := 0
	for _, v := range tokens {
		if v != "" {
			tokSet++
		}
	}

	// Git branch
	branch := "(not a git repo)"
	if out, err := exec.Command("git", "branch", "--show-current").Output(); err == nil {
		b := strings.TrimSpace(string(out))
		if b != "" {
			branch = b
		}
	}

	fmt.Printf("\U0001f43e profiles: %s (%d agents) \u00b7 tokens: %d \u00b7 branch: %s\n", profileStr, agentCount, tokSet, branch)
}

func readJSONDir(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	return names, nil
}
