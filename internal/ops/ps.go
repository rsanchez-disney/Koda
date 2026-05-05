package ops

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// KiroProcess represents a running kiro-related process.
type KiroProcess struct {
	PID     int
	Name    string
	MemMB   float64
	Elapsed time.Duration
	Type    string // "session", "sub-agent", "daemon", "tray", "mcp"
}

// ListKiroProcesses finds all running kiro-cli and MCP server processes.
func ListKiroProcesses() []KiroProcess {
	if runtime.GOOS == "windows" {
		return listProcessesWindows()
	}
	return listProcessesUnix()
}

func listProcessesUnix() []KiroProcess {
	// ps -eo pid,rss,etime,comm,args — get PID, RSS (KB), elapsed time, command
	out, err := exec.Command("ps", "-eo", "pid,rss,etime,args").Output()
	if err != nil {
		return nil
	}

	var procs []KiroProcess
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "PID") {
			continue
		}
		if !strings.Contains(line, "kiro") && !strings.Contains(line, "mcp-servers") {
			continue
		}
		// Skip our own ps command and grep
		if strings.Contains(line, "ps -eo") || strings.Contains(line, "grep") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		pid, _ := strconv.Atoi(fields[0])
		rssKB, _ := strconv.ParseFloat(fields[1], 64)
		elapsed := parseElapsed(fields[2])
		args := strings.Join(fields[3:], " ")

		if pid == os.Getpid() {
			continue
		}

		proc := KiroProcess{
			PID:     pid,
			MemMB:   rssKB / 1024.0,
			Elapsed: elapsed,
			Type:    classifyProcess(args),
		}

		// Determine display name
		if strings.Contains(args, "kiro-cli-chat") || strings.Contains(args, "kiro-cli chat") {
			proc.Name = "kiro-cli-chat"
		} else if strings.Contains(args, "kiro-cli") {
			proc.Name = "kiro-cli"
		} else if strings.Contains(args, "mcp-servers") {
			// Extract MCP server name from path
			parts := strings.Split(args, "mcp-servers/")
			if len(parts) > 1 {
				name := strings.Split(parts[1], "/")[0]
				proc.Name = name + "-mcp"
			} else {
				proc.Name = "mcp-server"
			}
		} else {
			continue // not a kiro process
		}

		procs = append(procs, proc)
	}

	sort.Slice(procs, func(i, j int) bool { return procs[i].MemMB > procs[j].MemMB })
	return procs
}

func listProcessesWindows() []KiroProcess {
	// Simplified Windows implementation using tasklist
	out, err := exec.Command("tasklist", "/FI", "IMAGENAME eq kiro*", "/FO", "CSV", "/NH").Output()
	if err != nil {
		return nil
	}
	var procs []KiroProcess
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Split(strings.TrimSpace(line), "\",\"")
		if len(fields) < 5 {
			continue
		}
		name := strings.Trim(fields[0], "\"")
		pid, _ := strconv.Atoi(strings.Trim(fields[1], "\""))
		memStr := strings.Trim(fields[4], "\" K\r")
		memStr = strings.ReplaceAll(memStr, ",", "")
		memKB, _ := strconv.ParseFloat(memStr, 64)
		procs = append(procs, KiroProcess{
			PID:   pid,
			Name:  name,
			MemMB: memKB / 1024.0,
			Type:  "session",
		})
	}
	return procs
}

func classifyProcess(args string) string {
	switch {
	case strings.Contains(args, "kiro-cli-chat") || strings.Contains(args, "kiro-cli chat"):
		if strings.Contains(args, "subagent") || strings.Contains(args, "--parent") {
			return "sub-agent"
		}
		return "session"
	case strings.Contains(args, "kiro-cli tray") || strings.Contains(args, "kiro tray"):
		return "tray"
	case strings.Contains(args, "mcp-servers"):
		return "mcp"
	case strings.Contains(args, "kiro-cli"):
		return "daemon"
	default:
		return "other"
	}
}

// parseElapsed parses ps elapsed time format: [[dd-]hh:]mm:ss
func parseElapsed(s string) time.Duration {
	s = strings.TrimSpace(s)
	var d time.Duration

	// Handle dd-hh:mm:ss
	if idx := strings.Index(s, "-"); idx > 0 {
		days, _ := strconv.Atoi(s[:idx])
		d += time.Duration(days) * 24 * time.Hour
		s = s[idx+1:]
	}

	parts := strings.Split(s, ":")
	switch len(parts) {
	case 3: // hh:mm:ss
		h, _ := strconv.Atoi(parts[0])
		m, _ := strconv.Atoi(parts[1])
		sec, _ := strconv.Atoi(parts[2])
		d += time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
	case 2: // mm:ss
		m, _ := strconv.Atoi(parts[0])
		sec, _ := strconv.Atoi(parts[1])
		d += time.Duration(m)*time.Minute + time.Duration(sec)*time.Second
	}
	return d
}

