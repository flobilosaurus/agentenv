package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/flobilosaurus/agent-env/internal/config"
	"github.com/flobilosaurus/agent-env/internal/profileimport"
)

var (
	accentStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("63")).Bold(true)
	selectedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true)
	mutedStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	errorStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	borderStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
)

type ProfileChoice struct {
	Profile string
	Create  bool
	Import  *profileimport.Intent
}

type ProfilePrompter interface {
	ChooseProfile(agent string, profiles []config.Profile, sources []profileimport.Source, groups []profileimport.Group) (ProfileChoice, error)
}

type ProfileRemovePrompter interface {
	ChooseProfileToRemove(profiles []config.Profile) (profile string, err error)
}

type WrapperUnwrapPrompter interface {
	ChooseWrapperToUnwrap(agents []string) (agent string, err error)
}

type BubblePrompter struct{}

func (BubblePrompter) ChooseProfile(agent string, profiles []config.Profile, sources []profileimport.Source, groups []profileimport.Group) (ProfileChoice, error) {
	m := newModel(agent, profiles, sources, groups)
	p := tea.NewProgram(m, tea.WithAltScreen())
	res, err := p.Run()
	if err != nil {
		return ProfileChoice{}, err
	}
	fm := res.(model)
	if fm.cancelled {
		return ProfileChoice{}, fmt.Errorf("profile selection cancelled")
	}
	return ProfileChoice{Profile: fm.selected, Create: fm.created, Import: fm.importIntent}, nil
}

func (BubblePrompter) ChooseProfileToRemove(profiles []config.Profile) (string, error) {
	m := newRemoveModel(profiles)
	p := tea.NewProgram(m, tea.WithAltScreen())
	res, err := p.Run()
	if err != nil {
		return "", err
	}
	fm := res.(removeModel)
	if fm.cancelled {
		return "", fmt.Errorf("profile removal cancelled")
	}
	return fm.selected, nil
}

func (BubblePrompter) ChooseWrapperToUnwrap(agents []string) (string, error) {
	m := newUnwrapModel(agents)
	p := tea.NewProgram(m, tea.WithAltScreen())
	res, err := p.Run()
	if err != nil {
		return "", err
	}
	fm := res.(unwrapModel)
	if fm.cancelled {
		return "", fmt.Errorf("unwrap cancelled")
	}
	return fm.selected, nil
}

type mode int

const (
	modeSelect mode = iota
	modeCreate
	modeImportSource
	modeImportGroups
	modeImportEmpty
)

type model struct {
	agent          string
	profiles       []config.Profile
	sources        []profileimport.Source
	groups         []profileimport.Group
	available      []profileimport.Group
	selectedGroups map[string]bool
	sourceCursor   int
	groupCursor    int
	cursor         int
	mode           mode
	input          textinput.Model
	error          string
	selected       string
	created        bool
	importIntent   *profileimport.Intent
	selectedSource profileimport.Source
	cancelled      bool
}

func newModel(agent string, profiles []config.Profile, importArgs ...interface{}) model {
	ti := textinput.New()
	ti.Placeholder = "profile-name"
	ti.PromptStyle = selectedStyle
	ti.PlaceholderStyle = mutedStyle
	ti.TextStyle = selectedStyle
	ti.Focus()
	m := model{agent: agent, profiles: profiles, input: ti, groups: profileimport.Catalog()}
	if len(importArgs) >= 1 {
		if sources, ok := importArgs[0].([]profileimport.Source); ok {
			m.sources = sources
		}
	}
	if len(importArgs) >= 2 {
		if groups, ok := importArgs[1].([]profileimport.Group); ok {
			m.groups = groups
		}
	}
	return m
}
func (m model) Init() tea.Cmd { return textinput.Blink }
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()
		switch key {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			m.moveUp()
		case "down", "j":
			m.moveDown()
		case " ":
			if m.mode == modeImportGroups && len(m.available) > 0 {
				id := m.available[m.groupCursor].ID
				m.selectedGroups[id] = !m.selectedGroups[id]
			}
		case "a":
			if m.mode == modeImportGroups {
				for _, group := range m.available {
					m.selectedGroups[group.ID] = true
				}
			}
		case "n":
			if m.mode == modeImportGroups {
				for _, group := range m.available {
					m.selectedGroups[group.ID] = false
				}
			}
		case "enter":
			return m.handleEnter()
		}
	}
	if m.mode == modeCreate {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m *model) moveUp() {
	switch m.mode {
	case modeSelect:
		if m.cursor > 0 {
			m.cursor--
		}
	case modeImportSource:
		if m.sourceCursor > 0 {
			m.sourceCursor--
		}
	case modeImportGroups:
		if m.groupCursor > 0 {
			m.groupCursor--
		}
	}
}

