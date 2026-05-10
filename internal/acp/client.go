package acp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"

	"github.disney.com/SANCR225/koda/internal/ops"
)

var debugLog *log.Logger

// EnableDebug writes ACP traffic to the given log file.
func EnableDebug(path string) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return
	}
	debugLog = log.New(f, "", log.LstdFlags|log.Lmicroseconds)
	debugLog.Println("=== Koda ACP debug log ===")
}

func dbg(format string, args ...interface{}) {
	if debugLog != nil {
		debugLog.Printf(format, args...)
	}
}

// TrustLevel controls how the ACP client handles permission requests.
type TrustLevel string

const (
	TrustAutonomous TrustLevel = "autonomous" // auto-approve all
	TrustSupervised TrustLevel = "supervised" // route to PermissionCh for human decision
	TrustStrict     TrustLevel = "strict"     // deny destructive, allow reads
)

// PermissionEvent is emitted when a permission request needs human decision (supervised mode).
type PermissionEvent struct {
	ID         interface{}
	ToolName   string
	ResponseCh chan string // send "allow_once", "allow_always", or "deny"
}

// Event types emitted by the ACP read loop.
type Event struct {
	Type    string // MessageChunk, ToolCall, ToolResult, Complete, Metadata, Permission
	Chunk   string
	Name    string
	Reason  string
	Usage   float64
}

// Client wraps a kiro-cli acp subprocess.
type Client struct {
	cmd          *exec.Cmd
	stdin        *json.Encoder
	mu           sync.Mutex
	reqID        atomic.Uint64
	pending      map[string]chan json.RawMessage
	pendingMu    sync.Mutex
	Events       chan Event
	sessionID    string
	TrustLevel   TrustLevel
	PermissionCh chan PermissionEvent
}

