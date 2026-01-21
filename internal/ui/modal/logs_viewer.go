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

// MatchLocation tracks the position of a search match in the rendered viewport.
type MatchLocation struct {
	StepIndex  int // index in filtered.Steps
	EntryIndex int // index in step.Entries
	LineNumber int // line number in viewport content (0-based)
}

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
	startTime      time.Time      // for calculating relative timestamps
	matches        []MatchLocation // all match positions in rendered content
	currentMatch   int             // index of current match (-1 if none)
	isStreaming    bool
	autoScroll     bool
	streamRunID    int64
	liveStatus     string
	lastUpdateTime time.Time
}

type logsViewerKeyMap struct {
	Close               key.Binding
	Search              key.Binding
	ToggleFilter        key.Binding
	NextMatch           key.Binding
	PrevMatch           key.Binding
	ExitSearch          key.Binding
	ToggleStep          key.Binding
	ExpandAll           key.Binding
	CollapseAll         key.Binding
	QuickFilterAll      key.Binding
	QuickFilterWarnings key.Binding
	QuickFilterErrors   key.Binding
	ToggleCaseSensitive key.Binding
	ToggleAutoScroll    key.Binding
}

func defaultLogsViewerKeyMap() logsViewerKeyMap {
	return logsViewerKeyMap{
		Close:               key.NewBinding(key.WithKeys("esc", "q")),
		Search:              key.NewBinding(key.WithKeys("/")),
		ToggleFilter:        key.NewBinding(key.WithKeys("f")),
		NextMatch:           key.NewBinding(key.WithKeys("n")),
		PrevMatch:           key.NewBinding(key.WithKeys("N")),
		ExitSearch:          key.NewBinding(key.WithKeys("esc")),
		ToggleStep:          key.NewBinding(key.WithKeys("enter", "space")),
		ExpandAll:           key.NewBinding(key.WithKeys("E")),
		CollapseAll:         key.NewBinding(key.WithKeys("C")),
		QuickFilterAll:      key.NewBinding(key.WithKeys("a")),
		QuickFilterWarnings: key.NewBinding(key.WithKeys("w")),
		QuickFilterErrors:   key.NewBinding(key.WithKeys("e")),
		ToggleCaseSensitive: key.NewBinding(key.WithKeys("i")),
		ToggleAutoScroll:    key.NewBinding(key.WithKeys("s")),
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
		matches:        []MatchLocation{},
		currentMatch:   -1,
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

		case key.Matches(msg, m.keys.QuickFilterAll):
			m.filterCfg.Level = logs.FilterAll
			m.applyFilter()
			return m, nil

		case key.Matches(msg, m.keys.QuickFilterWarnings):
			m.filterCfg.Level = logs.FilterWarnings
			m.applyFilter()
			return m, nil

		case key.Matches(msg, m.keys.QuickFilterErrors):
			m.filterCfg.Level = logs.FilterErrors
			m.applyFilter()
			return m, nil

		case key.Matches(msg, m.keys.ToggleCaseSensitive):
			m.filterCfg.CaseSensitive = !m.filterCfg.CaseSensitive
			if m.filterCfg.SearchTerm != "" {
				m.applyFilter()
			}
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

		case key.Matches(msg, m.keys.ToggleAutoScroll):
			m.toggleAutoScroll()
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
	m.buildMatchIndex()
	m.currentMatch = -1
	m.updateViewportContent()
}

// buildMatchIndex builds a list of all match locations in the filtered results.
func (m *LogsViewerModal) buildMatchIndex() {
	m.matches = []MatchLocation{}

	if m.filterCfg.SearchTerm == "" {
		return
	}

	lineNumber := 0
	for stepIdx, step := range m.filtered.Steps {
		// Step header line
		lineNumber++

		// Only count entry lines if step is not collapsed
		if !m.collapsedSteps[stepIdx] {
			for entryIdx, entry := range step.Entries {
				if len(entry.Matches) > 0 {
					m.matches = append(m.matches, MatchLocation{
						StepIndex:  stepIdx,
						EntryIndex: entryIdx,
						LineNumber: lineNumber,
					})
				}
				lineNumber++
			}
		}

		// Empty line after step
		lineNumber++
	}
}

// jumpToNextMatch scrolls to the next search match.
func (m *LogsViewerModal) jumpToNextMatch() {
	if len(m.matches) == 0 {
		return
	}

	m.currentMatch = (m.currentMatch + 1) % len(m.matches)
	m.scrollToMatch(m.currentMatch)
}

// jumpToPrevMatch scrolls to the previous search match.
func (m *LogsViewerModal) jumpToPrevMatch() {
	if len(m.matches) == 0 {
		return
	}

	if m.currentMatch == -1 {
		m.currentMatch = len(m.matches) - 1
	} else {
		m.currentMatch = (m.currentMatch - 1 + len(m.matches)) % len(m.matches)
	}
	m.scrollToMatch(m.currentMatch)
}

// scrollToMatch scrolls the viewport to show the specified match.
func (m *LogsViewerModal) scrollToMatch(matchIdx int) {
	if matchIdx < 0 || matchIdx >= len(m.matches) {
		return
	}

	match := m.matches[matchIdx]

	// Center the match in the viewport if possible
	targetLine := match.LineNumber
	visibleLines := m.viewport.Height
	centerOffset := targetLine - visibleLines/2

	// Ensure we don't scroll past the beginning
	if centerOffset < 0 {
		centerOffset = 0
	}

	// Ensure we don't scroll past the end
	totalLines := m.viewport.TotalLineCount()
	if centerOffset+visibleLines > totalLines {
		centerOffset = totalLines - visibleLines
		if centerOffset < 0 {
			centerOffset = 0
		}
	}

	m.viewport.SetYOffset(centerOffset)
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
			for j, entry := range step.Entries {
				line := m.renderLogEntry(&entry, i, j)
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
func (m *LogsViewerModal) renderLogEntry(entry *logs.FilteredLogEntry, stepIdx, entryIdx int) string {
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

	// Check if this is the current match
	isCurrentMatch := false
	if m.currentMatch >= 0 && m.currentMatch < len(m.matches) {
		match := m.matches[m.currentMatch]
		if match.StepIndex == stepIdx && match.EntryIndex == entryIdx {
			isCurrentMatch = true
		}
	}

	// Apply level-based styling to content
	contentStyle := m.getLogLevelStyle(entry.Original.Level)
	content := entry.Original.Content

	// Highlight matches if present
	if len(entry.Matches) > 0 {
		content = m.highlightMatches(content, entry.Matches, isCurrentMatch)
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
func (m *LogsViewerModal) highlightMatches(content string, matches []logs.MatchPosition, isCurrentMatch bool) string {
	if len(matches) == 0 {
		return content
	}

	// Regular match: yellow background, black text
	regularStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("220")).
		Foreground(lipgloss.Color("0")).
		Bold(true)

	// Current match: cyan background, black text, more prominent
	currentStyle := lipgloss.NewStyle().
		Background(lipgloss.Color("51")).
		Foreground(lipgloss.Color("0")).
		Bold(true)

	highlightStyle := regularStyle
	if isCurrentMatch {
		highlightStyle = currentStyle
	}

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

	// Live indicator (if streaming)
	if m.isStreaming {
		indicator := m.renderLiveIndicator()
		parts = append(parts, indicator)
	}

	// Filter level with descriptive label
	var filterLabel string
	switch m.filterCfg.Level {
	case logs.FilterErrors:
		filterLabel = "Filter: errors only"
	case logs.FilterWarnings:
		filterLabel = "Filter: warnings + errors"
	case logs.FilterAll:
		filterLabel = "Filter: all logs"
	default:
		filterLabel = fmt.Sprintf("Filter: %s", m.filterCfg.Level)
	}
	parts = append(parts, ui.SubtitleStyle.Render(filterLabel))

	// Search term with case sensitivity indicator
	if m.filterCfg.SearchTerm != "" {
		caseIndicator := "[aa]"
		if m.filterCfg.CaseSensitive {
			caseIndicator = "[Aa]"
		}
		searchLabel := fmt.Sprintf("Search: %q %s", m.filterCfg.SearchTerm, caseIndicator)
		parts = append(parts, ui.TableDimmedStyle.Render(searchLabel))

		// Match count and position
		if len(m.matches) > 0 && m.currentMatch >= 0 {
			matchLabel := fmt.Sprintf("Match %d of %d", m.currentMatch+1, len(m.matches))
			parts = append(parts, ui.TitleStyle.Render(matchLabel))
		} else if len(m.matches) == 0 {
			parts = append(parts, ui.TableDimmedStyle.Render("No matches"))
		}
	}

	// Result count
	count := m.filtered.TotalEntries()
	countLabel := fmt.Sprintf("%d entries", count)
	parts = append(parts, ui.TableDimmedStyle.Render(countLabel))

	return strings.Join(parts, "  ")
}

// renderLiveIndicator renders the live streaming status badge.
func (m *LogsViewerModal) renderLiveIndicator() string {
	var indicator string
	var style lipgloss.Style

	switch m.liveStatus {
	case "in_progress":
		indicator = "[LIVE]"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true) // Red
	case "queued":
		indicator = "[QUEUED]"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true) // Yellow
	case "completed":
		indicator = "[COMPLETED]"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("120")).Bold(true) // Green
	default:
		indicator = "[LIVE]"
		style = lipgloss.NewStyle().Foreground(lipgloss.Color("203")).Bold(true)
	}

	return style.Render(indicator)
}

// renderHelp renders help text.
func (m *LogsViewerModal) renderHelp() string {
	if m.searchMode {
		return ui.HelpStyle.Render("[enter] apply  [esc] cancel")
	}

	helpParts := []string{
		"[a] all",
		"[w] warnings",
		"[e] errors",
		"[/] search",
		"[i] case",
	}

	if len(m.matches) > 0 {
		helpParts = append(helpParts, "[n/N] next/prev match")
	}

	helpParts = append(helpParts,
		"[enter/space] toggle section",
		"[E] expand all",
		"[C] collapse all",
		"[↑↓] scroll",
	)

	if m.isStreaming {
		autoScrollStatus := "off"
		if m.autoScroll {
			autoScrollStatus = "on"
		}
		helpParts = append(helpParts, fmt.Sprintf("[s] auto-scroll: %s", autoScrollStatus))
	}

	helpParts = append(helpParts, "[q] close")

	return ui.HelpStyle.Render(strings.Join(helpParts, "  "))
}

// EnableStreaming enables streaming mode for this viewer.
func (m *LogsViewerModal) EnableStreaming(runID int64, autoScroll bool) {
	m.isStreaming = true
	m.streamRunID = runID
	m.autoScroll = autoScroll
	m.liveStatus = "in_progress"
	m.lastUpdateTime = time.Now()
}

// DisableStreaming disables streaming mode.
func (m *LogsViewerModal) DisableStreaming() {
	m.isStreaming = false
}

// IsStreaming returns whether streaming is active.
func (m *LogsViewerModal) IsStreaming() bool {
	return m.isStreaming
}

// StreamRunID returns the run ID being streamed.
func (m *LogsViewerModal) StreamRunID() int64 {
	return m.streamRunID
}

// AppendStreamUpdate appends new log entries from streaming.
func (m *LogsViewerModal) AppendStreamUpdate(update logs.StreamUpdate) {
	if update.Error != nil {
		return
	}

	// Update status
	if update.Status != "" {
		m.liveStatus = update.Status
	}

	// If run completed, disable streaming
	if update.Status == "completed" {
		m.isStreaming = false
	}

	// Append new steps to runLogs
	for _, newStep := range update.NewSteps {
		// Find existing step by index or append new one
		existingStep := m.runLogs.GetStep(newStep.StepIndex)
		if existingStep != nil {
			// Append new entries to existing step
			existingStep.Entries = append(existingStep.Entries, newStep.Entries...)
			existingStep.Status = newStep.Status
			existingStep.Conclusion = newStep.Conclusion
		} else {
			// Add as new step
			m.runLogs.AddStep(newStep)
		}
	}

	m.lastUpdateTime = time.Now()

	// Re-apply filter and update viewport
	m.applyFilter()

	// Auto-scroll to bottom if enabled and user is at bottom
	if m.shouldAutoScroll() {
		m.scrollToBottom()
	}
}

// toggleAutoScroll toggles the auto-scroll feature.
func (m *LogsViewerModal) toggleAutoScroll() {
	m.autoScroll = !m.autoScroll
}

// shouldAutoScroll returns true if we should auto-scroll on new content.
func (m *LogsViewerModal) shouldAutoScroll() bool {
	if !m.autoScroll {
		return false
	}

	// Only scroll if user is within 3 lines of bottom
	totalLines := m.viewport.TotalLineCount()
	currentOffset := m.viewport.YOffset
	visibleLines := m.viewport.Height
	bottomLine := currentOffset + visibleLines

	return totalLines-bottomLine <= 3
}

// scrollToBottom scrolls the viewport to the bottom.
func (m *LogsViewerModal) scrollToBottom() {
	totalLines := m.viewport.TotalLineCount()
	visibleLines := m.viewport.Height
	if totalLines > visibleLines {
		m.viewport.SetYOffset(totalLines - visibleLines)
	}
}

// IsDone returns true if the modal is finished.
func (m *LogsViewerModal) IsDone() bool {
	return m.done
}

// Result returns nil.
func (m *LogsViewerModal) Result() any {
	return nil
}
