package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.disney.com/SANCR225/koda/internal/config"
	mdl "github.disney.com/SANCR225/koda/internal/model"
	"github.disney.com/SANCR225/koda/internal/ops"
)

// cwField identifies which field is active in the create-workspace form.
type cwField int

const (
	cwName cwField = iota
	cwDescription
	cwTeam
	cwJiraPrefix
	cwProfiles
	cwAgent
	cwRules
	cwTools
	cwReposPath
	cwRepos     // repo list + add input
	cwFieldCount // sentinel
)

// cwRepoItem is a repo entry in the form (discovered or manual).
type cwRepoItem struct {
	repo     string // org/name
	name     string
	local    bool
	selected bool
}

// cwState holds all create-workspace form state.
type cwState struct {
	field     cwField
	name      string
	desc      string
	team      string
	jira      string
	profiles  []cwToggle
	agent     string
	rules     []cwToggle
	tools     bool
	reposPath string
	repos     []cwRepoItem
	repoInput string
	repoCursor int
	editing   bool
}

type cwToggle struct {
	id       string
	label    string
	selected bool
}

func newCWState(steerRoot, targetDir string) cwState {
	s := cwState{tools: true}

	// Load available profiles
	profiles, _ := ops.ListProfiles(steerRoot, targetDir)
	for _, p := range profiles {
		s.profiles = append(s.profiles, cwToggle{id: p.ID, label: fmt.Sprintf("%s (%d)", p.ID, p.AgentCount)})
	}

	// Load available rules
	for _, r := range ops.ListRules(steerRoot) {
		sel := r == "conventional_commit"
		s.rules = append(s.rules, cwToggle{id: r, label: r, selected: sel})
	}

	return s
}

func newCWStateFromWorkspace(steerRoot, targetDir string, ws mdl.Workspace) cwState {
	s := newCWState(steerRoot, targetDir)
	s.editing = true
	s.name = ws.Name
	s.desc = ws.Description
	s.team = ws.Team
	s.jira = ws.JiraPrefix
	s.agent = ws.DefaultAgent
	s.tools = ws.EnableTools
	s.reposPath = ws.WorkspacePath

	// Match profile toggles
	wsProfiles := map[string]bool{}
	for _, p := range ws.Profiles {
		wsProfiles[p] = true
	}
	for i := range s.profiles {
		s.profiles[i].selected = wsProfiles[s.profiles[i].id]
	}

	// Match rule toggles
	wsRules := map[string]bool{}
	for _, r := range ws.Rules {
		wsRules[r] = true
	}
	for i := range s.rules {
		s.rules[i].selected = wsRules[s.rules[i].id]
	}

	// Load repos
	for _, p := range ws.Projects {
		s.repos = append(s.repos, cwRepoItem{
			repo: p.Repo, name: p.Name, selected: true,
		})
	}

	return s
}

func (m model) updateCreateWorkspace(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	cw := &m.cw
	key := msg.String()

	// Global keys
	switch key {
	case "esc":
		m.screen = screenWorkspaces
		m.statusMsg = ""
		return m, nil
	case "ctrl+s":
		return m.saveWorkspace()
	case "ctrl+e":
		return m.openWorkspaceInEditor()
	}

	// Field-specific handling
	switch cw.field {
	case cwProfiles:
		return m.cwUpdateToggleList(key, cw.profiles)
	case cwRules:
		return m.cwUpdateToggleList(key, cw.rules)
	case cwTools:
		switch key {
		case " ", "enter":
			cw.tools = !cw.tools
		case "tab", "down", "j":
			cw.field++
		case "shift+tab", "up", "k":
			cw.field--
		}
		return m, nil
	case cwRepos:
		return m.cwUpdateRepos(key)
	default:
		return m.cwUpdateTextField(key)
	}
}

func (m model) cwUpdateTextField(key string) (tea.Model, tea.Cmd) {
	cw := &m.cw
	ptr := cw.activeTextPtr()

	switch key {
	case "tab", "down", "enter":
		// On reposPath change, trigger scan
		if cw.field == cwReposPath && cw.reposPath != "" {
			m.cwScanRepos()
		}
		if cw.field < cwFieldCount-1 {
			cw.field++
		}
	case "shift+tab", "up":
		if cw.field > 0 {
			cw.field--
		}
	case "backspace":
		if len(*ptr) > 0 {
			*ptr = (*ptr)[:len(*ptr)-1]
		}
	case "ctrl+u":
		*ptr = ""
	default:
		if len(key) == 1 && key[0] >= 32 {
			*ptr += key
		}
	}
	return m, nil
}

