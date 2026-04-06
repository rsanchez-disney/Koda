package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.disney.com/SANCR225/koda/internal/config"
	"github.disney.com/SANCR225/koda/internal/tray"
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

	// Gradient banner colors — adaptive for dark/light terminals
	bannerColors = []lipgloss.Style{
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#5B21B6", Dark: "#22D3EE"}),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#38BDF8"}),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#818CF8"}),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#8B5CF6", Dark: "#A78BFA"}),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#7C3AED", Dark: "#818CF8"}),
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.AdaptiveColor{Light: "#6D28D9", Dark: "#38BDF8"}),
	}
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
	screenFork
	screenCreateWorkspace
	screenEnvVars
	screenGitHub
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
	statusMsg     string
	syncing       bool
	quitting      bool
	launchChat    bool
	doctorResults []ops.DoctorResult
	rules         []ruleItem
	mcpServers    []mcpItem
	envVars       map[string]string
	ghRemotes     []mdl.GitHubRemote
	ghInput       string
	ghField       int // 0=name, 1=host, 2=token
	ghAdding      bool
	envVarKeys    []string
	envInput      string
	envNewKey     string
	ruleInput     string
	ruleEditing   string // rule name being edited
	forkForks     []string
	forkCursor    int
	forkBranch    string
	forkField     int // 0=list, 1=branch, 2=manual
	forkManual    string
	forkError     string
	cw            cwState
	ghIdentity    ops.GHIdentity
	kodaVersion   string
}

type profileItem struct {
	id         string
	sourceDir  string
	agentCount int
	installed  bool
	selected   bool
	workspace  string
}

type ruleItem struct {
	name     string
	selected bool
}

type mcpItem struct {
	name      string
	hasBundle bool
}

type editorFinishedMsg struct{ err error }

