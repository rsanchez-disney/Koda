package cli

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
	"path/filepath"
	"strings"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/ops"
	"golang.org/x/crypto/pbkdf2"
)

const steerReleaseAPI = "https://api.github.com/repos/rsanchez-disney/steer-runtime/releases/latest"

// releaseKey is set at build time via -ldflags from STEER_RELEASE_KEY env
var releaseKey string

type releaseInfo struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func cloneSteerRuntime() error {
	settings := config.ReadSteerSettings()
	dir := config.DefaultSteerRoot()

	fmt.Printf("   Target: %s\n\n", dir)

	if err := downloadFromRelease(dir); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	saveCloneSuccess(settings)

	// Enable auto-update on first install
	if err := ops.EnableAutoUpdate(); err == nil {
		fmt.Println("   ✅ Auto-update enabled (daily at 9:00 AM)")
	}

	return nil
}

func downloadFromRelease(dir string) error {
	fmt.Printf("   Fetching: %s\n", steerReleaseAPI)
	rel, err := fetchLatestRelease()
	if err != nil {
		return err
	}
	fmt.Printf("   Release: %s (%d assets)\n", rel.TagName, len(rel.Assets))

	// List assets for debugging
	for _, a := range rel.Assets {
		fmt.Printf("     • %s\n", a.Name)
	}

	// Prefer unencrypted, then encrypted
	url, encrypted := findTarball(rel)
	if url == "" {
		return fmt.Errorf("no .tar.gz asset found in release %s", rel.TagName)
	}

	if encrypted {
		fmt.Printf("   Downloading: %s (encrypted)\n", filepath.Base(url))
		if releaseKey == "" {
			return fmt.Errorf("encrypted release but no STEER_RELEASE_KEY compiled into this build.\n" +
				"   Rebuild with: make build  (ensure STEER_RELEASE_KEY env is set)")
		}
	} else {
		fmt.Printf("   Downloading: %s\n", filepath.Base(url))
	}

	// Download
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
	fmt.Printf("   Downloaded: %d bytes\n", len(data))

	// Decrypt if needed
	var tarData []byte
	if encrypted && len(data) > 8 && string(data[:8]) == "Salted__" {
		fmt.Println("   Decrypting...")
		tarData, err = decryptOpenSSL(data, releaseKey)
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
		fmt.Printf("   Decrypted: %d bytes\n", len(tarData))
	} else {
		tarData = data
	}

	// Extract
	fmt.Println("   Extracting...")
	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)
	if err := extractTarGz(tarData, dir); err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	os.WriteFile(filepath.Join(dir, "VERSION"), []byte(rel.TagName), 0644)
	fmt.Printf("   ✅ Installed %s to %s\n\n", rel.TagName, dir)
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
	// Prefer unencrypted
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz") && !strings.HasSuffix(a.Name, ".tar.gz.enc") {
			return a.URL, false
		}
	}
	// Fallback to encrypted
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz.enc") {
			return a.URL, true
		}
	}
	return "", false
}

func decryptOpenSSL(data []byte, passphrase string) ([]byte, error) {
	if len(data) < 16 || string(data[:8]) != "Salted__" {
		return nil, fmt.Errorf("not an OpenSSL encrypted file (header: %q)", string(data[:8]))
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
		return nil, fmt.Errorf("invalid padding (pad=%d, blockSize=%d) — wrong STEER_RELEASE_KEY?", padLen, aes.BlockSize)
	}
	for i := 0; i < padLen; i++ {
		if plaintext[len(plaintext)-1-i] != byte(padLen) {
			return nil, fmt.Errorf("corrupt padding at byte %d — wrong STEER_RELEASE_KEY?", i)
		}
	}
	return plaintext[:len(plaintext)-padLen], nil
}

func extractTarGz(data []byte, destDir string) error {
	gr, err := gzip.NewReader(strings.NewReader(string(data)))
	if err != nil {
		return fmt.Errorf("gzip: %w", err)
	}
	defer gr.Close()

	count := 0
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
			count++
		}
	}
	fmt.Printf("   Extracted: %d files\n", count)
	return nil
}

func saveCloneSuccess(settings config.SteerSettings) {
	settings.LastSync = time.Now().UTC().Format(time.RFC3339)
	config.SaveSteerSettings(settings)
}
