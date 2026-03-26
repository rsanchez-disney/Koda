package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	mdl "github.disney.com/SANCR225/koda/internal/model"
	"github.disney.com/SANCR225/koda/internal/ops"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	activeStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#22D3EE"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280"))
	checkStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#F59E0B"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	boxStyle    = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1, 2)
)

type screen int

const (
	screenDashboard screen = iota
	screenProfiles
	screenTokens
	screenWorkspaces
	screenAgents
	screenCleanConfirm
	screenDoctor
	screenRules
	screenMCP
)

type model struct {
	steerRoot   string
	targetDir   string
	screen      screen
	cursor      int
	report      ops.HealthReport
	profiles    []profileItem
	tokens      map[string]string
	tokenInput  string
	workspaces  []mdl.Workspace
	agents      []ops.AgentInfo
	agentFilter string
	statusMsg   string
	quitting      bool
	launchChat    bool
	doctorResults []ops.DoctorResult
	rules         []ruleItem
	mcpServers    []mcpItem
}

type profileItem struct {
	id         string
	agentCount int
	installed  bool
	selected   bool
}

type ruleItem struct {
	name     string
	selected bool
}

type mcpItem struct {
	name      string
	hasBundle bool
}

func Run(steerRoot, targetDir string) (bool, error) {
	m := initialModel(steerRoot, targetDir)
	p := tea.NewProgram(m, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return false, err
	}
	if fm, ok := finalModel.(model); ok && fm.launchChat {
		return true, nil
	}
	return false, nil
}

func initialModel(steerRoot, targetDir string) model {
	m := model{steerRoot: steerRoot, targetDir: targetDir}
	m.refresh()
	return m
}

func (m *model) refresh() {
	m.report = ops.CheckInstallation(m.steerRoot, m.targetDir)
	profiles, _ := ops.ListProfiles(m.steerRoot, m.targetDir)
	m.profiles = nil
	for _, p := range profiles {
		m.profiles = append(m.profiles, profileItem{
			id: p.ID, agentCount: p.AgentCount, installed: p.Installed, selected: p.Installed,
		})
	}
	m.tokens = ops.ReadTokens()
	m.workspaces, _ = ops.ListWorkspaces(m.steerRoot)
	m.agents = ops.AllAgents(m.steerRoot, m.targetDir)
	m.doctorResults = ops.RunDoctor(m.steerRoot, m.targetDir)
	availRules := ops.ListRules(m.steerRoot)
	m.rules = nil
	for _, r := range availRules {
		m.rules = append(m.rules, ruleItem{name: r})
	}
	mcpDir := filepath.Join(m.targetDir, "tools", "mcp-servers")
	m.mcpServers = nil
	if entries, err := os.ReadDir(mcpDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			bundle := filepath.Join(mcpDir, e.Name(), "dist", "index.cjs")
			_, err := os.Stat(bundle)
			m.mcpServers = append(m.mcpServers, mcpItem{name: e.Name(), hasBundle: err == nil})
		}
	}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch m.screen {
		case screenDashboard:
			return m.updateDashboard(msg)
		case screenProfiles:
			return m.updateProfiles(msg)
		case screenTokens:
			return m.updateTokens(msg)
		case screenWorkspaces:
			return m.updateWorkspaces(msg)
		case screenAgents:
			return m.updateAgents(msg)
		case screenCleanConfirm:
			return m.updateCleanConfirm(msg)
		case screenDoctor:
			return m.updateDoctor(msg)
		case screenRules:
			return m.updateRules(msg)
		case screenMCP:
			return m.updateMCP(msg)
		}
	}
	return m, nil
}

// --- Dashboard ---

