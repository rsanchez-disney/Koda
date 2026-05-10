# Spec: orchestration, harness, and context improvements

**Repo**: Koda
**Epics**: E1, E3, E4, E8, E9, E10, E11, E12, E13, E14
**Estimated points**: 55 (of 171 total across all repos)
**Go version**: 1.25

---

## E1: Conditional context loading (13 pts — Koda portion: 5 pts)

### Problem

Koda's `model.Agent.Resources` is `[]string`. It needs to support the new object format with `when` and `priority` fields, and validate resource paths in both formats.

### Design

#### New type: `ResourceEntry`

```go
// ResourceEntry supports both string and object resource declarations.
type ResourceEntry struct {
    Path     string `json:"path"`
    When     string `json:"when,omitempty"`     // always, task_contains:X, agent_is:X, profile_is:X
    Priority string `json:"priority,omitempty"` // critical, high, normal, low
}

// UnmarshalJSON handles both "file://path" strings and {"path":...} objects.
func (r *ResourceEntry) UnmarshalJSON(data []byte) error {
    if data[0] == '"' {
        var s string
        if err := json.Unmarshal(data, &s); err != nil {
            return err
        }
        r.Path = s
        r.When = "always"
        r.Priority = "normal"
        return nil
    }
    type alias ResourceEntry
    return json.Unmarshal(data, (*alias)(r))
}
```

#### Changes to `model.Agent`

```go
type Agent struct {
    // ... existing fields unchanged ...
    Resources []ResourceEntry `json:"resources,omitempty"` // was []string
}
```

#### Doctor validation

- Validate resource paths resolve for both formats
- Warn if any resource file >30KB without a `when` condition
- Warn if total static context per agent >50KB

### Files to modify

| File                        | Change                                          |
|-----------------------------|-------------------------------------------------|
| `internal/model/agent.go`  | Add `ResourceEntry` type, change `Resources` field |
| `internal/ops/doctor.go`   | Add context size audit                          |
| `internal/ops/install.go`  | Update resource path validation                 |

---

## E3: Trust level enforcement (13 pts)

### Problem

`handleServerRequest` in `acp/client.go` unconditionally auto-approves all permission requests. The `TrustLevel` type exists in `team/worker.go` but is never wired to the ACP client. Koda v0.5.0 added a `--trust` flag at session launch (which tools to allow), but E3 addresses per-permission enforcement *during* execution.

### Design

#### ACP client changes

```go
type Client struct {
    // ... existing fields ...
    TrustLevel  TrustLevel
    PermissionCh chan PermissionEvent // for supervised mode
}

type PermissionEvent struct {
    ID         interface{}
    Method     string
    ToolName   string
    Params     json.RawMessage
    ResponseCh chan string // "allow_once", "allow_always", "deny"
}

func SpawnWithTrust(agent string, trust TrustLevel) (*Client, error) {
    c, err := spawnInternal(agent, "")
    if err != nil {
        return nil, err
    }
    c.TrustLevel = trust
    return c, nil
}
```

#### Permission handling logic

```go
func (c *Client) handleServerRequest(method string, id interface{}, params json.RawMessage) {
    switch method {
    case "session/request_permission":
        switch c.TrustLevel {
        case TrustAutonomous:
            c.respondPermission(id, "allow_always")
        case TrustSupervised:
            evt := PermissionEvent{ID: id, Params: params, ResponseCh: make(chan string, 1)}
            // parse tool name from params for display
            var p map[string]interface{}
            json.Unmarshal(params, &p)
            evt.ToolName, _ = p["title"].(string)
            c.PermissionCh <- evt
            decision := <-evt.ResponseCh
            c.respondPermission(id, decision)
        case TrustStrict:
            if isDestructivePermission(params) {
                c.respondPermission(id, "deny")
            } else {
                c.respondPermission(id, "allow_once")
            }
        default:
            c.respondPermission(id, "allow_always")
        }
    }
}

func isDestructivePermission(params json.RawMessage) bool {
    var p map[string]interface{}
    json.Unmarshal(params, &p)
    title, _ := p["title"].(string)
    destructive := []string{"fs_write", "execute_bash", "shell", "write"}
    for _, d := range destructive {
        if strings.Contains(strings.ToLower(title), d) {
            return true
        }
    }
    return false
}
```

#### Worker integration

