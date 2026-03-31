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
	winTaskName      = "KodaAutoUpdate"
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

// EnableAutoUpdate registers a daily job: upgrade + sync --update.
func EnableAutoUpdate() error {
	switch runtime.GOOS {
	case "darwin":
		return enableMacOS()
	case "linux":
		return enableLinux()
	case "windows":
		return enableWindows()
	default:
		return fmt.Errorf("auto-update not supported on %s", runtime.GOOS)
	}
}

// DisableAutoUpdate removes the daily job.
func DisableAutoUpdate() error {
	switch runtime.GOOS {
	case "darwin":
		return disableMacOS()
	case "linux":
		return disableLinux()
	case "windows":
		return disableWindows()
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
	case "windows":
		out, _ := exec.Command("schtasks.exe", "/Query", "/TN", winTaskName).CombinedOutput()
		if strings.Contains(string(out), winTaskName) {
			return "enabled (Task Scheduler, daily at 9:00 AM)"
		}
		return "disabled"
	default:
		return "not supported on " + runtime.GOOS
	}
}

// --- macOS ---

func enableMacOS() error {
	bin := kodaBinary()
	plist := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>Program</key>
	<string>/bin/sh</string>
	<key>ProgramArguments</key>
	<array>
		<string>/bin/sh</string>
		<string>-c</string>
		<string>%s upgrade; %s sync --update</string>
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
</plist>`, launchAgentLabel, bin, bin)

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

// --- Linux ---

func enableLinux() error {
	out, _ := exec.Command("crontab", "-l").Output()
	existing := strings.TrimSpace(string(out))
	if strings.Contains(existing, cronComment) {
		return nil
	}
	bin := kodaBinary()
	entry := fmt.Sprintf("%s\n0 9 * * * %s upgrade >> /tmp/koda-autoupdate.log 2>&1 && %s sync --update >> /tmp/koda-autoupdate.log 2>&1",
		cronComment, bin, bin)
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

// --- Windows ---

func enableWindows() error {
	bin := kodaBinary()
	cmd := exec.Command("schtasks.exe", "/Create",
		"/TN", winTaskName,
		"/TR", fmt.Sprintf(`cmd /c "%s upgrade & %s sync --update"`, bin, bin),
		"/SC", "DAILY",
		"/ST", "09:00",
		"/F",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks create failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func disableWindows() error {
	cmd := exec.Command("schtasks.exe", "/Delete", "/TN", winTaskName, "/F")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("schtasks delete failed: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
