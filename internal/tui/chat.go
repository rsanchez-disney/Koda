package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.disney.com/SANCR225/koda/internal/acp"
)

var (
	userStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE")).Bold(true)
	botStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA"))
	toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
	inputStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	completionStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true)
)

var slashCommands = []string{"/quit", "/clear", "/agent", "/save"}

var delegateRe = regexp.MustCompile(`<delegate\s+agent="([^"]+)">((?s).*?)</delegate>`)

type chatMsg struct {
	role    string // user, assistant, tool, system
	content string
}

type acpEventMsg acp.Event

type delegationResult struct {
	agent  string
	result string
}

type chatModel struct {
	agent     string
	client    *acp.Client
	messages  []chatMsg
	streaming string // current streaming buffer
	input     string
	scroll    int
	width     int
	height    int
	ready     bool
	quitting  bool
	toolName  string
	suggestions []string
	suggestIdx  int
	agentNames  []string
}

// RunChat launches the chat TUI.
func RunChat(agent string) error {
	p := tea.NewProgram(initialChatModel(agent), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

func initialChatModel(agent string) chatModel {
	return chatModel{agent: agent}
}

func (m chatModel) Init() tea.Cmd {
	return func() tea.Msg {
		client, err := acp.Spawn(m.agent)
		if err != nil {
			return chatMsg{role: "system", content: fmt.Sprintf("Failed to start kiro-cli: %v", err)}
		}
		if err := client.CreateSession(m.agent); err != nil {
			client.Close()
			return chatMsg{role: "system", content: fmt.Sprintf("Session failed: %v", err)}
		}
		return acpConnected{client: client}
	}
}

type acpConnected struct{ client *acp.Client }

func listenForEvents(client *acp.Client) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-client.Events
		if !ok {
			return chatMsg{role: "system", content: "kiro-cli disconnected"}
		}
		return acpEventMsg(event)
	}
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case acpConnected:
		m.client = msg.client
		m.ready = true
		m.agentNames = loadAgentNames()
		m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Connected to %s", agentLabel(m.agent))})
		return m, listenForEvents(m.client)

	case acpEventMsg:
		switch msg.Type {
		case "MessageChunk":
			m.streaming += msg.Chunk
			m.toolName = ""
		case "ToolCall":
			m.toolName = msg.Name
		case "Complete":
			if m.streaming != "" {
				completed := m.streaming
				m.messages = append(m.messages, chatMsg{role: "assistant", content: completed})
				m.streaming = ""
				m.toolName = ""
				m.scrollToBottom()
				// Check for delegation tags
				if matches := delegateRe.FindAllStringSubmatch(completed, -1); len(matches) > 0 {
					return m, m.runDelegations(matches)
				}
			}
		}
		return m, listenForEvents(m.client)

	case delegationResult:
		m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Delegation from %s complete", msg.agent)})
		// Feed results back to orchestrator
		if m.client != nil {
			feedback := fmt.Sprintf("<delegation_results>\n[%s]: %s\n</delegation_results>\nContinue with these results.", msg.agent, msg.result)
			m.client.SendMessage(feedback)
		}
		m.scrollToBottom()
		return m, listenForEvents(m.client)

	case chatMsg:
		m.messages = append(m.messages, msg)
		m.scrollToBottom()
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m chatModel) runDelegations(matches [][]string) tea.Cmd {
	return func() tea.Msg {
		var results []string
		for _, match := range matches {
			agentID := match[1]
			task := strings.TrimSpace(match[2])

			// Spawn a sub kiro-cli session
			sub, err := acp.Spawn(agentID)
			if err != nil {
				results = append(results, fmt.Sprintf("[%s]: error: %v", agentID, err))
				continue
			}
			if err := sub.CreateSession(agentID); err != nil {
				sub.Close()
				results = append(results, fmt.Sprintf("[%s]: session error: %v", agentID, err))
				continue
			}
			sub.SendMessage(task)

			// Collect response
			var buf strings.Builder
			for event := range sub.Events {
				switch event.Type {
				case "MessageChunk":
					buf.WriteString(event.Chunk)
				case "Complete":
					goto done
				}
			}
		done:
			sub.Close()
			results = append(results, fmt.Sprintf("[%s]: %s", agentID, buf.String()))
		}
		return delegationResult{
			agent:  matches[0][1],
			result: strings.Join(results, "\n\n---\n\n"),
		}
	}
}

func loadAgentNames() []string {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".kiro", "agents")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}

func (m *chatModel) updateSuggestions() {
	m.suggestions = nil
	m.suggestIdx = 0
	if m.input == "" {
		return
	}
	if strings.HasPrefix(m.input, "/") {
		for _, cmd := range slashCommands {
			if strings.HasPrefix(cmd, m.input) && cmd != m.input {
				m.suggestions = append(m.suggestions, cmd)
			}
		}
		return
	}
	atIdx := strings.LastIndex(m.input, "@")
	if atIdx >= 0 {
		prefix := strings.ToLower(m.input[atIdx+1:])
		for _, name := range m.agentNames {
			if strings.HasPrefix(strings.ToLower(name), prefix) {
				m.suggestions = append(m.suggestions, name)
			}
		}
	}
}

