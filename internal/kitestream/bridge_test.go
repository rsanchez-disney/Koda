package kitestream

import (
	"testing"

	"github.disney.com/SANCR225/koda/internal/acp"
)

func TestTranslateEvent_MessageChunk(t *testing.T) {
	e := acp.Event{Type: "MessageChunk", Chunk: "hello"}
	ks := translateEvent(e, "sess-1")
	if ks.Type != "token" {
		t.Errorf("type = %q, want %q", ks.Type, "token")
	}
	if ks.Data != "hello" {
		t.Errorf("data = %v, want %q", ks.Data, "hello")
	}
	if ks.SessionID != "sess-1" {
		t.Errorf("sessionID = %q, want %q", ks.SessionID, "sess-1")
	}
}

func TestTranslateEvent_ToolCall(t *testing.T) {
	e := acp.Event{Type: "ToolCall", Name: "read_file"}
	ks := translateEvent(e, "sess-2")
	if ks.Type != "tool_call" {
		t.Errorf("type = %q, want %q", ks.Type, "tool_call")
	}
}

func TestTranslateEvent_ToolResult(t *testing.T) {
	e := acp.Event{Type: "ToolResult", Name: "read_file"}
	ks := translateEvent(e, "sess-2")
	if ks.Type != "tool_result" {
		t.Errorf("type = %q, want %q", ks.Type, "tool_result")
	}
}

func TestTranslateEvent_Complete(t *testing.T) {
	e := acp.Event{Type: "Complete", Reason: "end_turn"}
	ks := translateEvent(e, "sess-3")
	if ks.Type != "session_end" {
		t.Errorf("type = %q, want %q", ks.Type, "session_end")
	}
}

func TestTranslateEvent_Metadata(t *testing.T) {
	e := acp.Event{Type: "Metadata", Usage: 42.5}
	ks := translateEvent(e, "sess-4")
	if ks.Type != "thinking" {
		t.Errorf("type = %q, want %q", ks.Type, "thinking")
	}
}

func TestNewBridge(t *testing.T) {
	b := NewBridge()
	if b == nil {
		t.Fatal("NewBridge returned nil")
	}
	if len(b.sessions) != 0 {
		t.Errorf("sessions should be empty, got %d", len(b.sessions))
	}
	if len(b.wsConns) != 0 {
		t.Errorf("wsConns should be empty, got %d", len(b.wsConns))
	}
}

func TestBridge_ListSessions_Empty(t *testing.T) {
	b := NewBridge()
	sessions := b.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestBridge_GetSession_NotFound(t *testing.T) {
	b := NewBridge()
	s := b.GetSession("nonexistent")
	if s != nil {
		t.Error("expected nil for nonexistent session")
	}
}

func TestBridge_ActiveSessionID_Empty(t *testing.T) {
	b := NewBridge()
	if b.ActiveSessionID() != "" {
		t.Errorf("expected empty active session, got %q", b.ActiveSessionID())
	}
}

func TestBridge_SendMessage_NoSession(t *testing.T) {
	b := NewBridge()
	err := b.SendMessage("nonexistent", "hello")
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestBridge_CloseSession_NoOp(t *testing.T) {
	b := NewBridge()
	// Should not panic
	b.CloseSession("nonexistent")
}
