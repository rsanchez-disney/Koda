package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"

	"github.disney.com/SANCR225/koda/internal/acp"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var (
	userStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE")).Bold(true)
	botStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA"))
	toolStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Italic(true)
	inputStyle  = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	headerStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	completionStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B")).Bold(true)
	branchStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("#A78BFA"))
	workspaceStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#34D399"))
	toolCountStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	turnStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("#60A5FA"))
	sepStyle         = lipgloss.NewStyle().Foreground(lipgloss.Color("#4B5563"))
	agentBarStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE")).Bold(true)
)

var slashCommands = []string{"/quit", "/clear", "/agent", "/profile", "/save", "/load"}
var devSubProfiles = []string{"dev-core", "dev-web", "dev-mobile"}

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
	scroll     int // -1 = follow bottom
	width     int
	height    int
	ready     bool
	quitting  bool
	toolName  string
	suggestions []string
	suggestIdx  int
	agentNames  []string
	activeProfile string
	allAgents     []ops.AgentInfo
	profileNames  []string
	history       []string
	historyIdx    int
	historyDraft  string
	mdRenderer    *glamour.TermRenderer
	contextUsage  float64
	toolCalls     int
	turnCount     int
	gitBranch     string
	workspaceName string
}

// RunChat launches the chat TUI.
func RunChat(agent string) error {
	p := tea.NewProgram(initialChatModel(agent), tea.WithAltScreen(), tea.WithMouseCellMotion())
	_, err := p.Run()
	return err
}

func initialChatModel(agent string) chatModel {
	return chatModel{agent: agent, scroll: -1}
}

type sigtermMsg struct{}

func listenSigterm() tea.Msg {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM)
	<-ch
	return sigtermMsg{}
}

func (m *chatModel) saveSession(name string) {
	home, _ := os.UserHomeDir()
	sessDir := filepath.Join(home, ".kiro", "settings", "sessions")
	os.MkdirAll(sessDir, 0755)
	filename := filepath.Join(sessDir, name+".json")
	var msgs []map[string]string
	for _, msg := range m.messages {
		msgs = append(msgs, map[string]string{"role": msg.role, "content": msg.content})
	}
	data, _ := json.MarshalIndent(map[string]interface{}{
		"agent":    m.agent,
		"messages": msgs,
		"savedAt":  time.Now().Format(time.RFC3339),
	}, "", "  ")
	os.WriteFile(filename, data, 0644)
}

func (m chatModel) Init() tea.Cmd {
	return tea.Batch(listenSigterm, func() tea.Msg {
		agent := m.agent
		if agent == "" {
			if s := ops.LoadSettings(); s.LastAgent != "" {
				agent = s.LastAgent
			}
		}
		client, err := acp.Spawn(agent)
		if err != nil {
			return chatMsg{role: "system", content: fmt.Sprintf("Failed to start kiro-cli: %v", err)}
		}
		if err := client.CreateSession(agent); err != nil {
			client.Close()
			return chatMsg{role: "system", content: fmt.Sprintf("Session failed: %v", err)}
		}
		return acpConnected{client: client, agent: agent}
	})
}

