package kitestream

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.disney.com/SANCR225/koda/internal/acp"
	"github.disney.com/SANCR225/koda/internal/ops"

	"golang.org/x/net/websocket"
)

// Session holds an ACP client and metadata.
type Session struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Agent     string    `json:"agentId"`
	Workspace string    `json:"workspaceId"`
	CreatedAt time.Time `json:"createdAt"`
	client    *acp.Client
	prompted  bool
}

// Bridge manages ACP sessions and WebSocket clients.
type Bridge struct {
	mu       sync.RWMutex
	sessions map[string]*Session
	active   string // active session ID
	wsConns  map[*websocket.Conn]bool
	wsMu     sync.Mutex
}

// KiteStreamEvent is sent to WebSocket/SSE clients.
type KiteStreamEvent struct {
	Type      string      `json:"type"`
	SessionID string      `json:"sessionId"`
	Data      interface{} `json:"data"`
	Timestamp string      `json:"timestamp"`
}

func NewBridge() *Bridge {
	return &Bridge{
		sessions: make(map[string]*Session),
		wsConns:  make(map[*websocket.Conn]bool),
	}
}

// CreateSession spawns a new ACP subprocess for the given agent.
func (b *Bridge) CreateSession(id, agent, workspace string) (*Session, error) {
	client, err := acp.SpawnWithTrust(agent, acp.TrustSupervised)
	if err != nil {
		return nil, fmt.Errorf("spawn ACP: %w", err)
	}
	if err := client.CreateSession(agent); err != nil {
		client.Close()
		return nil, fmt.Errorf("create ACP session: %w", err)
	}

	sess := &Session{
		ID: id, Agent: agent, Workspace: workspace,
		CreatedAt: time.Now(), client: client,
	}

	b.mu.Lock()
	b.sessions[id] = sess
	b.active = id
	b.mu.Unlock()

	// Pump ACP events to WebSocket clients
	go b.pumpEvents(sess)

	// Pump permission requests to WebSocket clients
	go b.pumpPermissions(sess)

	return sess, nil
}

// SendMessage sends a prompt to the active session.
func (b *Bridge) SendMessage(sessionID, content string) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found")
	}
	err := sess.client.SendMessage(content)
	if err == nil && !sess.prompted {
		sess.prompted = true
		// Name the session from first prompt (truncated)
		name := content
		if len(name) > 60 {
			name = name[:60]
		}
		sess.Name = name
		// Persist as named session file (async, best-effort)
		go PersistSessionName(sess.client.SessionID(), name)
	}
	return err
}

// CloseSession kills the ACP subprocess for a session.
func (b *Bridge) CloseSession(sessionID string) {
	b.mu.Lock()
	if sess, ok := b.sessions[sessionID]; ok {
		sess.client.Close()
		delete(b.sessions, sessionID)
	}
	b.mu.Unlock()
}

// GetSession returns session metadata.
func (b *Bridge) GetSession(id string) *Session {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.sessions[id]
}

// ListSessions returns all sessions.
func (b *Bridge) ListSessions() []*Session {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]*Session, 0, len(b.sessions))
	for _, s := range b.sessions {
		out = append(out, s)
	}
	return out
}

// ActiveSessionID returns the current active session.
func (b *Bridge) ActiveSessionID() string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.active
}

