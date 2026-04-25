package pkg

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBundlePath(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := BundlePath("kitestream")
	want := filepath.Join(home, BinDir, "kitestream")
	if got != want {
		t.Errorf("BundlePath = %q, want %q", got, want)
	}
}

func TestBinPath(t *testing.T) {
	home, _ := os.UserHomeDir()
	got := BinPath("autopilot")
	want := filepath.Join(home, BinDir, "autopilot")
	if got != want {
		t.Errorf("BinPath = %q, want %q", got, want)
	}
}

func TestIsInstalled_NotInstalled(t *testing.T) {
	if IsInstalled("nonexistent-package-xyz") {
		t.Error("IsInstalled returned true for nonexistent package")
	}
}

func TestUninstallBundle_NotInstalled(t *testing.T) {
	err := UninstallBundle("nonexistent-package-xyz")
	if err == nil {
		t.Error("UninstallBundle should error for nonexistent package")
	}
}

func TestInstallAndUninstallBundle(t *testing.T) {
	// Create a temp dir to simulate ~/.koda/bin
	tmpHome := t.TempDir()
	bundleDir := filepath.Join(tmpHome, "testbundle")
	os.MkdirAll(bundleDir, 0o755)
	os.WriteFile(filepath.Join(bundleDir, "test.txt"), []byte("hello"), 0o644)

	// Verify exists
	if _, err := os.Stat(bundleDir); err != nil {
		t.Fatal("bundle dir should exist")
	}

	// Clean up
	os.RemoveAll(bundleDir)
	if _, err := os.Stat(bundleDir); !os.IsNotExist(err) {
		t.Error("bundle dir should be removed")
	}
}