type acpConnected struct {
	client *acp.Client
	agent  string
}

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

	case sigtermMsg:
		m.saveSession("autosave")
		m.quitting = true
		if m.client != nil {
			m.client.Close()
		}
		return m, tea.Quit

	case acpConnected:
		m.client = msg.client
		m.agent = msg.agent
		m.ready = true
		m.agentNames = loadAgentNames()
		home, _ := os.UserHomeDir()
		ops.UpdateLastAgent(m.agent)
		s := ops.LoadSettings()
		m.activeProfile = s.ActiveProfile
		m.allAgents = ops.AllAgents("", filepath.Join(home, ".kiro"))
		m.profileNames = loadProfileNames()
		m.filterAgentsByProfile()
		m.mdRenderer, _ = glamour.NewTermRenderer(
			glamour.WithAutoStyle(),
			glamour.WithWordWrap(0),
		)
		m.gitBranch = detectGitBranch()
		m.workspaceName = detectWorkspace()
		m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Connected to %s", agentLabel(m.agent))})
		if welcome := loadWelcomeMessage(m.agent); welcome != "" {
			m.messages = append(m.messages, chatMsg{role: "assistant", content: welcome})
		}
		return m, listenForEvents(m.client)

	case acpEventMsg:
		switch msg.Type {
		case "MessageChunk":
			m.streaming += msg.Chunk
			m.toolName = ""
		case "ToolCall":
			m.toolName = msg.Name
		case "ToolResult":
			m.toolCalls++
			m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("✓ %s done", msg.Name)})
			m.toolName = ""
		case "Metadata":
			m.contextUsage = msg.Usage
		case "Complete":
			m.turnCount++
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

	case tea.MouseMsg:
		switch msg.Button {
		case tea.MouseButtonWheelUp:
			if m.scroll < 0 {
				m.scroll = len(m.messages) * 4
			}
			m.scroll -= 3
			if m.scroll < 0 { m.scroll = 0 }
		case tea.MouseButtonWheelDown:
			if m.scroll >= 0 {
				m.scroll += 3
			}
		}
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

func loadWelcomeMessage(agent string) string {
	if agent == "" {
		return ""
	}
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".kiro", "agents", agent+".json"))
	if err != nil {
		return ""
	}
	var a struct {
		WelcomeMessage string `json:"welcomeMessage"`
	}
	json.Unmarshal(data, &a)
	msg := a.WelcomeMessage

	// Enrich with workspace context if active
	wsData, err := os.ReadFile(filepath.Join(home, ".kiro", "settings", "workspace.json"))
	if err != nil {
		return msg
	}
	var ws struct {
		Name       string `json:"name"`
		Team       string `json:"team"`
		JiraPrefix string `json:"jira_prefix"`
		Profiles   []string `json:"profiles"`
		Projects   []struct {
			Name string `json:"name"`
			Repo string `json:"repo,omitempty"`
		} `json:"projects"`
		Services []string `json:"services,omitempty"`
		Channels []string `json:"channels,omitempty"`
	}
	if json.Unmarshal(wsData, &ws) != nil || ws.Name == "" {
		return msg
	}

	var b strings.Builder
	b.WriteString(msg)
	b.WriteString("\n\n📋 Workspace: " + ws.Name)
	if ws.Team != "" {
		b.WriteString(" (" + ws.Team + ")")
	}
	if ws.JiraPrefix != "" {
		b.WriteString("\n  Jira: " + ws.JiraPrefix + "-*")
	}
	if len(ws.Profiles) > 0 {
		b.WriteString("\n  Profiles: " + strings.Join(ws.Profiles, ", "))
	}
	if len(ws.Projects) > 0 {
		b.WriteString("\n  Projects:")
		for _, p := range ws.Projects {
			line := "\n    • " + p.Name
			if p.Repo != "" {
				line += " (" + p.Repo + ")"
			}
			b.WriteString(line)
		}
	}
	if len(ws.Services) > 0 {
		b.WriteString("\n  Services: " + strings.Join(ws.Services, ", "))
	}
	if len(ws.Channels) > 0 {
		b.WriteString("\n  Channels: " + strings.Join(ws.Channels, ", "))
	}
	return b.String()
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
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") && !strings.HasPrefix(e.Name(), "._") {
			names = append(names, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	return names
}

func (m *chatModel) filterAgentsByProfile() {
	if m.activeProfile == "" {
		m.agentNames = loadAgentNames()
		return
	}
	// "dev" is an alias for dev-core + dev-web + dev-mobile
	matchProfiles := []string{m.activeProfile}
	if m.activeProfile == "dev" {
		matchProfiles = devSubProfiles
	}
	match := map[string]bool{}
	for _, p := range matchProfiles {
		match[p] = true
	}
	m.agentNames = nil
	for _, a := range m.allAgents {
		if match[a.ProfileID] {
			m.agentNames = append(m.agentNames, a.Name)
		}
	}
}

