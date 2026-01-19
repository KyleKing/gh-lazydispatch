package modal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// LogsViewerModal displays workflow logs with filtering and search.
type LogsViewerModal struct {
	runLogs     *logs.RunLogs
	filtered    *logs.FilteredResult
	filter      *logs.Filter
	filterCfg   *logs.FilterConfig
	viewport    viewport.Model
	searchInput textinput.Model
	activeTab   int // current step being viewed
	searchMode  bool
	done        bool
	keys        logsViewerKeyMap
	width       int
	height      int
}

type logsViewerKeyMap struct {
	Close        key.Binding
	NextTab      key.Binding
	PrevTab      key.Binding
	Search       key.Binding
	ToggleFilter key.Binding
	NextMatch    key.Binding
	PrevMatch    key.Binding
	ExitSearch   key.Binding
}

func defaultLogsViewerKeyMap() logsViewerKeyMap {
	return logsViewerKeyMap{
		Close:        key.NewBinding(key.WithKeys("esc", "q")),
		NextTab:      key.NewBinding(key.WithKeys("tab", "l", "right")),
		PrevTab:      key.NewBinding(key.WithKeys("shift+tab", "h", "left")),
		Search:       key.NewBinding(key.WithKeys("/")),
		ToggleFilter: key.NewBinding(key.WithKeys("f")),
		NextMatch:    key.NewBinding(key.WithKeys("n")),
		PrevMatch:    key.NewBinding(key.WithKeys("N")),
		ExitSearch:   key.NewBinding(key.WithKeys("esc")),
	}
}

// NewLogsViewerModal creates a new logs viewer modal.
func NewLogsViewerModal(runLogs *logs.RunLogs, width, height int) *LogsViewerModal {
	filterCfg := logs.NewFilterConfig()
	filter, _ := logs.NewFilter(filterCfg)
	filtered := filter.Apply(runLogs)

	vp := viewport.New(width-4, height-10)
	vp.SetContent("")

	searchInput := textinput.New()
	searchInput.Placeholder = "Search logs..."
	searchInput.CharLimit = 100

	m := &LogsViewerModal{
		runLogs:     runLogs,
		filtered:    filtered,
		filter:      filter,
		filterCfg:   filterCfg,
		viewport:    vp,
		searchInput: searchInput,
		activeTab:   0,
		searchMode:  false,
		keys:        defaultLogsViewerKeyMap(),
		width:       width,
		height:      height,
	}

	m.updateViewportContent()
	return m
}

// NewLogsViewerModalWithError creates a logs viewer pre-filtered for errors.
func NewLogsViewerModalWithError(runLogs *logs.RunLogs, width, height int) *LogsViewerModal {
	m := NewLogsViewerModal(runLogs, width, height)
	m.filterCfg.Level = logs.FilterErrors
	m.applyFilter()
	return m
}