func (m *model) moveDown() {
	switch m.mode {
	case modeSelect:
		if m.cursor < len(m.profiles) {
			m.cursor++
		}
	case modeImportSource:
		if m.sourceCursor < len(m.sources) {
			m.sourceCursor++
		}
	case modeImportGroups:
		if m.groupCursor < len(m.available)-1 {
			m.groupCursor++
		}
	}
}

func (m model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.mode {
	case modeSelect:
		if m.cursor == len(m.profiles) {
			m.mode = modeCreate
			return m, nil
		}
		m.selected = m.profiles[m.cursor].Name
		return m, tea.Quit
	case modeCreate:
		name := strings.TrimSpace(m.input.Value())
		if err := config.ValidateProfileName(name); err != nil {
			m.error = err.Error()
			return m, nil
		}
		m.selected, m.created = name, true
		m.mode = modeImportSource
		m.sourceCursor = 0
		return m, nil
	case modeImportSource:
		if m.sourceCursor == 0 {
			return m.finishCreate(nil)
		}
		m.selectedSource = m.sources[m.sourceCursor-1]
		m.available = profileimport.AvailableGroups(m.selectedSource, m.groups)
		if len(m.available) == 0 {
			m.mode = modeImportEmpty
			return m, nil
		}
		m.selectedGroups = map[string]bool{}
		for _, group := range m.available {
			m.selectedGroups[group.ID] = true
		}
		m.groupCursor = 0
		m.mode = modeImportGroups
		return m, nil
	case modeImportGroups:
		ids := make([]string, 0, len(m.available))
		for _, group := range m.available {
			if m.selectedGroups[group.ID] {
				ids = append(ids, group.ID)
			}
		}
		if len(ids) == 0 {
			return m.finishCreate(nil)
		}
		intent := &profileimport.Intent{Source: m.selectedSource, GroupIDs: ids}
		return m.finishCreate(intent)
	case modeImportEmpty:
		return m.finishCreate(nil)
	}
	return m, nil
}

func (m model) finishCreate(intent *profileimport.Intent) (tea.Model, tea.Cmd) {
	m.importIntent = intent
	return m, tea.Quit
}

func (m model) View() string {
	switch m.mode {
	case modeCreate:
		lines := []string{"", accentStyle.Render("  Create a Profile"), mutedStyle.Render("  Allowed: lowercase, numbers, dot, dash, underscore"), "", "  " + m.input.View(), "", mutedStyle.Render("  enter continue • esc/ctrl+c cancel")}
		if m.error != "" {
			lines = append(lines, errorStyle.Render("  "+m.error))
		}
		return renderProfileBox(strings.TrimSpace(m.input.Value()), m.agent, lines)
	case modeImportSource:
		return m.viewImportSource()
	case modeImportGroups:
		return m.viewImportGroups()
	case modeImportEmpty:
		return m.viewImportEmpty()
	}
	items := []string{"", accentStyle.Render("  Select a Profile"), mutedStyle.Render("  Choose an isolated HOME for this project"), ""}
	for i, p := range m.profiles {
		prefix := "    "
		line := prefix + p.Name
		if i == m.cursor {
			line = selectedStyle.Render("  ▸ " + p.Name)
		}
		items = append(items, line)
	}
	items = append(items, "")
	createLine := "    ＋ Create new profile"
	if m.cursor == len(m.profiles) {
		createLine = selectedStyle.Render("  ▸ ＋ Create new profile")
	}
	items = append(items, createLine)
	items = append(items, "", mutedStyle.Render("  ↑/↓/j/k move • enter select • esc/ctrl+c cancel"))
	return renderProfileBox(m.currentProfileLabel(), m.agent, items)
}

func (m model) viewImportSource() string {
	items := []string{"", accentStyle.Render("  Import from"), mutedStyle.Render("  Choose optional source for profile files"), ""}
	labels := append([]string{"No import"}, sourceLabels(m.sources)...)
	for i, label := range labels {
		line := "    " + label
		if i == m.sourceCursor {
			line = selectedStyle.Render("  ▸ " + label)
		}
		items = append(items, truncateLine(line, 56))
	}
	items = append(items, "", mutedStyle.Render("  ↑/↓/j/k move • enter select • esc/ctrl+c cancel"))
	return renderProfileBox(m.selected, m.agent, items)
}

func (m model) viewImportGroups() string {
	items := []string{"", accentStyle.Render("  Select groups to import"), mutedStyle.Render("  Source: " + m.selectedSource.Label), ""}
	for i, group := range m.available {
		checked := "[ ]"
		if m.selectedGroups[group.ID] {
			checked = "[x]"
		}
		line := fmt.Sprintf("    %s %-8s %-12s %s", checked, group.Agent, group.Label, group.SourceRel)
		if i == m.groupCursor {
			line = selectedStyle.Render("  ▸ " + strings.TrimSpace(line))
		}
		items = append(items, truncateLine(line, 56))
	}
	items = append(items, "", mutedStyle.Render("  space toggle • enter import • a all • n none • esc cancel"))
	return renderProfileBox(m.selected, m.agent, items)
}