func (m model) cwUpdateToggleList(key string, items []cwToggle) (tea.Model, tea.Cmd) {
	cw := &m.cw
	var list *[]cwToggle
	if cw.field == cwProfiles {
		list = &cw.profiles
	} else {
		list = &cw.rules
	}

	switch key {
	case "tab", "down":
		if cw.repoCursor < len(*list)-1 {
			cw.repoCursor++
		} else {
			cw.repoCursor = 0
			cw.field++
		}
	case "shift+tab", "up":
		if cw.repoCursor > 0 {
			cw.repoCursor--
		} else {
			cw.repoCursor = 0
			if cw.field > 0 {
				cw.field--
			}
		}
	case " ":
		if cw.repoCursor < len(*list) {
			(*list)[cw.repoCursor].selected = !(*list)[cw.repoCursor].selected
		}
	case "j":
		if cw.repoCursor < len(*list)-1 {
			cw.repoCursor++
		}
	case "k":
		if cw.repoCursor > 0 {
			cw.repoCursor--
		}
	}
	return m, nil
}

func (m model) cwUpdateRepos(key string) (tea.Model, tea.Cmd) {
	cw := &m.cw
	totalItems := len(cw.repos) + 1 // repos + add input

	switch key {
	case "tab", "down", "j":
		if cw.repoCursor < totalItems-1 {
			cw.repoCursor++
		}
	case "shift+tab", "up", "k":
		if cw.repoCursor > 0 {
			cw.repoCursor--
		} else {
			cw.field--
			cw.repoCursor = 0
		}
	case " ":
		// Toggle repo selection
		if cw.repoCursor < len(cw.repos) {
			cw.repos[cw.repoCursor].selected = !cw.repos[cw.repoCursor].selected
		}
	case "d":
		// Delete repo
		if cw.repoCursor < len(cw.repos) {
			cw.repos = append(cw.repos[:cw.repoCursor], cw.repos[cw.repoCursor+1:]...)
			if cw.repoCursor >= len(cw.repos)+1 {
				cw.repoCursor = len(cw.repos)
			}
		}
	case "enter":
		// Add repo from input
		if cw.repoCursor >= len(cw.repos) && cw.repoInput != "" {
			name := cw.repoInput
			if i := strings.LastIndex(name, "/"); i >= 0 {
				name = name[i+1:]
			}
			cw.repos = append(cw.repos, cwRepoItem{
				repo: cw.repoInput, name: name, local: false, selected: true,
			})
			cw.repoInput = ""
			cw.repoCursor = len(cw.repos) // move to new add line
		}
	case "backspace":
		if cw.repoCursor >= len(cw.repos) && len(cw.repoInput) > 0 {
			cw.repoInput = cw.repoInput[:len(cw.repoInput)-1]
		}
	case "ctrl+u":
		if cw.repoCursor >= len(cw.repos) {
			cw.repoInput = ""
		}
	default:
		if cw.repoCursor >= len(cw.repos) && len(key) == 1 && key[0] >= 32 {
			cw.repoInput += key
		}
	}
	return m, nil
}

func (m *model) cwScanRepos() {
	discovered := ops.ScanRepos(m.cw.reposPath)
	// Merge: keep existing manual adds, add new discoveries
	existing := map[string]bool{}
	for _, r := range m.cw.repos {
		existing[r.repo] = true
	}
	for _, d := range discovered {
		if !existing[d.Repo] {
			m.cw.repos = append(m.cw.repos, cwRepoItem{
				repo: d.Repo, name: d.Name, local: true, selected: true,
			})
		}
	}
}


func (m model) openWorkspaceInEditor() (tea.Model, tea.Cmd) {
	cw := &m.cw
	if cw.name == "" {
		m.statusMsg = "Name is required before opening editor"
		return m, nil
	}
	// Build workspace from current form state and write to disk
	ws := m.cwBuildWorkspace()
	if err := ops.CreateWorkspace(m.steerRoot, ws); err != nil {
		m.statusMsg = "Save failed: " + err.Error()
		return m, nil
	}
	path := filepath.Join(m.steerRoot, config.WorkspacesDir, ws.Name, "workspace.json")
	m.cw.editing = true
	c := ops.EditorCmd(path)
	return m, tea.ExecProcess(c, func(err error) tea.Msg {
		return wsEditorFinishedMsg{name: ws.Name, err: err}
	})
}

type wsEditorFinishedMsg struct {
	name string
	err  error
}