func Run(steerRoot, targetDir, version string) (bool, error) {
	m := initialModel(steerRoot, targetDir, version)
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

func initialModel(steerRoot, targetDir, version string) model {
	m := model{steerRoot: steerRoot, targetDir: targetDir, kodaVersion: version}
	m.ghIdentity = ops.GetGHIdentity()
	m.refresh()
	return m
}

func (m *model) refresh() {
	m.report = ops.CheckInstallation(m.steerRoot, m.targetDir)
	profiles, _ := ops.ListProfiles(m.steerRoot, m.targetDir)
	m.profiles = nil
	for _, p := range profiles {
		m.profiles = append(m.profiles, profileItem{
			id: p.ID, sourceDir: p.SourceDir, agentCount: p.AgentCount, installed: p.Installed, selected: p.Installed, workspace: p.WorkspaceName,
		})
	}
	m.tokens = ops.ReadTokens()
	m.workspaces, _ = ops.ListWorkspaces(m.steerRoot)
	m.agents = ops.AllAgents(m.steerRoot, m.targetDir)
	m.envVars = ops.ReadEnvVars()
	m.ghRemotes = ops.ReadGitHubRemotes()
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
	case editorFinishedMsg:
		// Editor closed — publish the rule
		if m.ruleEditing != "" {
			settings := config.ReadSteerSettings()
			var prURL string
			var err error
			if settings.Source == "git" {
				prURL, err = ops.PublishRule(m.steerRoot, m.ruleEditing)
			} else if ops.CanWriteRepo(config.DefaultSteerRepo) {
				prURL, err = ops.PublishRuleToUpstream(m.steerRoot, m.ruleEditing)
			}
			m.refresh()
			if prURL != "" {
				m.statusMsg = fmt.Sprintf("Rule '%s' — PR: %s", m.ruleEditing, prURL)
			} else if err != nil {
				m.statusMsg = fmt.Sprintf("Rule '%s' saved (PR failed: %s)", m.ruleEditing, err)
			} else {
				m.statusMsg = fmt.Sprintf("Rule '%s' created!", m.ruleEditing)
			}
			m.ruleEditing = ""
		}
		return m, nil
	case syncDoneMsg:
		m.syncing = false
		if msg.err != nil {
			m.statusMsg = "Sync failed: " + msg.err.Error()
		} else {
			m.refresh()
			m.statusMsg = "✅ Synced!"
		}
		return m, nil
	case doctorFixDoneMsg:
		m.envVars = ops.ReadEnvVars()
	m.ghRemotes = ops.ReadGitHubRemotes()
	m.doctorResults = ops.RunDoctor(m.steerRoot, m.targetDir)
		if msg.err != nil {
			m.statusMsg = "Fix failed: " + msg.err.Error()
		} else {
			m.statusMsg = "Fix applied!"
		}
		return m, nil
	case wsEditorFinishedMsg:
		// Editor closed — reload workspace from disk into form
		if msg.err == nil && msg.name != "" {
			if ws, err := ops.GetWorkspace(m.steerRoot, msg.name); err == nil {
				m.cw = newCWStateFromWorkspace(m.steerRoot, m.targetDir, ws)
				m.screen = screenCreateWorkspace
				m.statusMsg = "Loaded from editor"
			}
		}
		return m, nil
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
		case screenFork:
			return m.updateFork(msg)
		case screenEnvVars:
			return m.updateEnvVars(msg)
		case screenGitHub:
			return m.updateGitHub(msg)
		case screenCreateWorkspace:
			return m.updateCreateWorkspace(msg)
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
		if !m.syncing {
			m.syncing = true
			m.statusMsg = "⏳ Syncing..."
			steerRoot, targetDir := m.steerRoot, m.targetDir
			return m, func() tea.Msg {
				err := ops.SyncSteerRuntime(steerRoot, targetDir)
				return syncDoneMsg{err: err}
			}
		}
	case "g":
		m.ghRemotes = ops.ReadGitHubRemotes()
		m.ghAdding = false
		m.ghInput = ""
		m.ghField = 0
		m.cursor = 0
		m.screen = screenGitHub
	case "y":
		if tray.AutoStartEnabled() {
			tray.DisableAutoStart()
			m.statusMsg = "Tray auto-start disabled"
		} else {
			if err := tray.EnableAutoStart(); err == nil {
				m.statusMsg = "Tray auto-start enabled — launches on login"
			} else {
				m.statusMsg = "Tray: " + err.Error()
			}
		}
	case "f":
		settings := config.ReadSteerSettings()
		if settings.Source == "git" {
			// Unfork: switch back to tarball
			if err := ops.UnforkSteerRuntime(m.steerRoot); err != nil {
				m.statusMsg = "Unfork failed: " + err.Error()
			} else {
				m.refresh()
				m.statusMsg = "Unforked! Back to official tarball."
			}
		} else {
			// Fork: load forks and show screen
			forks, forkErr := ops.ListForks()
			m.forkForks = forks
			m.forkCursor = 0
			m.forkBranch = "main"
			m.forkManual = ""
			m.forkError = forkErr
			m.forkField = 0
			if len(forks) == 0 {
				m.forkField = 2 // jump to manual input
			}
			m.screen = screenFork
		}
	case "c":
		m.screen = screenCleanConfirm
	case "d":
		m.screen = screenDoctor
	case "r":
		m.screen = screenRules
		m.cursor = 0
	case "e":
		m.refreshEnvVarKeys()
		m.screen = screenEnvVars
		m.cursor = 0
		m.envInput = ""
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

	bannerLines := []string{
		"   \u2588\u2588\u2557  \u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2557 \u2588\u2588\u2588\u2588\u2588\u2588\u2557  \u2588\u2588\u2588\u2588\u2588\u2557",
		"   \u2588\u2588\u2551 \u2588\u2588\u2554\u255d\u2588\u2588\u2554\u2550\u2550\u2550\u2588\u2588\u2557\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2557",
		"   \u2588\u2588\u2588\u2588\u2588\u2554\u255d \u2588\u2588\u2551   \u2588\u2588\u2551\u2588\u2588\u2551  \u2588\u2588\u2551\u2588\u2588\u2588\u2588\u2588\u2588\u2588\u2551",
		"   \u2588\u2588\u2554\u2550\u2588\u2588\u2557 \u2588\u2588\u2551   \u2588\u2588\u2551\u2588\u2588\u2551  \u2588\u2588\u2551\u2588\u2588\u2554\u2550\u2550\u2588\u2588\u2551",
		"   \u2588\u2588\u2551  \u2588\u2588\u2557\u255a\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255d\u2588\u2588\u2588\u2588\u2588\u2588\u2554\u255d\u2588\u2588\u2551  \u2588\u2588\u2551",
		"   \u255a\u2550\u255d  \u255a\u2550\u255d \u255a\u2550\u2550\u2550\u2550\u2550\u255d \u255a\u2550\u2550\u2550\u2550\u2550\u255d \u255a\u2550\u255d  \u255a\u2550\u255d",
	}
	for i, line := range bannerLines {
		b.WriteString(bannerColors[i].Render(line))
		if i < len(bannerLines)-1 {
			b.WriteString("\n")
		}
	}
	b.WriteString("")
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

	settings := config.ReadSteerSettings()
	runtimeInfo := settings.Source
	if ver, err := os.ReadFile(filepath.Join(m.steerRoot, "VERSION")); err == nil {
		runtimeInfo = strings.TrimSpace(string(ver)) + " (" + settings.Source + ")"
	} else if settings.Source == "git" {
		runtimeInfo = settings.Repo + "@" + settings.Branch + " (git)"
	}
	b.WriteString(fmt.Sprintf("  Runtime:   %s\n", dimStyle.Render(runtimeInfo)))
	if ws := config.ReadSteerSettings().ActiveWorkspace; ws != "" {
		b.WriteString(fmt.Sprintf("  Workspace: %s\n", checkStyle.Render(ws)))
	}

	if m.kodaVersion != "" {
		b.WriteString(fmt.Sprintf("  Koda:      %s\n", dimStyle.Render(m.kodaVersion)))
	}
	if m.ghIdentity.Login != "" {
		userStr := m.ghIdentity.Login
		if m.ghIdentity.Name != "" {
			userStr += " (" + m.ghIdentity.Name + ")"
		}
		b.WriteString(fmt.Sprintf("  User:      %s\n", dimStyle.Render(userStr)))
	}

	b.WriteString("\n")
	b.WriteString(activeStyle.Render("  [p]") + " Profiles    ")
	b.WriteString(activeStyle.Render("[t]") + " Tokens    ")
	b.WriteString(activeStyle.Render("[w]") + " Workspaces\n")
	b.WriteString(activeStyle.Render("  [a]") + " Agents      ")
	b.WriteString(activeStyle.Render("[d]") + " Doctor    ")
	b.WriteString(activeStyle.Render("[r]") + " Rules\n")
	b.WriteString(activeStyle.Render("  [m]") + " MCP         ")
	b.WriteString(activeStyle.Render("[e]") + " Env Vars\n")
	b.WriteString(activeStyle.Render("  [g]") + fmt.Sprintf(" GitHub (%d) ", len(m.ghRemotes)))
	b.WriteString(activeStyle.Render("  [s]") + " Sync        ")
	b.WriteString(activeStyle.Render("[c]") + " Clean\n")
	if settings.Source == "git" {
		b.WriteString(activeStyle.Render("  [f]") + " Unfork      ")
	} else {
		b.WriteString(activeStyle.Render("  [f]") + " Fork        ")
	}
	if tray.AutoStartEnabled() {
		b.WriteString(activeStyle.Render("[y]") + " Tray \u2713\n")
	} else {
		b.WriteString(activeStyle.Render("[y]") + " Tray\n")
	}
	b.WriteString(activeStyle.Render("  [enter]") + " Chat       ")
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
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.doctorResults)-1 {
			m.cursor++
		}
	case "f":
		if m.cursor < len(m.doctorResults) {
			r := m.doctorResults[m.cursor]
			if r.Fix != "" && !r.OK {
				c := exec.Command("sh", "-c", r.Fix)
				return m, tea.ExecProcess(c, func(err error) tea.Msg {
					return doctorFixDoneMsg{err: err}
				})
			}
		}
	}
	return m, nil
}

