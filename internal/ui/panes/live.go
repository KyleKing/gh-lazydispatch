package panes

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// liveGutter is the selection indicator, the mark cell, and the status glyph.
const liveGutter = ui.RowGutterWidth + 3

// liveColumns describes the live-runs table.
func liveColumns() []table.Column {
	return []table.Column{
		{
			Key: ui.ColKeyWorkflow, Title: ui.ColTitleWorkflow, Min: ui.ColMinName, Max: ui.ColMaxName,
			Weight: ui.WeightHigh,
		},
		{Key: "status", Title: "Status", Min: ui.ColMinLabel, Max: ui.ColMaxStatus, Weight: ui.WeightLow},
	}
}

const statusUnknown = "unknown"

// LiveRunsModel manages the live runs display.
type LiveRunsModel struct {
	marks         ui.MarkSet
	runs          []watcher.WatchedRun
	selectedIndex int
	width         int
	height        int
	focused       bool
}

// NewLiveRunsModel creates a new live runs model.
func NewLiveRunsModel() LiveRunsModel {
	return LiveRunsModel{selectedIndex: 0}
}

// SetRuns updates the list of watched runs.
func (m *LiveRunsModel) SetRuns(runs []watcher.WatchedRun) {
	m.runs = runs
	if m.selectedIndex >= len(runs) && len(runs) > 0 {
		m.selectedIndex = len(runs) - 1
	}
}

// SetSize updates the pane dimensions.
func (m *LiveRunsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused updates the focus state.
func (m *LiveRunsModel) SetFocused(focused bool) {
	m.focused = focused
}

// MoveUp moves selection up.
func (m *LiveRunsModel) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

// MoveDown moves selection down.
func (m *LiveRunsModel) MoveDown() {
	if m.selectedIndex < len(m.runs)-1 {
		m.selectedIndex++
	}
}

// ToggleMark marks or unmarks the run under the cursor, which is what makes a
// verb act on a set rather than on one row.
func (m *LiveRunsModel) ToggleMark() {
	if run, ok := m.SelectedRun(); ok {
		m.marks.Toggle(strconv.FormatInt(run.RunID, 10))
	}
}

// MarkedRuns are the run IDs marked, or the selection when nothing is marked:
// a verb always has something to act on.
func (m LiveRunsModel) MarkedRuns() []int64 {
	if m.marks.Len() == 0 {
		if run, ok := m.SelectedRun(); ok {
			return []int64{run.RunID}
		}

		return nil
	}

	ids := make([]int64, 0, m.marks.Len())

	for _, key := range m.marks.Keys() {
		if id, err := strconv.ParseInt(key, 10, 64); err == nil {
			ids = append(ids, id)
		}
	}

	return ids
}

// MarkCount is how many runs are marked.
func (m LiveRunsModel) MarkCount() int { return m.marks.Len() }

// ClearMarks drops every mark, which finishing a batch verb does.
func (m *LiveRunsModel) ClearMarks() { m.marks.Clear() }

// SelectedRun returns the currently selected run.
func (m LiveRunsModel) SelectedRun() (watcher.WatchedRun, bool) {
	if len(m.runs) == 0 || m.selectedIndex >= len(m.runs) {
		return watcher.WatchedRun{}, false
	}

	return m.runs[m.selectedIndex], true
}

// SelectedIndex returns the current selection index.
func (m LiveRunsModel) SelectedIndex() int {
	return m.selectedIndex
}

// RunCount returns the number of runs.
func (m LiveRunsModel) RunCount() int {
	return len(m.runs)
}

// Update handles messages for the live runs model.
func (m LiveRunsModel) Update(_ tea.Msg) (LiveRunsModel, tea.Cmd) {
	return m, nil
}

// ViewContent renders the live runs content without the pane border.
func (m LiveRunsModel) ViewContent() string {
	if len(m.runs) == 0 {
		var content strings.Builder

		content.WriteString(ui.SubtitleStyle.Render("No active runs"))
		content.WriteString("\n\n")
		content.WriteString(ui.NormalStyle.Render("Runs appear here when"))
		content.WriteString("\n")
		content.WriteString(ui.NormalStyle.Render("Watch is enabled."))
		content.WriteString("\n\n")
		content.WriteString(ui.HelpStyle.Render("Toggle with [w] in config"))

		return content.String()
	}

	var content strings.Builder

	layout := ui.FitColumns(liveColumns(), m.width, liveGutter)

	content.WriteString(ui.TableHeader(layout, liveGutter))
	content.WriteString("\n")

	first, last := ui.ScrollWindow(m.selectedIndex, len(m.runs), m.height-listPaneChrome)

	for i := first; i < last; i++ {
		run := &m.runs[i]

		icon := runStatusIcon(run.Status, run.Conclusion)

		var status string

		switch {
		case run.Status != "" && run.Status != github.StatusCompleted:
			status = run.Status
		case run.Conclusion != "":
			status = run.Conclusion
		default:
			status = statusUnknown
		}

		indicator := "  "
		if i == m.selectedIndex {
			indicator = "> "
		}

		cells := ui.TableRow(layout, map[string]string{
			ui.ColKeyWorkflow: run.Workflow,
			"status":          status,
		})

		rowStyle := ui.TableRowStyle
		if i == m.selectedIndex {
			rowStyle = ui.TableSelectedStyle
		}

		mark := ui.MarkGlyph(m.marks.Has(strconv.FormatInt(run.RunID, 10)))

		content.WriteString(rowStyle.Render(indicator + mark + icon + " " + cells))

		if i < last-1 {
			content.WriteString("\n")
		}
	}

	if first > 0 || last < len(m.runs) {
		content.WriteString("\n")
		content.WriteString(ui.RenderScrollIndicator(last < len(m.runs), first > 0))
	}

	return content.String()
}

func runStatusIcon(status, conclusion string) string {
	switch status {
	case github.StatusQueued:
		return "o"
	case github.StatusInProgress:
		return "*"
	case github.StatusCompleted:
		switch conclusion {
		case github.ConclusionSuccess:
			return "+"
		case github.ConclusionFailure:
			return "x"
		case github.ConclusionCancelled:
			return "-"
		default:
			return "?"
		}
	default:
		return "?"
	}
}

// ActiveCount returns the number of active runs.
func (m LiveRunsModel) ActiveCount() int {
	count := 0

	for i := range m.runs {
		if m.runs[i].IsActive() {
			count++
		}
	}

	return count
}
