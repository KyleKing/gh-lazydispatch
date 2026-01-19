package modal

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// LogsViewerModal displays workflow logs in a unified view with collapsible sections.
type LogsViewerModal struct {
	runLogs        *logs.RunLogs
	filtered       *logs.FilteredResult
	filter         *logs.Filter
	filterCfg      *logs.FilterConfig
	viewport       viewport.Model
	searchInput    textinput.Model
	collapsedSteps map[int]bool // track which steps are collapsed
	searchMode     bool
	done           bool
	keys           logsViewerKeyMap
	width          int
	height         int
	startTime      time.Time // for calculating relative timestamps
}

type logsViewerKeyMap struct {
	Close        key.Binding
	Search       key.Binding
	ToggleFilter key.Binding
	NextMatch    key.Binding
	PrevMatch    key.Binding
	ExitSearch   key.Binding
	ToggleStep   key.Binding
	ExpandAll    key.Binding
	CollapseAll  key.Binding
}

func defaultLogsViewerKeyMap() logsViewerKeyMap {
	return logsViewerKeyMap{
		Close:        key.NewBinding(key.WithKeys("esc", "q")),
		Search:       key.NewBinding(key.WithKeys("/")),
		ToggleFilter: key.NewBinding(key.WithKeys("f")),
		NextMatch:    key.NewBinding(key.WithKeys("n")),
		PrevMatch:    key.NewBinding(key.WithKeys("N")),
		ExitSearch:   key.NewBinding(key.WithKeys("esc")),
		ToggleStep:   key.NewBinding(key.WithKeys("enter", "space")),
		ExpandAll:    key.NewBinding(key.WithKeys("E")),
		CollapseAll:  key.NewBinding(key.WithKeys("C")),
	}
}

// NewLogsViewerModal creates a new unified logs viewer modal.
func NewLogsViewerModal(runLogs *logs.RunLogs, width, height int) *LogsViewerModal {
	filterCfg := logs.NewFilterConfig()
	filter, _ := logs.NewFilter(filterCfg)
	filtered := filter.Apply(runLogs)

	vp := viewport.New(width-4, height-10)
	vp.SetContent("")

	searchInput := textinput.New()
	searchInput.Placeholder = "Search logs..."
	searchInput.CharLimit = 100

	// Find earliest timestamp to use as start time
	startTime := time.Now()
	for _, step := range runLogs.AllSteps() {
		for _, entry := range step.Entries {
			if entry.Timestamp.Before(startTime) {
				startTime = entry.Timestamp
			}
		}
	}

	m := &LogsViewerModal{
		runLogs:        runLogs,
		filtered:       filtered,
		filter:         filter,
		filterCfg:      filterCfg,
		viewport:       vp,
		searchInput:    searchInput,
		collapsedSteps: make(map[int]bool),
		searchMode:     false,
		keys:           defaultLogsViewerKeyMap(),
		width:          width,
		height:         height,
		startTime:      startTime,
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

		case key.Matches(msg, m.keys.ToggleStep):
			m.toggleStepAtCursor()
			return m, nil

		case key.Matches(msg, m.keys.ExpandAll):
			m.expandAll()
			return m, nil

		case key.Matches(msg, m.keys.CollapseAll):
			m.collapseAll()
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

// toggleStepAtCursor toggles the collapsed state of the step under the cursor.
func (m *LogsViewerModal) toggleStepAtCursor() {
	// This is a simplified implementation
	// In a real implementation, track cursor position and toggle the appropriate step
	if len(m.filtered.Steps) > 0 {
		stepIdx := 0 // Would determine from cursor position
		m.collapsedSteps[stepIdx] = !m.collapsedSteps[stepIdx]
		m.updateViewportContent()
	}
}

// expandAll expands all step sections.
func (m *LogsViewerModal) expandAll() {
	m.collapsedSteps = make(map[int]bool)
	m.updateViewportContent()
}

// collapseAll collapses all step sections.
func (m *LogsViewerModal) collapseAll() {
	for i := range m.filtered.Steps {
		m.collapsedSteps[i] = true
	}
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
		return
	}

	m.filter = filter
	m.filtered = filter.Apply(m.runLogs)
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

	content := m.renderUnifiedLogs()
	m.viewport.SetContent(content)
}

// renderUnifiedLogs renders all logs in a unified view with collapsible sections.
func (m *LogsViewerModal) renderUnifiedLogs() string {
	var sb strings.Builder

	for i, step := range m.filtered.Steps {
		// Render step header
		sb.WriteString(m.renderStepHeader(i, step))
		sb.WriteString("\n")

		// Render step logs if not collapsed
		if !m.collapsedSteps[i] {
			for _, entry := range step.Entries {
				line := m.renderLogEntry(&entry)
				sb.WriteString(line)
				sb.WriteString("\n")
			}
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// renderStepHeader renders a collapsible step header.
func (m *LogsViewerModal) renderStepHeader(idx int, step *logs.FilteredStepLogs) string {
	var icon string
	if m.collapsedSteps[idx] {
		icon = "▶"
	} else {
		icon = "▼"
	}

	headerStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("62")).
		Foreground(lipgloss.Color("230")).
		Padding(0, 1).
		Bold(true)

	entryCount := len(step.Entries)
	header := fmt.Sprintf("%s Step %d: %s (%d entries)",
		icon, step.StepIndex+1, step.StepName, entryCount)

	return headerStyle.Render(header)
}

// renderLogEntry renders a single log entry with highlighting.
func (m *LogsViewerModal) renderLogEntry(entry *logs.FilteredLogEntry) string {
	// Calculate time since start
	timeSinceStart := entry.Original.Timestamp.Sub(m.startTime)

	// Format: [+00:05:23] [12:34:56] log content
	timePrefix := fmt.Sprintf("[+%s] [%s] ",
		formatDuration(timeSinceStart),
		entry.Original.Timestamp.Format("15:04:05"))

	// Style the time prefix
	timeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("243")). // Dimmed
		Italic(true)

	styledTimePrefix := timeStyle.Render(timePrefix)

	// Apply level-based styling to content
	contentStyle := m.getLogLevelStyle(entry.Original.Level)
	content := entry.Original.Content

	// Highlight matches if present
	if len(entry.Matches) > 0 {
		content = m.highlightMatches(content, entry.Matches)
	}

	return styledTimePrefix + contentStyle.Render(content)
}

// formatDuration formats a duration as HH:MM:SS.
func formatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d:%02d", hours, minutes, seconds)
}

// getLogLevelStyle returns the style for a log level.
func (m *LogsViewerModal) getLogLevelStyle(level logs.LogLevel) lipgloss.Style {
	switch level {
	case logs.LogLevelError:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("203")) // Red
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
		"[enter/space] toggle section  [E] expand all  [C] collapse all  [f] filter  [/] search  [↑↓] scroll  [q] close",
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
