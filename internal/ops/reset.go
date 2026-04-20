package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
)

// Reset backs up ~/.kiro, preserves tokens/env, and reinstalls fresh.
func Reset(steerRoot string) error {
	kiroRoot := config.KiroRoot()

	// 1. Check if .kiro exists
	if _, err := os.Stat(kiroRoot); err != nil {
		return fmt.Errorf("nothing to reset: %s does not exist", kiroRoot)
	}

	// 2. Read tokens and env vars before backup
	tokens := ReadTokens()
	envVars := ReadEnvVars()

	// 3. Backup: rename .kiro → .kiro_{timestamp}_backup
	timestamp := time.Now().Format("20060102_150405")
	backupDir := kiroRoot + "_" + timestamp + "_backup"
	fmt.Printf("  📦 Backing up %s → %s\n", kiroRoot, filepath.Base(backupDir))
	if err := os.Rename(kiroRoot, backupDir); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}

	// 4. Create fresh .kiro
	os.MkdirAll(kiroRoot, 0755)

	// 5. Restore tokens and env vars
	if len(tokens) > 0 {
		WriteTokens(tokens)
		fmt.Printf("  ✓ Restored %d tokens\n", len(tokens))
	}
	if len(envVars) > 0 {
		WriteEnvVars(envVars)
		fmt.Printf("  ✓ Restored %d env vars\n", len(envVars))
	}

	// 6. Reinstall shared + steer-runtime link
	if steerRoot != "" {
		target := config.TargetDir("")
		InstallShared(steerRoot, target)
		GenerateMcpJson(FindNodeExe())
		fmt.Println("  ✓ Reinstalled shared hooks and MCP bundles")
	}

	fmt.Printf("\n✅ Reset complete. Backup at: %s\n", filepath.Base(backupDir))
	fmt.Println("   Run 'koda install <profiles>' to reinstall your profiles.")
	return nil
}