// cwBuildWorkspace constructs a Workspace from the current form state.
func (m model) cwBuildWorkspace() mdl.Workspace {
	cw := &m.cw
	ws := mdl.Workspace{
		Name:          cw.name,
		Description:   cw.desc,
		Team:          cw.team,
		JiraPrefix:    cw.jira,
		DefaultAgent:  cw.agent,
		EnableTools:   cw.tools,
		WorkspacePath: cw.reposPath,
	}
	for _, p := range cw.profiles {
		if p.selected {
			ws.Profiles = append(ws.Profiles, p.id)
		}
	}
	for _, r := range cw.rules {
		if r.selected {
			ws.Rules = append(ws.Rules, r.id)
		}
	}
	for _, r := range cw.repos {
		if !r.selected {
			continue
		}
		path := r.name
		if cw.reposPath != "" {
			path = filepath.Join(cw.reposPath, r.name)
		}
		ws.Projects = append(ws.Projects, mdl.WorkspaceProject{
			Name: r.name, Path: path, Repo: r.repo,
		})
	}
	return ws
}


func (m model) saveWorkspace() (tea.Model, tea.Cmd) {
	cw := &m.cw
	if cw.name == "" {
		m.statusMsg = "Name is required"
		return m, nil
	}

	ws := m.cwBuildWorkspace()
	if err := ops.CreateWorkspace(m.steerRoot, ws); err != nil {
		m.statusMsg = "Save failed: " + err.Error()
		return m, nil
	}


	verb := "Created"
	if cw.editing {
		verb = "Updated"
	}
	// Publish via PR
	settings := config.ReadSteerSettings()
	if settings.Source == "git" {
		// Git fork: publish directly
		prURL, err := ops.PublishWorkspace(m.steerRoot, ws.Name, cw.editing)
		m.refresh()
		m.screen = screenWorkspaces
		if err != nil {
			m.statusMsg = fmt.Sprintf(verb+" '%s' (PR failed: %s)", ws.Name, err)
		} else {
			m.statusMsg = fmt.Sprintf(verb+" '%s' — PR: %s", ws.Name, prURL)
		}
	} else if ops.CanWriteRepo(config.DefaultSteerRepo) {
		// Tarball + write access to upstream: init git temporarily, publish, clean up
		prURL, err := ops.PublishWorkspaceToUpstream(m.steerRoot, ws.Name, cw.editing)
		m.refresh()
		m.screen = screenWorkspaces
		if err != nil {
			m.statusMsg = fmt.Sprintf(verb+" '%s' (PR failed: %s)", ws.Name, err)
		} else {
			m.statusMsg = fmt.Sprintf(verb+" '%s' — PR: %s", ws.Name, prURL)
		}
	} else {
		m.refresh()
		m.screen = screenWorkspaces
		m.statusMsg = fmt.Sprintf("Workspace '%s' saved locally.", ws.Name)
	}
	return m, nil
}

func (cw *cwState) activeTextPtr() *string {
	switch cw.field {
	case cwName:
		return &cw.name
	case cwDescription:
		return &cw.desc
	case cwTeam:
		return &cw.team
	case cwJiraPrefix:
		return &cw.jira
	case cwAgent:
		return &cw.agent
	case cwReposPath:
		return &cw.reposPath
	}
	return &cw.name // fallback
}

