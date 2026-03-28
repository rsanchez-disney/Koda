package ops

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const scorerPort = 8222

type ScoreResult struct {
	Total           float64                       `json:"total"`
	EstimatedTokens int                           `json:"estimated_tokens"`
	SessionTokens   int                           `json:"session_tokens"`
	Dimensions      map[string]DimensionResult     `json:"dimensions"`
}

type DimensionResult struct {
	Score   int      `json:"score"`
	Reasons []string `json:"reasons"`
}

func scorerURL() string {
	return fmt.Sprintf("http://localhost:%d", scorerPort)
}

// ScorerRunning checks if the scorer API is reachable.
func ScorerRunning() bool {
	resp, err := http.Get(scorerURL() + "/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// StartScorer launches the prompt-scorer server in the background.
func StartScorer(scorerDir string) error {
	if ScorerRunning() {
		return nil
	}
	cmd := exec.Command("python3", "__main__.py", fmt.Sprintf("%d", scorerPort))
	cmd.Dir = scorerDir
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start scorer: %w", err)
	}
	// Wait for it to be ready
	for i := 0; i < 20; i++ {
		time.Sleep(250 * time.Millisecond)
		if ScorerRunning() {
			return nil
		}
	}
	return fmt.Errorf("scorer did not start within 5s")
}

// ScorePrompt calls the scorer API and returns the result.
func ScorePrompt(prompt string, sessionID string) (*ScoreResult, error) {
	body, _ := json.Marshal(map[string]string{
		"prompt":     prompt,
		"session_id": sessionID,
	})
	resp, err := http.Post(scorerURL()+"/score", "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result ScoreResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

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
