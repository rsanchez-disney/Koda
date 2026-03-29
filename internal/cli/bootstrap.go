package cli

import (
	"archive/tar"
	"compress/gzip"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.disney.com/SANCR225/koda/internal/config"
	"golang.org/x/crypto/pbkdf2"
)

const steerReleaseAPI = "https://api.github.com/repos/rsanchez-disney/steer-runtime/releases/latest"

// releaseKey is set at build time via ldflags
var releaseKey string

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

func findEncryptedTarball(rel *releaseInfo) string {
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz.enc") {
			return a.URL
		}
	}
	// Fallback to unencrypted
	for _, a := range rel.Assets {
		if strings.HasSuffix(a.Name, ".tar.gz") {
			return a.URL
		}
	}
	return ""
}

func decryptOpenSSL(data []byte, passphrase string) ([]byte, error) {
	// OpenSSL enc format: "Salted__" + 8-byte salt + ciphertext
	if len(data) < 16 || string(data[:8]) != "Salted__" {
		return nil, fmt.Errorf("not an OpenSSL encrypted file")
	}
	salt := data[8:16]
	ciphertext := data[16:]

	// Derive key+IV using PBKDF2 (matches openssl -pbkdf2)
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

	// Remove PKCS7 padding
	if len(plaintext) == 0 {
		return nil, fmt.Errorf("empty plaintext")
	}
	padLen := int(plaintext[len(plaintext)-1])
	if padLen > aes.BlockSize || padLen == 0 {
		return nil, fmt.Errorf("invalid padding")
	}
	return plaintext[:len(plaintext)-padLen], nil
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

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Decrypt if encrypted
	var tarData []byte
	if len(data) > 8 && string(data[:8]) == "Salted__" {
		if releaseKey == "" {
			return fmt.Errorf("encrypted release but no key compiled into this build")
		}
		key, _ := hex.DecodeString(releaseKey)
		tarData, err = decryptOpenSSL(data, string(key))
		if err != nil {
			return fmt.Errorf("decryption failed: %w", err)
		}
	} else {
		tarData = data
	}

	// Extract gzipped tar
	gr, err := gzip.NewReader(strings.NewReader(string(tarData)))
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

	url := findEncryptedTarball(rel)
	if url == "" {
		return fmt.Errorf("no tarball found in release %s", rel.TagName)
	}

	fmt.Printf("   Version: %s\n", rel.TagName)

	os.RemoveAll(dir)
	os.MkdirAll(dir, 0755)

	if err := downloadAndExtract(url, dir); err != nil {
		return fmt.Errorf("extract failed: %w", err)
	}

	os.WriteFile(filepath.Join(dir, "VERSION"), []byte(rel.TagName), 0644)

	fmt.Printf("   \u2705 Installed to %s\n\n", dir)

	settings := config.ReadSteerSettings()
	settings.LastSync = time.Now().UTC().Format(time.RFC3339)
	config.SaveSteerSettings(settings)
	return nil
}
