package ops

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
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

// KillOrphanProcesses kills kiro-cli-chat sub-agent processes.
func KillOrphanProcesses() int {
	procs := ListKiroProcesses()
	killed := 0
	for _, p := range procs {
		if p.Type == "sub-agent" || (p.Type == "session" && p.PID != os.Getpid()) {
			proc, err := os.FindProcess(p.PID)
			if err == nil {
				proc.Signal(os.Interrupt)
				killed++
				fmt.Printf("  ✓ Killed %s (PID %d, %.1f MB)\n", p.Name, p.PID, p.MemMB)
			}
		}
	}
	return killed
}