type doctorFixDoneMsg struct{ err error }
type syncDoneMsg struct{ err error }

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
	key := msg.String()

	// Name input mode
	if m.ruleInput != "" || key == "n" {
		switch key {
		case "n":
			if m.ruleInput == "" {
				m.ruleInput = " " // activate input mode (trimmed on use)
				return m, nil
			}
			m.ruleInput += key
		case "esc":
			m.ruleInput = ""
		case "enter":
			name := strings.TrimSpace(m.ruleInput)
			m.ruleInput = ""
			if name == "" {
				return m, nil
			}
			path, err := ops.CreateRule(m.steerRoot, name)
			if err != nil {
				m.statusMsg = err.Error()
				return m, nil
			}
			m.ruleEditing = name
			c := ops.EditorCmd(path)
			return m, tea.ExecProcess(c, func(err error) tea.Msg {
				return editorFinishedMsg{err}
			})
		case "backspace":
			s := strings.TrimSpace(m.ruleInput)
			if len(s) > 0 {
				m.ruleInput = s[:len(s)-1]
			}
		case "ctrl+u":
			m.ruleInput = ""
		default:
			if len(key) == 1 && key[0] >= 32 {
				m.ruleInput = strings.TrimSpace(m.ruleInput) + key
			}
		}
		return m, nil
	}

	switch key {
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
	b.WriteString(titleStyle.Render("Rules") + dimStyle.Render("  space=toggle  enter=install  n=new  esc=back"))
	b.WriteString("\n\n")
	if strings.TrimSpace(m.ruleInput) != "" || m.ruleInput == " " {
		name := strings.TrimSpace(m.ruleInput)
		b.WriteString("  " + activeStyle.Render("New rule: "+name+"█") + "\n\n")
	}
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


// --- Env Vars ---

func (m *model) refreshEnvVarKeys() {
	m.envVarKeys = nil
	known := map[string]bool{}
	for _, e := range mdl.KnownEnvVars {
		m.envVarKeys = append(m.envVarKeys, e.Key)
		known[e.Key] = true
	}
	for k := range m.envVars {
		if !known[k] {
			m.envVarKeys = append(m.envVarKeys, k)
		}
	}
}

func (m model) updateEnvVars(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	// Adding new custom key
	if m.envNewKey != "" {
		switch key {
		case "esc":
			m.envNewKey = ""
		case "enter":
			k := strings.TrimSpace(m.envNewKey)
			m.envNewKey = ""
			if k != "" {
				m.envVars[k] = ""
				m.refreshEnvVarKeys()
				// Move cursor to new key
				for i, ek := range m.envVarKeys {
					if ek == k {
						m.cursor = i
						break
					}
				}
				m.envInput = ""
			}
		case "backspace":
			if len(m.envNewKey) > 1 {
				m.envNewKey = m.envNewKey[:len(m.envNewKey)-1]
			}
		default:
			if len(key) == 1 && key[0] >= 32 {
				m.envNewKey = strings.TrimSpace(m.envNewKey) + key
			}
		}
		return m, nil
	}

	switch key {
	case "esc":
		if m.envInput != "" {
			m.envInput = ""
		} else {
			m.screen = screenDashboard
			m.statusMsg = ""
		}
	case "up", "k", "shift+tab":
		if m.cursor > 0 {
			m.cursor--
			m.envInput = ""
		}
	case "down", "j", "tab":
		if m.cursor < len(m.envVarKeys)-1 {
			m.cursor++
			m.envInput = ""
		}
	case "enter":
		if m.envInput != "" {
			// Save current edit
			m.envVars[m.envVarKeys[m.cursor]] = m.envInput
			m.envInput = ""
			if m.cursor < len(m.envVarKeys)-1 {
				m.cursor++
			}
		} else {
			// Save all and return
			ops.WriteEnvVars(m.envVars)
			m.refresh()
			m.screen = screenDashboard
			m.statusMsg = "Env vars saved!"
		}
	case "n":
		if m.envInput == "" {
			m.envNewKey = " "
		} else {
			m.envInput += key
		}
	case "d":
		if m.envInput == "" && m.cursor < len(m.envVarKeys) {
			k := m.envVarKeys[m.cursor]
			// Only allow deleting custom keys
			isKnown := false
			for _, e := range mdl.KnownEnvVars {
				if e.Key == k {
					isKnown = true
					break
				}
			}
			if !isKnown {
				delete(m.envVars, k)
				m.refreshEnvVarKeys()
				if m.cursor >= len(m.envVarKeys) {
					m.cursor = len(m.envVarKeys) - 1
				}
			}
		} else {
			m.envInput += key
		}
	case "backspace":
		if len(m.envInput) > 0 {
			m.envInput = m.envInput[:len(m.envInput)-1]
		}
	case "ctrl+u":
		m.envInput = ""
	default:
		if len(key) >= 1 && key[0] >= 32 {
			m.envInput += key
		}
	}
	return m, nil
}

func (m model) viewEnvVars() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Env Vars") + dimStyle.Render("  type=edit  enter=save  n=new  d=delete  esc=back"))
	b.WriteString("\n\n")

	if strings.TrimSpace(m.envNewKey) != "" || m.envNewKey == " " {
		name := strings.TrimSpace(m.envNewKey)
		b.WriteString("  " + activeStyle.Render("New key: "+name+"█") + "\n\n")
	}

	for i, k := range m.envVarKeys {
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("▸ ")
		}
		val := m.envVars[k]
		label := fmt.Sprintf("%-20s", k)
		if i == m.cursor {
			label = activeStyle.Render(label)
		}

		// Show description for known vars
		desc := ""
		for _, e := range mdl.KnownEnvVars {
			if e.Key == k {
				desc = e.Description
				break
			}
		}

		if val == "" {
			b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, label, dimStyle.Render("—")))
		} else {
			b.WriteString(fmt.Sprintf("%s%s %s\n", cursor, label, val))
		}

		if i == m.cursor && m.envInput != "" {
			b.WriteString(fmt.Sprintf("    %s\n", activeStyle.Render(m.envInput+"█")))
		} else if i == m.cursor {
			b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render("type to edit...█")))
		}
		if i == m.cursor && desc != "" {
			b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render(desc)))
		}
	}

	b.WriteString("\n" + dimStyle.Render("  enter with empty input = save all & return"))
	if m.statusMsg != "" {
		b.WriteString("\n  " + checkStyle.Render(m.statusMsg))
	}
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
			if p.workspace != "" {
				ops.InstallProfileFrom(p.sourceDir, m.targetDir)
			} else {
				ops.InstallProfile(m.steerRoot, p.id, m.targetDir)
			}
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

	// Separate global and workspace profiles
	var globals, wsProfiles []int
	for i, p := range m.profiles {
		if p.workspace != "" {
			wsProfiles = append(wsProfiles, i)
		} else {
			globals = append(globals, i)
		}
	}

	renderItem := func(i int) string {
		p := m.profiles[i]
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("▸ ")
		}
		check := dimStyle.Render("[ ]")
		if p.selected {
			check = checkStyle.Render("[✓]")
		}
		name := fmt.Sprintf("%-14s", p.id)
		if i == m.cursor {
			name = activeStyle.Render(name)
		}
		return fmt.Sprintf("%s%s %s %s", cursor, check, name, dimStyle.Render(fmt.Sprintf("%d agents", p.agentCount)))
	}

	b.WriteString(dimStyle.Render("  ── Global ──") + "\n")
	for _, i := range globals {
		b.WriteString(renderItem(i) + "\n")
	}

	if len(wsProfiles) > 0 {
		// Group by workspace name
		wsGroups := map[string][]int{}
		var wsOrder []string
		for _, i := range wsProfiles {
			ws := m.profiles[i].workspace
			if _, seen := wsGroups[ws]; !seen {
				wsOrder = append(wsOrder, ws)
			}
			wsGroups[ws] = append(wsGroups[ws], i)
		}
		for _, ws := range wsOrder {
			b.WriteString("\n" + warnStyle.Render("  ── "+ws+" ──") + "\n")
			for _, i := range wsGroups[ws] {
				b.WriteString(renderItem(i) + "\n")
			}
		}
	}

	return boxStyle.Render(b.String())
}