func (m chatModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		m.quitting = true
		if m.client != nil {
			m.client.Close()
		}
		return m, tea.Quit
	case "enter":
		// Accept suggestion
		if len(m.suggestions) > 0 {
			selected := m.suggestions[m.suggestIdx]
			if strings.HasPrefix(m.input, "/") {
				m.input = selected + " "
			} else if atIdx := strings.LastIndex(m.input, "@"); atIdx >= 0 {
				m.input = m.input[:atIdx+1] + selected + " "
			}
			m.suggestions = nil
			return m, nil
		}
		text := strings.TrimSpace(m.input)
		m.input = ""
		if text == "" {
			return m, nil
		}
		// Slash commands
		if strings.HasPrefix(text, "/") {
			return m.handleSlash(text)
		}
		// Send to ACP
		m.messages = append(m.messages, chatMsg{role: "user", content: text})
		m.scrollToBottom()
		if m.client != nil {
			m.client.SendMessage(text)
		}
		return m, nil
	case "backspace":
		if len(m.input) > 0 {
			m.input = m.input[:len(m.input)-1]
		}
	case "ctrl+u":
		m.input = ""
	case "pgup":
		if m.scroll > 0 {
			m.scroll -= 5
			if m.scroll < 0 {
				m.scroll = 0
			}
		}
	case "pgdown":
		m.scroll += 5
	case "tab":
		if len(m.suggestions) > 0 {
			m.suggestIdx = (m.suggestIdx + 1) % len(m.suggestions)
		}
	default:
		key := msg.String()
		if len(key) == 1 && key[0] >= 32 {
			m.input += key
		}
	}
	m.updateSuggestions()
	return m, nil
}

func (m chatModel) handleSlash(text string) (tea.Model, tea.Cmd) {
	parts := strings.Fields(text)
	switch parts[0] {
	case "/quit", "/q":
		m.quitting = true
		if m.client != nil {
			m.client.Close()
		}
		return m, tea.Quit
	case "/clear":
		m.messages = nil
		m.streaming = ""
	case "/agent":
		if len(parts) > 1 {
			m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Switching to %s...", parts[1])})
			if m.client != nil {
				m.client.Close()
			}
			m.agent = parts[1]
			m.ready = false
			m.streaming = ""
			return m, m.Init()
		}
		m.messages = append(m.messages, chatMsg{role: "system", content: "Usage: /agent <name>"})
	default:
		m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Unknown command: %s", parts[0])})
	}
	return m, nil
}

func (m *chatModel) scrollToBottom() {
	m.scroll = len(m.messages)
}

func (m chatModel) View() string {
	if m.quitting {
		return ""
	}

	w := m.width
	if w == 0 {
		w = 80
	}
	h := m.height
	if h == 0 {
		h = 24
	}

	// Header
	header := headerStyle.Render(fmt.Sprintf(" \U0001f43e %s ", agentLabel(m.agent)))

	// Messages area
	var lines []string
	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			lines = append(lines, userStyle.Render("You: ")+msg.content)
		case "assistant":
			lines = append(lines, botStyle.Render("\U0001f916 ")+msg.content)
		case "system":
			lines = append(lines, toolStyle.Render("\u2022 "+msg.content))
		}
		lines = append(lines, "")
	}

	// Streaming
	if m.streaming != "" {
		lines = append(lines, botStyle.Render("\U0001f916 ")+m.streaming+"\u2588")
		lines = append(lines, "")
	}

	// Tool indicator
	if m.toolName != "" {
		lines = append(lines, toolStyle.Render(fmt.Sprintf("\u2699 %s...", m.toolName)))
	}

	// Scroll
	msgArea := strings.Join(lines, "\n")
	msgLines := strings.Split(msgArea, "\n")
	available := h - 5 // header + input + borders
	if available < 3 {
		available = 3
	}
	start := 0
	if len(msgLines) > available {
		start = len(msgLines) - available
	}
	visible := strings.Join(msgLines[start:], "\n")

	// Suggestions
	var suggestLine string
	if len(m.suggestions) > 0 {
		var parts []string
		for i, s := range m.suggestions {
			if i > 5 {
				parts = append(parts, toolStyle.Render(fmt.Sprintf("+%d more", len(m.suggestions)-5)))
				break
			}
			if i == m.suggestIdx {
				parts = append(parts, completionStyle.Render(s))
			} else {
				parts = append(parts, toolStyle.Render(s))
			}
		}
		suggestLine = "  " + strings.Join(parts, "  ") + "\n"
	}

	// Input
	prompt := "> "
	if !m.ready {
		prompt = "Connecting..."
	}
	inputLine := inputStyle.Width(w - 4).Render(prompt + m.input + "\u2588")

	// Status bar
	status := toolStyle.Render("/quit \u00b7 /clear \u00b7 /agent <name> \u00b7 pgup/pgdn")

	return fmt.Sprintf("%s\n%s\n%s%s\n%s", header, visible, suggestLine, inputLine, status)
}

func agentLabel(agent string) string {
	if agent == "" {
		return "kiro (default)"
	}
	return agent
}
