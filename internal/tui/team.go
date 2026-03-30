package tui

import (
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.disney.com/SANCR225/koda/internal/team"
)

var (
	cardStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1).Width(38)
	runningStyle = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#2563EB", Dark: "#38BDF8"})
	doneStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#059669", Dark: "#10B981"})
	failStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#DC2626", Dark: "#EF4444"})
	idleStyle    = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "#6B7280", Dark: "#9CA3AF"})
)

type teamEventMsg team.WorkerEvent
type teamDoneMsg struct{}
type teamTickMsg time.Time

type teamModel struct {
	team     *team.Team
	spec     team.TeamSpec
	goal     string
	repoRoot string
	cursor   int
	width    int
	height   int
	started  bool
	done     bool
	err      error
}

// RunTeamDashboard launches the TUI team dashboard.
func RunTeamDashboard(spec team.TeamSpec, goal, repoRoot string) error {
	p := tea.NewProgram(initialTeamModel(spec, goal, repoRoot), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func initialTeamModel(spec team.TeamSpec, goal, repoRoot string) teamModel {
	return teamModel{spec: spec, goal: goal, repoRoot: repoRoot}
}

func (m teamModel) Init() tea.Cmd {
	return func() tea.Msg {
		teamID := fmt.Sprintf("%s-%s", m.spec.Name, time.Now().Format("20060102-150405"))
		t := team.NewTeam(teamID, m.spec, m.goal, m.repoRoot)

		// Run team in background
		go func() {
			t.Run()
		}()

		return teamStarted{team: t}
	}
}

type teamStarted struct{ team *team.Team }

func listenTeamEvents(t *team.Team) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-t.Events
		if !ok {
			return teamDoneMsg{}
		}
		return teamEventMsg(event)
	}
}

func tickCmd() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
		return teamTickMsg(t)
	})
}

func (m teamModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case teamStarted:
		m.team = msg.team
		m.started = true
		return m, tea.Batch(listenTeamEvents(m.team), tickCmd())

	case teamEventMsg:
		if msg.Type == "Complete" || msg.Type == "StateChange" {
			// Check if all workers done
			allDone := true
			for _, id := range m.team.WorkerOrder {
				s := m.team.Workers[id].GetState()
				if s != team.StateCompleted && s != team.StateFailed {
					allDone = false
				}
			}
			if allDone {
				m.done = true
			}
		}
		return m, listenTeamEvents(m.team)

	case teamDoneMsg:
		m.done = true
		return m, nil

	case teamTickMsg:
		if !m.done {
			return m, tickCmd()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			if m.team != nil && !m.done {
				m.team.Abort()
			}
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.team != nil && m.cursor < len(m.team.WorkerOrder)-1 {
				m.cursor++
			}
		case "a":
			// Abort selected worker
			if m.team != nil && m.cursor < len(m.team.WorkerOrder) {
				id := m.team.WorkerOrder[m.cursor]
				w := m.team.Workers[id]
				if w.GetState() == team.StateRunning {
					w.Abort()
				}
			}
		}
	}
	return m, nil
}

func (m teamModel) View() string {
	if !m.started {
		return "  Starting team...\n"
	}

	var b strings.Builder

	// Header
	b.WriteString(headerStyle.Render(fmt.Sprintf(" \U0001f43e Team: %s ", m.spec.Name)))
	if m.goal != "" {
		b.WriteString("  " + toolStyle.Render(m.goal))
	}
	b.WriteString("\n\n")

	// Worker cards
	for i, id := range m.team.WorkerOrder {
		w := m.team.Workers[id]
		state := w.GetState()

		// Status styling
		var stateStr string
		switch state {
		case team.StateRunning:
			stateStr = runningStyle.Render("\u25b6 RUNNING")
		case team.StateCompleted:
			stateStr = doneStyle.Render("\u2713 DONE")
		case team.StateFailed:
			stateStr = failStyle.Render("\u2717 FAILED")
		case team.StateProvisioning, team.StateInitializing:
			stateStr = runningStyle.Render("\u25d4 STARTING")
		default:
			stateStr = idleStyle.Render("\u25cb IDLE")
		}

		// Card content
		var card strings.Builder
		card.WriteString(fmt.Sprintf("%s  %s\n", w.Role, stateStr))
		card.WriteString(idleStyle.Render(fmt.Sprintf("agent: %s", w.Agent)) + "\n")

		if w.Branch != "" {
			card.WriteString(idleStyle.Render(fmt.Sprintf("branch: %s", w.Branch)) + "\n")
		}

		// Context bar
		usage, lastLine := w.Snapshot()

		if usage > 0 {
			bar := contextBar(usage, 20)
			card.WriteString(fmt.Sprintf("ctx: %s %.0f%%\n", bar, usage*100))
		}

		if lastLine != "" {
			trunc := lastLine
			if len(trunc) > 34 {
				trunc = trunc[:33] + "\u2026"
			}
			card.WriteString(idleStyle.Render(trunc) + "\n")
		}

		if w.Error != "" {
			card.WriteString(failStyle.Render(w.Error) + "\n")
		}

		// Duration
		if !w.StartedAt.IsZero() {
			end := w.FinishedAt
			if end.IsZero() {
				end = time.Now()
			}
			card.WriteString(idleStyle.Render(fmt.Sprintf("%s", end.Sub(w.StartedAt).Round(time.Second))))
		}

		// Render card with cursor
		style := cardStyle
		if i == m.cursor {
			style = style.BorderForeground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"})
		}
		b.WriteString(style.Render(card.String()))
		b.WriteString("\n")
	}

	// Summary bar
	doneCount := 0
	for _, id := range m.team.WorkerOrder {
		if m.team.Workers[id].GetState() == team.StateCompleted {
			doneCount++
		}
	}
	b.WriteString(fmt.Sprintf("\n  %d/%d workers complete", doneCount, len(m.team.WorkerOrder)))
	if m.done {
		b.WriteString(doneStyle.Render("  \u2714 Team finished"))
	}
	b.WriteString("\n")

	// Controls
	b.WriteString("\n" + idleStyle.Render("  \u2191\u2193=select  a=abort worker  q=quit"))

	return b.String()
}

func contextBar(usage float64, width int) string {
	filled := int(usage * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("\u2588", filled) + strings.Repeat("\u2591", width-filled)
	if usage > 0.8 {
		return failStyle.Render(bar)
	}
	return runningStyle.Render(bar)
}

// Ensure we have access to the Worker's mu field — it's exported in worker.go
var _ = func() { _ = os.Getenv("") } // keep os import for future use
