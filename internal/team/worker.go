package team

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.disney.com/SANCR225/koda/internal/acp"
)

// WorkerState represents the lifecycle state of a worker.
type WorkerState string

const (
	StateIdle               WorkerState = "IDLE"
	StateProvisioning       WorkerState = "PROVISIONING"
	StateInitializing       WorkerState = "INITIALIZING"
	StateRunning            WorkerState = "RUNNING"
	StateAwaitingPermission WorkerState = "AWAITING_PERMISSION"
	StateCompleted          WorkerState = "COMPLETED"
	StateFailed             WorkerState = "FAILED"
)

// TrustLevel controls tool approval behavior.
type TrustLevel string

const (
	TrustAutonomous TrustLevel = "autonomous"
	TrustSupervised TrustLevel = "supervised"
	TrustStrict     TrustLevel = "strict"
)

// Worker represents one kiro-cli ACP process in a team.
type Worker struct {
	mu           sync.RWMutex
	ID           string
	Role         string
	Agent        string
	Model        string
	Trust        TrustLevel
	Task         string
	DependsOn    []string
	WorktreePath string
	Branch       string
	State        WorkerState
	Client       *acp.Client
	Output       []string
	Messages     []string
	PermissionCh chan PermissionRequest
	LastLine     string
	ContextUsage float64
	Credits      float64
	StartedAt    time.Time
	FinishedAt   time.Time
	Error        string
	Result       string
	Events       chan WorkerEvent
}

// WorkerEvent is emitted by a worker for the orchestrator.
type WorkerEvent struct {
	WorkerID string
	Type     string // StateChange, Chunk, ToolCall, ToolResult, Complete, Metadata, Permission
	Data     string
}

// PermissionRequest is sent when a tool call needs user approval.
type PermissionRequest struct {
	ToolCallID string
	Title      string
	ResponseCh chan string // send "allow_once", "allow_always", or "reject_once"
}

// NewWorker creates a worker from a spec entry.
func NewWorker(id, role, agent, model string, trust TrustLevel, task string, dependsOn []string) *Worker {
	return &Worker{
		ID:        id,
		Role:      role,
		Agent:     agent,
		Model:     model,
		Trust:     trust,
		Task:      task,
		DependsOn:    dependsOn,
		State:        StateIdle,
		Events:       make(chan WorkerEvent, 100),
		PermissionCh: make(chan PermissionRequest, 5),
	}
}

// SetState transitions the worker state and emits an event.
func (w *Worker) SetState(s WorkerState) {
	w.mu.Lock()
	w.State = s
	w.mu.Unlock()
	w.Events <- WorkerEvent{WorkerID: w.ID, Type: "StateChange", Data: string(s)}
}

// GetState returns the current state.
func (w *Worker) GetState() WorkerState {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.State
}

// Start spawns the kiro-cli process, creates a session, and sends the handoff.
func (w *Worker) Start(handoff string) error {
	w.SetState(StateProvisioning)
	w.StartedAt = time.Now()

	// Spawn ACP client with worktree as cwd
	client, err := acp.SpawnWithCwd(w.Agent, w.WorktreePath)
	if err != nil {
		w.Error = err.Error()
		w.SetState(StateFailed)
		return err
	}
	w.Client = client

	w.SetState(StateInitializing)
	if err := client.CreateSession(w.Agent); err != nil {
		w.Error = err.Error()
		w.SetState(StateFailed)
		client.Close()
		return err
	}

	w.SetState(StateRunning)

	// Send handoff as first prompt
	if err := client.SendMessage(handoff); err != nil {
		w.Error = err.Error()
		w.SetState(StateFailed)
		return err
	}

	// Stream events in background
	go w.streamEvents()
	return nil
}

// Abort kills the worker process.
func (w *Worker) Abort() {
	if w.Client != nil {
		w.Client.Close()
	}
	w.Error = "aborted by user"
	w.SetState(StateFailed)
	w.FinishedAt = time.Now()
}

// SendPrompt injects a user message into the running session.
func (w *Worker) SendPrompt(text string) error {
	if w.Client == nil {
		return fmt.Errorf("worker not connected")
	}
	w.mu.Lock()
	w.Messages = append(w.Messages, "user: "+text)
	w.mu.Unlock()
	return w.Client.SendMessage(text)
}

// SetTrust updates the worker's trust level.
func (w *Worker) SetTrust(level TrustLevel) {
	w.mu.Lock()
	w.Trust = level
	w.mu.Unlock()
}

// GetMessages returns a copy of the message history.
func (w *Worker) GetMessages() []string {
	w.mu.RLock()
	defer w.mu.RUnlock()
	cp := make([]string, len(w.Messages))
	copy(cp, w.Messages)
	return cp
}

func (w *Worker) streamEvents() {
	var buf strings.Builder
	for event := range w.Client.Events {
		switch event.Type {
		case "MessageChunk":
			buf.WriteString(event.Chunk)
			w.mu.Lock()
			w.LastLine = lastLine(buf.String())
			w.mu.Unlock()
			w.Events <- WorkerEvent{WorkerID: w.ID, Type: "Chunk", Data: event.Chunk}

		case "ToolCall":
			w.Events <- WorkerEvent{WorkerID: w.ID, Type: "ToolCall", Data: event.Name}

		case "ToolResult":
			w.Events <- WorkerEvent{WorkerID: w.ID, Type: "ToolResult", Data: event.Name}

		case "Complete":
			w.mu.Lock()
			w.Result = buf.String()
			w.Output = append(w.Output, w.Result)
			w.Messages = append(w.Messages, "assistant: "+w.Result)
			w.FinishedAt = time.Now()
			w.mu.Unlock()
			w.SetState(StateCompleted)
			w.Events <- WorkerEvent{WorkerID: w.ID, Type: "Complete", Data: w.Result}
			return

		case "Metadata":
			w.mu.Lock()
			w.ContextUsage = event.Usage
			w.mu.Unlock()
			w.Events <- WorkerEvent{WorkerID: w.ID, Type: "Metadata", Data: fmt.Sprintf("%.1f%%", event.Usage*100)}
		}
	}
	// Channel closed — process died
	if w.GetState() == StateRunning {
		w.Error = "kiro-cli process exited unexpectedly"
		w.SetState(StateFailed)
		w.FinishedAt = time.Now()
	}
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) == 0 {
		return ""
	}
	l := lines[len(lines)-1]
	if len(l) > 80 {
		return l[:80]
	}
	return l
}

// Snapshot returns a thread-safe copy of volatile fields.
func (w *Worker) Snapshot() (usage float64, lastLine string) {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return w.ContextUsage, w.LastLine
}