// --- Tokens ---

func (m model) updateTokens(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		if err := ops.WriteTokens(m.tokens); err != nil {
			m.statusMsg = "Save failed: " + err.Error()
		} else {
			ops.InjectAgentTokens(m.targetDir)
			m.refresh()
			m.statusMsg = "Tokens saved!"
		}
		m.screen = screenDashboard
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
			if err := ops.WriteTokens(m.tokens); err != nil {
				m.statusMsg = "Save failed: " + err.Error()
			} else {
				ops.InjectAgentTokens(m.targetDir)
				m.refresh()
				m.statusMsg = "Tokens saved!"
			}
			m.screen = screenDashboard
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
		if len(key) >= 1 && key[0] >= 32 {
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
		if i == m.cursor && tk.Hint != "" {
			b.WriteString(fmt.Sprintf("    %s\n", dimStyle.Render(tk.Hint)))
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
	case "n":
		m.cw = newCWState(m.steerRoot, m.targetDir)
		m.screen = screenCreateWorkspace
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
	case "e":
		if m.cursor < len(m.workspaces) {
			ws := m.workspaces[m.cursor]
			m.cw = newCWStateFromWorkspace(m.steerRoot, m.targetDir, ws)
			m.screen = screenCreateWorkspace
			m.statusMsg = ""
		}
	case "x":
		if m.cursor < len(m.workspaces) {
			parent := m.workspaces[m.cursor]
			m.cw = newCWState(m.steerRoot, m.targetDir)
			m.cw.extends = parent.Name
			m.screen = screenCreateWorkspace
			m.statusMsg = ""
		}
	}
	return m, nil
}

func (m model) viewWorkspaces() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Workspaces") + dimStyle.Render("  enter=apply  e=edit  x=extend  n=new  esc=back"))
	b.WriteString("\n\n")

	if len(m.workspaces) == 0 {
		b.WriteString(dimStyle.Render("  No workspaces found"))
		return boxStyle.Render(b.String())
	}

	// Build parent→children map for tree display
	children := map[string][]int{}
	roots := []int{}
	for i, ws := range m.workspaces {
		if ws.Extends == "" {
			roots = append(roots, i)
		} else {
			children[ws.Extends] = append(children[ws.Extends], i)
		}
	}

	var renderWS func(idx int, prefix string, last bool)
	renderWS = func(idx int, prefix string, last bool) {
		ws := m.workspaces[idx]
		cursor := "  "
		if idx == m.cursor {
			cursor = activeStyle.Render("\u25b8 ")
		}
		tree := prefix
		if prefix != "" {
			if last {
				tree += "└─ "
			} else {
				tree += "├─ "
			}
		}
		isParent := len(children[ws.Name]) > 0
		name := ws.Name
		if idx == m.cursor {
			name = activeStyle.Render(name)
		} else if isParent {
			name = titleStyle.Render(name)
		}
		profiles := dimStyle.Render(strings.Join(ws.Profiles, ", "))
		b.WriteString(fmt.Sprintf("%s%s%s %s\n", cursor, dimStyle.Render(tree), name, profiles))
		if idx == m.cursor && ws.Description != "" {
			b.WriteString(fmt.Sprintf("    %s%s\n", dimStyle.Render(prefix), dimStyle.Render(ws.Description)))
		}
		kids := children[ws.Name]
		childPrefix := prefix
		if prefix != "" {
			if last {
				childPrefix += "   "
			} else {
				childPrefix += "│  "
			}
		}
		for j, kid := range kids {
			renderWS(kid, childPrefix, j == len(kids)-1)
		}
	}

	for _, idx := range roots {
		renderWS(idx, "", true)
	}
	// Show orphans (extends a non-existent parent)
	shown := map[int]bool{}
	var markShown func(idx int)
	markShown = func(idx int) {
		shown[idx] = true
		for _, kid := range children[m.workspaces[idx].Name] {
			markShown(kid)
		}
	}
	for _, idx := range roots {
		markShown(idx)
	}
	for i := range m.workspaces {
		if !shown[i] {
			renderWS(i, "", true)
		}
	}

	return boxStyle.Render(b.String())
}

