//go:build !scorer

package ops

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// ScoreResult wraps the scoring result with session tracking.
type ScoreResult struct {
	Total           float64                `json:"total"`
	EstimatedTokens int                    `json:"estimated_tokens"`
	SessionTokens   int                    `json:"session_tokens"`
	Dimensions      map[string]interface{} `json:"dimensions"`
}

// ScorePrompt returns a basic token estimate when the scorer library is not available.
func ScorePrompt(prompt string, sessionID string) (*ScoreResult, error) {
	return &ScoreResult{
		Total:           0,
		EstimatedTokens: EstimateTokens(prompt),
		SessionTokens:   0,
		Dimensions:      nil,
	}, nil
}

// ScorerRunning returns false when the scorer library is not available.
func ScorerRunning() bool { return false }

// StartScorer is a no-op.
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