// Update handles input for the logs viewer modal.
func (m *LogsViewerModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewport.Width = msg.Width - 4
		m.viewport.Height = msg.Height - 10
		m.updateViewportContent()

	case tea.KeyMsg:
		if m.searchMode {
			return m.handleSearchInput(msg)
		}

		switch {
		case key.Matches(msg, m.keys.Close):
			m.done = true
			return m, nil

		case key.Matches(msg, m.keys.NextTab):
			m.nextTab()
			return m, nil

		case key.Matches(msg, m.keys.PrevTab):
			m.prevTab()
			return m, nil

		case key.Matches(msg, m.keys.Search):
			m.searchMode = true
			m.searchInput.Focus()
			return m, textinput.Blink

		case key.Matches(msg, m.keys.ToggleFilter):
			m.cycleFilterLevel()
			return m, nil

		case key.Matches(msg, m.keys.NextMatch):
			m.jumpToNextMatch()
			return m, nil

		case key.Matches(msg, m.keys.PrevMatch):
			m.jumpToPrevMatch()
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// handleSearchInput processes input when in search mode.
func (m *LogsViewerModal) handleSearchInput(msg tea.KeyMsg) (Context, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.ExitSearch):
		m.searchMode = false
		m.searchInput.Blur()
		return m, nil

	case msg.Type == tea.KeyEnter:
		m.filterCfg.SearchTerm = m.searchInput.Value()
		m.applyFilter()
		m.searchMode = false
		m.searchInput.Blur()
		return m, nil
	}

	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

// nextTab moves to the next step tab.
func (m *LogsViewerModal) nextTab() {
	if len(m.filtered.Steps) == 0 {
		return
	}
	m.activeTab = (m.activeTab + 1) % len(m.filtered.Steps)
	m.updateViewportContent()
}

// prevTab moves to the previous step tab.
func (m *LogsViewerModal) prevTab() {
	if len(m.filtered.Steps) == 0 {
		return
	}
	m.activeTab = (m.activeTab - 1 + len(m.filtered.Steps)) % len(m.filtered.Steps)
	m.updateViewportContent()
}

// cycleFilterLevel cycles through filter levels: all -> errors -> warnings -> all.
func (m *LogsViewerModal) cycleFilterLevel() {
	switch m.filterCfg.Level {
	case logs.FilterAll:
		m.filterCfg.Level = logs.FilterErrors
	case logs.FilterErrors:
		m.filterCfg.Level = logs.FilterWarnings
	case logs.FilterWarnings:
		m.filterCfg.Level = logs.FilterAll
	}
	m.applyFilter()
}

// applyFilter reapplies the current filter configuration.
func (m *LogsViewerModal) applyFilter() {
	filter, err := logs.NewFilter(m.filterCfg)
	if err != nil {
		// Keep previous filter on error
		return
	}

	m.filter = filter
	m.filtered = filter.Apply(m.runLogs)

	// Reset active tab if out of range
	if m.activeTab >= len(m.filtered.Steps) {
		m.activeTab = 0
	}

	m.updateViewportContent()
}

// jumpToNextMatch scrolls to the next search match.
func (m *LogsViewerModal) jumpToNextMatch() {
	// TODO: Implement match navigation
}

// jumpToPrevMatch scrolls to the previous search match.
func (m *LogsViewerModal) jumpToPrevMatch() {
	// TODO: Implement match navigation
}

// updateViewportContent refreshes the viewport with current filtered logs.
func (m *LogsViewerModal) updateViewportContent() {
	if len(m.filtered.Steps) == 0 {
		m.viewport.SetContent(ui.TableDimmedStyle.Render("No logs match the current filter"))
		return
	}

	if m.activeTab >= len(m.filtered.Steps) {
		m.activeTab = 0
	}

	step := m.filtered.Steps[m.activeTab]
	content := m.renderStepLogs(step)
	m.viewport.SetContent(content)
}

// renderStepLogs renders the logs for a single step with highlighting.
func (m *LogsViewerModal) renderStepLogs(step *logs.FilteredStepLogs) string {
	var sb strings.Builder

	for _, entry := range step.Entries {
		line := m.renderLogEntry(&entry)
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if sb.Len() == 0 {
		sb.WriteString(ui.TableDimmedStyle.Render("No log entries for this step"))
	}

	return sb.String()
}

// renderLogEntry renders a single log entry with highlighting.
func (m *LogsViewerModal) renderLogEntry(entry *logs.FilteredLogEntry) string {
	// Apply level-based styling
	style := m.getLogLevelStyle(entry.Original.Level)

	content := entry.Original.Content

	// Highlight matches if present
	if len(entry.Matches) > 0 {
		content = m.highlightMatches(content, entry.Matches)
	}

	return style.Render(content)
}

// getLogLevelStyle returns the style for a log level.
func (m *LogsViewerModal) getLogLevelStyle(level logs.LogLevel) lipgloss.Style {
	switch level {
	case logs.LogLevelError:
		return ui.ErrorStyle
	case logs.LogLevelWarning:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	case logs.LogLevelDebug:
		return ui.TableDimmedStyle
	default:
		return lipgloss.NewStyle()
	}
}

// highlightMatches applies highlighting to matched portions of text.
func (m *LogsViewerModal) highlightMatches(content string, matches []logs.MatchPosition) string {
	if len(matches) == 0 {
		return content
	}

	highlightStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("220")).
		Foreground(lipgloss.Color("0")).
		Bold(true)

	var result strings.Builder
	lastEnd := 0

	for _, match := range matches {
		// Add text before match
		if match.Start > lastEnd {
			result.WriteString(content[lastEnd:match.Start])
		}

		// Add highlighted match
		result.WriteString(highlightStyle.Render(content[match.Start:match.End]))
		lastEnd = match.End
	}

	// Add remaining text
	if lastEnd < len(content) {
		result.WriteString(content[lastEnd:])
	}

	return result.String()
}

// View renders the logs viewer modal.
func (m *LogsViewerModal) View() string {
	var s strings.Builder

	// Title
	title := fmt.Sprintf("Logs: %s", m.runLogs.ChainName)
	if m.runLogs.Branch != "" {
		title += fmt.Sprintf(" (%s)", m.runLogs.Branch)
	}
	s.WriteString(ui.TitleStyle.Render(title))
	s.WriteString("\n\n")

	// Tabs (step names)
	s.WriteString(m.renderTabs())
	s.WriteString("\n\n")

	// Filter status
	s.WriteString(m.renderFilterStatus())
	s.WriteString("\n")

	// Search input (if active)
	if m.searchMode {
		s.WriteString(ui.SubtitleStyle.Render("Search: "))
		s.WriteString(m.searchInput.View())
		s.WriteString("\n\n")
	}

	// Viewport with logs
	s.WriteString(m.viewport.View())
	s.WriteString("\n\n")

	// Help
	s.WriteString(m.renderHelp())

	return s.String()
}

// renderTabs renders the step tabs.
func (m *LogsViewerModal) renderTabs() string {
	if len(m.filtered.Steps) == 0 {
		return ui.TableDimmedStyle.Render("No steps available")
	}

	activeStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 2).
		Bold(true)

	inactiveStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("235")).
		Foreground(lipgloss.Color("250")).
		Padding(0, 2)

	var tabs []string
	for i, step := range m.filtered.Steps {
		label := fmt.Sprintf("%d: %s", step.StepIndex+1, step.StepName)
		if i == m.activeTab {
			tabs = append(tabs, activeStyle.Render(label))
		} else {
			tabs = append(tabs, inactiveStyle.Render(label))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Top, tabs...)
}

