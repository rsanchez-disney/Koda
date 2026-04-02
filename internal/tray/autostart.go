package tray

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

const launchAgentLabel = "com.koda.tray"

func plistPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchAgentLabel+".plist")
}

func kodaBinary() string {
	path, err := os.Executable()
	if err != nil {
		return "koda"
	}
	return path
}

// EnableAutoStart registers the tray to launch on login.
func EnableAutoStart() error {
	if runtime.GOOS != "darwin" {
		return fmt.Errorf("tray auto-start only supported on macOS")
	}
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

// DisableAutoStart removes the login launch agent.
func DisableAutoStart() error {
	return os.Remove(plistPath())
}

// AutoStartEnabled returns whether the tray auto-starts on login.
func AutoStartEnabled() bool {
	_, err := os.Stat(plistPath())
	return err == nil
}
