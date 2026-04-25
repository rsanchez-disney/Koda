package kitestream

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestScorePrompt(t *testing.T) {
	result := ScorePrompt("Review the auth module for security vulnerabilities", "test-session")
	if result.Total <= 0 {
		t.Errorf("expected positive score, got %f", result.Total)
	}
	if result.EstimatedTokens <= 0 {
		t.Errorf("expected positive tokens, got %d", result.EstimatedTokens)
	}
	if len(result.Dimensions) != 7 {
		t.Errorf("expected 7 dimensions, got %d", len(result.Dimensions))
	}
}

func TestScorePrompt_Empty(t *testing.T) {
	result := ScorePrompt("", "")
	if result.Total != 0 {
		t.Errorf("expected 0 for empty prompt, got %f", result.Total)
	}
}

func TestSessionTokens(t *testing.T) {
	ResetSessionTokens("test-tok")
	ScorePrompt("hello world", "test-tok")
	tokens := GetSessionTokens("test-tok")
	if tokens <= 0 {
		t.Errorf("expected positive tokens, got %d", tokens)
	}
	ScorePrompt("another prompt here", "test-tok")
	tokens2 := GetSessionTokens("test-tok")
	if tokens2 <= tokens {
		t.Errorf("expected cumulative tokens to increase: %d -> %d", tokens, tokens2)
	}
	ResetSessionTokens("test-tok")
	if GetSessionTokens("test-tok") != 0 {
		t.Error("expected 0 after reset")
	}
}

func TestScoreEndpoint(t *testing.T) {
	srv := NewServer("/tmp", "/tmp", 0, "tok")
	body := `{"prompt":"Implement a REST API for user authentication with JWT tokens","sessionId":"s1"}`
	req := httptest.NewRequest("POST", "/api/score", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != 200 {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	var result map[string]interface{}
	json.NewDecoder(w.Body).Decode(&result)
	if result["total"].(float64) <= 0 {
		t.Errorf("expected positive score")
	}
	dims := result["dimensions"].(map[string]interface{})
	if len(dims) != 7 {
		t.Errorf("expected 7 dimensions, got %d", len(dims))
	}
}

func TestTokensEndpoint(t *testing.T) {
	srv := NewServer("/tmp", "/tmp", 0, "tok")

	// Score something first
	body := `{"prompt":"test prompt","sessionId":"tok-test"}`
	req := httptest.NewRequest("POST", "/api/score", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Get tokens
	req2 := httptest.NewRequest("GET", "/api/tokens/tok-test", nil)
	req2.Header.Set("Authorization", "Bearer tok")
	w2 := httptest.NewRecorder()
	srv.mux.ServeHTTP(w2, req2)

	if w2.Code != 200 {
		t.Fatalf("status = %d", w2.Code)
	}
	var result map[string]int
	json.NewDecoder(w2.Body).Decode(&result)
	if result["tokens"] <= 0 {
		t.Errorf("expected positive tokens, got %d", result["tokens"])
	}
}
