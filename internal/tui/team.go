package tui

import (
	"fmt"
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

type teamView int

const (
	viewDashboard teamView = iota
	viewWorkerChat
)

type teamEventMsg team.WorkerEvent
type teamDoneMsg struct{}
type teamTickMsg time.Time

type teamModel struct {
	team       *team.Team
	spec       team.TeamSpec
	goal       string
	repoRoot   string
	cursor     int
	width      int
	height     int
	started    bool
	done       bool
	err        error
	view       teamView
	chatWorker string
	chatInput  string
	chatScroll int
}

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
		go t.Run()
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
		if m.view == viewWorkerChat {
			return m.updateWorkerChat(msg)
		}
		return m.updateDashboardKeys(msg)
	}
	return m, nil
}

// --- Dashboard keys ---

func (m teamModel) updateDashboardKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case "enter":
		if m.team != nil && m.cursor < len(m.team.WorkerOrder) {
			m.chatWorker = m.team.WorkerOrder[m.cursor]
			m.chatInput = ""
			m.view = viewWorkerChat
		}
	case "a":
		if m.team != nil && m.cursor < len(m.team.WorkerOrder) {
			id := m.team.WorkerOrder[m.cursor]
			w := m.team.Workers[id]
			if w.GetState() == team.StateRunning {
				w.Abort()
			}
		}
	}
	return m, nil
}

// --- Worker Chat ---

func (m teamModel) updateWorkerChat(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.view = viewDashboard
		m.chatInput = ""
	case "ctrl+c":
		if m.team != nil && !m.done {
			m.team.Abort()
		}
		return m, tea.Quit
	case "enter":
		text := strings.TrimSpace(m.chatInput)
		m.chatInput = ""
		if text == "" {
			return m, nil
		}
		// Trust commands
		if strings.HasPrefix(text, "/trust ") {
			level := strings.TrimPrefix(text, "/trust ")
			if w, ok := m.team.Workers[m.chatWorker]; ok {
				switch level {
				case "autonomous", "supervised", "strict":
					w.SetTrust(team.TrustLevel(level))
				}
			}
			return m, nil
		}
		// Mid-task prompt injection
		if w, ok := m.team.Workers[m.chatWorker]; ok {
			w.SendPrompt(text)
		}
	case "backspace":
		if len(m.chatInput) > 0 {
			m.chatInput = m.chatInput[:len(m.chatInput)-1]
		}
	case "ctrl+u":
		m.chatInput = ""
	case "pgup":
		m.chatScroll -= 5
		if m.chatScroll < 0 {
			m.chatScroll = 0
		}
	case "pgdown":
		m.chatScroll += 5
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 {
			m.chatInput += key
		}
	}
	return m, nil
}

// --- View ---

func (m teamModel) View() string {
	if !m.started {
		return "  Starting team...\n"
	}
	if m.view == viewWorkerChat {
		return m.viewWorkerChat()
	}
	return m.viewDashboardCards()
}

func (m teamModel) viewWorkerChat() string {
	w, ok := m.team.Workers[m.chatWorker]
	if !ok {
		return "Worker not found"
	}

	var b strings.Builder

	// Header
	state := w.GetState()
	stateStr := stateLabel(state)
	b.WriteString(headerStyle.Render(fmt.Sprintf(" %s ", w.Role)) + "  " + stateStr + "  " + idleStyle.Render("trust:"+string(w.Trust)))
	b.WriteString("\n\n")

	// Messages
	msgs := w.GetMessages()
	var lines []string
	for _, msg := range msgs {
		if strings.HasPrefix(msg, "user: ") {
			lines = append(lines, userStyle.Render("You: ")+strings.TrimPrefix(msg, "user: "))
		} else if strings.HasPrefix(msg, "assistant: ") {
			content := strings.TrimPrefix(msg, "assistant: ")
			if len(content) > 200 {
				content = content[:200] + "\u2026"
			}
			lines = append(lines, botStyle.Render("\U0001f916 ")+content)
		}
		lines = append(lines, "")
	}

	// Live streaming
	_, lastLine := w.Snapshot()
	if state == team.StateRunning && lastLine != "" {
		lines = append(lines, botStyle.Render("\U0001f916 ")+lastLine+"\u2588")
	}

	// Scroll
	h := m.height
	if h == 0 {
		h = 24
	}
	available := h - 6
	if available < 3 {
		available = 3
	}
	start := 0
	if len(lines) > available {
		start = len(lines) - available
	}
	for _, line := range lines[start:] {
		b.WriteString(line + "\n")
	}

	// Input
	w2 := m.width
	if w2 == 0 {
		w2 = 80
	}
	inputLine := inputStyle.Width(w2 - 4).Render("> " + m.chatInput + "\u2588")
	b.WriteString("\n" + inputLine + "\n")
	b.WriteString(idleStyle.Render("  esc=back  /trust autonomous|supervised|strict  pgup/pgdn"))

	return b.String()
}

func (m teamModel) viewDashboardCards() string {
	var b strings.Builder

	b.WriteString(headerStyle.Render(fmt.Sprintf(" \U0001f43e Team: %s ", m.spec.Name)))
	if m.goal != "" {
		b.WriteString("  " + toolStyle.Render(m.goal))
	}
	b.WriteString("\n\n")

	for i, id := range m.team.WorkerOrder {
		w := m.team.Workers[id]
		state := w.GetState()
		stateStr := stateLabel(state)

		var card strings.Builder
		card.WriteString(fmt.Sprintf("%s  %s\n", w.Role, stateStr))
		card.WriteString(idleStyle.Render(fmt.Sprintf("agent: %s", w.Agent)) + "\n")

		if w.Branch != "" {
			card.WriteString(idleStyle.Render(fmt.Sprintf("branch: %s", w.Branch)) + "\n")
		}

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

		if !w.StartedAt.IsZero() {
			end := w.FinishedAt
			if end.IsZero() {
				end = time.Now()
			}
			card.WriteString(idleStyle.Render(fmt.Sprintf("%s", end.Sub(w.StartedAt).Round(time.Second))))
		}

		style := cardStyle
		if i == m.cursor {
			style = style.BorderForeground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#A78BFA"})
		}
		b.WriteString(style.Render(card.String()))
		b.WriteString("\n")
	}

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
	b.WriteString("\n" + idleStyle.Render("  \u2191\u2193=select  enter=chat  a=abort  q=quit"))

	return b.String()
}

func stateLabel(s team.WorkerState) string {
	switch s {
	case team.StateRunning:
		return runningStyle.Render("\u25b6 RUNNING")
	case team.StateCompleted:
		return doneStyle.Render("\u2713 DONE")
	case team.StateFailed:
		return failStyle.Render("\u2717 FAILED")
	case team.StateProvisioning, team.StateInitializing:
		return runningStyle.Render("\u25d4 STARTING")
	default:
		return idleStyle.Render("\u25cb IDLE")
	}
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