// --- GitHub Remotes ---

func (m model) updateGitHub(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.ghAdding {
		switch key {
		case "esc":
			m.ghAdding = false
			m.ghInput = ""
			m.ghField = 0
		case "enter":
			val := strings.TrimSpace(m.ghInput)
			m.ghInput = ""
			switch m.ghField {
			case 0: // name entered
				if val != "" {
					m.ghRemotes = append(m.ghRemotes, mdl.GitHubRemote{Name: val})
					m.ghField = 1
				}
			case 1: // host entered
				if val != "" {
					m.ghRemotes[len(m.ghRemotes)-1].Host = val
					m.ghField = 2
				}
			case 2: // token entered
				if val != "" {
					r := &m.ghRemotes[len(m.ghRemotes)-1]
					r.Token = val
					ops.WriteGitHubRemote(*r)
					m.ghAdding = false
					m.ghField = 0
					m.statusMsg = fmt.Sprintf("Added remote '%s'", r.Name)
				}
			}
		case "backspace":
			if len(m.ghInput) > 0 {
				m.ghInput = m.ghInput[:len(m.ghInput)-1]
			}
		default:
			if len(key) >= 1 && key[0] >= 32 {
				m.ghInput += key
			}
		}
		return m, nil
	}

	switch key {
	case "esc":
		m.screen = screenDashboard
		m.statusMsg = ""
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.ghRemotes)-1 {
			m.cursor++
		}
	case "n":
		m.ghAdding = true
		m.ghField = 0
		m.ghInput = ""
	case "d":
		if m.cursor < len(m.ghRemotes) {
			name := m.ghRemotes[m.cursor].Name
			ops.RemoveGitHubRemote(name)
			m.ghRemotes = ops.ReadGitHubRemotes()
			if m.cursor >= len(m.ghRemotes) {
				m.cursor = len(m.ghRemotes) - 1
			}
			m.statusMsg = fmt.Sprintf("Removed remote '%s'", name)
		}
	}
	return m, nil
}

