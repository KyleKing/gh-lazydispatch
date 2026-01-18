package panes

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/lazydispatch/internal/frecency"
	"github.com/kyleking/lazydispatch/internal/ui"
)

// HistoryItem represents a history entry in the list.
type HistoryItem struct {
	entry frecency.HistoryEntry
}

func (i HistoryItem) Title() string {
	return i.entry.Branch
}

func (i HistoryItem) Description() string {
	parts := make([]string, 0, len(i.entry.Inputs))
	for k, v := range i.entry.Inputs {
		if v != "" {
			parts = append(parts, k+"="+v)
		}
	}
	desc := strings.Join(parts, ", ")
	if desc != "" {
		desc += " | "
	}
	desc += formatTimeAgo(i.entry.LastRunAt)
	return desc
}

func (i HistoryItem) FilterValue() string {
	return i.entry.Branch + " " + i.entry.Workflow
}

func (i HistoryItem) Entry() frecency.HistoryEntry {
	return i.entry
}

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
	list           list.Model
	focused        bool
	width          int
	height         int
	workflowFilter string
}

// NewHistoryModel creates a new history pane model.
func NewHistoryModel() HistoryModel {
	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = ui.SelectedStyle
	delegate.Styles.SelectedDesc = ui.SubtitleStyle
	delegate.Styles.NormalTitle = ui.NormalStyle
	delegate.Styles.NormalDesc = ui.SubtitleStyle

	l := list.New([]list.Item{}, delegate, 0, 0)
	l.Title = "Recent Runs"
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.Styles.Title = ui.TitleStyle

	return HistoryModel{list: l}
}

// SetEntries updates the history entries.
func (m *HistoryModel) SetEntries(entries []frecency.HistoryEntry, workflowFilter string) {
	items := make([]list.Item, len(entries))
	for i, e := range entries {
		items[i] = HistoryItem{entry: e}
	}
	m.list.SetItems(items)
	m.workflowFilter = workflowFilter
	if workflowFilter != "" {
		m.list.Title = "Recent Runs (" + workflowFilter + ")"
	} else {
		m.list.Title = "Recent Runs"
	}
}

// SetSize updates the pane dimensions.
func (m *HistoryModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width-4, height-4)
}

// SetFocused updates the focus state.
func (m *HistoryModel) SetFocused(focused bool) {
	m.focused = focused
}

// Update handles messages for the history pane.
func (m HistoryModel) Update(msg tea.Msg) (HistoryModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

// View renders the history pane.
func (m HistoryModel) View() string {
	style := ui.PaneStyle(m.width, m.height, m.focused)
	return style.Render(m.list.View())
}

// SelectedEntry returns the currently selected history entry.
func (m HistoryModel) SelectedEntry() *frecency.HistoryEntry {
	item := m.list.SelectedItem()
	if item == nil {
		return nil
	}
	hi, ok := item.(HistoryItem)
	if !ok {
		return nil
	}
	entry := hi.Entry()
	return &entry
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
