package cli

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func captureOutput(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestAppsCmd_NoArgs_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("koda apps panicked: %v", r)
		}
	}()
	out := captureOutput(func() { appsCmd.Run(appsCmd, []string{}) })
	if !strings.Contains(out, "Available apps:") {
		t.Errorf("expected app list, got: %s", out)
	}
}

func TestAppsInstallCmd_UnknownApp(t *testing.T) {
	err := appsInstallCmd.RunE(appsInstallCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown app")
	}
	if !strings.Contains(err.Error(), "unknown app: nonexistent") {
		t.Errorf("got %q, want mention of unknown app", err.Error())
	}
}

func TestAppsUpdateCmd_UnknownApp(t *testing.T) {
	err := appsUpdateCmd.RunE(appsUpdateCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown app")
	}
}

func TestAppsUninstallCmd_UnknownApp(t *testing.T) {
	err := appsUninstallCmd.RunE(appsUninstallCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown app")
	}
}

func TestAppsStartCmd_UnknownApp(t *testing.T) {
	err := appsStartCmd.RunE(appsStartCmd, []string{"nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown app")
	}
}

func TestAppsSearchCmd_NoArgs(t *testing.T) {
	out := captureOutput(func() { appsSearchCmd.Run(appsSearchCmd, []string{}) })
	if !strings.Contains(out, "kite") {
		t.Errorf("expected all apps listed, got: %s", out)
	}
	if !strings.Contains(out, "To install:") {
		t.Errorf("expected install hint, got: %s", out)
	}
}

func TestAppsSearchCmd_WithQuery(t *testing.T) {
	out := captureOutput(func() { appsSearchCmd.Run(appsSearchCmd, []string{"kite"}) })
	if !strings.Contains(out, "kite") {
		t.Errorf("expected kite in results, got: %s", out)
	}
	if !strings.Contains(out, "To install:") {
		t.Errorf("expected install hint, got: %s", out)
	}
}

func TestAppsSearchCmd_NoMatch(t *testing.T) {
	out := captureOutput(func() { appsSearchCmd.Run(appsSearchCmd, []string{"zzzzz"}) })
	if !strings.Contains(out, "No apps matching 'zzzzz'") {
		t.Errorf("expected no-match message, got: %s", out)
	}
}

func TestAppsSearchCmd_ArgsValidation(t *testing.T) {
	err := cobra.MaximumNArgs(1)(appsSearchCmd, []string{"a", "b"})
	if err == nil {
		t.Fatal("expected error for too many args")
	}
}

func TestAppsInstallCmd_ArgsValidation(t *testing.T) {
	err := cobra.ExactArgs(1)(appsInstallCmd, []string{})
	if err == nil {
		t.Fatal("expected error for missing arg")
	}
}
