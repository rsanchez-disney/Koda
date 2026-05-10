package ops

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// SessionSummary aggregates usage for one session.
type SessionSummary struct {
	Agent        string
	Interactions int
	InputTokens  int
	OutputTokens int
	AvgScore     float64
}

// ReadUsage reads all entries from usage.jsonl.
func ReadUsage(since time.Time) []UsageEntry {
	path := filepath.Join(os.Getenv("HOME"), ".kiro", "settings", "usage.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var entries []UsageEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e UsageEntry
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			if t, err := time.Parse(time.RFC3339, e.Timestamp); err == nil && t.After(since) {
				entries = append(entries, e)
			}
		}
	}
	return entries
}

// PrintStats prints a usage summary.
func PrintStats(days int) {
	since := time.Now().AddDate(0, 0, -days)
	entries := ReadUsage(since)
	if len(entries) == 0 {
		fmt.Printf("No usage data in the last %d days.\n", days)
		return
	}

	// Aggregate by agent
	agentMap := map[string]*SessionSummary{}
	for _, e := range entries {
		s, ok := agentMap[e.Agent]
		if !ok {
			s = &SessionSummary{Agent: e.Agent}
			agentMap[e.Agent] = s
		}
		s.Interactions++
		s.InputTokens += e.InputTokens
		s.OutputTokens += e.OutputTokens
		s.AvgScore += e.PromptScore
	}

	var summaries []SessionSummary
	for _, s := range agentMap {
		if s.Interactions > 0 {
			s.AvgScore /= float64(s.Interactions)
		}
		summaries = append(summaries, *s)
	}
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].InputTokens+summaries[i].OutputTokens > summaries[j].InputTokens+summaries[j].OutputTokens
	})

	totalIn, totalOut, totalInteractions := 0, 0, 0
	fmt.Printf("\n📊 Usage Report (last %d days)\n\n", days)
	fmt.Printf("  %-25s %6s %10s %10s %6s\n", "Agent", "Chats", "In Tokens", "Out Tokens", "Score")
	fmt.Printf("  %-25s %6s %10s %10s %6s\n", "─────", "─────", "─────────", "──────────", "─────")
	for _, s := range summaries {
		fmt.Printf("  %-25s %6d %10d %10d %5.1f\n", s.Agent, s.Interactions, s.InputTokens, s.OutputTokens, s.AvgScore)
		totalIn += s.InputTokens
		totalOut += s.OutputTokens
		totalInteractions += s.Interactions
	}
	fmt.Printf("  %-25s %6s %10s %10s\n", "─────", "─────", "─────────", "──────────")
	fmt.Printf("  %-25s %6d %10d %10d\n", "TOTAL", totalInteractions, totalIn, totalOut)
	fmt.Println()
}

// TelemetryEntry represents one line from telemetry.jsonl.
type TelemetryEntry struct {
	Ts       string  `json:"ts"`
	Event    string  `json:"event"`
	Agent    string  `json:"agent"`
	Duration int     `json:"duration_ms"`
	Tools    int     `json:"tool_calls"`
	CtxPct   float64 `json:"context_usage_pct"`
}

// ReadTelemetry reads entries from telemetry.jsonl.
func ReadTelemetry(since time.Time) []TelemetryEntry {
	path := filepath.Join(os.Getenv("HOME"), ".kiro", "logs", "telemetry.jsonl")
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var entries []TelemetryEntry
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var e TelemetryEntry
		if json.Unmarshal(scanner.Bytes(), &e) == nil {
			if t, err := time.Parse(time.RFC3339, e.Ts); err == nil && t.After(since) {
				entries = append(entries, e)
			}
		}
	}
	return entries
}

// PrintTelemetryStats prints session telemetry summary.
func PrintTelemetryStats(days int) {
	since := time.Now().AddDate(0, 0, -days)
	entries := ReadTelemetry(since)
	if len(entries) == 0 {
		fmt.Printf("No telemetry data in the last %d days.\n", days)
		return
	}

	agentCount := map[string]int{}
	var totalDuration, totalTools int
	for _, e := range entries {
		agentCount[e.Agent]++
		totalDuration += e.Duration
		totalTools += e.Tools
	}

	fmt.Printf("\n📈 Session Telemetry (last %d days)\n\n", days)
	fmt.Printf("  Sessions: %d | Avg duration: %ds | Total tool calls: %d\n\n", len(entries), totalDuration/len(entries)/1000, totalTools)

	// Top agents
	type agentStat struct {
		name  string
		count int
	}
	var stats []agentStat
	for name, count := range agentCount {
		stats = append(stats, agentStat{name, count})
	}
	sort.Slice(stats, func(i, j int) bool { return stats[i].count > stats[j].count })

	fmt.Printf("  %-25s %6s\n", "Agent", "Sessions")
	fmt.Printf("  %-25s %6s\n", "─────", "────────")
	for i, s := range stats {
		if i >= 10 { break }
		fmt.Printf("  %-25s %6d\n", s.name, s.count)
	}
	fmt.Println()
}