```go
// In Worker.Start():
client, err := acp.SpawnWithTrust(w.Agent, acp.TrustLevel(w.Trust))

// In Worker.streamEvents() — new case:
case "Permission":
    req := PermissionRequest{ToolCallID: event.Name, ResponseCh: make(chan string, 1)}
    w.PermissionCh <- req
    decision := <-req.ResponseCh
    w.Client.RespondPermission(event.ID, decision)
```

#### KiteStream integration

- `POST /api/sessions` accepts optional `trust` parameter (default: `supervised`)
- Permission events forwarded over WebSocket
- Web UI shows approve/deny buttons

### Files to modify

| File                                | Change                                              |
|-------------------------------------|-----------------------------------------------------|
| `internal/acp/client.go`           | Add TrustLevel, PermissionCh, SpawnWithTrust, update handleServerRequest |
| `internal/team/worker.go`          | Wire trust to ACP spawn, handle Permission events   |
| `internal/team/orchestrator.go`    | Pass trust from WorkerSpec to Worker                |
| `internal/team/planner.go`         | Update prompt with trust level guidance             |
| `internal/team/teamspec.go`        | Add trust validation to ValidateDeps               |
| `internal/kitestream/bridge.go`    | Pass trust to ACP spawn                            |
| `internal/kitestream/handlers.go`  | Accept trust param, forward permission events      |

### Backward compatibility

Default trust is `autonomous` — identical to current behavior. Existing code paths unchanged unless trust is explicitly set.

---

## E4: Extended hook lifecycle (10 pts — Koda portion: 2 pts)

### Problem

Koda's `model.AgentHooks` struct only has `AgentSpawn`, `PreToolUse`, `PostToolUse`. It needs to support the 3 new events.

### Design

```go
type AgentHooks struct {
    AgentSpawn    []HookDef `json:"agentSpawn,omitempty"`
    PreToolUse    []HookDef `json:"preToolUse,omitempty"`
    PostToolUse   []HookDef `json:"postToolUse,omitempty"`
    AgentComplete []HookDef `json:"agentComplete,omitempty"` // NEW
    AgentFailed   []HookDef `json:"agentFailed,omitempty"`   // NEW
    AgentTimeout  []HookDef `json:"agentTimeout,omitempty"`  // NEW
}
```

Doctor validates new hook event references. Install copies hook scripts for new events.

### Files to modify

| File                        | Change                              |
|-----------------------------|-------------------------------------|
| `internal/model/agent.go`  | Add 3 fields to `AgentHooks`        |
| `internal/ops/doctor.go`   | Validate new hook references        |
| `internal/ops/install.go`  | Handle new hook events              |

---

## E8: Token budgeting (10 pts — Koda portion: 4 pts)

### Problem

Koda needs to read `contextBudget` from agent JSON and pass it through ACP session creation.

### Design

```go
type Agent struct {
    // ... existing fields ...
    ContextBudget map[string]float64 `json:"contextBudget,omitempty"`
}
```

#### Doctor validation

- Warn if budget values sum to >1.0
- Warn if any category <0.05

#### ACP session creation

```go
func (c *Client) CreateSession(agent string, cwd ...string) error {
    params := map[string]interface{}{
        "cwd":        dir,
        "mcpServers": []interface{}{},
    }
    if agent != "" {
        params["agentId"] = agent
    }
    if c.contextBudget != nil {
        params["contextBudget"] = c.contextBudget
    }
    // ...
}
```

### Files to modify

| File                        | Change                                  |
|-----------------------------|-----------------------------------------|
| `internal/model/agent.go`  | Add `ContextBudget` field               |
| `internal/ops/doctor.go`   | Add budget validation                   |
| `internal/acp/client.go`   | Include budget in session/new params    |

---

## E9: Worker retry and recovery (10 pts)

### Problem

When a worker fails, it's marked `StateFailed` with no retry. The entire team stalls for dependents. No recovery mechanism exists.

### Design

#### WorkerSpec extension

```go
type WorkerSpec struct {
    // ... existing fields ...
    MaxRetries int    `json:"maxRetries,omitempty"`  // default 0
    RetryDelay string `json:"retryDelay,omitempty"`  // e.g. "5s"
    OnFailure  string `json:"onFailure,omitempty"`   // "skip" | "abort" (default) | "replan"
}
```

#### Retry loop in Team.Run()