func (m model) viewImportEmpty() string {
	items := []string{"", accentStyle.Render("  No importable groups found"), mutedStyle.Render("  Source: " + m.selectedSource.Label), "", "  This source has no supported files or directories.", "", mutedStyle.Render("  enter create without import • esc/ctrl+c cancel")}
	return renderProfileBox(m.selected, m.agent, items)
}

func sourceLabels(sources []profileimport.Source) []string {
	labels := make([]string, len(sources))
	for i, source := range sources {
		labels[i] = source.Label + "  " + source.Path
	}
	return labels
}

func truncateLine(line string, width int) string {
	if lipgloss.Width(line) <= width {
		return line
	}
	runes := []rune(line)
	for lipgloss.Width(string(runes)+"…") > width && len(runes) > 0 {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}

func (m model) currentProfileLabel() string {
	if m.cursor >= 0 && m.cursor < len(m.profiles) {
		return m.profiles[m.cursor].Name
	}
	return ""
}

type removeModel struct {
	profiles  []config.Profile
	cursor    int
	selected  string
	cancelled bool
}

func newRemoveModel(profiles []config.Profile) removeModel {
	return removeModel{profiles: profiles}
}

func (m removeModel) Init() tea.Cmd { return nil }
func (m removeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.profiles)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.profiles) > 0 {
				m.selected = m.profiles[m.cursor].Name
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m removeModel) View() string {
	items := []string{"", accentStyle.Render("  Remove a Profile"), mutedStyle.Render("  Select a profile to delete with its folder"), ""}
	for i, p := range m.profiles {
		line := "    " + p.Name
		if i == m.cursor {
			line = selectedStyle.Render("  ▸ " + p.Name)
		}
		items = append(items, line)
	}
	items = append(items, "", mutedStyle.Render("  ↑/↓/j/k move • enter remove • esc/ctrl+c cancel"))
	return renderActionBox(items)
}

type unwrapModel struct {
	agents    []string
	cursor    int
	selected  string
	cancelled bool
}

func newUnwrapModel(agents []string) unwrapModel {
	return unwrapModel{agents: agents}
}

func (m unwrapModel) Init() tea.Cmd { return nil }
func (m unwrapModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.cancelled = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.agents)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.agents) > 0 {
				m.selected = m.agents[m.cursor]
			}
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m unwrapModel) View() string {
	items := []string{"", accentStyle.Render("  Unwrap an Agent"), mutedStyle.Render("  Select a wrapper binary to delete"), ""}
	for i, agent := range m.agents {
		line := "    " + agent
		if i == m.cursor {
			line = selectedStyle.Render("  ▸ " + agent)
		}
		items = append(items, line)
	}
	items = append(items, "", mutedStyle.Render("  ↑/↓/j/k move • enter unwrap • esc/ctrl+c cancel"))
	return renderActionBox(items)
}

func renderActionBox(lines []string) string {
	const width = 58
	var b strings.Builder
	b.WriteString(borderStyle.Render("╭─ ") + accentStyle.Render("agentenv") + borderStyle.Render(" ───────────────────────────────────────────────╮") + "\n")
	for _, line := range lines {
		b.WriteString(profileLine(width, line))
	}
	b.WriteString(borderStyle.Render("╰──────────────────────────────────────────────────────────╯"))
	return b.String()
}

func renderProfileBox(profile, agent string, lines []string) string {
	const width = 58
	var b strings.Builder
	b.WriteString(borderStyle.Render("╭─ ") + accentStyle.Render("agentenv") + borderStyle.Render(" ───────────────────────────────────────────────╮") + "\n")
	b.WriteString(profileLine(width, fmt.Sprintf(" %s • %s", profile, agent)))
	b.WriteString(borderStyle.Render("├──────────────────────────────────────────────────────────┤") + "\n")
	for _, line := range lines {
		b.WriteString(profileLine(width, line))
	}
	b.WriteString(borderStyle.Render("╰──────────────────────────────────────────────────────────╯"))
	return b.String()
}

func profileLine(width int, line string) string {
	if pad := width - lipgloss.Width(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return borderStyle.Render("│") + line + borderStyle.Render("│") + "\n"
}

func Banner(profile, agent string) string {
	const width = 46
	text := fmt.Sprintf("%s • %s", profile, agent)
	line := " " + text
	if pad := width - lipgloss.Width(line); pad > 0 {
		line += strings.Repeat(" ", pad)
	}
	return borderStyle.Render("┌─ ") + accentStyle.Render("agentenv") + borderStyle.Render(" ───────────────────────────────────┐") + "\n" +
		borderStyle.Render("│") + line + borderStyle.Render("│") + "\n" +
		borderStyle.Render("└──────────────────────────────────────────────┘")
}