func loadProfileNames() []string {
	home, _ := os.UserHomeDir()
	settingsPath := filepath.Join(home, ".kiro", "settings", "profiles.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		return nil
	}
	var manifest struct {
		Profiles []struct {
			ID        string `json:"id"`
			Installed bool   `json:"installed"`
		} `json:"profiles"`
	}
	if json.Unmarshal(data, &manifest) != nil {
		return nil
	}
	var names []string
	hasSub := map[string]bool{}
	for _, p := range manifest.Profiles {
		if p.Installed {
			names = append(names, p.ID)
			hasSub[p.ID] = true
		}
	}
	// Synthesize "dev" alias if all sub-profiles installed
	if hasSub["dev-core"] && hasSub["dev-web"] && hasSub["dev-mobile"] {
		names = append([]string{"dev"}, names...)
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
		// /profile <name> completion
		if strings.HasPrefix(m.input, "/profile ") {
			prefix := strings.ToLower(strings.TrimPrefix(m.input, "/profile "))
			for _, name := range m.profileNames {
				if strings.HasPrefix(strings.ToLower(name), prefix) {
					m.suggestions = append(m.suggestions, name)
				}
			}
			return
		}
		// /agent <name> completion
		if strings.HasPrefix(m.input, "/agent ") {
			prefix := strings.ToLower(strings.TrimPrefix(m.input, "/agent "))
			for _, name := range m.agentNames {
				if strings.HasPrefix(strings.ToLower(name), prefix) {
					m.suggestions = append(m.suggestions, name)
				}
			}
			return
		}
		// /load <session> completion
		if strings.HasPrefix(m.input, "/load ") || strings.HasPrefix(m.input, "/save ") {
			cmd := "/load "
			if strings.HasPrefix(m.input, "/save ") {
				cmd = "/save "
			}
			prefix := strings.ToLower(strings.TrimPrefix(m.input, cmd))
			home, _ := os.UserHomeDir()
			entries, _ := os.ReadDir(filepath.Join(home, ".kiro", "settings", "sessions"))
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".json") {
					name := strings.TrimSuffix(e.Name(), ".json")
					if strings.HasPrefix(strings.ToLower(name), prefix) {
						m.suggestions = append(m.suggestions, name)
					}
				}
			}
			return
		}
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
			if strings.HasPrefix(m.input, "/profile ") {
				m.input = "/profile " + selected
				m.suggestions = nil
				return m.handleSlash(m.input)
			} else if strings.HasPrefix(m.input, "/agent ") {
				m.input = "/agent " + selected
				m.suggestions = nil
				return m.handleSlash(m.input)
			} else if strings.HasPrefix(m.input, "/load ") {
				m.input = "/load " + selected
				m.suggestions = nil
				return m.handleSlash(m.input)
			} else if strings.HasPrefix(m.input, "/") {
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
			m.history = append(m.history, text)
			m.historyIdx = len(m.history)
			m.historyDraft = ""
			return m.handleSlash(text)
		}
		// Save to history
		m.history = append(m.history, text)
		m.historyIdx = len(m.history)
		m.historyDraft = ""
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
	case "up":
		if len(m.history) > 0 {
			if m.historyIdx == len(m.history) {
				m.historyDraft = m.input
			}
			if m.historyIdx > 0 {
				m.historyIdx--
				m.input = m.history[m.historyIdx]
			}
		}
	case "down":
		if m.historyIdx < len(m.history) {
			m.historyIdx++
			if m.historyIdx == len(m.history) {
				m.input = m.historyDraft
			} else {
				m.input = m.history[m.historyIdx]
			}
		}
	case "pgup", "ctrl+b":
		if m.scroll < 0 {
			// Was following bottom — estimate position from message count
			m.scroll = len(m.messages) * 4 // rough line estimate
		}
		m.scroll -= 10
		if m.scroll < 0 { m.scroll = 0 }
	case "pgdown", "ctrl+f":
		if m.scroll >= 0 {
			m.scroll += 10
		}
	case "ctrl+d":
		if m.scroll >= 0 {
			m.scroll += 5
		}
	case "home":
		m.scroll = 0
	case "end":
		m.scrollToBottom()
	case "tab":
		if len(m.suggestions) > 0 {
			selected := m.suggestions[m.suggestIdx]
			if strings.HasPrefix(m.input, "/profile ") {
				m.input = "/profile " + selected
			} else if strings.HasPrefix(m.input, "/agent ") {
				m.input = "/agent " + selected
			} else if strings.HasPrefix(m.input, "/load ") {
				m.input = "/load " + selected
			} else if strings.HasPrefix(m.input, "/save ") {
				m.input = "/save " + selected
			} else if strings.HasPrefix(m.input, "/") {
				m.input = selected + " "
			} else if atIdx := strings.LastIndex(m.input, "@"); atIdx >= 0 {
				m.input = m.input[:atIdx+1] + selected + " "
			}
			m.suggestions = nil
		}
	default:
		if msg.Paste {
			m.input += string(msg.Runes)
		} else {
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 {
				m.input += key
			}
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
			ops.UpdateLastAgent(m.agent)
			m.ready = false
			m.streaming = ""
			return m, m.Init()
		}
		m.messages = append(m.messages, chatMsg{role: "system", content: "Usage: /agent <name>"})
	case "/profile":
		if len(parts) > 1 {
			m.activeProfile = parts[1]
			ops.UpdateActiveProfile(m.activeProfile)
			m.filterAgentsByProfile()
			count := len(m.agentNames)
			m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Profile: %s (%d agents)", m.activeProfile, count)})
			// Auto-switch to profile's orchestrator
			for _, name := range m.agentNames {
				if strings.Contains(name, "orchestrator") {
					m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Switching to %s...", name)})
					if m.client != nil {
						m.client.Close()
					}
					m.agent = name
					ops.UpdateLastAgent(m.agent)
					m.ready = false
					m.streaming = ""
					return m, m.Init()
				}
			}
		} else if m.activeProfile != "" {
			m.activeProfile = ""
			ops.UpdateActiveProfile("")
			m.filterAgentsByProfile()
			m.messages = append(m.messages, chatMsg{role: "system", content: "Profile filter cleared (all agents)"})
		} else {
			m.messages = append(m.messages, chatMsg{role: "system", content: "Usage: /profile <name> (or /profile to clear)"})
		}
	case "/save":
		name := "session"
		if len(parts) > 1 {
			name = parts[1]
		}
		m.saveSession(name)
		m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Session saved: %s", name)})
	case "/load":
		home, _ := os.UserHomeDir()
		sessDir := filepath.Join(home, ".kiro", "settings", "sessions")
		if len(parts) > 1 {
			filename := filepath.Join(sessDir, parts[1]+".json")
			data, err := os.ReadFile(filename)
			if err != nil {
				m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Session not found: %s", parts[1])})
				return m, nil
			}
			var sess struct {
				Agent    string              `json:"agent"`
				Messages []map[string]string `json:"messages"`
			}
			json.Unmarshal(data, &sess)
			m.messages = nil
			for _, msg := range sess.Messages {
				m.messages = append(m.messages, chatMsg{role: msg["role"], content: msg["content"]})
			}
			m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Loaded session: %s (%d messages)", parts[1], len(sess.Messages))})
		} else {
			entries, _ := os.ReadDir(sessDir)
			if len(entries) == 0 {
				m.messages = append(m.messages, chatMsg{role: "system", content: "No saved sessions. Use /save <name> to save."})
				return m, nil
			}
			var names []string
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".json") {
					names = append(names, strings.TrimSuffix(e.Name(), ".json"))
				}
			}
			m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Available sessions:\n  %s\n\nUse /load <name> to load.", strings.Join(names, "\n  "))})
		}
	default:
		m.messages = append(m.messages, chatMsg{role: "system", content: fmt.Sprintf("Unknown command: %s", parts[0])})
	}
	return m, nil
}

