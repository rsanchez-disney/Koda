package cli

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
)

const steerReleaseAPI = "https://api.github.com/repos/rsanchez-disney/steer-runtime/releases/latest"

type releaseInfo struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetchLatestRelease() (*releaseInfo, error) {
	resp, err := http.Get(steerReleaseAPI)
	if err != nil {
		return nil, fmt.Errorf("cannot reach GitHub: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}
	var rel releaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func findTarball(rel *releaseInfo) string {
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz") {
			return a.URL
		}
	}
	return ""
}

func downloadAndExtract(url, destDir string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned %d", resp.StatusCode)
	}

	gr, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("gzip error: %w", err)
	}
	defer gr.Close()

	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, hdr.Name)

		// Security: prevent path traversal
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(destDir)) {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			io.Copy(f, tr)
			f.Close()
			os.Chmod(target, os.FileMode(hdr.Mode))
		}
	}
	return nil
}

func cloneSteerRuntime() error {
	dir := config.DefaultSteerRoot()

	fmt.Println("   Downloading latest steer-runtime release...")

	rel, err := fetchLatestRelease()
	if err != nil {
		return fmt.Errorf("cannot fetch release info: %w", err)
	}

	url := findTarball(rel)
	if url == "" {
		return fmt.Errorf("no tarball found in release %s", rel.TagName)
	}

	fmt.Printf("   Version: %s\n", rel.TagName)

	// Clean existing and extract
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)

	if err := downloadAndExtract(url, dir); err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	// Write version marker
	os.WriteFile(filepath.Join(dir, "VERSION"), []byte(rel.TagName), 0644)

	fmt.Printf("   \u2705 Installed to %s\n\n", dir)

	// Save settings
	settings := config.ReadSteerSettings()
	settings.LastSync = time.Now().UTC().Format(time.RFC3339)
	config.SaveSteerSettings(settings)
	return nil
}