func (m model) updateDashboard(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "enter":
		m.launchChat = true
		return m, tea.Quit
	case "p":
		m.screen = screenProfiles
		m.cursor = 0
	case "t":
		m.screen = screenTokens
		m.cursor = 0
		m.tokenInput = ""
	case "a":
		m.screen = screenAgents
		m.cursor = 0
		m.agentFilter = ""
	case "w":
		m.screen = screenWorkspaces
		m.cursor = 0
	case "s":
		installed := ops.DetectInstalled(m.steerRoot, m.targetDir)
		ops.InstallShared(m.steerRoot, m.targetDir)
		for _, p := range installed {
			ops.InstallProfile(m.steerRoot, p, m.targetDir)
		}
		ops.InjectAgentTokens(m.targetDir)
		m.refresh()
		m.statusMsg = "Synced!"
	case "c":
		m.screen = screenCleanConfirm
	case "d":
		m.screen = screenDoctor
	case "r":
		m.screen = screenRules
		m.cursor = 0
	case "m":
		m.screen = screenMCP
		m.cursor = 0
	}
	return m, nil
}

func (m model) updateCleanConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		for _, sub := range []string{"agents", "prompts", "context", "powers", "skills", "steering"} {
			ops.RemoveDir(m.targetDir + "/" + sub)
		}
		m.refresh()
		m.screen = screenDashboard
		m.statusMsg = "Cleaned!"
	case "n", "N", "esc", "q":
		m.screen = screenDashboard
		m.statusMsg = ""
	}
	return m, nil
}

func (m model) viewDashboard() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("\U0001f43e Koda \u2014 Agent Runtime Manager"))
	b.WriteString("\n\n")

	if len(m.report.Profiles) > 0 {
		b.WriteString(fmt.Sprintf("  Installed: %s (%d agents)\n",
			checkStyle.Render(strings.Join(m.report.Profiles, " \u00b7 ")), m.report.TotalAgents))
	} else {
		b.WriteString(warnStyle.Render("  No profiles installed") + "\n")
	}

	tokSet := len(m.report.TokensSet)
	tokTotal := tokSet + len(m.report.TokensMissing)
	b.WriteString(fmt.Sprintf("  Tokens:    %d/%d configured\n", tokSet, tokTotal))
	b.WriteString(fmt.Sprintf("  Target:    %s\n", dimStyle.Render(m.targetDir)))

	b.WriteString("\n")
	b.WriteString(activeStyle.Render("  [p]") + " Profiles    ")
	b.WriteString(activeStyle.Render("[t]") + " Tokens    ")
	b.WriteString(activeStyle.Render("[w]") + " Workspaces\n")
	b.WriteString(activeStyle.Render("  [a]") + " Agents      ")
	b.WriteString(activeStyle.Render("[d]") + " Doctor    ")
	b.WriteString(activeStyle.Render("[r]") + " Rules\n")
	b.WriteString(activeStyle.Render("  [m]") + " MCP         ")
	b.WriteString(activeStyle.Render("[s]") + " Sync      ")
	b.WriteString(activeStyle.Render("[c]") + " Clean\n")
	b.WriteString(activeStyle.Render("  [enter]") + " Chat   ")
	b.WriteString(activeStyle.Render("[q]") + " Quit\n")

	if m.statusMsg != "" {
		b.WriteString("\n  " + checkStyle.Render(m.statusMsg) + "\n")
	}

	return boxStyle.Render(b.String())
}

func (m model) viewCleanConfirm() string {
	var b strings.Builder
	b.WriteString(errStyle.Render("\u26a0 Clean ALL profiles?"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  This will remove %d agents from:\n", m.report.TotalAgents))
	b.WriteString(fmt.Sprintf("  %s\n\n", dimStyle.Render(m.targetDir)))
	b.WriteString(activeStyle.Render("  [y]") + " Yes, clean    ")
	b.WriteString(activeStyle.Render("[n]") + " Cancel\n")
	return boxStyle.Render(b.String())
}

// --- Doctor ---

func (m model) updateDoctor(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "d":
		m.screen = screenDashboard
		m.statusMsg = ""
	}
	return m, nil
}

func (m model) viewDoctor() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Doctor") + dimStyle.Render("  esc=back"))
	b.WriteString("\n\n")
	for _, r := range m.doctorResults {
		icon := checkStyle.Render("\u2713")
		if !r.OK {
			icon = errStyle.Render("\u2717")
		}
		b.WriteString(fmt.Sprintf("  %s %-16s %s\n", icon, r.Name, dimStyle.Render(r.Detail)))
	}
	return boxStyle.Render(b.String())
}