func (m *chatModel) scrollToBottom() {
	m.scroll = -1
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
	profileTag := ""
	if m.activeProfile != "" {
		profileTag = toolStyle.Render(fmt.Sprintf(" [%s]", m.activeProfile))
	}
	header := headerStyle.Render(fmt.Sprintf(" \U0001f43e %s", agentLabel(m.agent))) + profileTag

	// Messages area
	var lines []string
	for _, msg := range m.messages {
		switch msg.role {
		case "user":
			lines = append(lines, userStyle.Render("You: ")+msg.content)
		case "assistant":
			lines = append(lines, botStyle.Render("\U0001f916 ")+m.renderMD(msg.content))
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

	// Scroll viewport
	msgArea := strings.Join(lines, "\n")
	msgLines := strings.Split(msgArea, "\n")
	available := h - 5 // header + input + borders
	if available < 3 {
		available = 3
	}
	totalLines := len(msgLines)
	maxScroll := totalLines - available
	if maxScroll < 0 { maxScroll = 0 }
	scroll := m.scroll
	if scroll < 0 { scroll = maxScroll }
	if scroll > maxScroll { scroll = maxScroll }
	end := scroll + available
	if end > totalLines { end = totalLines }
	visible := strings.Join(msgLines[scroll:end], "\n")
	// Scroll indicator
	if scroll < maxScroll {
		visible += "\n" + toolStyle.Render(fmt.Sprintf("  ↓ %d more lines (pgdn/end)", maxScroll-scroll))
	}
	if scroll > 0 {
		visible = toolStyle.Render(fmt.Sprintf("  ↑ %d lines above (pgup/home)", scroll)) + "\n" + visible
	}

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

	// Status bar with colored live indicators
	sep := sepStyle.Render(" · ")
	var statusParts []string
	statusParts = append(statusParts, agentBarStyle.Render(agentLabel(m.agent)))
	if m.gitBranch != "" {
		statusParts = append(statusParts, branchStyle.Render("⎇ "+m.gitBranch))
	}
	if m.workspaceName != "" {
		statusParts = append(statusParts, workspaceStyle.Render("⬡ "+m.workspaceName))
	}
	if m.toolCalls > 0 {
		statusParts = append(statusParts, toolCountStyle.Render(fmt.Sprintf("⚙ %d tools", m.toolCalls)))
	}
	if m.contextUsage > 0 {
		usageColor := "#34D399"
		if m.contextUsage > 75 {
			usageColor = "#EF4444"
		} else if m.contextUsage > 50 {
			usageColor = "#F59E0B"
		}
		usageStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(usageColor))
		statusParts = append(statusParts, usageStyle.Render(fmt.Sprintf("ctx %.0f%%", m.contextUsage)))
	}
	if m.turnCount > 0 {
		statusParts = append(statusParts, turnStyle.Render(fmt.Sprintf("↻ %d", m.turnCount)))
	}
	status := strings.Join(statusParts, sep)

	help := sepStyle.Render("ctrl+b/f=scroll  ctrl+d=½page  end=bottom  @=agent  /quit /clear /agent /profile")
	return fmt.Sprintf("%s\n%s\n%s%s\n%s\n%s", header, visible, suggestLine, inputLine, status, help)
}

func agentLabel(agent string) string {
	if agent == "" {
		return "kiro (default)"
	}
	return agent
}


func (m chatModel) renderMD(content string) string {
	if m.mdRenderer == nil {
		return content
	}
	out, err := m.mdRenderer.Render(content)
	if err != nil {
		return content
	}
	return strings.TrimSpace(out)
}

func detectGitBranch() string {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func detectWorkspace() string {
	s := ops.LoadSettings()
	if s.SteerRuntime != nil && s.SteerRuntime.ActiveWorkspace != "" {
		return s.SteerRuntime.ActiveWorkspace
	}
	return ""
}
