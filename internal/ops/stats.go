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
