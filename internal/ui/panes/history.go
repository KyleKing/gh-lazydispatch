package panes

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

func formatTimeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	default:
		return t.Format("Jan 2")
	}
}

// HistoryModel manages the history list pane.
type HistoryModel struct {
	entries        []frecency.HistoryEntry
	selectedIndex  int
	focused        bool
	width          int
	height         int
	workflowFilter string
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
func (m HistoryModel) Update(msg tea.Msg) (HistoryModel, tea.Cmd) {
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

	content.WriteString(ui.TableHeaderStyle.Render(
		"  Branch           Inputs                       Time"))
	content.WriteString("\n")

	for i, entry := range m.entries {
		branch := ui.TruncateWithEllipsis(entry.Branch, 15)

		inputParts := make([]string, 0, len(entry.Inputs))
		for k, v := range entry.Inputs {
			if v != "" {
				inputParts = append(inputParts, k+"="+v)
			}
		}
		inputs := strings.Join(inputParts, ", ")
		inputs = ui.TruncateWithEllipsis(inputs, 25)
		if inputs == "" {
			inputs = "(no inputs)"
		}

		timeAgo := formatTimeAgo(entry.LastRunAt)

		indicator := "  "
		if i == m.selectedIndex {
			indicator = "> "
		}

		row := indicator + ui.PadRight(branch, 15) + "  " + ui.PadRight(inputs, 25) + "  " + timeAgo

		var rowStyle = ui.TableRowStyle
		if i == m.selectedIndex {
			rowStyle = ui.TableSelectedStyle
		}

		content.WriteString(rowStyle.Render(row))
		if i < len(m.entries)-1 {
			content.WriteString("\n")
		}
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