// SwitchAgent creates a new session with a different agent, preserving the old one.
func (b *Bridge) SwitchAgent(newID, agent, workspace string) (*Session, error) {
	sess, err := b.CreateSession(newID, agent, workspace)
	if err != nil {
		return nil, err
	}
	b.broadcast(KiteStreamEvent{
		Type: "agent_switch", SessionID: newID,
		Data:      map[string]string{"agentId": agent},
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	return sess, nil
}

// HandleWebSocket upgrades an HTTP connection to WebSocket.
func (b *Bridge) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	wsServer := websocket.Server{Handler: func(ws *websocket.Conn) {
		b.wsMu.Lock()
		b.wsConns[ws] = true
		b.wsMu.Unlock()

		defer func() {
			b.wsMu.Lock()
			delete(b.wsConns, ws)
			b.wsMu.Unlock()
			ws.Close()
		}()

		// Read client messages (prompts, approvals)
		for {
			var msg struct {
				Method string `json:"method"`
				Params struct {
					SessionID string `json:"sessionId"`
					Content   string `json:"content"`
				} `json:"params"`
			}
			if err := websocket.JSON.Receive(ws, &msg); err != nil {
				return
			}
			switch msg.Method {
			case "message":
				b.SendMessage(msg.Params.SessionID, msg.Params.Content)
			}
		}
	}}
	wsServer.ServeHTTP(w, r)
}

// pumpEvents reads ACP events and broadcasts to all WebSocket clients.
// pumpPermissions forwards permission requests from ACP to WebSocket clients.
func (b *Bridge) pumpPermissions(sess *Session) {
	for evt := range sess.client.PermissionCh {
		b.broadcast(KiteStreamEvent{
			Type:      "permission_request",
			SessionID: sess.ID,
			Data: map[string]interface{}{
				"id":       evt.ID,
				"toolName": evt.ToolName,
			},
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		})
		// Wait for response (with timeout)
		select {
		case decision := <-evt.ResponseCh:
			_ = decision // already sent to ACP by the handler
		case <-time.After(5 * time.Minute):
			evt.ResponseCh <- "deny"
		}
	}
}

// RespondPermission handles a permission decision from the WebSocket client.
func (b *Bridge) RespondPermission(sessionID string, permID interface{}, decision string) error {
	b.mu.RLock()
	sess, ok := b.sessions[sessionID]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("session not found")
	}
	sess.client.RespondPermission(permID, decision)
	return nil
}

func (b *Bridge) pumpEvents(sess *Session) {
	for event := range sess.client.Events {
		ksEvent := translateEvent(event, sess.ID)
		b.broadcast(ksEvent)
	}
	// ACP closed
	b.broadcast(KiteStreamEvent{
		Type: "session_end", SessionID: sess.ID,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func translateEvent(e acp.Event, sessionID string) KiteStreamEvent {
	ts := time.Now().UTC().Format(time.RFC3339)
	switch e.Type {
	case "MessageChunk":
		return KiteStreamEvent{Type: "token", SessionID: sessionID, Data: e.Chunk, Timestamp: ts}
	case "ToolCall":
		return KiteStreamEvent{Type: "tool_call", SessionID: sessionID, Data: map[string]interface{}{"id": e.Name, "name": e.Name, "status": "in_progress", "params": map[string]interface{}{}}, Timestamp: ts}
	case "ToolResult":
		return KiteStreamEvent{Type: "tool_result", SessionID: sessionID, Data: map[string]interface{}{"id": e.Name, "name": e.Name, "status": "completed", "params": map[string]interface{}{}}, Timestamp: ts}
	case "Complete":
		return KiteStreamEvent{Type: "session_end", SessionID: sessionID, Data: map[string]string{"reason": e.Reason}, Timestamp: ts}
	case "Metadata":
		return KiteStreamEvent{Type: "thinking", SessionID: sessionID, Data: map[string]float64{"usage": e.Usage}, Timestamp: ts}
	default:
		return KiteStreamEvent{Type: e.Type, SessionID: sessionID, Timestamp: ts}
	}
}

func (b *Bridge) broadcast(event KiteStreamEvent) {
	b.wsMu.Lock()
	defer b.wsMu.Unlock()
	for ws := range b.wsConns {
		websocket.JSON.Send(ws, event)
	}
}

// ResolveBestAgent finds the best default agent for a workspace.
func ResolveBestAgent(steerRoot, targetDir string) string {
	return ops.SuggestDefaultAgent(steerRoot, targetDir)
}