func (m model) viewGitHub() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("GitHub Remotes") + dimStyle.Render("  n=add  d=delete  esc=back"))
	b.WriteString("\n\n")

	if len(m.ghRemotes) == 0 && !m.ghAdding {
		b.WriteString(dimStyle.Render("  No GitHub remotes configured"))
		b.WriteString("\n")
	}

	for i, r := range m.ghRemotes {
		cursor := "  "
		if i == m.cursor && !m.ghAdding {
			cursor = activeStyle.Render("\u25b8 ")
		}
		name := r.Name
		host := dimStyle.Render(r.Host)
		tok := errStyle.Render("no token")
		if r.Token != "" {
			tok = checkStyle.Render(ops.MaskToken(r.Token))
		}
		b.WriteString(fmt.Sprintf("%s%-12s %s  %s\n", cursor, name, host, tok))
	}

	if m.ghAdding {
		b.WriteString("\n")
		labels := []string{"  Name:  ", "  Host:  ", "  Token: "}
		for i, label := range labels {
			if i == m.ghField {
				b.WriteString(activeStyle.Render("\u25b8 "+label) + activeStyle.Render(m.ghInput+"\u2588") + "\n")
			} else if i < m.ghField {
				var val string
				switch i {
				case 0:
					val = m.ghRemotes[len(m.ghRemotes)-1].Name
				case 1:
					val = m.ghRemotes[len(m.ghRemotes)-1].Host
				}
				b.WriteString("  " + label + checkStyle.Render(val) + "\n")
			}
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


// --- Fork ---

func (m model) updateFork(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.screen = screenDashboard
		m.statusMsg = ""
	case "tab":
		if len(m.forkForks) > 0 {
			m.forkField = (m.forkField + 1) % 3
		} else {
			// No forks: toggle between manual and branch
			if m.forkField == 2 {
				m.forkField = 1
			} else {
				m.forkField = 2
			}
		}
	case "up", "k":
		if m.forkField == 0 && m.forkCursor > 0 {
			m.forkCursor--
		}
	case "down", "j":
		if m.forkField == 0 && m.forkCursor < len(m.forkForks)-1 {
			m.forkCursor++
		}
	case "enter":
		var repo string
		if m.forkManual != "" {
			repo = m.forkManual
		} else if len(m.forkForks) > 0 {
			repo = m.forkForks[m.forkCursor]
		}
		if repo == "" {
			return m, nil
		}
		if err := ops.ForkSteerRuntime(m.steerRoot, repo, m.forkBranch); err != nil {
			m.statusMsg = "Fork failed: " + err.Error()
		} else {
			m.refresh()
			m.statusMsg = fmt.Sprintf("Forked to %s@%s!", repo, m.forkBranch)
		}
		m.screen = screenDashboard
	case "backspace":
		if m.forkField == 2 && len(m.forkManual) > 0 {
			m.forkManual = m.forkManual[:len(m.forkManual)-1]
		} else if m.forkField == 1 && len(m.forkBranch) > 0 {
			m.forkBranch = m.forkBranch[:len(m.forkBranch)-1]
		}
	case "ctrl+u":
		if m.forkField == 2 {
			m.forkManual = ""
		} else if m.forkField == 1 {
			m.forkBranch = ""
		}
	default:
		if len(key) == 1 && key[0] >= 32 {
			if m.forkField == 2 {
				m.forkManual += key
			} else if m.forkField == 1 {
				m.forkBranch += key
			}
		}
	}
	return m, nil
}

func (m model) viewFork() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Fork steer-runtime") + dimStyle.Render("  ↑↓=select  tab=next  enter=fork  esc=back"))
	b.WriteString("\n\n")

	// Fork list
	if len(m.forkForks) > 0 {
		if m.forkField == 0 {
			b.WriteString(activeStyle.Render("▸ Select fork:") + "\n")
		} else {
			b.WriteString("  Select fork:\n")
		}
		start := 0
		if m.forkCursor > 8 {
			start = m.forkCursor - 8
		}
		end := start + 12
		if end > len(m.forkForks) {
			end = len(m.forkForks)
		}
		for i := start; i < end; i++ {
			cursor := "    "
			name := m.forkForks[i]
			if i == m.forkCursor {
				if m.forkField == 0 {
					cursor = "  " + activeStyle.Render("▸ ")
					name = activeStyle.Render(name)
				} else {
					cursor = "  " + checkStyle.Render("✓ ")
				}
			}
			b.WriteString(cursor + name + "\n")
		}
		b.WriteString(fmt.Sprintf("\n    %s\n", dimStyle.Render(fmt.Sprintf("%d/%d forks", m.forkCursor+1, len(m.forkForks)))))
	} else if m.forkError != "" {
		b.WriteString("  " + warnStyle.Render("⚠ "+m.forkError) + "\n")
	}

	// Manual input
	b.WriteString("\n")
	manualLabel := "  Or enter repo: "
	if m.forkField == 2 {
		manualLabel = activeStyle.Render("▸ Or enter repo: ")
	}
	manualVal := m.forkManual
	if m.forkField == 2 {
		if manualVal == "" {
			manualVal = dimStyle.Render("org/steer-runtime█")
		} else {
			manualVal = activeStyle.Render(manualVal + "█")
		}
	} else if manualVal == "" {
		manualVal = dimStyle.Render("—")
	}
	b.WriteString(manualLabel + manualVal + "\n")

	// Branch
	b.WriteString("\n")
	branchLabel := "  Branch: "
	if m.forkField == 1 {
		branchLabel = activeStyle.Render("▸ Branch: ")
	}
	branchVal := m.forkBranch
	if m.forkField == 1 {
		branchVal = activeStyle.Render(m.forkBranch + "█")
	}
	b.WriteString(branchLabel + branchVal + "\n")

	// Clone URL preview
	var previewRepo string
	if m.forkManual != "" {
		previewRepo = m.forkManual
	} else if len(m.forkForks) > 0 {
		previewRepo = m.forkForks[m.forkCursor]
	}
	if previewRepo != "" {
		b.WriteString("\n" + dimStyle.Render("  Clone URL: https://"+config.GHHost+"/"+previewRepo+".git"))
	}
	return boxStyle.Render(b.String())
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
	case screenFork:
		return m.viewFork()
	case screenEnvVars:
		return m.viewEnvVars()
	case screenGitHub:
		return m.viewGitHub()
	case screenCreateWorkspace:
		return m.viewCreateWorkspace()
	default:
		return m.viewDashboard()
	}
}
