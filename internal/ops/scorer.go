//go:build scorer

package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	ps "github.disney.com/SANCR225/prompt-scorer/go-prompt-scorer"
)

// ScoreResult wraps the Go library result with session tracking.
type ScoreResult struct {
	Total           float64                    `json:"total"`
	EstimatedTokens int                        `json:"estimated_tokens"`
	SessionTokens   int                        `json:"session_tokens"`
	Dimensions      map[string]ps.DimensionResult `json:"dimensions"`
}

// sessionTokens tracks cumulative tokens per session in-process.
var sessionTokens = map[string]int{}

// ScorePrompt scores a prompt using the Go library (no HTTP, no Python).
func ScorePrompt(prompt string, sessionID string) (*ScoreResult, error) {
	result := ps.Score(prompt, nil)

	st := 0
	if sessionID != "" {
		sessionTokens[sessionID] += result.EstimatedTokens
		st = sessionTokens[sessionID]
	}

	return &ScoreResult{
		Total:           result.Total,
		EstimatedTokens: result.EstimatedTokens,
		SessionTokens:   st,
		Dimensions:      result.Dimensions,
	}, nil
}

// ScorerRunning always returns true since scoring is now in-process.
func ScorerRunning() bool { return true }

// StartScorer is a no-op — scoring is now in-process via the Go library.
func StartScorer(_ string) error { return nil }

// EstimateTokens estimates token count using ~4 chars per token.
func EstimateTokens(text string) int {
	n := len(text) / 4
	if n < 1 {
		n = 1
	}
	return n
}

// UsageEntry is one interaction logged to usage.jsonl.
type UsageEntry struct {
	Timestamp    string  `json:"ts"`
	Agent        string  `json:"agent"`
	SessionID    string  `json:"session"`
	InputTokens  int     `json:"in"`
	OutputTokens int     `json:"out"`
	PromptScore  float64 `json:"score"`
}

// LogUsage appends an entry to ~/.kiro/settings/usage.jsonl.
func LogUsage(entry UsageEntry) {
	entry.Timestamp = time.Now().UTC().Format(time.RFC3339)
	dir := filepath.Join(os.Getenv("HOME"), ".kiro", "settings")
	os.MkdirAll(dir, 0755)
	f, err := os.OpenFile(filepath.Join(dir, "usage.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	data, _ := json.Marshal(entry)
	f.Write(append(data, '\n'))
}