// FormatElapsed formats a duration as "2h 15m" or "3m 20s".
func FormatElapsed(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// PrintProcesses prints the process list in a formatted table.
func PrintProcesses(procs []KiroProcess) {
	if len(procs) == 0 {
		fmt.Println("  No kiro processes found.")
		return
	}

	var totalMB float64
	fmt.Printf("  %-8s %10s  %-24s %-10s %s\n", "PID", "MEM", "NAME", "TYPE", "AGE")
	for _, p := range procs {
		fmt.Printf("  %-8d %8.1f MB  %-24s %-10s %s\n", p.PID, p.MemMB, p.Name, p.Type, FormatElapsed(p.Elapsed))
		totalMB += p.MemMB
	}

	fmt.Println()
	sp := DetectSystemProfile()
	fmt.Printf("  Total: %.1f MB across %d processes\n", totalMB, len(procs))
	fmt.Printf("  System: %d GB RAM — %s tier (max %d agents)\n", sp.TotalRAMGB, sp.Tier, sp.MaxAgents)
}

// killProcess terminates a process by PID (cross-platform).
func killProcess(pid int) error {
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F").Run()
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return proc.Signal(os.Kill)
}

// gracefulKillProcess sends SIGTERM and polls until the process exits or the grace period expires.
// Falls back to SIGKILL if the process doesn't exit in time.
func gracefulKillProcess(pid int, grace time.Duration) error {
	if runtime.GOOS == "windows" {
		return exec.Command("taskkill", "/PID", strconv.Itoa(pid), "/F").Run()
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	deadline := time.Now().Add(grace)
	for time.Now().Before(deadline) {
		if proc.Signal(syscall.Signal(0)) != nil {
			return nil // process exited
		}
		time.Sleep(200 * time.Millisecond)
	}
	return proc.Signal(os.Kill)
}

// KillOrphanProcesses kills kiro-cli-chat sub-agent processes and gracefully stops sessions.
func KillOrphanProcesses() int {
	procs := ListKiroProcesses()
	killed := 0
	for _, p := range procs {
		if p.PID == os.Getpid() {
			continue
		}
		switch p.Type {
		case "sub-agent":
			if killProcess(p.PID) == nil {
				killed++
				fmt.Printf("  ✓ Killed %s (PID %d, %.1f MB)\n", p.Name, p.PID, p.MemMB)
			}
		case "session":
			if gracefulKillProcess(p.PID, 5*time.Second) == nil {
				killed++
				fmt.Printf("  ✓ Stopped %s (PID %d, %.1f MB)\n", p.Name, p.PID, p.MemMB)
			}
		// daemon and other types are intentionally skipped
		}
	}
	return killed
}

// CleanStaleProcesses auto-kills disposable processes (sub-agents, tray, MCP servers)
// and prompts the user about active sessions so they can save context first.
func CleanStaleProcesses() {
	procs := ListKiroProcesses()

	var disposable, sessions []KiroProcess
	for _, p := range procs {
		if p.PID == os.Getpid() {
			continue
		}
		switch p.Type {
		case "sub-agent", "tray", "mcp":
			disposable = append(disposable, p)
		case "session":
			sessions = append(sessions, p)
		}
	}

	// Auto-kill disposable processes silently
	if len(disposable) > 0 {
		killed := 0
		var freedMB float64
		for _, p := range disposable {
			if killProcess(p.PID) == nil {
				killed++
				freedMB += p.MemMB
			}
		}
		if killed > 0 {
			fmt.Printf("  ✓ Stopped %d background process(es) (%.0f MB freed)\n", killed, freedMB)
		}
	}

	if len(sessions) == 0 {
		return
	}

	// Prompt about active sessions
	fmt.Printf("\n⚠ %d active chat session(s) found:\n", len(sessions))
	for _, p := range sessions {
		fmt.Printf("  PID %-8d %8.1f MB  %s\n", p.PID, p.MemMB, FormatElapsed(p.Elapsed))
	}
	fmt.Println("\n  Sessions will auto-save memories on exit, or skip to keep them running.")
	fmt.Print("  Stop active sessions? [y/N] ")
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimRight(answer, "\r\n")
	if answer != "y" && answer != "Y" {
		fmt.Println("  Skipped — sessions left running (will use old binary until restarted).")
		return
	}

	fmt.Print("  Saving sessions...")
	killed := 0
	for _, p := range sessions {
		if gracefulKillProcess(p.PID, 5*time.Second) == nil {
			killed++
		}
	}
	if killed > 0 {
		fmt.Printf("\r  ✓ Saved and stopped %d session(s)\n", killed)
	}
}
