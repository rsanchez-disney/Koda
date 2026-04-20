package tui

import (
	"encoding/json"
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
	screenResetConfirm
	screenDoctor
	screenRules
	screenMCP
	screenFork
	screenCreateWorkspace
	screenEnvVars
	screenGitHub
	screenKiroIDE
	screenYax
)

type model struct {
	steerRoot   string
	targetDir   string
	screen      screen
	cursor       int
	scrollOffset int
	maxVisible   int
	report      ops.HealthReport
	profiles    []profileItem
	tokens      map[string]string
	tokenInput  string
	workspaces      []mdl.Workspace
	wsDisplayOrder  []int // visual row → slice index for tree navigation
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
	jiraInstances []mdl.JiraInstance
	confInstances []mdl.ConfluenceInstance
	mcpSection    int  // 0=github, 1=jira, 2=confluence, 3=other
	mcpRow        int  // row within current section
	mcpAdding     bool
	mcpEditing    bool
	mcpEditField  int  // 0=url/host, 1=token
	mcpInput      string
	mcpField      int  // field index during add (0=name, 1=url/host, 2=token)
	kiroSettings  map[string]string
	kiroAgents    []string
	kiroAgentPick   bool
	kiroAgentFilter string
	envVarKeys    []string
	wsMCPKeys     []string
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
	forking       bool
	cw            cwState
	ghIdentity    ops.GHIdentity
	kodaVersion   string
	memoryStatus  ops.MemoryStatusInfo
	yaxStatus     ops.YaxStatus
	yaxProjects   []ops.YaxProject
	yaxLines      []string // recent or search results
	yaxSearch     string
	yaxSearching  bool
	yaxProject    string // selected project filter
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
	name      string
	selected  bool
	workspace string
}

type mcpItem struct {
	name      string
	hasBundle bool
}

