package kitestream

import (
	"sync"

	ps "github.com/disney/prompt-scorer/go-prompt-scorer"
)

// SessionTokens tracks cumulative token usage per session.
type SessionTokens struct {
	mu     sync.RWMutex
	tokens map[string]int
}

var sessionTokens = &SessionTokens{tokens: make(map[string]int)}

// ScorePrompt scores a prompt and tracks tokens for the session.
func ScorePrompt(prompt, sessionID string) ps.Result {
	result := ps.Score(prompt, nil)

	if sessionID != "" {
		sessionTokens.mu.Lock()
		sessionTokens.tokens[sessionID] += result.EstimatedTokens
		sessionTokens.mu.Unlock()
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
