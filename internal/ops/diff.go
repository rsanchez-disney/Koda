package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
)

// DiffEntry represents a single file change.
type DiffEntry struct {
	Action string // "add", "update", "remove"
	Path   string
}

// DiffSync compares what's installed vs what would be installed.
func DiffSync(steerRoot, targetDir string) []DiffEntry {
	installed := DetectInstalled(steerRoot, targetDir)
	if len(installed) == 0 {
		return nil
	}

	var entries []DiffEntry

	// Check each installed profile's agents
	for _, profileID := range installed {
		srcDir := filepath.Join(steerRoot, config.ProfilePrefix+profileID, config.AgentsDir)
		srcEntries, err := os.ReadDir(srcDir)
		if err != nil {
			continue
		}
		for _, e := range srcEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			dstPath := filepath.Join(targetDir, config.AgentsDir, e.Name())
			srcPath := filepath.Join(srcDir, e.Name())

			srcData, _ := os.ReadFile(srcPath)
			dstData, err := os.ReadFile(dstPath)

			if err != nil {
				entries = append(entries, DiffEntry{"add", "agents/" + e.Name()})
			} else if string(srcData) != string(dstData) {
				entries = append(entries, DiffEntry{"update", "agents/" + e.Name()})
			}
		}
	}

	// Check for agents in target that don't exist in any source
	dstEntries, _ := os.ReadDir(filepath.Join(targetDir, config.AgentsDir))
	for _, e := range dstEntries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		found := false
		for _, profileID := range installed {
			srcPath := filepath.Join(steerRoot, config.ProfilePrefix+profileID, config.AgentsDir, e.Name())
			if _, err := os.Stat(srcPath); err == nil {
				found = true
				break
			}
		}
		if !found {
			entries = append(entries, DiffEntry{"orphan", "agents/" + e.Name()})
		}
	}

	return entries
}

// PrintDiff prints diff entries.
func PrintDiff(entries []DiffEntry) {
	if len(entries) == 0 {
		fmt.Println("\u2705 Everything up to date")
		return
	}
	fmt.Printf("%d changes:\n\n", len(entries))
	for _, e := range entries {
		var icon string
		switch e.Action {
		case "add":
			icon = "+ "
		case "update":
			icon = "~ "
		case "orphan":
			icon = "? "
		default:
			icon = "  "
		}
		fmt.Printf("  %s%s\n", icon, e.Path)
	}
}