// cleanKey strips terminal bracket-paste markers from pasted text.
// Bubbletea wraps pasted text in [...] when Paste=true on KeyMsg.
func cleanKey(msg tea.KeyMsg) string {
	if msg.Paste {
		return string(msg.Runes)
	}
	return msg.String()
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
	m.jiraInstances = ops.ReadJiraInstances()
	m.confInstances = ops.ReadConfluenceInstances()
	m.doctorResults = ops.RunDoctor(m.steerRoot, m.targetDir)
	m.memoryStatus = ops.MemoryStatus(m.targetDir)
	m.yaxStatus = ops.GetYaxStatus()

	// First-run: apply recommended kiro settings
	s := config.ReadSteerSettings()
	if !s.KiroSettingsApplied && len(m.report.Profiles) > 0 {
		ops.ConfigureKiroSettings(m.steerRoot, m.targetDir)
		s.KiroSettingsApplied = true
		config.SaveSteerSettings(s)
	}
	availRules := ops.ListRulesAll(m.steerRoot)
	m.rules = nil
	for _, r := range availRules {
		_, installed := os.Stat(filepath.Join(m.targetDir, config.RulesDir, r.Name+".md"))
		m.rules = append(m.rules, ruleItem{name: r.Name, workspace: r.WorkspaceName, selected: installed == nil})
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
	// Include SSE/remote servers from mcp.json (e.g., compass)
	mcpJSON := filepath.Join(m.targetDir, config.SettingsDir, "mcp.json")
	if data, err := os.ReadFile(mcpJSON); err == nil {
		var cfg struct {
			Servers map[string]struct{ Type string `json:"type"` } `json:"mcpServers"`
		}
		if json.Unmarshal(data, &cfg) == nil {
			bundleSet := map[string]bool{}
			for _, s := range m.mcpServers {
				bundleSet[s.name] = true
			}
			for name, srv := range cfg.Servers {
				if srv.Type == "sse" && !bundleSet[name] {
					m.mcpServers = append(m.mcpServers, mcpItem{name: name + " (sse)", hasBundle: true})
				}
			}
		}
	}

	// Auto-upgrade check
	if config.ReadSteerSettings().AutoUpgrade && m.kodaVersion != "" {
		if latest := ops.CheckForUpdate(m.kodaVersion); latest != "" {
			m.statusMsg = fmt.Sprintf("⬆ Koda %s available (current: %s) — run koda upgrade", latest, m.kodaVersion)
		}
	}
}


// adjustScroll keeps the cursor visible within the scroll window.
func (m *model) adjustScroll(listLen int) {
	if m.maxVisible <= 0 {
		m.maxVisible = 20 // default
	}
	if m.cursor < m.scrollOffset {
		m.scrollOffset = m.cursor
	}
	if m.cursor >= m.scrollOffset+m.maxVisible {
		m.scrollOffset = m.cursor - m.maxVisible + 1
	}
	if m.scrollOffset < 0 {
		m.scrollOffset = 0
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
	case forkDoneMsg:
		m.forking = false
		if msg.err != nil {
			m.statusMsg = "❌ " + msg.err.Error()
		} else {
			m.refresh()
			if msg.repo == "official" {
				m.statusMsg = "✅ Unforked! Back to official tarball."
			} else {
				m.statusMsg = fmt.Sprintf("✅ Forked to %s!", msg.repo)
			}
		}
		return m, nil
	case kiroIDEDoneMsg:
		m.statusMsg = fmt.Sprintf("\u2705 Kiro IDE %s: %d steering, %d skills, %d hooks, %d MCP", msg.action, msg.result.Steering, msg.result.Skills, msg.result.Hooks, msg.result.MCP)
		return m, nil
	case mcpRegenDoneMsg:
		m.refresh()
		if msg.err != nil {
			m.statusMsg = "❌ Regenerate failed: " + msg.err.Error()
		} else {
			m.statusMsg = "✅ mcp.json regenerated"
		}
	case doctorFixDoneMsg:
		m.envVars = ops.ReadEnvVars()
		m.ghRemotes = ops.ReadGitHubRemotes()
		m.jiraInstances = ops.ReadJiraInstances()
		m.confInstances = ops.ReadConfluenceInstances()
		m.doctorResults = ops.RunDoctor(m.steerRoot, m.targetDir)
	m.memoryStatus = ops.MemoryStatus(m.targetDir)
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
		case screenResetConfirm:
			return m.updateResetConfirm(msg)
		case screenDoctor:
			return m.updateDoctor(msg)
		case screenRules:
			return m.updateRules(msg)
		case screenMCP:
			return m.updateMCP(msg)
		case screenFork:
			return m.updateFork(msg)
		case screenKiroIDE:
			return m.updateKiroIDE(msg)
		case screenEnvVars:
			return m.updateEnvVars(msg)
		case screenGitHub:
			return m.updateGitHub(msg)
		case screenCreateWorkspace:
			return m.updateCreateWorkspace(msg)
		case screenYax:
			return m.updateYax(msg)
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
		m.scrollOffset = 0
	case "t":
		m.buildWSDisplayOrder()
		m.mcpSection = 3 // Other Tokens section
		m.mcpRow = 0
		m.mcpAdding = false
		m.mcpEditing = false
		m.screen = screenMCP
	case "a":
		m.screen = screenAgents
		m.cursor = 0
		m.agentFilter = ""
	case "w":
		m.buildWSDisplayOrder()
		m.screen = screenWorkspaces
		m.cursor = 0
		m.scrollOffset = 0
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
		m.jiraInstances = ops.ReadJiraInstances()
		m.confInstances = ops.ReadConfluenceInstances()
		m.mcpSection = 0 // GitHub section
		m.mcpRow = 0
		m.mcpAdding = false
		m.screen = screenMCP
	case "y":
		if m.yaxStatus.Installed {
			m.yaxProjects = ops.YaxProjects()
			m.yaxLines = nil
			m.yaxSearch = ""
			m.yaxSearching = false
			m.yaxProject = ""
			m.cursor = 0
			for _, line := range ops.YaxRecent("", 20) {
				m.yaxLines = append(m.yaxLines, line.Title)
			}
			m.screen = screenYax
		} else {
			m.statusMsg = "yax not installed — run koda upgrade"
		}
	case "M":
		if false { // memory-mcp deprecated — replaced by yax
			if false {
				m.statusMsg = "⏳ Stopping memory-mcp..."
				targetDir := m.targetDir
				return m, func() tea.Msg {
					ops.MemoryStop(targetDir)
					return syncDoneMsg{}
				}
			} else {
				m.statusMsg = "⏳ Starting memory-mcp..."
				targetDir := m.targetDir
				return m, func() tea.Msg {
					ops.MemoryStart(targetDir)
					return syncDoneMsg{}
				}
			}
		}
	case "f":
		settings := config.ReadSteerSettings()
		if settings.Source == "git" {
			// Unfork: switch back to tarball
			m.forking = true
			m.statusMsg = "⏳ Switching to official tarball..."
			steerRoot := m.steerRoot
			return m, func() tea.Msg {
				err := ops.UnforkSteerRuntime(steerRoot)
				return forkDoneMsg{err: err, repo: "official"}
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
		m.screen = screenResetConfirm
	case "d":
		m.screen = screenDoctor
	case "r":
		m.screen = screenRules
		m.cursor = 0
	case "k":
		m.kiroSettings = ops.ReadKiroSettings()
		m.kiroAgentPick = false
		agents := ops.AllAgents(m.steerRoot, m.targetDir)
		var orch, rest []string
		for _, a := range agents {
			if a.Name == "orchestrator" || strings.HasSuffix(a.Name, "_orchestrator_agent") {
				orch = append(orch, a.Name)
			} else {
				rest = append(rest, a.Name)
			}
		}
		m.kiroAgents = append(orch, rest...)
		m.cursor = 0
		m.screen = screenKiroIDE
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

func (m model) updateResetConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if err := ops.Reset(m.steerRoot); err != nil {
			m.statusMsg = "Reset failed: " + err.Error()
		} else {
			m.statusMsg = "✅ Reset complete — run install to add profiles"
		}
		m.refresh()
		m.screen = screenDashboard
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
	if agent := ops.SuggestDefaultAgent(m.steerRoot, m.targetDir); agent != "" {
		b.WriteString(fmt.Sprintf("  Agent:     %s\n", checkStyle.Render(agent)))
	}

	if m.kodaVersion != "" {
		b.WriteString(fmt.Sprintf("  Koda:      %s\n", dimStyle.Render(m.kodaVersion)))
	}
	// Memory status

	if m.yaxStatus.Installed {
		detail := m.yaxStatus.Version
		if m.yaxStatus.Observations > 0 {
			detail += fmt.Sprintf(" — %d obs, %d sessions, %d edges", m.yaxStatus.Observations, m.yaxStatus.Sessions, m.yaxStatus.Edges)
		}
		b.WriteString(fmt.Sprintf("  Yax:       %s\n", checkStyle.Render(detail)))
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
	mcpCount := len(m.ghRemotes) + len(m.jiraInstances) + len(m.confInstances)
	b.WriteString(activeStyle.Render("  [m]") + fmt.Sprintf(" MCP (%d)     ", mcpCount))
	b.WriteString(activeStyle.Render("[e]") + " Env Vars  ")
	b.WriteString(activeStyle.Render("[k]") + " Kiro\n")
	b.WriteString(activeStyle.Render("  [g]") + fmt.Sprintf(" GitHub (%d) ", len(m.ghRemotes)))

	b.WriteString(activeStyle.Render("  [s]") + " Sync        ")
	b.WriteString(activeStyle.Render("[c]") + " Clean\n")
	if settings.Source == "git" {
		b.WriteString(activeStyle.Render("  [f]") + " Unfork      ")
	} else {
		b.WriteString(activeStyle.Render("  [f]") + " Fork        ")
	}
	if m.yaxStatus.Installed {
		b.WriteString(activeStyle.Render("[y]") + fmt.Sprintf(" Yax (%d)\n", m.yaxStatus.Observations))
	} else {
		b.WriteString(activeStyle.Render("[y]") + " Yax\n")
	}

	if m.yaxStatus.Installed {
		b.WriteString(activeStyle.Render("  [x]") + fmt.Sprintf(" Yax (%d)    ", m.yaxStatus.Observations))
	}
	b.WriteString(activeStyle.Render("  [enter]") + " Chat       ")
	b.WriteString(activeStyle.Render("[q]") + " Quit\n")

	if m.statusMsg != "" {
		b.WriteString("\n  " + checkStyle.Render(m.statusMsg) + "\n")
	}

	return boxStyle.Render(b.String())
}

func (m model) viewResetConfirm() string {
	var b strings.Builder
	b.WriteString(errStyle.Render("\u26a0 Reset Koda installation?"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("  This will backup ~/.kiro and reinstall fresh.\n  Tokens and env vars will be preserved.\n\n  Current: %d agents in:\n", m.report.TotalAgents))
	b.WriteString(fmt.Sprintf("  %s\n\n", dimStyle.Render(m.targetDir)))
	b.WriteString(activeStyle.Render("  [y]") + " Yes, reset    ")
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
type forkDoneMsg struct {
	err  error
	repo string
}

type mcpRegenDoneMsg struct{ err error }
type kiroIDEDoneMsg struct {
	result ops.KiroIDEResult
	action string
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
	key := msg.String()

	// Name input mode
	if m.ruleInput != "" || key == "n" {
		switch key {
		case "n":
			if m.ruleInput == "" {
				m.ruleInput = " " // activate input mode (trimmed on use)
				return m, nil
			}
			m.ruleInput += cleanKey(msg)
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
		var selected []ops.RuleInfo
		for _, r := range m.rules {
			if r.selected {
				selected = append(selected, ops.RuleInfo{Name: r.name, WorkspaceName: r.workspace})
			}
		}
		if len(selected) > 0 {
			ops.InstallRulesAll(m.steerRoot, m.targetDir, selected)
		}
		m.screen = screenDashboard
		m.statusMsg = fmt.Sprintf("%d rules installed!", len(selected))
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

	renderItem := func(i int) string {
		r := m.rules[i]
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("▸ ")
		}
		check := dimStyle.Render("[ ]")
		if r.selected {
			check = checkStyle.Render("[✓]")
		}
		name := r.name
		if i == m.cursor {
			name = activeStyle.Render(name)
		}
		return fmt.Sprintf("%s%s %s", cursor, check, name)
	}

	// Separate global and workspace rules
	var globals, wsRules []int
	for i, r := range m.rules {
		if r.workspace != "" {
			wsRules = append(wsRules, i)
		} else {
			globals = append(globals, i)
		}
	}

	b.WriteString(dimStyle.Render("  ── Global ──") + "\n")
	for _, i := range globals {
		b.WriteString(renderItem(i) + "\n")
	}

	if len(wsRules) > 0 {
		wsGroups := map[string][]int{}
		var wsOrder []string
		for _, i := range wsRules {
			ws := m.rules[i].workspace
			if _, seen := wsGroups[ws]; !seen {
				wsOrder = append(wsOrder, ws)
			}
			wsGroups[ws] = append(wsGroups[ws], i)
		}
		for _, ws := range wsOrder {
			b.WriteString("\n" + activeStyle.Render("  ── "+ws+" ──") + "\n")
			for _, i := range wsGroups[ws] {
				b.WriteString(renderItem(i) + "\n")
			}
		}
	}

	return boxStyle.Render(b.String())
}

// --- MCP ---

func (m model) updateMCP(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.mcpAdding {
		switch key {
		case "esc":
			m.mcpAdding = false
			m.mcpInput = ""
			m.mcpField = 0
		case "enter":
			val := strings.TrimSpace(m.mcpInput)
			m.mcpInput = ""
			if val == "" {
				break
			}
			switch m.mcpField {
			case 0: // name
				switch m.mcpSection {
				case 0:
					m.ghRemotes = append(m.ghRemotes, mdl.GitHubRemote{Name: val})
				case 1:
					m.jiraInstances = append(m.jiraInstances, mdl.JiraInstance{Name: val})
				case 2:
					m.confInstances = append(m.confInstances, mdl.ConfluenceInstance{Name: val})
				}
				m.mcpField = 1
			case 1: // url/host
				switch m.mcpSection {
				case 0:
					m.ghRemotes[len(m.ghRemotes)-1].Host = val
				case 1:
					m.jiraInstances[len(m.jiraInstances)-1].URL = val
				case 2:
					m.confInstances[len(m.confInstances)-1].URL = val
				}
				m.mcpField = 2
			case 2: // token
				switch m.mcpSection {
				case 0:
					r := &m.ghRemotes[len(m.ghRemotes)-1]
					r.Token = val
					ops.WriteGitHubRemote(*r)
				case 1:
					inst := &m.jiraInstances[len(m.jiraInstances)-1]
					inst.Token = val
					ops.WriteJiraInstance(*inst)
				case 2:
					inst := &m.confInstances[len(m.confInstances)-1]
					inst.Token = val
					ops.WriteConfluenceInstance(*inst)
				}
				ops.GenerateMcpJson(ops.FindNodeExe())
				m.mcpAdding = false
				m.mcpField = 0
				m.statusMsg = "Instance added"
			}
		case "backspace":
			if len(m.mcpInput) > 0 {
				m.mcpInput = m.mcpInput[:len(m.mcpInput)-1]
			}
		default:
			if ck := cleanKey(msg); len(ck) >= 1 && ck[0] >= 32 {
				m.mcpInput += ck
			}
		}
		return m, nil
	}

	if m.mcpEditing {
		switch key {
		case "esc":
			m.mcpEditing = false
			m.mcpInput = ""
			m.mcpEditField = 0
		case "tab":
			// Toggle between url/host (0) and token (1)
			m.mcpEditField = (m.mcpEditField + 1) % 2
			m.mcpInput = ""
		case "enter":
			val := strings.TrimSpace(m.mcpInput)
			m.mcpInput = ""
			switch m.mcpSection {
			case 0:
				r := m.mcpGHRow(m.mcpRow)
				if m.mcpEditField == 0 && val != "" {
					r.Host = val
				} else if m.mcpEditField == 1 && val != "" {
					r.Token = val
				}
				ops.WriteGitHubRemote(r)
			case 1:
				inst := m.mcpJiraRow(m.mcpRow)
				if m.mcpEditField == 0 && val != "" {
					inst.URL = val
				} else if m.mcpEditField == 1 && val != "" {
					inst.Token = val
				}
				ops.WriteJiraInstance(inst)
			case 2:
				inst := m.mcpConfRow(m.mcpRow)
				if m.mcpEditField == 0 && val != "" {
					inst.URL = val
				} else if m.mcpEditField == 1 && val != "" {
					inst.Token = val
				}
				ops.WriteConfluenceInstance(inst)
			case 3:
				if m.mcpRow < len(mdl.KnownTokens) && val != "" {
					tk := mdl.KnownTokens[m.mcpRow]
					m.tokens[tk.Key] = val
					ops.WriteTokens(m.tokens)
				}
			}
			ops.GenerateMcpJson(ops.FindNodeExe())
			m.ghRemotes = ops.ReadGitHubRemotes()
			m.jiraInstances = ops.ReadJiraInstances()
			m.confInstances = ops.ReadConfluenceInstances()
			m.mcpEditing = false
			m.mcpEditField = 0
			m.statusMsg = "Saved"
		case "backspace":
			if len(m.mcpInput) > 0 {
				m.mcpInput = m.mcpInput[:len(m.mcpInput)-1]
			}
		default:
			if ck := cleanKey(msg); len(ck) >= 1 && ck[0] >= 32 {
				m.mcpInput += ck
			}
		}
		return m, nil
	}

	switch key {
	case "esc", "q":
		m.screen = screenDashboard
		m.statusMsg = ""
	case "tab":
		m.mcpSection = (m.mcpSection + 1) % 4
		m.mcpRow = 0
	case "shift+tab":
		m.mcpSection = (m.mcpSection + 3) % 4
		m.mcpRow = 0
	case "up", "k":
		if m.mcpRow > 0 {
			m.mcpRow--
		}
	case "down", "j":
		max := m.mcpSectionLen() - 1
		if m.mcpRow < max {
			m.mcpRow++
		}
	case "n":
		if m.mcpSection < 3 { // can't add bundles
			m.mcpAdding = true
			m.mcpField = 0
			m.mcpInput = ""
		}
	case "enter":
		if m.mcpSection <= 3 && m.mcpSectionLen() > 0 {
			m.mcpEditing = true
			m.mcpEditField = 1 // default to token field
			m.mcpInput = ""
		}
	case "d":
		switch m.mcpSection {
		case 0:
			if m.mcpRow < len(m.ghRemotes) {
				name := m.ghRemotes[m.mcpRow].Name
				ops.RemoveGitHubRemote(name)
				m.ghRemotes = ops.ReadGitHubRemotes()
				m.jiraInstances = ops.ReadJiraInstances()
				m.confInstances = ops.ReadConfluenceInstances()
				m.statusMsg = fmt.Sprintf("Removed '%s'", name)
			}
		case 1:
			if m.mcpRow < len(m.jiraInstances) {
				name := m.jiraInstances[m.mcpRow].Name
				ops.RemoveJiraInstance(name)
				m.jiraInstances = ops.ReadJiraInstances()
				m.statusMsg = fmt.Sprintf("Removed '%s'", name)
			}
		case 2:
			if m.mcpRow < len(m.confInstances) {
				name := m.confInstances[m.mcpRow].Name
				ops.RemoveConfluenceInstance(name)
				m.confInstances = ops.ReadConfluenceInstances()
				m.statusMsg = fmt.Sprintf("Removed '%s'", name)
			}
		}
		ops.GenerateMcpJson(ops.FindNodeExe())
		if m.mcpRow >= m.mcpSectionLen() {
			m.mcpRow = m.mcpSectionLen() - 1
		}
		if m.mcpRow < 0 {
			m.mcpRow = 0
		}
	case "ctrl+d":
		// Clear token on selected instance
		switch m.mcpSection {
		case 0:
			r := m.mcpGHRow(m.mcpRow)
			if r.Token != "" {
				ops.RemoveGitHubRemote(r.Name)
				m.ghRemotes = ops.ReadGitHubRemotes()
				m.statusMsg = fmt.Sprintf("Cleared token for '%s'", r.Name)
			}
		case 1:
			inst := m.mcpJiraRow(m.mcpRow)
			if inst.Token != "" {
				ops.RemoveJiraInstance(inst.Name)
				m.jiraInstances = ops.ReadJiraInstances()
				m.statusMsg = fmt.Sprintf("Cleared token for '%s'", inst.Name)
			}
		case 2:
			inst := m.mcpConfRow(m.mcpRow)
			if inst.Token != "" {
				ops.RemoveConfluenceInstance(inst.Name)
				m.confInstances = ops.ReadConfluenceInstances()
				m.statusMsg = fmt.Sprintf("Cleared token for '%s'", inst.Name)
			}
		case 3:
			if m.mcpRow < len(mdl.KnownTokens) {
				tk := mdl.KnownTokens[m.mcpRow]
				delete(m.tokens, tk.Key)
				ops.WriteTokens(m.tokens)
				m.statusMsg = fmt.Sprintf("Cleared %s", tk.Label)
			}
		}
		ops.GenerateMcpJson(ops.FindNodeExe())
	case "r", "R":
		if err := ops.GenerateMcpJson(ops.FindNodeExe()); err != nil {
			m.statusMsg = "❌ Regenerate failed: " + err.Error()
		} else {
			m.refresh()
			m.statusMsg = "✅ mcp.json regenerated"
		}
	}
	return m, nil
}

func (m model) mcpSectionLen() int {
	switch m.mcpSection {
	case 0:
		return m.ghAllCount()
	case 1:
		return m.jiraAllCount()
	case 2:
		return m.confAllCount()
	case 3:
		return len(mdl.KnownTokens)
	}
	return 0
}

func (m model) ghAllCount() int {
	n := len(m.ghRemotes)
	for _, d := range mdl.DefaultGitHubRemotes {
		found := false
		for _, r := range m.ghRemotes {
			if r.Name == d.Name {
				found = true
				break
			}
		}
		if !found {
			n++
		}
	}
	return n
}

func (m model) jiraAllCount() int {
	n := len(m.jiraInstances)
	for _, d := range mdl.DefaultJiraInstances {
		found := false
		for _, inst := range m.jiraInstances {
			if inst.Name == d.Name {
				found = true
				break
			}
		}
		if !found {
			n++
		}
	}
	return n
}

func (m model) confAllCount() int {
	n := len(m.confInstances)
	for _, d := range mdl.DefaultConfluenceInstances {
		found := false
		for _, inst := range m.confInstances {
			if inst.Name == d.Name {
				found = true
				break
			}
		}
		if !found {
			n++
		}
	}
	return n
}

// mcpGHRow returns the GitHub remote at the given row index (configured + unconfigured defaults).
func (m model) mcpGHRow(row int) mdl.GitHubRemote {
	all := append([]mdl.GitHubRemote{}, m.ghRemotes...)
	for _, d := range mdl.DefaultGitHubRemotes {
		found := false
		for _, r := range m.ghRemotes {
			if r.Name == d.Name {
				found = true
				break
			}
		}
		if !found {
			all = append(all, d)
		}
	}
	if row < len(all) {
		return all[row]
	}
	return mdl.GitHubRemote{}
}

// mcpJiraRow returns the Jira instance at the given row index.
func (m model) mcpJiraRow(row int) mdl.JiraInstance {
	all := append([]mdl.JiraInstance{}, m.jiraInstances...)
	for _, d := range mdl.DefaultJiraInstances {
		found := false
		for _, inst := range m.jiraInstances {
			if inst.Name == d.Name {
				found = true
				break
			}
		}
		if !found {
			all = append(all, d)
		}
	}
	if row < len(all) {
		return all[row]
	}
	return mdl.JiraInstance{}
}

// mcpConfRow returns the Confluence instance at the given row index.
func (m model) mcpConfRow(row int) mdl.ConfluenceInstance {
	all := append([]mdl.ConfluenceInstance{}, m.confInstances...)
	for _, d := range mdl.DefaultConfluenceInstances {
		found := false
		for _, inst := range m.confInstances {
			if inst.Name == d.Name {
				found = true
				break
			}
		}
		if !found {
			all = append(all, d)
		}
	}
	if row < len(all) {
		return all[row]
	}
	return mdl.ConfluenceInstance{}
}

func (m model) viewMCP() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("MCP Instances") + dimStyle.Render("  enter=edit  n=add  d=delete  ctrl+d=clear  r=regenerate  tab=section  esc=back"))
	b.WriteString("\n\n")

	sections := []struct {
		title string
		idx   int
	}{
		{"GitHub", 0}, {"Jira", 1}, {"Confluence", 2}, {"Other Tokens", 3},
	}

	for _, sec := range sections {
		isActive := m.mcpSection == sec.idx
		header := sec.title
		if isActive {
			header = activeStyle.Render("▸ " + sec.title)
		} else {
			header = dimStyle.Render("  " + sec.title)
		}
		b.WriteString(header + "\n")

		switch sec.idx {
		case 0: // GitHub
			row := 0
			allGH := append([]mdl.GitHubRemote{}, m.ghRemotes...)
			for _, d := range mdl.DefaultGitHubRemotes {
				found := false
				for _, r := range m.ghRemotes {
					if r.Name == d.Name {
						found = true
						break
					}
				}
				if !found {
					allGH = append(allGH, d)
				}
			}
			for _, r := range allGH {
				cur := "    "
				if isActive && m.mcpRow == row && !m.mcpAdding && !m.mcpEditing {
					cur = activeStyle.Render("  ▸ ")
				}
				tok := errStyle.Render("not set")
				if r.Token != "" {
					tok = checkStyle.Render(ops.MaskToken(r.Token))
				}
				b.WriteString(fmt.Sprintf("%s%-12s %s  %s\n", cur, r.Name, dimStyle.Render(r.Host), tok))
				if isActive && m.mcpEditing && m.mcpRow == row {
					b.WriteString(m.renderEditPrompt(r.Host, r.Token))
				}
				row++
			}
		case 1: // Jira
			row := 0
			allJira := append([]mdl.JiraInstance{}, m.jiraInstances...)
			for _, d := range mdl.DefaultJiraInstances {
				found := false
				for _, inst := range m.jiraInstances {
					if inst.Name == d.Name {
						found = true
						break
					}
				}
				if !found {
					allJira = append(allJira, d)
				}
			}
			for _, inst := range allJira {
				cur := "    "
				if isActive && m.mcpRow == row && !m.mcpAdding && !m.mcpEditing {
					cur = activeStyle.Render("  ▸ ")
				}
				tok := errStyle.Render("not set")
				if inst.Token != "" {
					tok = checkStyle.Render(ops.MaskToken(inst.Token))
				}
				b.WriteString(fmt.Sprintf("%s%-12s %s  %s\n", cur, inst.Name, dimStyle.Render(inst.URL), tok))
				if isActive && m.mcpEditing && m.mcpRow == row {
					b.WriteString(m.renderEditPrompt(inst.URL, inst.Token))
				}
				row++
			}
		case 2: // Confluence
			row := 0
			allConf := append([]mdl.ConfluenceInstance{}, m.confInstances...)
			for _, d := range mdl.DefaultConfluenceInstances {
				found := false
				for _, inst := range m.confInstances {
					if inst.Name == d.Name {
						found = true
						break
					}
				}
				if !found {
					allConf = append(allConf, d)
				}
			}
			for _, inst := range allConf {
				cur := "    "
				if isActive && m.mcpRow == row && !m.mcpAdding && !m.mcpEditing {
					cur = activeStyle.Render("  ▸ ")
				}
				tok := errStyle.Render("not set")
				if inst.Token != "" {
					tok = checkStyle.Render(ops.MaskToken(inst.Token))
				}
				b.WriteString(fmt.Sprintf("%s%-12s %s  %s\n", cur, inst.Name, dimStyle.Render(inst.URL), tok))
				if isActive && m.mcpEditing && m.mcpRow == row {
					b.WriteString(m.renderEditPrompt(inst.URL, inst.Token))
				}
				row++
			}
		case 3: // Other Tokens
			for i, tk := range mdl.KnownTokens {
				cur := "    "
				if isActive && m.mcpRow == i && !m.mcpAdding && !m.mcpEditing {
					cur = activeStyle.Render("  ▸ ")
				}
				val := m.tokens[tk.Key]
				status := errStyle.Render("not set")
				if val != "" {
					status = checkStyle.Render(ops.MaskToken(val))
				}
				b.WriteString(fmt.Sprintf("%s%-22s %s\n", cur, tk.Label, status))
				if isActive && m.mcpEditing && m.mcpRow == i {
					b.WriteString("    " + activeStyle.Render("▸ Token: ") + activeStyle.Render(m.mcpInput+"█"))
					if val != "" {
						b.WriteString(dimStyle.Render("  (current: "+ops.MaskToken(val)+")"))
					}
					b.WriteString("\n")
					b.WriteString("    " + dimStyle.Render("enter=save  esc=cancel") + "\n")
				}
			}
		}
		b.WriteString("\n")
	}

	// Bundles footer (read-only)
	if len(m.mcpServers) > 0 {
		b.WriteString(dimStyle.Render("  Bundles") + "\n")
		for _, s := range m.mcpServers {
			icon := checkStyle.Render("✓")
			if !s.hasBundle {
				icon = errStyle.Render("✗")
			}
			b.WriteString(fmt.Sprintf("    %s %s\n", icon, dimStyle.Render(s.name)))
		}
		b.WriteString("\n")
	}

	if m.mcpAdding {
		labels := []string{"  Name:  ", "  URL:   ", "  Token: "}
		if m.mcpSection == 0 {
			labels[1] = "  Host:  "
		}
		for i, label := range labels {
			if i == m.mcpField {
				b.WriteString(activeStyle.Render("▸ "+label) + activeStyle.Render(m.mcpInput+"█") + "\n")
			} else if i < m.mcpField {
				b.WriteString("  " + label + checkStyle.Render("✓") + "\n")
			}
		}
	}

	if m.statusMsg != "" {
		b.WriteString("\n  " + checkStyle.Render(m.statusMsg) + "\n")
	}

	return boxStyle.Render(b.String())
}

func (m model) renderEditPrompt(currentURL, currentToken string) string {
	var b strings.Builder
	urlLabel := "  URL:   "
	if m.mcpSection == 0 {
		urlLabel = "  Host:  "
	}
	tokLabel := "  Token: "

	if m.mcpEditField == 0 {
		b.WriteString("    " + activeStyle.Render("▸ "+urlLabel) + activeStyle.Render(m.mcpInput+"█"))
		b.WriteString(dimStyle.Render("  (current: "+currentURL+")") + "\n")
		b.WriteString("    " + dimStyle.Render("  "+tokLabel+ops.MaskToken(currentToken)) + "\n")
	} else {
		b.WriteString("    " + dimStyle.Render("  "+urlLabel+currentURL) + "\n")
		b.WriteString("    " + activeStyle.Render("▸ "+tokLabel) + activeStyle.Render(m.mcpInput+"█"))
		if currentToken != "" {
			b.WriteString(dimStyle.Render("  (current: "+ops.MaskToken(currentToken)+")"))
		}
		b.WriteString("\n")
	}
	b.WriteString("    " + dimStyle.Render("tab=switch field  enter=save  esc=cancel") + "\n")
	return b.String()
}


// --- Env Vars ---

func (m *model) refreshEnvVarKeys() {
	m.envVarKeys = nil
	known := map[string]bool{}
	for _, e := range mdl.KnownEnvVars {
		m.envVarKeys = append(m.envVarKeys, e.Key)
		known[e.Key] = true
	}
	// Workspace MCP env keys (from mcp-meta.json) — values live in tokens.env
	m.wsMCPKeys = ops.WorkspaceMCPEnvVarKeys(m.steerRoot)
	for _, k := range m.wsMCPKeys {
		if !known[k] {
			m.envVarKeys = append(m.envVarKeys, k)
			known[k] = true
			if v := m.tokens[k]; v != "" {
				m.envVars[k] = v
			}
		}
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
			m.saveEnvVars()
			if m.cursor < len(m.envVarKeys)-1 {
				m.cursor++
			}
		} else {
			m.saveEnvVars()
			m.refresh()
			m.screen = screenDashboard
			m.statusMsg = "Env vars saved!"
		}
	case "n":
		if m.envInput == "" {
			m.envNewKey = " "
		} else {
			m.envInput += cleanKey(msg)
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
				m.saveEnvVars()
				m.refreshEnvVarKeys()
				if m.cursor >= len(m.envVarKeys) {
					m.cursor = len(m.envVarKeys) - 1
				}
			}
		} else {
			m.envInput += cleanKey(msg)
		}
	case "backspace":
		if len(m.envInput) > 0 {
			m.envInput = m.envInput[:len(m.envInput)-1]
		}
	case "ctrl+u":
		m.envInput = ""
	default:
		if ck := cleanKey(msg); len(ck) >= 1 && ck[0] >= 32 {
			m.envInput += ck
		}
	}
	return m, nil
}

func (m *model) saveEnvVars() {
	wsMCP := make(map[string]bool, len(m.wsMCPKeys))
	for _, k := range m.wsMCPKeys {
		wsMCP[k] = true
		if v := m.envVars[k]; v != "" {
			m.tokens[k] = v
		}
	}
	ops.WriteTokens(m.tokens)
	envOnly := make(map[string]string, len(m.envVars))
	for k, v := range m.envVars {
		if !wsMCP[k] {
			envOnly[k] = v
		}
	}
	ops.WriteEnvVars(envOnly)
	ops.GenerateMcpJson(ops.FindNodeExe())
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
			m.adjustScroll(len(m.profiles))
		}
	case "down", "j":
		if m.cursor < len(m.profiles)-1 {
			m.cursor++
			m.adjustScroll(len(m.profiles))
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
				// Install global base first, then workspace specialization
				ops.InstallProfile(m.steerRoot, p.id, m.targetDir)
				ops.InstallProfileFrom(p.sourceDir, m.targetDir)
			} else {
				ops.InstallProfile(m.steerRoot, p.id, m.targetDir)
			}
		} else if !p.selected && p.installed {
			ops.RemoveProfile(m.steerRoot, p.id, m.targetDir)
		}
	}
	ops.InjectAgentTokens(m.targetDir)
	ops.EnrichWelcomeMessages(m.targetDir)
	ops.WriteProfilesManifest(m.steerRoot, m.targetDir)
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
		if ck := cleanKey(msg); len(ck) >= 1 && ck[0] >= 32 {
			m.tokenInput += ck
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
		if m.cursor < len(m.wsDisplayOrder)-1 {
			m.cursor++
		}
	case "enter":
		if m.cursor < len(m.wsDisplayOrder) {
			ws := m.workspaces[m.wsDisplayOrder[m.cursor]]
			ops.ApplyWorkspace(m.steerRoot, m.targetDir, ws)
			m.refresh()
			m.screen = screenDashboard
			m.statusMsg = fmt.Sprintf("Workspace '%s' applied!", ws.Name)
		}
	case "e":
		if m.cursor < len(m.wsDisplayOrder) {
			ws := m.workspaces[m.wsDisplayOrder[m.cursor]]
			m.cw = newCWStateFromWorkspace(m.steerRoot, m.targetDir, ws)
			m.screen = screenCreateWorkspace
			m.statusMsg = ""
		}
	case "x":
		if m.cursor < len(m.wsDisplayOrder) {
			parent := m.workspaces[m.wsDisplayOrder[m.cursor]]
			m.cw = newCWState(m.steerRoot, m.targetDir)
			m.cw.extends = parent.Name
			m.screen = screenCreateWorkspace
			m.statusMsg = ""
		}
	}
	return m, nil
}

func (m *model) buildWSDisplayOrder() {
	children := map[string][]int{}
	var roots []int
	for i, ws := range m.workspaces {
		if ws.Extends == "" {
			roots = append(roots, i)
		} else {
			children[ws.Extends] = append(children[ws.Extends], i)
		}
	}
	m.wsDisplayOrder = nil
	var walk func(idx int)
	walk = func(idx int) {
		m.wsDisplayOrder = append(m.wsDisplayOrder, idx)
		for _, kid := range children[m.workspaces[idx].Name] {
			walk(kid)
		}
	}
	for _, idx := range roots {
		walk(idx)
	}
	// Orphans
	shown := map[int]bool{}
	for _, idx := range m.wsDisplayOrder {
		shown[idx] = true
	}
	for i := range m.workspaces {
		if !shown[i] {
			walk(i)
		}
	}
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

	// Map slice index → visual row for cursor highlight
	sliceToRow := map[int]int{}
	for row, idx := range m.wsDisplayOrder {
		sliceToRow[idx] = row
	}

	active := config.ReadSteerSettings().ActiveWorkspace

	var renderWS func(idx int, prefix string, last bool)
	renderWS = func(idx int, prefix string, last bool) {
		ws := m.workspaces[idx]
		row := sliceToRow[idx]
		cursor := "  "
		if row == m.cursor {
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
		if ws.Name == active {
			name = name + " ●"
		}
		if row == m.cursor {
			name = activeStyle.Render(name)
		} else if isParent {
			name = titleStyle.Render(name)
		}
		profiles := dimStyle.Render(strings.Join(ws.Profiles, ", "))
		b.WriteString(fmt.Sprintf("%s%s%s %s\n", cursor, dimStyle.Render(tree), name, profiles))
		if row == m.cursor && ws.Description != "" {
			b.WriteString(fmt.Sprintf("    %s%s\n", dimStyle.Render(prefix), dimStyle.Render(ws.Description)))
		}
		if row == m.cursor && len(ws.Services) > 0 {
			b.WriteString(fmt.Sprintf("    %s%s %s\n", dimStyle.Render(prefix), dimStyle.Render("services:"), dimStyle.Render(strings.Join(ws.Services, ", "))))
		}
		if row == m.cursor && len(ws.Channels) > 0 {
			b.WriteString(fmt.Sprintf("    %s%s %s\n", dimStyle.Render(prefix), dimStyle.Render("channels:"), dimStyle.Render(strings.Join(ws.Channels, ", "))))
		}
		if row == m.cursor && ws.Team != "" {
			b.WriteString(fmt.Sprintf("    %s%s %s\n", dimStyle.Render(prefix), dimStyle.Render("team:"), dimStyle.Render(ws.Team)))
		}
		if row == m.cursor && len(ws.Projects) > 0 {
			b.WriteString(fmt.Sprintf("    %s%s\n", dimStyle.Render(prefix), dimStyle.Render(fmt.Sprintf("projects: %d", len(ws.Projects)))))
		}
		kids := children[ws.Name]
		childPrefix := prefix
		if last {
			childPrefix += "   "
		} else {
			childPrefix += "│  "
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
					ops.GenerateMcpJson(ops.FindNodeExe())
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
			if ck := cleanKey(msg); len(ck) >= 1 && ck[0] >= 32 {
				m.ghInput += ck
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
			ops.GenerateMcpJson(ops.FindNodeExe())
			m.ghRemotes = ops.ReadGitHubRemotes()
			m.jiraInstances = ops.ReadJiraInstances()
			m.confInstances = ops.ReadConfluenceInstances()
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
		m.forking = true
		m.statusMsg = "⏳ Cloning fork..."
		m.screen = screenDashboard
		steerRoot, branch := m.steerRoot, m.forkBranch
		return m, func() tea.Msg {
			err := ops.ForkSteerRuntime(steerRoot, repo, branch)
			return forkDoneMsg{err: err, repo: repo}
		}
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
	case screenResetConfirm:
		return m.viewResetConfirm()
	case screenDoctor:
		return m.viewDoctor()
	case screenRules:
		return m.viewRules()
	case screenMCP:
		return m.viewMCP()
	case screenFork:
		return m.viewFork()
	case screenKiroIDE:
		return m.viewKiroIDE()
	case screenEnvVars:
		return m.viewEnvVars()
	case screenGitHub:
		return m.viewGitHub()
	case screenCreateWorkspace:
		return m.viewCreateWorkspace()
	case screenYax:
		return m.viewYax()
	default:
		return m.viewDashboard()
	}
}

func (m model) updateKiroIDE(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.kiroAgentPick {
		filtered := m.filteredKiroAgents()
		switch key {
		case "esc":
			if m.kiroAgentFilter != "" {
				m.kiroAgentFilter = ""
				m.cursor = 0
			} else {
				m.kiroAgentPick = false
				m.kiroAgentFilter = ""
				m.cursor = 2
			}
		case "up":
			if m.cursor > 0 { m.cursor-- }
		case "down":
			if m.cursor < len(filtered)-1 { m.cursor++ }
		case "enter":
			if m.cursor < len(filtered) {
				agent := filtered[m.cursor]
				ops.SetKiroSetting("chat.defaultAgent", agent)
				m.kiroSettings["chat.defaultAgent"] = agent
				m.kiroAgentPick = false
				m.kiroAgentFilter = ""
				m.cursor = 2
				m.statusMsg = fmt.Sprintf("Default agent: %s", agent)
			}
		case "backspace":
			if len(m.kiroAgentFilter) > 0 {
				m.kiroAgentFilter = m.kiroAgentFilter[:len(m.kiroAgentFilter)-1]
				m.cursor = 0
			}
		default:
			if len(key) == 1 && key[0] >= 32 {
				m.kiroAgentFilter += key
				m.cursor = 0
			}
		}
		return m, nil
	}

	ideItems := 2
	maxItem := ideItems + len(ops.ManagedKiroSettings) - 1

	switch key {
	case "esc":
		m.screen = screenDashboard
	case "i":
		steerRoot := m.steerRoot
		// Resolve workspace dir from active workspace
		var wsDir string
		active := config.ReadSteerSettings().ActiveWorkspace
		for _, ws := range m.workspaces {
			if ws.Name == active && ws.WorkspacePath != "" {
				wsDir = ws.WorkspacePath
				break
			}
		}
		m.statusMsg = "⏳ Installing Kiro IDE..."
		return m, func() tea.Msg {
			r, err := ops.InstallKiroIDE(steerRoot, wsDir)
			action := "install"
			if err != nil {
				return kiroIDEDoneMsg{result: r, action: action + " (error: " + err.Error() + ")"}
			}
			return kiroIDEDoneMsg{result: r, action: action}
		}
	case "s":
		steerRoot := m.steerRoot
		m.statusMsg = "\u23f3 Syncing Kiro IDE..."
		return m, func() tea.Msg {
			r := ops.SyncKiroIDE(steerRoot)
			return kiroIDEDoneMsg{result: r, action: "sync"}
		}
	case "r":
		var wsDir string
		for _, ws := range m.workspaces {
			if ws.WorkspacePath != "" {
				wsDir = ws.WorkspacePath
				break
			}
		}
		if wsDir != "" {
			ops.RemoveKiroIDE(wsDir)
			m.statusMsg = "✅ Hooks removed"
		} else {
			m.statusMsg = "No workspace_path configured"
		}
	case "up":
		if m.cursor > ideItems { m.cursor-- }
	case "down":
		if m.cursor < maxItem { m.cursor++ }
	case " ":
		if m.cursor >= ideItems {
			si := m.cursor - ideItems
			s := ops.ManagedKiroSettings[si]
			if s.Type == "bool" {
				newVal := "true"
				if m.kiroSettings[s.Key] == "true" { newVal = "false" }
				ops.SetKiroSetting(s.Key, newVal)
				m.kiroSettings[s.Key] = newVal
			}
		}
	case "enter":
		if m.cursor >= ideItems {
			si := m.cursor - ideItems
			s := ops.ManagedKiroSettings[si]
			if s.Type == "agent" {
				m.kiroAgentPick = true
				m.kiroAgentFilter = ""
				m.cursor = 0
			}
		}
	case "t":
		if tray.AutoStartEnabled() {
			tray.DisableAutoStart()
			m.statusMsg = "Tray auto-start disabled"
		} else {
			if err := tray.EnableAutoStart(); err == nil {
				m.statusMsg = "Tray auto-start enabled"
			} else {
				m.statusMsg = "Tray: " + err.Error()
			}
		}
	case "u":
		s := config.ReadSteerSettings()
		s.AutoUpgrade = !s.AutoUpgrade
		config.SaveSteerSettings(s)
		if s.AutoUpgrade {
			m.statusMsg = "Auto-upgrade enabled"
		} else {
			m.statusMsg = "Auto-upgrade disabled"
		}
	}
	return m, nil
}

func (m model) viewKiroIDE() string {
	if m.kiroAgentPick {
		return m.viewKiroAgentPicker()
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("Kiro") + dimStyle.Render("  i=install  s=sync  r=remove hooks  t=tray  u=auto-upgrade  esc=back"))
	b.WriteString("\n\n")

	// IDE section
	b.WriteString(dimStyle.Render("  IDE") + "\n")
	status := ops.CheckKiroIDE("")
	if status.SteeringCount > 0 {
		b.WriteString(fmt.Sprintf("  ✓ %d steering files\n", status.SteeringCount))
	} else {
		b.WriteString("  ✗ No steering files\n")
	}
	if status.SkillsCount > 0 {
		b.WriteString(fmt.Sprintf("  ✓ %d skills\n", status.SkillsCount))
	} else {
		b.WriteString("  ✗ No skills\n")
	}

	// Settings section
	b.WriteString("\n")
	if tray.AutoStartEnabled() {
		b.WriteString("  " + checkStyle.Render("☑") + " Tray auto-start" + dimStyle.Render("  (t to toggle)") + "\n")
	} else {
		b.WriteString("  ☐ Tray auto-start" + dimStyle.Render("  (t to toggle)") + "\n")
	}
	if config.ReadSteerSettings().AutoUpgrade {
		b.WriteString("  " + checkStyle.Render("☑") + " Auto-upgrade" + dimStyle.Render("  (u to toggle)") + "\n")
	} else {
		b.WriteString("  ☐ Auto-upgrade" + dimStyle.Render("  (u to toggle)") + "\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("  Preferences") + dimStyle.Render("  space=toggle  enter=select agent") + "\n\n")
	ideItems := 2 // offset for IDE section (not selectable)
	for i, s := range ops.ManagedKiroSettings {
		cursor := "  "
		if m.cursor == i+ideItems {
			cursor = activeStyle.Render("▸ ")
		}
		if s.Type == "agent" {
			val := m.kiroSettings[s.Key]
			if val == "" {
				val = dimStyle.Render("not set")
			}
			b.WriteString(fmt.Sprintf("%s%s: %s\n", cursor, s.Label, val))
		} else {
			val := m.kiroSettings[s.Key]
			check := "☐"
			if val == "true" {
				check = checkStyle.Render("☑")
			}
			rec := ""
			if s.Recommended {
				rec = dimStyle.Render("  ★ recommended")
			}
			b.WriteString(fmt.Sprintf("%s%s %s%s\n", cursor, check, s.Label, rec))
		}
	}

	return boxStyle.Render(b.String())
}

func (m model) filteredKiroAgents() []string {
	if m.kiroAgentFilter == "" {
		return m.kiroAgents
	}
	f := strings.ToLower(m.kiroAgentFilter)
	var out []string
	for _, name := range m.kiroAgents {
		if strings.Contains(strings.ToLower(name), f) {
			out = append(out, name)
		}
	}
	return out
}

func (m model) viewKiroAgentPicker() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Select Default Agent") + dimStyle.Render("  type to filter  enter=select  esc=back"))
	b.WriteString("\n")

	// Filter input
	if m.kiroAgentFilter != "" {
		b.WriteString(fmt.Sprintf("  🔍 %s", activeStyle.Render(m.kiroAgentFilter)))
	}
	b.WriteString("\n")

	filtered := m.filteredKiroAgents()
	current := m.kiroSettings["chat.defaultAgent"]

	// Viewport: show max 15 items around cursor
	viewSize := 15
	start := 0
	if m.cursor >= viewSize {
		start = m.cursor - viewSize/2
	}
	end := start + viewSize
	if end > len(filtered) {
		end = len(filtered)
		start = end - viewSize
		if start < 0 { start = 0 }
	}

	if start > 0 {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↑ %d more\n", start)))
	}
	for i := start; i < end; i++ {
		name := filtered[i]
		cursor := "  "
		if i == m.cursor {
			cursor = activeStyle.Render("▸ ")
		}
		label := name
		if name == "orchestrator" || strings.HasSuffix(name, "_orchestrator_agent") {
			label = "★ " + label
		}
		if name == current {
			label = checkStyle.Render(label) + dimStyle.Render(" (current)")
		}
		b.WriteString(fmt.Sprintf("%s%s\n", cursor, label))
	}
	if end < len(filtered) {
		b.WriteString(dimStyle.Render(fmt.Sprintf("  ↓ %d more\n", len(filtered)-end)))
	}

	if len(filtered) == 0 {
		b.WriteString(dimStyle.Render("  No agents match filter\n"))
	}

	return boxStyle.Render(b.String())
}

// --- Yax Memory ---

func (m model) updateYax(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.yaxSearching {
		switch msg.String() {
		case "enter":
			m.yaxSearching = false
			if m.yaxSearch != "" {
				m.yaxLines = ops.YaxSearch(m.yaxSearch)
				if len(m.yaxLines) == 0 {
					m.yaxLines = []string{"No results for: " + m.yaxSearch}
				}
			}
			m.cursor = 0
		case "esc":
			m.yaxSearching = false
			m.yaxSearch = ""
		case "backspace":
			if len(m.yaxSearch) > 0 {
				m.yaxSearch = m.yaxSearch[:len(m.yaxSearch)-1]
			}
		default:
			key := msg.String()
			if len(key) == 1 && key[0] >= 32 {
				m.yaxSearch += key
			}
		}
		return m, nil
	}

	switch msg.String() {
	case "esc":
		m.screen = screenDashboard
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.yaxLines)-1 {
			m.cursor++
		}
	case "/":
		m.yaxSearching = true
		m.yaxSearch = ""
	case "a":
		// Show all (clear project filter)
		m.yaxProject = ""
		m.yaxLines = nil
		for _, line := range ops.YaxRecent("", 20) {
			m.yaxLines = append(m.yaxLines, line.Title)
		}
		m.cursor = 0
	case "tab":
		// Cycle through projects
		if len(m.yaxProjects) > 0 {
			found := false
			for i, p := range m.yaxProjects {
				if p.Name == m.yaxProject {
					if i+1 < len(m.yaxProjects) {
						m.yaxProject = m.yaxProjects[i+1].Name
					} else {
						m.yaxProject = ""
					}
					found = true
					break
				}
			}
			if !found {
				m.yaxProject = m.yaxProjects[0].Name
			}
			m.yaxLines = nil
			for _, line := range ops.YaxRecent(m.yaxProject, 20) {
				m.yaxLines = append(m.yaxLines, line.Title)
			}
			m.cursor = 0
		}
	case "P":
		// Prune
		if out, err := ops.YaxPrune(180, false); err == nil {
			m.statusMsg = out
		} else {
			m.statusMsg = "Prune failed: " + err.Error()
		}
		m.yaxStatus = ops.GetYaxStatus()
	}
	return m, nil
}

func (m model) viewYax() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Yax Memory") + dimStyle.Render("  /=search  tab=project  a=all  P=prune  esc=back"))
	b.WriteString("\n\n")

	// Stats line
	b.WriteString(fmt.Sprintf("  %s — %d observations, %d sessions, %d edges, %d prompts\n",
		checkStyle.Render(m.yaxStatus.Version),
		m.yaxStatus.Observations, m.yaxStatus.Sessions, m.yaxStatus.Edges, m.yaxStatus.Prompts))

	// Projects
	if len(m.yaxProjects) > 0 {
		b.WriteString("\n" + dimStyle.Render("  Projects: "))
		for _, p := range m.yaxProjects {
			if p.Name == m.yaxProject {
				b.WriteString(activeStyle.Render("["+p.Name+"]") + " ")
			} else {
				b.WriteString(dimStyle.Render(p.Name) + " ")
			}
		}
		if m.yaxProject == "" {
			b.WriteString(activeStyle.Render("[all]"))
		}
		b.WriteString("\n")
	}

	// Search bar
	if m.yaxSearching {
		b.WriteString("\n  " + activeStyle.Render("Search: ") + m.yaxSearch + "█\n")
	} else if m.yaxSearch != "" {
		b.WriteString("\n  " + dimStyle.Render("Search: "+m.yaxSearch) + "\n")
	}

	// Observations list
	b.WriteString("\n")
	if len(m.yaxLines) == 0 {
		b.WriteString("  " + dimStyle.Render("No observations.") + "\n")
	} else {
		for i, line := range m.yaxLines {
			cursor := "  "
			if i == m.cursor {
				cursor = activeStyle.Render("▸ ")
			}
			if i == m.cursor {
				b.WriteString(cursor + activeStyle.Render(line) + "\n")
			} else {
				b.WriteString(cursor + line + "\n")
			}
		}
	}

	if m.statusMsg != "" {
		b.WriteString("\n  " + checkStyle.Render(m.statusMsg) + "\n")
	}

	return boxStyle.Render(b.String())
}
