package pkg

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.disney.com/SANCR225/koda/internal/ops"
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

	// Decrypt using Go-native decryption (same as steer-runtime)
	fmt.Printf("  Decrypting...\n")
	encData, err := os.ReadFile(tmpEnc.Name())
	if err != nil {
		return fmt.Errorf("read encrypted file: %w", err)
	}
	tarData, err := ops.DecryptOpenSSL(encData, decryptKey)
	if err != nil {
		return fmt.Errorf("decrypt failed: %w", err)
	}

	// Extract
	fmt.Printf("  Installing...\n")
	tmpTar, err := os.CreateTemp("", name+"-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tmpTar.Name())
	os.WriteFile(tmpTar.Name(), tarData, 0o644)

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

// BundlePath returns the full path to an installed bundle directory.
func BundlePath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, BinDir, name)
}

// InstallBundle downloads an encrypted artifact and extracts the full directory to ~/.koda/bin/{name}/.
func InstallBundle(name, downloadURL, decryptKey string) error {
	home, _ := os.UserHomeDir()
	destDir := filepath.Join(home, BinDir, name)

	// Clean previous install
	os.RemoveAll(destDir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create bundle dir: %w", err)
	}

	// Download
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
	encData, err := os.ReadFile(tmpEnc.Name())
	if err != nil {
		return err
	}
	tarData, err := ops.DecryptOpenSSL(encData, decryptKey)
	if err != nil {
		return fmt.Errorf("decrypt failed: %w", err)
	}

	// Extract to bundle dir
	fmt.Printf("  Installing...\n")
	tmpTar, err := os.CreateTemp("", name+"-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(tmpTar.Name())
	os.WriteFile(tmpTar.Name(), tarData, 0o644)

	if err := exec.Command("tar", "-xzf", tmpTar.Name(), "-C", destDir).Run(); err != nil {
		return fmt.Errorf("extract bundle: %w", err)
	}

	// macOS: clear quarantine and re-sign .app bundles so Gatekeeper allows launch
	if runtime.GOOS == "darwin" {
		if entries, err := os.ReadDir(destDir); err == nil {
			for _, e := range entries {
				if e.IsDir() && filepath.Ext(e.Name()) == ".app" {
					app := filepath.Join(destDir, e.Name())
					exec.Command("xattr", "-cr", app).Run()
					if err := exec.Command("codesign", "--force", "--deep", "--sign", "-", app).Run(); err != nil {
						fmt.Fprintf(os.Stderr, "  ⚠️  codesign failed for %s: %v\n", e.Name(), err)
					}
				}
			}
		}
	}

	return nil
}

// UninstallBundle removes an installed bundle directory.
func UninstallBundle(name string) error {
	path := BundlePath(name)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return fmt.Errorf("%s is not installed", name)
	}
	return os.RemoveAll(path)
}
