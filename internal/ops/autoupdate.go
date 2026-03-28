package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	launchAgentLabel = "com.koda.autoupdate"
	cronComment      = "# koda-auto-update"
)

func launchAgentPath() string {
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

// EnableAutoUpdate registers a daily sync job.
func EnableAutoUpdate() error {
	switch runtime.GOOS {
	case "darwin":
		return enableMacOS()
	case "linux":
		return enableLinux()
	default:
		return fmt.Errorf("auto-update not supported on %s (use Task Scheduler manually)", runtime.GOOS)
	}
}

// DisableAutoUpdate removes the daily sync job.
func DisableAutoUpdate() error {
	switch runtime.GOOS {
	case "darwin":
		return disableMacOS()
	case "linux":
		return disableLinux()
	default:
		return fmt.Errorf("auto-update not supported on %s", runtime.GOOS)
	}
}

// AutoUpdateStatus returns whether auto-update is enabled.
func AutoUpdateStatus() string {
	switch runtime.GOOS {
	case "darwin":
		if _, err := os.Stat(launchAgentPath()); err == nil {
			return "enabled (macOS LaunchAgent, daily at 9:00 AM)"
		}
		return "disabled"
	case "linux":
		out, err := exec.Command("crontab", "-l").Output()
		if err == nil && strings.Contains(string(out), cronComment) {
			return "enabled (cron, daily at 9:00 AM)"
		}
		return "disabled"
	default:
		return "not supported on " + runtime.GOOS
	}
}

func enableMacOS() error {
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>sync</string>
	</array>
	<key>StartCalendarInterval</key>
	<dict>
		<key>Hour</key>
		<integer>9</integer>
		<key>Minute</key>
		<integer>0</integer>
	</dict>
	<key>StandardOutPath</key>
	<string>/tmp/koda-autoupdate.log</string>
	<key>StandardErrorPath</key>
	<string>/tmp/koda-autoupdate.log</string>
</dict>
</plist>`, launchAgentLabel, kodaBinary())

	path := launchAgentPath()
	os.MkdirAll(filepath.Dir(path), 0755)
	if err := os.WriteFile(path, []byte(plist), 0644); err != nil {
		return err
	}
	exec.Command("launchctl", "unload", path).Run()
	if err := exec.Command("launchctl", "load", path).Run(); err != nil {
		return fmt.Errorf("launchctl load failed: %w", err)
	}
	return nil
}

func disableMacOS() error {
	path := launchAgentPath()
	exec.Command("launchctl", "unload", path).Run()
	os.Remove(path)
	return nil
}

func enableLinux() error {
	out, _ := exec.Command("crontab", "-l").Output()
	existing := strings.TrimSpace(string(out))
	if strings.Contains(existing, cronComment) {
		return nil // already installed
	}
	entry := fmt.Sprintf("%s\n0 9 * * * %s sync >> /tmp/koda-autoupdate.log 2>&1", cronComment, kodaBinary())
	newCron := existing
	if newCron != "" {
		newCron += "\n"
	}
	newCron += entry + "\n"
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(newCron)
	return cmd.Run()
}

func disableLinux() error {
	out, err := exec.Command("crontab", "-l").Output()
	if err != nil {
		return nil
	}
	var lines []string
	skipNext := false
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, cronComment) {
			skipNext = true
			continue
		}
		if skipNext {
			skipNext = false
			continue
		}
		lines = append(lines, line)
	}
	cmd := exec.Command("crontab", "-")
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	return cmd.Run()
}