```go
func (t *Team) executeWorker(w *Worker, handoff string) error {
    maxAttempts := w.MaxRetries + 1
    for attempt := 1; attempt <= maxAttempts; attempt++ {
        err := w.Start(handoff)
        if err == nil {
            // wait for completion via events
            return nil
        }
        if attempt < maxAttempts {
            delay, _ := time.ParseDuration(w.RetryDelay)
            time.Sleep(delay)
            handoff = fmt.Sprintf("%s\n\n---\nPrevious attempt %d/%d failed: %s. Retry.",
                handoff, attempt, maxAttempts, err)
            w.Reset() // reset state to IDLE, close old client
        }
    }
    return handleFailure(w)
}

func handleFailure(w *Worker) error {
    switch w.OnFailure {
    case "skip":
        w.SetState(StateSkipped)
        return nil
    case "replan":
        return ErrReplanRequested
    default: // "abort"
        return fmt.Errorf("worker %s failed: %s", w.ID, w.Error)
    }
}
```

#### Replan

```go
func (t *Team) Replan(failedID, error string) (*TeamSpec, error) {
    augmented := fmt.Sprintf("%s\n\nWorker '%s' failed: %s\nCompleted: %v",
        t.Goal, failedID, error, t.completedWorkerIDs())
    return GeneratePlan(augmented)
}
```

Limited to 1 replan per team run.

### Files to modify

| File                              | Change                                  |
|-----------------------------------|-----------------------------------------|
| `internal/team/teamspec.go`      | Add retry fields, validation            |
| `internal/team/worker.go`        | Add `Reset()` method                    |
| `internal/team/orchestrator.go`  | Add retry loop, replan logic            |
| `internal/team/planner.go`       | Update prompt with retry guidance       |

---

## E10: Structured observability (13 pts — Koda portion: 3 pts)

### Problem

No centralized telemetry. `koda stats` doesn't exist or is minimal.

### Design

