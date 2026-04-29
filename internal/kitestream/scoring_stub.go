//go:build !scorer

package kitestream

import (
	"sync"

	"github.disney.com/SANCR225/koda/internal/ops"
)

// ScorerResult is a minimal result when the scorer library is not available.
type ScorerResult struct {
	Total           float64
	EstimatedTokens int
}

// Score returns a basic token estimate.
func Score(prompt string, _ interface{}) ScorerResult {
	return ScorerResult{EstimatedTokens: ops.EstimateTokens(prompt)}
}

// SessionTokens tracks cumulative token usage per session.
type SessionTokens struct {
	mu     sync.RWMutex
	tokens map[string]int
}

var sessionTokens = &SessionTokens{tokens: make(map[string]int)}

// ScorePrompt scores a prompt and tracks tokens for the session.
func ScorePrompt(prompt, sessionID string) ScorerResult {
	result := Score(prompt, nil)

	if sessionID != "" {
		sessionTokens.mu.Lock()
		sessionTokens.tokens[sessionID] += result.EstimatedTokens
		sessionTokens.mu.Unlock()

		ops.LogUsage(ops.UsageEntry{
			Agent:       sessionID,
			SessionID:   sessionID,
			InputTokens: result.EstimatedTokens,
			PromptScore: result.Total,
		})
	}

	return result
}

// GetSessionTokens returns cumulative tokens for a session.
func GetSessionTokens(sessionID string) int {
	sessionTokens.mu.RLock()
	defer sessionTokens.mu.RUnlock()
	return sessionTokens.tokens[sessionID]
}

// ResetSessionTokens clears token tracking for a session.
func ResetSessionTokens(sessionID string) {
	sessionTokens.mu.Lock()
	delete(sessionTokens.tokens, sessionID)
	sessionTokens.mu.Unlock()
}
