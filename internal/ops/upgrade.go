package ops

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
)

const releaseURL = "https://github.disney.com/api/v3/repos/SANCR225/Koda/releases/latest"

type ghRelease struct {
	TagName string    `json:"tag_name"`
	Assets  []ghAsset `json:"assets"`
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Upgrade downloads the latest release binary and replaces the current one.
func Upgrade(currentVersion string) error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot find current binary: %w", err)
	}

	// Fetch latest release
	fmt.Println("\U0001f50d Checking for updates...")
	resp, err := http.Get(releaseURL)
	if err != nil {
		return fmt.Errorf("cannot reach GitHub: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var rel ghRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return fmt.Errorf("cannot parse release: %w", err)
	}

	if rel.TagName == currentVersion || rel.TagName == "v"+currentVersion {
		fmt.Printf("\u2705 Already on latest: %s\n", currentVersion)
		return nil
	}

	// Find matching asset
	want := fmt.Sprintf("koda-%s-%s", runtime.GOOS, runtime.GOARCH)
	var downloadURL string
	for _, a := range rel.Assets {
		if strings.Contains(a.Name, want) {
			downloadURL = a.URL
			break
		}
	}
	if downloadURL == "" {
		return fmt.Errorf("no binary found for %s/%s in release %s", runtime.GOOS, runtime.GOARCH, rel.TagName)
	}

	// Download
	fmt.Printf("\U0001f4e5 Downloading %s...\n", rel.TagName)
	dlResp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer dlResp.Body.Close()

	tmpFile := exePath + ".new"
	f, err := os.Create(tmpFile)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, dlResp.Body); err != nil {
		f.Close()
		os.Remove(tmpFile)
		return err
	}
	f.Close()
	os.Chmod(tmpFile, 0755)

	// Atomic swap
	oldFile := exePath + ".old"
	os.Rename(exePath, oldFile)
	if err := os.Rename(tmpFile, exePath); err != nil {
		os.Rename(oldFile, exePath) // rollback
		return err
	}
	os.Remove(oldFile)

	fmt.Printf("\u2705 Upgraded: %s \u2192 %s\n", currentVersion, rel.TagName)
	return nil
}
