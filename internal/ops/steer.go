package ops

import (
	"archive/tar"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.disney.com/SANCR225/koda/internal/config"
	"golang.org/x/crypto/pbkdf2"
)

const steerReleaseAPI = "https://api.github.com/repos/rsanchez-disney/steer-runtime/releases/latest"

// releaseKey is set at build time via -ldflags
var releaseKey string

type releaseInfo struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// SyncSteerRuntime fetches latest changes based on source type, then re-installs profiles.
func SyncSteerRuntime(steerRoot, targetDir string) error {
	settings := config.ReadSteerSettings()

	// Safety: if steerRoot has .git, always use git sync regardless of settings
	hasGit := false
	if _, err := os.Stat(filepath.Join(steerRoot, ".git")); err == nil {
		hasGit = true
	}

	if settings.Source == "git" || hasGit {
		if err := syncGit(steerRoot); err != nil {
			return err
		}
	} else {
		if err := DownloadFromRelease(steerRoot); err != nil {
			return err
		}
	}

	config.MarkSynced()

	// Re-install profiles
	installed := DetectInstalled(steerRoot, targetDir)
	InstallShared(steerRoot, targetDir)
	for _, p := range installed {
		InstallProfile(steerRoot, p, targetDir)
	}
	InjectAgentTokens(targetDir)
	return nil
}

func syncGit(steerRoot string) error {
	cmd := exec.Command("git", "-C", steerRoot, "pull", "--ff-only")
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

// ForkSteerRuntime replaces the tarball install with a git clone of the given fork.
func ForkSteerRuntime(steerRoot, repo, branch string) error {
	url := fmt.Sprintf("git@%s:%s.git", config.GHHost, repo)
	os.RemoveAll(steerRoot)
	cmd := exec.Command("git", "clone", "--depth", "1", "-b", branch, url, steerRoot)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	s := config.ReadSteerSettings()
	s.Source = "git"
	s.Repo = repo
	s.Branch = branch
	config.SaveSteerSettings(s)
	config.MarkSynced()
	return nil
}

// UnforkSteerRuntime switches back to the canonical tarball source.
func UnforkSteerRuntime(steerRoot string) error {
	os.RemoveAll(steerRoot)
	if err := DownloadFromRelease(steerRoot); err != nil {
		return fmt.Errorf("tarball download failed: %w", err)
	}

	s := config.ReadSteerSettings()
	s.Source = "tarball"
	s.Repo = config.DefaultSteerRepo
	s.Branch = config.DefaultSteerBranch
	config.SaveSteerSettings(s)
	config.MarkSynced()
	return nil
}

// --- Tarball download (moved from cli/bootstrap.go) ---

func DownloadFromRelease(dir string) error {
	rel, err := fetchLatestRelease()
	if err != nil {
		return err
	}

	url, encrypted := findTarball(rel)
	if url == "" {
		return fmt.Errorf("no .tar.gz asset found in release %s", rel.TagName)
	}

	if encrypted && releaseKey == "" {
		return fmt.Errorf("encrypted release but no STEER_RELEASE_KEY compiled into this build")
	}

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("download error: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var tarData []byte
	if encrypted && len(data) > 8 && string(data[:8]) == "Salted__" {
		tarData, err = DecryptOpenSSL(data, releaseKey)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
	} else {
		tarData = data
	}

	// Refuse to nuke a directory that contains .git (or is a symlink to one)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return fmt.Errorf("refusing to overwrite git repo at %s", dir)
	}
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	if err := ExtractTarGz(tarData, dir); err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	os.WriteFile(filepath.Join(dir, "VERSION"), []byte(rel.TagName), 0644)
	return nil
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

func findTarball(rel *releaseInfo) (url string, encrypted bool) {
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz") && !strings.HasSuffix(a.Name, ".tar.gz.enc") {
			return a.URL, false
		}
	}
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz.enc") {
			return a.URL, true
		}
	}
	return "", false
}

func DecryptOpenSSL(data []byte, passphrase string) ([]byte, error) {
	if len(data) < 16 || string(data[:8]) != "Salted__" {
		return nil, fmt.Errorf("not an OpenSSL encrypted file")
	}
	salt := data[8:16]
	ciphertext := data[16:]

	keyIV := pbkdf2.Key([]byte(passphrase), salt, 10000, 48, sha256.New)
	key := keyIV[:32]
	iv := keyIV[32:48]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	mode := cipher.NewCBCDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	mode.CryptBlocks(plaintext, ciphertext)

	if len(plaintext) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	padLen := int(plaintext[len(plaintext)-1])
	if padLen > aes.BlockSize || padLen == 0 {
		return nil, fmt.Errorf("invalid padding — wrong STEER_RELEASE_KEY?")
	}
	for i := 0; i < padLen; i++ {
		if plaintext[len(plaintext)-1-i] != byte(padLen) {
			return nil, fmt.Errorf("corrupt padding — wrong STEER_RELEASE_KEY?")
		}
	}
	return plaintext[:len(plaintext)-padLen], nil
}

func ExtractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
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
