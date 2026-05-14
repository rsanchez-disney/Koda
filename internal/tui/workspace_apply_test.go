package tui

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	mdl "github.disney.com/SANCR225/koda/internal/model"
)

func newTestModel(workspaces []mdl.Workspace) model {
	m := model{
		screen:         screenWorkspaces,
		workspaces:     workspaces,
		wsDisplayOrder: make([]int, len(workspaces)),
	}
	for i := range workspaces {
		m.wsDisplayOrder[i] = i
	}
	return m
}

func TestUpdateWorkspaces_EnterSetsApplyingState(t *testing.T) {
	m := newTestModel([]mdl.Workspace{{Name: "test-ws"}})

	result, cmd := m.updateWorkspaces(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("enter")})
	// tea.KeyMsg for "enter" is actually tea.KeyEnter
	// Retry with correct key type
	result, cmd = m.updateWorkspaces(tea.KeyMsg{Type: tea.KeyEnter})

	rm := result.(model)
	if !rm.applyingWS {
		t.Error("expected applyingWS to be true after enter")
	}
	if rm.spinnerTick != 0 {
		t.Errorf("expected spinnerTick=0, got %d", rm.spinnerTick)
	}
	if cmd == nil {
		t.Error("expected a command to be returned")
	}
}

func TestUpdateWorkspaces_BlocksInputWhileApplying(t *testing.T) {
	m := newTestModel([]mdl.Workspace{{Name: "test-ws"}})
	m.applyingWS = true

	result, cmd := m.updateWorkspaces(tea.KeyMsg{Type: tea.KeyEnter})

	rm := result.(model)
	// Should remain unchanged — input blocked
	if rm.screen != screenWorkspaces {
		t.Error("expected screen to remain screenWorkspaces")
	}
	if cmd != nil {
		t.Error("expected no command when input is blocked")
	}
}

func TestUpdateWorkspaces_NavigationBlockedWhileApplying(t *testing.T) {
	m := newTestModel([]mdl.Workspace{{Name: "ws1"}, {Name: "ws2"}})
	m.applyingWS = true
	m.cursor = 0

	result, _ := m.updateWorkspaces(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	rm := result.(model)
	if rm.cursor != 0 {
		t.Errorf("expected cursor to stay at 0 while applying, got %d", rm.cursor)
	}
}

func TestUpdateWorkspaces_EscBlockedWhileApplying(t *testing.T) {
	m := newTestModel([]mdl.Workspace{{Name: "ws1"}})
	m.applyingWS = true

	result, _ := m.updateWorkspaces(tea.KeyMsg{Type: tea.KeyEsc})

	rm := result.(model)
	if rm.screen != screenWorkspaces {
		t.Error("expected screen to remain screenWorkspaces while applying")
	}
}

func TestWsApplyDoneMsg_Success(t *testing.T) {
	m := model{
		screen:     screenWorkspaces,
		applyingWS: true,
		steerRoot:  "/tmp/test",
		targetDir:  "/tmp/target",
	}

	msg := wsApplyDoneMsg{err: nil, name: "my-ws"}
	result, _ := m.Update(msg)

	rm := result.(model)
	if rm.applyingWS {
		t.Error("expected applyingWS to be false after done")
	}
	if rm.screen != screenDashboard {
		t.Error("expected screen to switch to dashboard on success")
	}
	if rm.statusMsg != "✅ Workspace 'my-ws' applied!" {
		t.Errorf("unexpected statusMsg: %s", rm.statusMsg)
	}
}

func TestWsApplyDoneMsg_Error(t *testing.T) {
	m := model{
		screen:     screenWorkspaces,
		applyingWS: true,
	}

	msg := wsApplyDoneMsg{err: fmt.Errorf("disk full"), name: "bad-ws"}
	result, _ := m.Update(msg)

	rm := result.(model)
	if rm.applyingWS {
		t.Error("expected applyingWS to be false after error")
	}
	if rm.screen == screenDashboard {
		t.Error("expected screen to NOT switch to dashboard on error")
	}
	if rm.statusMsg != "⚠ Workspace 'bad-ws' failed: disk full" {
		t.Errorf("unexpected statusMsg: %s", rm.statusMsg)
	}
}

func TestSpinnerTickMsg_IncrementsWhileApplying(t *testing.T) {
	m := model{applyingWS: true, spinnerTick: 3}

	result, cmd := m.Update(spinnerTickMsg{})

	rm := result.(model)
	if rm.spinnerTick != 4 {
		t.Errorf("expected spinnerTick=4, got %d", rm.spinnerTick)
	}
	if cmd == nil {
		t.Error("expected tick command to continue")
	}
}

func TestSpinnerTickMsg_StopsWhenNotApplying(t *testing.T) {
	m := model{applyingWS: false, spinnerTick: 5}

	result, cmd := m.Update(spinnerTickMsg{})

	rm := result.(model)
	if rm.spinnerTick != 5 {
		t.Errorf("expected spinnerTick unchanged, got %d", rm.spinnerTick)
	}
	if cmd != nil {
		t.Error("expected no command when not applying")
	}
}

func TestUpdateWorkspaces_DoubleEnterPrevented(t *testing.T) {
	m := newTestModel([]mdl.Workspace{{Name: "test-ws"}})

	// First enter — should start applying
	result, _ := m.updateWorkspaces(tea.KeyMsg{Type: tea.KeyEnter})
	rm := result.(model)
	if !rm.applyingWS {
		t.Fatal("expected applyingWS after first enter")
	}

	// Second enter — should be blocked
	result2, cmd2 := rm.updateWorkspaces(tea.KeyMsg{Type: tea.KeyEnter})
	rm2 := result2.(model)
	if cmd2 != nil {
		t.Error("expected no command on second enter")
	}
	if rm2.spinnerTick != rm.spinnerTick {
		t.Error("state should not change on blocked input")
	}
}