// --- Rules ---

func (m model) updateRules(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.screen = screenDashboard
		m.statusMsg = ""
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.rules)-1 {
			m.cursor++
		}
	case " ":
		if m.cursor < len(m.rules) {
			m.rules[m.cursor].selected = !m.rules[m.cursor].selected
		}
	case "enter":
		var names []string
		for _, r := range m.rules {
			if r.selected {
				names = append(names, r.name)
			}
		}
		if len(names) > 0 {
			ops.InstallRules(m.steerRoot, m.targetDir, names)
		}
		m.screen = screenDashboard
		m.statusMsg = fmt.Sprintf("%d rules installed!", len(names))
	}
	return m, nil
}

func (m model) viewRules() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Rules") + dimStyle.Render("  space=toggle  enter=install  esc=back"))
	b.WriteString("\n\n")
	if len(m.rules) == 0 {
		b.WriteString(dimStyle.Render("  No rules found"))
		return boxStyle.Render(b.String())
	}
	for i, r := range m.rules {
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("\u25b8 ")
		}
		check := dimStyle.Render("[ ]")
		if r.selected {
			check = checkStyle.Render("[\u2713]")
		}
		name := r.name
		if i == m.cursor {
			name = activeStyle.Render(name)
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, check, name))
	}
	return boxStyle.Render(b.String())
}

// --- MCP ---

func (m model) updateMCP(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q", "m":
		m.screen = screenDashboard
		m.statusMsg = ""
	}
	return m, nil
}

func (m model) viewMCP() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("MCP Servers") + dimStyle.Render("  esc=back"))
	b.WriteString("\n\n")
	if len(m.mcpServers) == 0 {
		b.WriteString(dimStyle.Render("  No MCP servers found"))
		return boxStyle.Render(b.String())
	}
	for _, s := range m.mcpServers {
		icon := checkStyle.Render("\u2713")
		if !s.hasBundle {
			icon = errStyle.Render("\u2717")
		}
		b.WriteString(fmt.Sprintf("  %s %s\n", icon, s.name))
	}
	ready := 0
	for _, s := range m.mcpServers {
		if s.hasBundle {
			ready++
		}
	}
	b.WriteString(fmt.Sprintf("\n  %s", dimStyle.Render(fmt.Sprintf("%d/%d ready", ready, len(m.mcpServers)))))
	return boxStyle.Render(b.String())
}

// --- Profiles ---

func (m model) updateProfiles(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.screen = screenDashboard
		m.statusMsg = ""
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.profiles)-1 {
			m.cursor++
		}
	case " ":
		if m.cursor < len(m.profiles) {
			m.profiles[m.cursor].selected = !m.profiles[m.cursor].selected
		}
	case "enter":
		m.applyProfileChanges()
		m.refresh()
		m.screen = screenDashboard
		m.statusMsg = "Profiles updated!"
	}
	return m, nil
}

func (m *model) applyProfileChanges() {
	ops.InstallShared(m.steerRoot, m.targetDir)
	for _, p := range m.profiles {
		if p.selected && !p.installed {
			ops.InstallProfile(m.steerRoot, p.id, m.targetDir)
		} else if !p.selected && p.installed {
			ops.RemoveProfile(m.steerRoot, p.id, m.targetDir)
		}
	}
	ops.InjectAgentTokens(m.targetDir)
}

