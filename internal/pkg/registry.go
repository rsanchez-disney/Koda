package pkg

import (
	"encoding/json"
	"fmt"
	"strings"
	"net/http"
	"runtime"
)

type PackageManifest struct {
	Name        string     `json:"name" yaml:"name"`
	Description string     `json:"description" yaml:"description"`
	Type        string     `json:"type" yaml:"type"`
	Repository  string     `json:"repository" yaml:"repository"`
	Platforms   []Platform `json:"platforms" yaml:"platforms"`
}

type Platform struct {
	OS       string `json:"os" yaml:"os"`
	Arch     string `json:"arch" yaml:"arch"`
	Artifact string `json:"artifact" yaml:"artifact"`
}

type GitHubRelease struct {
	TagName string  `json:"tag_name"`
	Assets  []Asset `json:"assets"`
}

type Asset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// FetchLatestRelease gets the latest release from a GitHub repo.
func FetchLatestRelease(repo string) (*GitHubRelease, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", repo)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetch release: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("release not found (HTTP %d)", resp.StatusCode)
	}
	var rel GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

// ResolveArtifact finds the correct artifact for the current platform.
func ResolveArtifact(manifest *PackageManifest) (*Platform, error) {
	for _, p := range manifest.Platforms {
		if p.OS == runtime.GOOS && p.Arch == runtime.GOARCH {
			return &p, nil
		}
	}
	return nil, fmt.Errorf("no artifact for %s/%s", runtime.GOOS, runtime.GOARCH)
}

// FindAssetURL matches a platform artifact name to a release asset download URL.
func FindAssetURL(release *GitHubRelease, artifactName string) (string, error) {
	// Exact match first
	for _, a := range release.Assets {
		if a.Name == artifactName {
			return a.BrowserDownloadURL, nil
		}
	}
	// Prefix match: autopilot-darwin-arm64 matches autopilot-darwin-arm64-abc123.tar.gz.enc
	prefix := strings.TrimSuffix(artifactName, ".tar.gz.enc")
	for _, a := range release.Assets {
		if strings.HasPrefix(a.Name, prefix) && strings.HasSuffix(a.Name, ".tar.gz.enc") {
			return a.BrowserDownloadURL, nil
		}
	}
	return "", fmt.Errorf("asset %s not found in release %s", artifactName, release.TagName)
}