type jsonRPCRequest struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      uint64      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type jsonRPCResponse struct {
	ID     interface{}     `json:"id,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
	Method string          `json:"method,omitempty"`
	Params json.RawMessage `json:"params,omitempty"`
}

// SpawnWithCwd starts kiro-cli acp with a custom working directory.
func SpawnWithCwd(agent, cwd string) (*Client, error) {
	return spawnInternal(agent, cwd, TrustAutonomous)
}

// SpawnWithTrust starts kiro-cli acp with a specific trust level.
func SpawnWithTrust(agent string, trust TrustLevel) (*Client, error) {
	return spawnInternal(agent, "", trust)
}

// SpawnWithCwdAndTrust starts kiro-cli acp with a custom cwd and trust level.
func SpawnWithCwdAndTrust(agent, cwd string, trust TrustLevel) (*Client, error) {
	return spawnInternal(agent, cwd, trust)
}

// Spawn starts kiro-cli acp and returns a connected client.
func Spawn(agent string) (*Client, error) {
	return spawnInternal(agent, "", TrustAutonomous)
}

func spawnInternal(agent, cwd string, trust TrustLevel) (*Client, error) {
	kiroPath := ops.FindKiroCLI()

	args := []string{"acp", "-a"}
	if agent != "" {
		args = append(args, "--agent", agent)
	}

	cmd := exec.Command(kiroPath, args...)
	if cwd != "" {
		cmd.Dir = cwd
	}
	if debugLog != nil {
		stderrPipe, _ := cmd.StderrPipe()
		go func() {
			if stderrPipe == nil {
				return
			}
			sc := bufio.NewScanner(stderrPipe)
			for sc.Scan() {
				dbg("STDERR: %s", sc.Text())
			}
		}()
	} else {
		cmd.Stderr = os.Stderr
	}

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	dbg("spawn: %s %v (trust=%s)", kiroPath, args, trust)
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start kiro-cli: %w", err)
	}
	dbg("spawn: pid=%d", cmd.Process.Pid)

	c := &Client{
		cmd:          cmd,
		stdin:        json.NewEncoder(stdinPipe),
		pending:      make(map[string]chan json.RawMessage),
		Events:       make(chan Event, 100),
		TrustLevel:   trust,
		PermissionCh: make(chan PermissionEvent, 5),
	}

	// Read loop
	go func() {
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
		for scanner.Scan() {
			line := scanner.Text()
			dbg("<< %s", truncLog(line, 500))
			var resp jsonRPCResponse
			if err := json.Unmarshal([]byte(line), &resp); err != nil {
				dbg("<< PARSE ERROR: %v line=%q", err, truncLog(line, 200))
				continue
			}

			// Server request (has both id and method) — e.g., permission requests
			if resp.ID != nil && resp.Method != "" {
				dbg("<< server request: %s id=%v", resp.Method, resp.ID)
				c.handleServerRequest(resp.Method, resp.ID, resp.Params)
				continue
			}

			// Response to our request
			if resp.ID != nil {
				idStr := fmt.Sprintf("%v", resp.ID)
				c.pendingMu.Lock()
				if ch, ok := c.pending[idStr]; ok {
					delete(c.pending, idStr)
					if resp.Error != nil {
						ch <- json.RawMessage(fmt.Sprintf(`{"error":%q}`, resp.Error.Message))
					} else {
						ch <- resp.Result
					}
				}
				c.pendingMu.Unlock()

				// Check for stopReason in result
				var result map[string]interface{}
				if json.Unmarshal(resp.Result, &result) == nil {
					if reason, ok := result["stopReason"].(string); ok {
						c.Events <- Event{Type: "Complete", Reason: reason}
					}
				}
				continue
			}

			// Notification (no id)
			if resp.Method != "" {
				c.handleNotification(resp.Method, resp.Params)
			}
		}
		dbg("read loop ended")
		close(c.Events)
	}()

	// Initialize
	_, err = c.request("initialize", map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]interface{}{},
		"clientInfo":      map[string]string{"name": "koda", "version": "0.1.0"},
	})
	if err != nil {
		cmd.Process.Kill()
		return nil, fmt.Errorf("ACP initialize failed: %w", err)
	}

	return c, nil
}

// CreateSession creates a new ACP session.
// cwd sets the working directory context; if empty, defaults to ~/.kiro.
func (c *Client) CreateSession(agent string, cwd ...string) error {
	home, _ := os.UserHomeDir()
	dir := home + "/.kiro"
	if len(cwd) > 0 && cwd[0] != "" {
		dir = cwd[0]
	}
	params := map[string]interface{}{
		"cwd":        dir,
		"mcpServers": []interface{}{},
	}
	if agent != "" {
		params["agentId"] = agent
	}
	if ctx := buildRecentSessionContext(home); ctx != "" {
		params["sessionContext"] = ctx
	}

	result, err := c.request("session/new", params)
	if err != nil {
		return err
	}

	var parsed map[string]interface{}
	if json.Unmarshal(result, &parsed) == nil {
		if sid, ok := parsed["sessionId"].(string); ok {
			c.sessionID = sid
		}
	}
	return nil
}

// SendMessage sends a prompt to the current session (fire-and-forget).
func (c *Client) SendMessage(content string) error {
	if c.sessionID == "" {
		return fmt.Errorf("no session")
	}

	id := c.reqID.Add(1)
	req := jsonRPCRequest{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "session/prompt",
		Params: map[string]interface{}{
			"sessionId": c.sessionID,
			"prompt":    []map[string]string{{"type": "text", "text": content}},
		},
	}

	dbg(">> [%d] session/prompt len=%d", id, len(content))
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.stdin.Encode(req)
}

// RespondPermission sends a permission decision for a pending request.
func (c *Client) RespondPermission(id interface{}, decision string) {
	c.respondPermission(id, decision)
}

// SessionID returns the current session ID.
func (c *Client) SessionID() string {
	return c.sessionID
}

// Close kills the subprocess.
func (c *Client) Close() {
	if c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
}

func (c *Client) request(method string, params interface{}) (json.RawMessage, error) {
	id := c.reqID.Add(1)
	ch := make(chan json.RawMessage, 1)
	idStr := fmt.Sprintf("%d", id)

	c.pendingMu.Lock()
	c.pending[idStr] = ch
	c.pendingMu.Unlock()

	req := jsonRPCRequest{JSONRPC: "2.0", ID: id, Method: method, Params: params}
	dbg(">> [%d] %s", id, method)
	c.mu.Lock()
	err := c.stdin.Encode(req)
	c.mu.Unlock()
	if err != nil {
		dbg(">> SEND ERROR: %v", err)
		return nil, err
	}

	result := <-ch
	dbg("<< [%d] result: %s", id, truncLog(string(result), 200))
	return result, nil
}

func (c *Client) handleNotification(method string, params json.RawMessage) {
	var p map[string]interface{}
	if json.Unmarshal(params, &p) != nil {
		return
	}

	dbg("<< notify: %s", method)
	switch method {
	case "session/update":
		update, _ := p["update"].(map[string]interface{})
		if update == nil {
			return
		}
		switch update["sessionUpdate"] {
		case "agent_message_chunk":
			content, _ := update["content"].(map[string]interface{})
			if text, ok := content["text"].(string); ok {
				dbg("   chunk: %s", truncLog(text, 100))
				c.Events <- Event{Type: "MessageChunk", Chunk: text}
			}
		case "tool_call", "tool_call_update":
			name, _ := update["title"].(string)
			dbg("   tool: %s", name)
			c.Events <- Event{Type: "ToolCall", Name: name}
		case "tool_result":
			name, _ := update["title"].(string)
			dbg("   tool done: %s", name)
			c.Events <- Event{Type: "ToolResult", Name: name}
		}
	case "_kiro.dev/metadata":
		if usage, ok := p["contextUsagePercentage"].(float64); ok {
			c.Events <- Event{Type: "Metadata", Usage: usage}
		}
	}
}

func (c *Client) handleServerRequest(method string, id interface{}, params json.RawMessage) {
	switch method {
	case "session/request_permission":
		c.handlePermission(id, params)
	default:
		dbg("   unhandled server request: %s", method)
	}
}

func (c *Client) handlePermission(id interface{}, params json.RawMessage) {
	toolName := extractToolName(params)

	switch c.TrustLevel {
	case TrustSupervised:
		dbg("   supervised: requesting approval for %s", toolName)
		evt := PermissionEvent{
			ID:         id,
			ToolName:   toolName,
			ResponseCh: make(chan string, 1),
		}
		c.PermissionCh <- evt
		decision := <-evt.ResponseCh
		dbg("   supervised: decision=%s for %s", decision, toolName)
		c.respondPermission(id, decision)

	case TrustStrict:
		if isDestructiveTool(toolName) {
			dbg("   strict: deny %s", toolName)
			c.respondPermission(id, "deny")
		} else {
			dbg("   strict: allow_once %s", toolName)
			c.respondPermission(id, "allow_once")
		}

	default: // TrustAutonomous or unset
		dbg("   autonomous: allow_always %s", toolName)
		c.respondPermission(id, "allow_always")
	}
}

func (c *Client) respondPermission(id interface{}, optionID string) {
	resp := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      id,
		"result":  map[string]string{"optionId": optionID},
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stdin.Encode(resp)
	dbg(">> permission response: %s", optionID)
}

// extractToolName parses the tool name from permission request params.
func extractToolName(params json.RawMessage) string {
	var p map[string]interface{}
	if json.Unmarshal(params, &p) != nil {
		return "unknown"
	}
	if title, ok := p["title"].(string); ok {
		return title
	}
	if name, ok := p["name"].(string); ok {
		return name
	}
	return "unknown"
}

// isDestructiveTool returns true for tools that modify state.
func isDestructiveTool(name string) bool {
	lower := strings.ToLower(name)
	destructive := []string{"write", "fs_write", "shell", "execute_bash", "delete", "remove"}
	for _, d := range destructive {
		if strings.Contains(lower, d) {
			return true
		}
	}
	return false
}

// buildRecentSessionContext reads last 3 sessions from telemetry for cross-session awareness.
func buildRecentSessionContext(home string) string {
	path := home + "/.kiro/logs/telemetry.jsonl"
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	// Read last 3 entries (scan all, keep tail)
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) == 0 {
		return ""
	}
	start := len(lines) - 3
	if start < 0 {
		start = 0
	}

	var b strings.Builder
	b.WriteString("## Recent Sessions\n\n")
	for _, line := range lines[start:] {
		var e struct {
			Ts    string `json:"ts"`
			Agent string `json:"agent"`
			Dur   int    `json:"duration_ms"`
			Tools int    `json:"tool_calls"`
		}
		if json.Unmarshal([]byte(line), &e) == nil && e.Agent != "" {
			b.WriteString(fmt.Sprintf("- %s: %s (%ds, %d tools)\n", e.Ts, e.Agent, e.Dur/1000, e.Tools))
		}
	}
	result := b.String()
	if len(result) > 2048 {
		result = result[:2048]
	}
	return result
}

func truncLog(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