// renderFilterStatus shows current filter settings.
func (m *LogsViewerModal) renderFilterStatus() string {
	var parts []string

	// Filter level
	filterLabel := fmt.Sprintf("Filter: %s", m.filterCfg.Level)
	parts = append(parts, ui.SubtitleStyle.Render(filterLabel))

	// Search term
	if m.filterCfg.SearchTerm != "" {
		searchLabel := fmt.Sprintf("Search: %q", m.filterCfg.SearchTerm)
		parts = append(parts, ui.TableDimmedStyle.Render(searchLabel))
	}

	// Result count
	count := m.filtered.TotalEntries()
	countLabel := fmt.Sprintf("%d entries", count)
	parts = append(parts, ui.TableDimmedStyle.Render(countLabel))

	return strings.Join(parts, "  ")
}

// renderHelp renders help text.
func (m *LogsViewerModal) renderHelp() string {
	if m.searchMode {
		return ui.HelpStyle.Render("[enter] apply  [esc] cancel")
	}

	return ui.HelpStyle.Render(
		"[←→/tab] switch step  [f] filter  [/] search  [↑↓] scroll  [q] close",
	)
}

// IsDone returns true if the modal is finished.
func (m *LogsViewerModal) IsDone() bool {
	return m.done
}

// Result returns nil.
func (m *LogsViewerModal) Result() any {
	return nil
}