func (m model) viewProfiles() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Profiles") + dimStyle.Render("  space=toggle  enter=apply  esc=back"))
	b.WriteString("\n\n")

	for i, p := range m.profiles {
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("\u25b8 ")
		}
		check := dimStyle.Render("[ ]")
		if p.selected {
			check = checkStyle.Render("[\u2713]")
		}
		name := fmt.Sprintf("%-14s", p.id)
		if i == m.cursor {
			name = activeStyle.Render(name)
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", cursor, check, name,
			dimStyle.Render(fmt.Sprintf("%d agents", p.agentCount))))
	}

	return boxStyle.Render(b.String())
}

// --- Tokens ---

func (m model) updateTokens(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.screen = screenDashboard
		m.statusMsg = ""
		m.tokenInput = ""
	case "up", "shift+tab":
		if m.cursor > 0 {
			m.cursor--
			m.tokenInput = ""
		}
	case "down", "tab":
		if m.cursor < len(mdl.KnownTokens)-1 {
			m.cursor++
			m.tokenInput = ""
		}
	case "enter":
		if m.tokenInput != "" {
			tk := mdl.KnownTokens[m.cursor]
			m.tokens[tk.Key] = m.tokenInput
			m.tokenInput = ""
			if m.cursor < len(mdl.KnownTokens)-1 {
				m.cursor++
			}
		} else {
			ops.WriteTokens(m.tokens)
			ops.InjectAgentTokens(m.targetDir)
			m.refresh()
			m.screen = screenDashboard
			m.statusMsg = "Tokens saved!"
		}
	case "backspace":
		if len(m.tokenInput) > 0 {
			m.tokenInput = m.tokenInput[:len(m.tokenInput)-1]
		}
	case "ctrl+u":
		m.tokenInput = ""
	case "ctrl+d":
		tk := mdl.KnownTokens[m.cursor]
		delete(m.tokens, tk.Key)
		m.tokenInput = ""
	default:
		if len(key) == 1 && key[0] >= 32 {
			m.tokenInput += key
		}
	}
	return m, nil
}

func (m model) viewTokens() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Tokens") + dimStyle.Render("  type to set  enter=save  ctrl+d=clear  esc=back"))
	b.WriteString("\n\n")

	for i, tk := range mdl.KnownTokens {
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("\u25b8 ")
		}
		val := m.tokens[tk.Key]
		status := errStyle.Render("not set")
		if val != "" {
			status = checkStyle.Render(ops.MaskToken(val))
		}
		label := fmt.Sprintf("%-22s", tk.Label)
		if i == m.cursor {
			label = activeStyle.Render(label)
		}
		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, label, status))
		if i == m.cursor && m.tokenInput != "" {
			masked := strings.Repeat("\u2022", len(m.tokenInput))
			b.WriteString(fmt.Sprintf("    %s\n", activeStyle.Render(masked+"\u2588")))
		} else if i == m.cursor {
			b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render("type to set...\u2588")))
		}
	}

	b.WriteString("\n" + dimStyle.Render("  enter with empty input = save all & return"))
	return boxStyle.Render(b.String())
}

// --- Workspaces ---

func (m model) updateWorkspaces(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.screen = screenDashboard
		m.statusMsg = ""
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.workspaces)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor < len(m.workspaces) {
			ws := m.workspaces[m.cursor]
			ops.ApplyWorkspace(m.steerRoot, m.targetDir, ws)
			m.refresh()
			m.screen = screenDashboard
			m.statusMsg = fmt.Sprintf("Workspace '%s' applied!", ws.Name)
		}
	}
	return m, nil
}

func (m model) viewWorkspaces() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Workspaces") + dimStyle.Render("  enter=apply  esc=back"))
	b.WriteString("\n\n")

	if len(m.workspaces) == 0 {
		b.WriteString(dimStyle.Render("  No workspaces found"))
		return boxStyle.Render(b.String())
	}

	for i, ws := range m.workspaces {
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("\u25b8 ")
		}
		name := fmt.Sprintf("%-20s", ws.Name)
		if i == m.cursor {
			name = activeStyle.Render(name)
		}
		profiles := dimStyle.Render(strings.Join(ws.Profiles, ", "))
		b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, name, profiles))
		if i == m.cursor && ws.Description != "" {
			b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render(ws.Description)))
		}
	}

	return boxStyle.Render(b.String())
}

