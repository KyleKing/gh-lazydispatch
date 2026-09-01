package panes

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// historyGutter is the selection indicator plus the workflow/chain type glyph.
const historyGutter = ui.RowGutterWidth + 2

// historyColumns describes the history table. Branch gives way before the run
// name, and the relative time is the first thing a narrow pane drops.
func historyColumns() []table.Column {
	return []table.Column{
		{Key: ui.ColKeyName, Title: ui.ColTitleName, Min: ui.ColMinName, Max: ui.ColMaxName, Weight: ui.WeightHigh},
		{
			Key: ui.ColKeyBranch, Title: ui.ColTitleBranch, Min: ui.ColMinShort, Max: ui.ColMaxBranch,
			Weight: ui.WeightMid, Priority: ui.PrioSecondToGo,
		},
		{
			Key: ui.ColKeyTime, Title: ui.ColTitleTime, Min: ui.ColMinLabel,
			Max: ui.ColMaxTime, Priority: ui.PrioFirstToGo,
		},
	}
}

// HistoryModel manages the history list pane.
type HistoryModel struct {
	workflowFilter string
	entries        []frecency.HistoryEntry
	selectedIndex  int
	width          int
	height         int
	focused        bool
}

// NewHistoryModel creates a new history pane model.
func NewHistoryModel() HistoryModel {
	return HistoryModel{selectedIndex: 0}
}

// SetEntries updates the history entries.
func (m *HistoryModel) SetEntries(entries []frecency.HistoryEntry, workflowFilter string) {
	m.entries = entries
	m.workflowFilter = workflowFilter

	if m.selectedIndex >= len(entries) && len(entries) > 0 {
		m.selectedIndex = len(entries) - 1
	}
}

// SetSize updates the pane dimensions.
func (m *HistoryModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused updates the focus state.
func (m *HistoryModel) SetFocused(focused bool) {
	m.focused = focused
}

// MoveUp moves selection up.
func (m *HistoryModel) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

// MoveDown moves selection down.
func (m *HistoryModel) MoveDown() {
	if m.selectedIndex < len(m.entries)-1 {
		m.selectedIndex++
	}
}

// Update handles messages for the history pane.
func (m HistoryModel) Update(_ tea.Msg) (HistoryModel, tea.Cmd) {
	return m, nil
}

// View renders the history pane.
func (m HistoryModel) View() string {
	style := ui.PaneStyle(m.width, m.height, m.focused)

	title := "Recent Runs"
	if m.workflowFilter != "" {
		title = "Recent Runs (" + m.workflowFilter + ")"
	}

	return style.Render(ui.TitleStyle.Render(title) + "\n" + m.ViewContent())
}

// ViewContent renders just the list content without the pane border.
func (m HistoryModel) ViewContent() string {
	if len(m.entries) == 0 {
		var content strings.Builder

		content.WriteString(ui.SubtitleStyle.Render("No recent runs"))
		content.WriteString("\n\n")
		content.WriteString(ui.NormalStyle.Render("Run a workflow to see"))
		content.WriteString("\n")
		content.WriteString(ui.NormalStyle.Render("history here."))

		return content.String()
	}

	var content strings.Builder

	layout := ui.FitColumns(historyColumns(), m.width, historyGutter)

	content.WriteString(ui.TableHeader(layout, historyGutter))
	content.WriteString("\n")

	first, last := ui.ScrollWindow(m.selectedIndex, len(m.entries), m.height-listPaneChrome)

	for i := first; i < last; i++ {
		entry := &m.entries[i]

		indicator := "  "
		if i == m.selectedIndex {
			indicator = "> "
		}

		typeIcon := "w"
		name := entry.Workflow

		if entry.Type == frecency.EntryTypeChain || entry.ChainName != "" {
			typeIcon = "c"

			name = entry.ChainName
			if len(entry.StepResults) > 0 {
				name = fmt.Sprintf("%s (%d steps)", name, len(entry.StepResults))
			}
		}

		cells := ui.TableRow(layout, map[string]string{
			ui.ColKeyName:   name,
			ui.ColKeyBranch: entry.Branch,
			ui.ColKeyTime:   display.RelativeTimeCompact(entry.LastRunAt),
		})

		rowStyle := ui.TableRowStyle
		if i == m.selectedIndex {
			rowStyle = ui.TableSelectedStyle
		}

		content.WriteString(rowStyle.Render(indicator + typeIcon + " " + cells))

		if i < last-1 {
			content.WriteString("\n")
		}
	}

	if first > 0 || last < len(m.entries) {
		content.WriteString("\n")
		content.WriteString(ui.RenderScrollIndicator(last < len(m.entries), first > 0))
	}

	return content.String()
}

// SelectedEntry returns the currently selected history entry.
func (m HistoryModel) SelectedEntry() *frecency.HistoryEntry {
	if len(m.entries) == 0 || m.selectedIndex >= len(m.entries) {
		return nil
	}

	return &m.entries[m.selectedIndex]
}

// HistorySelectedMsg is sent when a history entry is selected.
type HistorySelectedMsg struct {
	Entry frecency.HistoryEntry
}

// HandleSelect processes a selection and returns a message.
func (m HistoryModel) HandleSelect() tea.Cmd {
	entry := m.SelectedEntry()
	if entry == nil {
		return nil
	}

	return func() tea.Msg {
		return HistorySelectedMsg{Entry: *entry}
	}
}

// HistoryViewLogsMsg is sent when the user wants to view logs for a history entry.
type HistoryViewLogsMsg struct {
	Entry frecency.HistoryEntry
}

// HandleViewLogs processes a view logs request and returns a message.
func (m HistoryModel) HandleViewLogs() tea.Cmd {
	entry := m.SelectedEntry()
	if entry == nil {
		return nil
	}
	// Only allow viewing logs for chain entries that have step results
	if entry.Type != frecency.EntryTypeChain || len(entry.StepResults) == 0 {
		return nil
	}

	return func() tea.Msg {
		return HistoryViewLogsMsg{Entry: *entry}
	}
}
