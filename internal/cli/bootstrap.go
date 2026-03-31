package cli

import (
	"fmt"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
)

func cloneSteerRuntime() error {
	settings := config.ReadSteerSettings()
	dir := config.DefaultSteerRoot()

	fmt.Printf("   Target: %s\n\n", dir)

	if err := ops.DownloadFromRelease(dir); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	settings.LastSync = time.Now().UTC().Format(time.RFC3339)
	config.SaveSteerSettings(settings)

	// Enable auto-update on first install
	if err := ops.EnableAutoUpdate(); err == nil {
		fmt.Println("   ✅ Auto-update enabled (daily at 9:00 AM)")
	}

	return nil
}