func (m model) viewCreateWorkspace() string {
	cw := &m.cw
	var b strings.Builder

	title := "Create Workspace"
	if cw.editing {
		title = "Edit Workspace"
	}
	b.WriteString(titleStyle.Render(title) + dimStyle.Render("  tab=next  ctrl+s=save  ctrl+e=editor  esc=back"))
	b.WriteString("\n\n")

	// Text fields
	textFields := []struct {
		field cwField
		label string
		value string
	}{
		{cwName, "Name", cw.name},
		{cwDescription, "Description", cw.desc},
		{cwTeam, "Team", cw.team},
		{cwJiraPrefix, "Jira Prefix", cw.jira},
	}
	for _, tf := range textFields {
		prefix := "  "
		if cw.field == tf.field {
			prefix = activeStyle.Render("▸ ")
		}
		label := fmt.Sprintf("%-14s", tf.label+":")
		val := tf.value
		if cw.field == tf.field {
			val = activeStyle.Render(val + "█")
		} else if val == "" {
			val = dimStyle.Render("—")
		}
		b.WriteString(prefix + label + val + "\n")
	}

	// Profiles (toggle list)
	b.WriteString("\n")
	if cw.field == cwProfiles {
		b.WriteString(activeStyle.Render("▸ Profiles:\n"))
	} else {
		selected := []string{}
		for _, p := range cw.profiles {
			if p.selected {
				selected = append(selected, p.id)
			}
		}
		if len(selected) > 0 {
			b.WriteString("  Profiles:   " + checkStyle.Render(strings.Join(selected, ", ")) + "\n")
		} else {
			b.WriteString("  Profiles:   " + dimStyle.Render("none") + "\n")
		}
	}
	if cw.field == cwProfiles {
		for i, p := range cw.profiles {
			cursor := "    "
			if i == cw.repoCursor {
				cursor = "  " + activeStyle.Render("▸ ")
			}
			check := dimStyle.Render("[ ]")
			if p.selected {
				check = checkStyle.Render("[✓]")
			}
			b.WriteString(cursor + check + " " + p.label + "\n")
		}
	}

	// Default agent
	prefix := "  "
	if cw.field == cwAgent {
		prefix = activeStyle.Render("▸ ")
	}
	agentVal := cw.agent
	if cw.field == cwAgent {
		agentVal = activeStyle.Render(agentVal + "█")
	} else if agentVal == "" {
		agentVal = dimStyle.Render("—")
	}
	b.WriteString(prefix + "Agent:        " + agentVal + "\n")

	// Rules (toggle list)
	b.WriteString("\n")
	if cw.field == cwRules {
		b.WriteString(activeStyle.Render("▸ Rules:\n"))
	} else {
		selected := []string{}
		for _, r := range cw.rules {
			if r.selected {
				selected = append(selected, r.id)
			}
		}
		if len(selected) > 0 {
			b.WriteString("  Rules:      " + checkStyle.Render(strings.Join(selected, ", ")) + "\n")
		} else {
			b.WriteString("  Rules:      " + dimStyle.Render("none") + "\n")
		}
	}
	if cw.field == cwRules {
		for i, r := range cw.rules {
			cursor := "    "
			if i == cw.repoCursor {
				cursor = "  " + activeStyle.Render("▸ ")
			}
			check := dimStyle.Render("[ ]")
			if r.selected {
				check = checkStyle.Render("[✓]")
			}
			b.WriteString(cursor + check + " " + r.label + "\n")
		}
	}

	// Enable tools toggle
	prefix = "  "
	if cw.field == cwTools {
		prefix = activeStyle.Render("▸ ")
	}
	toolsVal := checkStyle.Render("enabled")
	if !cw.tools {
		toolsVal = dimStyle.Render("disabled")
	}
	b.WriteString(prefix + "Tools:        " + toolsVal + "\n")

	// Repos path
	b.WriteString("\n")
	prefix = "  "
	if cw.field == cwReposPath {
		prefix = activeStyle.Render("▸ ")
	}
	rpVal := cw.reposPath
	if cw.field == cwReposPath {
		rpVal = activeStyle.Render(rpVal + "█")
	} else if rpVal == "" {
		rpVal = dimStyle.Render("~/Workspace/team")
	}
	b.WriteString(prefix + "Repos Path:   " + rpVal + "\n")

	// Repos list
	if cw.field == cwRepos {
		b.WriteString(activeStyle.Render("▸ Repositories:") + "\n")
	} else {
		b.WriteString("  Repositories:\n")
	}

	if len(cw.repos) == 0 && cw.field != cwRepos {
		b.WriteString("    " + dimStyle.Render("none — set repos path to scan") + "\n")
	}

	// Windowed repo list (max 10 visible)
	repoStart := 0
	if cw.field == cwRepos && cw.repoCursor > 8 && cw.repoCursor < len(cw.repos) {
		repoStart = cw.repoCursor - 8
	}
	repoEnd := repoStart + 10
	if repoEnd > len(cw.repos) {
		repoEnd = len(cw.repos)
	}
	if repoStart > 0 {
		b.WriteString("    " + dimStyle.Render(fmt.Sprintf("↑ %d more", repoStart)) + "\n")
	}
	for i := repoStart; i < repoEnd; i++ {
		r := cw.repos[i]
		cursor := "    "
		if cw.field == cwRepos && i == cw.repoCursor {
			cursor = "  " + activeStyle.Render("▸ ")
		}
		check := dimStyle.Render("[ ]")
		if r.selected {
			check = checkStyle.Render("[✓]")
		}
		tag := ""
		if r.local {
			tag = dimStyle.Render(" (local)")
		} else {
			tag = warnStyle.Render(" (clone on apply)")
		}
		b.WriteString(cursor + check + " " + r.repo + tag + "\n")
	}
	if repoEnd < len(cw.repos) {
		b.WriteString("    " + dimStyle.Render(fmt.Sprintf("↓ %d more", len(cw.repos)-repoEnd)) + "\n")
	}

	// Add input line
	if cw.field == cwRepos {
		cursor := "    "
		if cw.repoCursor >= len(cw.repos) {
			cursor = "  " + activeStyle.Render("▸ ")
		}
		addVal := cw.repoInput
		if cw.repoCursor >= len(cw.repos) {
			if addVal == "" {
				addVal = dimStyle.Render("[+] org/repo-name...█")
			} else {
				addVal = activeStyle.Render("[+] " + addVal + "█")
			}
		} else {
			addVal = dimStyle.Render("[+] add repo...")
		}
		b.WriteString(cursor + addVal + "\n")
	}

	if m.statusMsg != "" {
		b.WriteString("\n  " + warnStyle.Render(m.statusMsg) + "\n")
	}

	return boxStyle.Render(b.String())
}
