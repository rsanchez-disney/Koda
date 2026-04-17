package pkg

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

const BinDir = ".koda/bin"

// BinPath returns the full path to an installed binary.
func BinPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, BinDir, name)
}

// IsInstalled checks if a package binary exists.
func IsInstalled(name string) bool {
	_, err := os.Stat(BinPath(name))
	return err == nil
}

// Install downloads an encrypted artifact, decrypts it, and installs the binary.
func Install(name, downloadURL, decryptKey string) error {
	home, _ := os.UserHomeDir()
	binDir := filepath.Join(home, BinDir)
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return fmt.Errorf("create bin dir: %w", err)
	}

	// Download .enc file
	fmt.Printf("  Downloading %s...\n", name)
	tmpEnc, err := os.CreateTemp("", name+"-*.tar.gz.enc")
	if err != nil {
		return err
	}
	defer os.Remove(tmpEnc.Name())

	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed (HTTP %d)", resp.StatusCode)
	}
	io.Copy(tmpEnc, resp.Body)
	tmpEnc.Close()

	// Decrypt
	fmt.Printf("  Decrypting...\n")
	tmpTar, err := os.CreateTemp("", name+"-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tmpTar.Name())
	tmpTar.Close()

	cmd := exec.Command("openssl", "enc", "-d", "-aes-256-cbc", "-pbkdf2",
		"-in", tmpEnc.Name(), "-out", tmpTar.Name(), "-pass", "pass:"+decryptKey)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("decrypt failed: %s", string(out))
	}

	// Extract
	fmt.Printf("  Installing...\n")
	tmpDir, _ := os.MkdirTemp("", name+"-extract-*")
	defer os.RemoveAll(tmpDir)

	exec.Command("tar", "-xzf", tmpTar.Name(), "-C", tmpDir).Run()

	// Find the binary in extracted files
	entries, _ := os.ReadDir(tmpDir)
	for _, e := range entries {
		if !e.IsDir() {
			src := filepath.Join(tmpDir, e.Name())
			dst := filepath.Join(binDir, name)
			data, _ := os.ReadFile(src)
			if err := os.WriteFile(dst, data, 0o755); err != nil {
				return fmt.Errorf("install binary: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("no binary found in archive")
}

// Uninstall removes an installed binary.
func Uninstall(name string) error {
	path := BinPath(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("%s is not installed", name)
	}
	return os.Remove(path)
}