// --- Agents ---

func (m model) updateAgents(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		if m.agentFilter != "" {
			m.agentFilter = ""
			m.cursor = 0
		} else {
			m.screen = screenDashboard
			m.statusMsg = ""
		}
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		filtered := m.filteredAgents()
		if m.cursor < len(filtered)-1 {
			m.cursor++
		}
	case "backspace":
		if len(m.agentFilter) > 0 {
			m.agentFilter = m.agentFilter[:len(m.agentFilter)-1]
			m.cursor = 0
		}
	case "ctrl+u":
		m.agentFilter = ""
		m.cursor = 0
	default:
		if len(key) == 1 && key[0] >= 32 {
			m.agentFilter += key
			m.cursor = 0
		}
	}
	return m, nil
}

func (m model) filteredAgents() []ops.AgentInfo {
	if m.agentFilter == "" {
		return m.agents
	}
	filter := strings.ToLower(m.agentFilter)
	var out []ops.AgentInfo
	for _, a := range m.agents {
		if strings.Contains(strings.ToLower(a.Name), filter) ||
			strings.Contains(strings.ToLower(a.ProfileID), filter) ||
			strings.Contains(strings.ToLower(a.Description), filter) {
			out = append(out, a)
		}
	}
	return out
}

func (m model) viewAgents() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Agents") + dimStyle.Render("  type to filter  esc=clear/back"))
	b.WriteString("\n\n")

	if m.agentFilter != "" {
		b.WriteString(fmt.Sprintf("  %s %s\n\n", dimStyle.Render(">"), activeStyle.Render(m.agentFilter+"\u2588")))
	} else {
		b.WriteString(fmt.Sprintf("  %s\n\n", dimStyle.Render("> type to search...\u2588")))
	}

	filtered := m.filteredAgents()
	if len(filtered) == 0 {
		b.WriteString(dimStyle.Render("  No agents match"))
		return boxStyle.Render(b.String())
	}

	start := 0
	if m.cursor > 12 {
		start = m.cursor - 12
	}
	end := start + 15
	if end > len(filtered) {
		end = len(filtered)
	}

	for i := start; i < end; i++ {
		a := filtered[i]
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("\u25b8 ")
		}
		name := fmt.Sprintf("%-26s", a.Name)
		profile := dimStyle.Render(fmt.Sprintf("%-12s", a.ProfileID))
		if i == m.cursor {
			name = activeStyle.Render(name)
		}
		b.WriteString(fmt.Sprintf("%s%s %s %s\n", cursor, name, profile,
			dimStyle.Render(truncate(a.Description, 40))))
		if i == m.cursor {
			if len(a.Tools) > 0 {
				b.WriteString(fmt.Sprintf("    Tools: %s\n", dimStyle.Render(strings.Join(a.Tools, ", "))))
			}
			if len(a.MCPServers) > 0 {
				b.WriteString(fmt.Sprintf("    MCP:   %s\n", checkStyle.Render(strings.Join(a.MCPServers, ", "))))
			}
		}
	}

	b.WriteString(fmt.Sprintf("\n  %s", dimStyle.Render(fmt.Sprintf("%d/%d agents", len(filtered), len(m.agents)))))
	return boxStyle.Render(b.String())
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-1] + "\u2026"
}

// --- View router ---

func (m model) View() string {
	if m.quitting {
		return ""
	}
	switch m.screen {
	case screenProfiles:
		return m.viewProfiles()
	case screenTokens:
		return m.viewTokens()
	case screenWorkspaces:
		return m.viewWorkspaces()
	case screenAgents:
		return m.viewAgents()
	case screenCleanConfirm:
		return m.viewCleanConfirm()
	case screenDoctor:
		return m.viewDoctor()
	case screenRules:
		return m.viewRules()
	case screenMCP:
		return m.viewMCP()
	default:
		return m.viewDashboard()
	}
}