New `koda stats` command reads `~/.kiro/logs/telemetry.jsonl` (written by steer-runtime's telemetry hook).

```text
$ koda stats
Today: 12 sessions, 145K tokens, avg 38s
Top agents: backend (5), orchestrator (3), ui (2), code_review (1), webapi (1)

$ koda stats --week
Mon: 8 sessions | Tue: 15 sessions | Wed: 12 sessions | ...

$ koda stats --agent backend
Sessions: 23 (last 7 days), avg duration: 42s, avg context: 31%
```

### Files to modify

| File                        | Change                              |
|-----------------------------|-------------------------------------|
| `internal/cli/stats.go`   | Parse telemetry JSONL, render stats |

---

## E11: Blackboard for multi-agent coordination (10 pts)

### Problem

Team workers are completely isolated. No mid-execution communication. A worker can't share discoveries with downstream workers beyond the one-shot handoff.

### Design

#### Blackboard lifecycle

```go
// In Team.Run() — before worker execution:
blackboardPath := filepath.Join(".koda", "team", t.ID, "blackboard.md")
os.MkdirAll(filepath.Dir(blackboardPath), 0755)
initBlackboard(blackboardPath, t.Goal, t.Workers)

// Each worker's worktree gets a symlink:
os.Symlink(blackboardPath, filepath.Join(worktreePath, ".blackboard.md"))
```

#### Handoff includes blackboard

```go
func BuildHandoff(spec WorkerSpec, goal string, priorResults map[string]string, blackboard string) string {
    // ... existing handoff ...
    if blackboard != "" {
        sb.WriteString("\n## Shared Blackboard\n\n")
        sb.WriteString(blackboard)
    }
    // ...
}
```

#### Worker output parsing

Workers write `[KODA_BLACKBOARD] <content>` markers. After each worker completes, the orchestrator appends the content to the blackboard file.

#### Conflict detection

Before launching each wave, read blackboard for `[KODA_CONFLICT] file: <path>` markers. Log warnings for flagged conflicts (advisory only, doesn't block).

### Files to modify

| File                              | Change                                  |
|-----------------------------------|-----------------------------------------|
| `internal/team/orchestrator.go`  | Create/manage blackboard, read between waves |
| `internal/team/teamspec.go`      | Include blackboard in BuildHandoff      |
| `internal/team/worker.go`        | Parse KODA_BLACKBOARD markers           |
| `internal/team/merge.go`         | Clean up blackboard on merge            |

---

## E12: Hierarchical memory (15 pts — Koda portion: 8 pts)

### Problem

No cross-session context injection. Each session starts fresh with no awareness of recent work.

### Design

#### Session context injection

On `session/new`, Koda reads last 3 session summaries from `~/.kiro/logs/telemetry.jsonl` and injects as context:

```go
func buildSessionContext() string {
    entries := readLastNSessions("~/.kiro/logs/telemetry.jsonl", 3)
    if len(entries) == 0 {
        return ""
    }
    var sb strings.Builder
    sb.WriteString("## Recent Sessions\n\n")
    for _, e := range entries {
        sb.WriteString(fmt.Sprintf("- %s: %s (%ds, %d tools)\n", e.Ts, e.Agent, e.DurationMs/1000, e.ToolCalls))
    }
    return sb.String() // max 2KB
}
```

#### Memory staleness detection

```go
// In doctor.go:
func checkMemoryBankStaleness() {
    activeCtx := filepath.Join(kiroRoot, "memory-bank", "active-context.md")
    info, _ := os.Stat(activeCtx)
    if time.Since(info.ModTime()) > 7*24*time.Hour {
        warn("active-context.md is %d days stale", daysSince(info.ModTime()))
    }
}
```

#### `koda memory refresh`

Triggers a summarization pass: reads recent telemetry, generates a summary, appends to `active-context.md`.

### Files to modify

| File                          | Change                                  |
|-------------------------------|-----------------------------------------|
| `internal/acp/client.go`    | Inject session context in CreateSession |
| `internal/ops/doctor.go`    | Add staleness check                     |
| `internal/cli/memory.go`    | Add `refresh` subcommand                |

---

## E13: RAG-based context retrieval (15 pts — Koda portion: 10 pts)

### Problem

Large knowledge bases load unconditionally. A retrieval approach would load only relevant chunks.

### Design

#### Index building at install time

```go
// In ops/install.go — after copying context files:
func BuildContextIndex(contextDir string) error {
    index := TFIDFIndex{}
    files, _ := filepath.Glob(filepath.Join(contextDir, "*.md"))
    for _, f := range files {
        content, _ := os.ReadFile(f)
        chunks := splitIntoChunks(string(content), 500) // ~500 tokens per chunk
        for i, chunk := range chunks {
            index.Add(f, i, chunk)
        }
    }
    return index.Save(filepath.Join(contextDir, "_index.json"))
}
```

#### Query interface

```go
func (idx *TFIDFIndex) Query(query string, topK int) []Chunk {
    queryTerms := tokenize(query)
    scores := map[string]float64{}
    for _, term := range queryTerms {
        for docID, tfidf := range idx.Scores[term] {
            scores[docID] += tfidf
        }
    }
    return topKByScore(scores, topK)
}
```

#### Eval command

```go
// koda eval context-retrieval
// Runs 10 predefined queries, checks expected file matches, reports precision/recall
```

### Files to modify

| File                            | Change                              |
|---------------------------------|-------------------------------------|
| `internal/ops/install.go`      | Call BuildContextIndex after sync   |
| `internal/ops/index.go`        | Create — TF-IDF index implementation |
| `internal/cli/eval.go`         | Add context-retrieval eval command  |

---

## E14: Dynamic tool injection by phase (13 pts — Koda portion: 5 pts)

### Problem

Koda needs to read `phases` from agent JSON and pass the active phase to kiro-cli during session creation.

### Design

```go
type Agent struct {
    // ... existing fields ...
    Phases map[string]PhaseConfig `json:"phases,omitempty"`
}

type PhaseConfig struct {
    Tools        []string `json:"tools"`
    AllowedTools []string `json:"allowedTools"`
}
```

#### ACP session creation with phase

```go
func (c *Client) CreateSessionWithPhase(agent, phase string, cwd ...string) error {
    params := map[string]interface{}{
        "cwd":        dir,
        "mcpServers": []interface{}{},
        "agentId":    agent,
    }
    if phase != "" {
        params["phase"] = phase
    }
    // ...
}
```

### Files to modify

| File                        | Change                                  |
|-----------------------------|-----------------------------------------|
| `internal/model/agent.go`  | Add `Phases` and `PhaseConfig`          |
| `internal/acp/client.go`   | Add `CreateSessionWithPhase`            |
