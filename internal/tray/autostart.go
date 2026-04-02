//go:build darwin || windows

package tray

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	launchAgentLabel = "com.koda.tray"
	winRegKey        = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`
	winRegValue      = "KodaTray"
)

func kodaBinary() string {
	path, err := os.Executable()
	if err != nil {
		return "koda"
	}
	return path
}

// EnableAutoStart registers the tray to launch on login.
func EnableAutoStart() error {
	switch runtime.GOOS {
	case "darwin":
		return enableMacOS()
	case "windows":
		return enableWindows()
	default:
		return fmt.Errorf("tray auto-start not supported on %s", runtime.GOOS)
	}
}

// DisableAutoStart removes the login auto-start.
func DisableAutoStart() error {
	switch runtime.GOOS {
	case "darwin":
		return os.Remove(plistPath())
	case "windows":
		return exec.Command("reg", "delete", winRegKey, "/v", winRegValue, "/f").Run()
	default:
		return fmt.Errorf("tray auto-start not supported on %s", runtime.GOOS)
	}
}

// AutoStartEnabled returns whether the tray auto-starts on login.
func AutoStartEnabled() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat(plistPath())
		return err == nil
	case "windows":
		out, err := exec.Command("reg", "query", winRegKey, "/v", winRegValue).CombinedOutput()
		return err == nil && strings.Contains(string(out), winRegValue)
	default:
		return false
	}
}

// --- macOS ---

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func enableMacOS() error {
	bin := kodaBinary()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>tray</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<false/>
	<key>StandardOutPath</key>
	<string>/tmp/koda-tray.log</string>
	<key>StandardErrorPath</key>
	<string>/tmp/koda-tray.log</string>
</dict>
</plist>`, launchAgentLabel, bin)

	path := plistPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	return os.WriteFile(path, []byte(plist), 0644)
}

// --- Windows ---

func enableWindows() error {
	bin := kodaBinary()
	return exec.Command("reg", "add", winRegKey, "/v", winRegValue, "/t", "REG_SZ", "/d", bin+" tray", "/f").Run()
}
