package team

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// Team is a running team instance.
type Team struct {
	mu          sync.RWMutex
	ID          string
	Spec        TeamSpec
	Goal        string
	Workers     map[string]*Worker
	WorkerOrder []string
	Results     map[string]string
	Worktrees   *GitWorktreeManager
	RepoRoot    string
	StartedAt   time.Time
	Events      chan WorkerEvent
}

// NewTeam creates a team from a spec.
func NewTeam(id string, spec TeamSpec, goal, repoRoot string) *Team {
	t := &Team{
		ID:       id,
		Spec:     spec,
		Goal:     goal,
		Workers:  make(map[string]*Worker),
		Results:  make(map[string]string),
		RepoRoot: repoRoot,
		Events:   make(chan WorkerEvent, 200),
	}

	for _, ws := range spec.Workers {
		w := NewWorker(ws.ID, ws.Role, ws.AgentConfig, ws.Model, TrustLevel(ws.TrustLevel), ws.TaskTemplate, ws.DependsOn)
		t.Workers[ws.ID] = w
		t.WorkerOrder = append(t.WorkerOrder, ws.ID)
	}

	return t
}

// Run executes the team plan respecting dependencies.
func (t *Team) Run() error {
	t.StartedAt = time.Now()
	t.Worktrees = NewWorktreeManager(t.RepoRoot)

	// Provision worktrees
	for _, ws := range t.Spec.Workers {
		w := t.Workers[ws.ID]
		wtPath, branch, err := t.Worktrees.Create(t.ID, ws.ID, t.Spec.BaseBranch)
		if err != nil {
			return fmt.Errorf("worktree for %s: %w", ws.ID, err)
		}
		w.WorktreePath = wtPath
		w.Branch = branch
	}

	// Execute in dependency order
	completed := map[string]bool{}
	for len(completed) < len(t.Spec.Workers) {
		// Find ready workers (all deps completed)
		var ready []*Worker
		for _, ws := range t.Spec.Workers {
			w := t.Workers[ws.ID]
			if completed[ws.ID] || w.GetState() == StateRunning {
				continue
			}
			depsOK := true
			for _, dep := range ws.DependsOn {
				if !completed[dep] {
					depsOK = false
					break
				}
			}
			if depsOK && w.GetState() == StateIdle {
				ready = append(ready, w)
			}
		}

		if len(ready) == 0 {
			// Wait for a running worker to finish
			event := <-t.Events
			t.handleEvent(event, completed)
			continue
		}

		// Launch ready workers in parallel
		var wg sync.WaitGroup
		for _, w := range ready {
			wg.Add(1)
			go func(w *Worker) {
				defer wg.Done()
				spec := t.findSpec(w.ID)
				handoff := BuildHandoff(spec, t.Goal, t.Results)
				if err := w.Start(handoff); err != nil {
					t.Events <- WorkerEvent{WorkerID: w.ID, Type: "StateChange", Data: string(StateFailed)}
					return
				}
				// Forward worker events to team
				for evt := range w.Events {
					t.Events <- evt
				}
			}(w)
		}

		// Process events while workers run
		for {
			select {
			case event := <-t.Events:
				t.handleEvent(event, completed)
				// Check if all running workers from this batch are done
				allDone := true
				for _, w := range ready {
					s := w.GetState()
					if s != StateCompleted && s != StateFailed {
						allDone = false
					}
				}
				if allDone {
					goto nextPhase
				}
			}
		}
	nextPhase:
		wg.Wait()
	}

	return nil
}

// Abort kills all running workers.
func (t *Team) Abort() {
	for _, w := range t.Workers {
		if w.GetState() == StateRunning {
			w.Abort()
		}
	}
}

// Status returns a summary string.
func (t *Team) Status() string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Team: %s (%s)\n", t.Spec.Name, t.ID))
	b.WriteString(fmt.Sprintf("Goal: %s\n\n", t.Goal))

	done := 0
	for _, id := range t.WorkerOrder {
		w := t.Workers[id]
		state := w.GetState()
		if state == StateCompleted {
			done++
		}
		b.WriteString(fmt.Sprintf("  [%s] %-20s %s\n", stateIcon(state), w.Role, state))
		if w.LastLine != "" {
			b.WriteString(fmt.Sprintf("       %s\n", w.LastLine))
		}
	}
	b.WriteString(fmt.Sprintf("\nProgress: %d/%d workers\n", done, len(t.Workers)))
	return b.String()
}

func (t *Team) handleEvent(event WorkerEvent, completed map[string]bool) {
	if event.Type == "Complete" {
		completed[event.WorkerID] = true
		w := t.Workers[event.WorkerID]
		t.mu.Lock()
		t.Results[event.WorkerID] = w.Result
		t.mu.Unlock()
	}
	if event.Type == "StateChange" && event.Data == string(StateFailed) {
		completed[event.WorkerID] = true
	}
}

func (t *Team) findSpec(workerID string) WorkerSpec {
	for _, ws := range t.Spec.Workers {
		if ws.ID == workerID {
			return ws
		}
	}
	return WorkerSpec{}
}

func stateIcon(s WorkerState) string {
	switch s {
	case StateIdle:
		return "\u25cb"
	case StateProvisioning, StateInitializing:
		return "\u25d4"
	case StateRunning:
		return "\u25b6"
	case StateCompleted:
		return "\u2713"
	case StateFailed:
		return "\u2717"
	case StateAwaitingPermission:
		return "\u26a0"
	default:
		return "?"
	}
}
