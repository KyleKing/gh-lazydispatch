package panes

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// RunsScope names which of GitHub's runs the pane asks for. The branch scope
// answers "is what I am working on green"; the two pull request scopes answer
// the same question about work waiting on someone.
type RunsScope int

// Run scopes, in the order the pane cycles them.
const (
	ScopeBranch RunsScope = iota
	ScopeMine
	ScopeReviewing
)

const runsScopeCount = 3

// Label names the scope in the pane's title.
func (s RunsScope) Label() string {
	switch s {
	case ScopeMine:
		return "my PRs"
	case ScopeReviewing:
		return "awaiting my review"
	default:
		return "branch"
	}
}

// listPaneChrome is the vertical space a list in the right panel spends on its
// column header and its scroll indicator. The panel's own border and tab bar
// are already out of the height it hands down.
const listPaneChrome = 2

// runsGutter is the selection indicator plus the run status glyph.
const runsGutter = ui.RowGutterWidth + 2

// runsColumns describes the runs table. A branch-scoped listing drops the
// branch column, since every row shares it.
func runsColumns(scope RunsScope) []table.Column {
	cols := []table.Column{
		{Key: ui.ColKeyWorkflow, Title: "Workflow", Min: ui.ColMinName, Max: ui.ColMaxName, Weight: ui.WeightHigh},
	}

	if scope != ScopeBranch {
		cols = append(cols, table.Column{
			Key: ui.ColKeyBranch, Title: ui.ColTitleBranch, Min: ui.ColMinShort, Max: ui.ColMaxBranch,
			Weight: ui.WeightMid, Priority: ui.PrioSecondToGo,
		})
	}

	return append(cols, table.Column{
		Key: ui.ColKeyTime, Title: ui.ColTitleTime, Min: ui.ColMinLabel, Max: ui.ColMaxTime, Priority: ui.PrioFirstToGo,
	})
}

// RunsModel shows the current state of each workflow on GitHub, which is a
// different question from the local dispatch history the History tab keeps.
type RunsModel struct {
	err           error
	branch        string
	runs          []github.WorkflowRun
	scope         RunsScope
	selectedIndex int
	width         int
	height        int
	focused       bool
	loading       bool
	loaded        bool
}

// NewRunsModel creates an empty runs pane, which loads nothing until the tab is
// opened.
func NewRunsModel() RunsModel {
	return RunsModel{}
}

// SetRuns replaces what the pane shows and marks the scope loaded.
func (m *RunsModel) SetRuns(scope RunsScope, branch string, runs []github.WorkflowRun) {
	m.scope = scope
	m.branch = branch
	m.runs = runs
	m.loading = false
	m.loaded = true
	m.err = nil

	if m.selectedIndex >= len(runs) {
		m.selectedIndex = 0
	}
}

// SetError records a failed load so the pane says so rather than reading empty.
func (m *RunsModel) SetError(err error) {
	m.err = err
	m.loading = false
	m.loaded = true
}

// SetLoading marks a fetch in flight.
func (m *RunsModel) SetLoading() {
	m.loading = true
	m.err = nil
}

// Loaded reports whether a fetch has already answered for the current scope,
// which is what keeps opening the tab from refetching.
func (m RunsModel) Loaded() bool { return m.loaded }

// Loading reports whether a fetch is in flight.
func (m RunsModel) Loading() bool { return m.loading }

// Scope returns the scope the pane is showing.
func (m RunsModel) Scope() RunsScope { return m.scope }

// SetScope moves straight to one scope, for a caller that named it.
func (m *RunsModel) SetScope(scope RunsScope) {
	if scope == m.scope {
		return
	}

	m.scope = scope
	m.runs = nil
	m.loaded = false
	m.selectedIndex = 0
}

// NextScope cycles to the next scope and invalidates what is on screen.
func (m *RunsModel) NextScope() RunsScope {
	m.scope = (m.scope + 1) % runsScopeCount
	m.runs = nil
	m.loaded = false
	m.selectedIndex = 0

	return m.scope
}

// Invalidate drops the loaded answer so the next open refetches.
func (m *RunsModel) Invalidate() {
	m.loaded = false
	m.runs = nil
}

// SetSize updates the pane dimensions.
func (m *RunsModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused updates the focus state.
func (m *RunsModel) SetFocused(focused bool) { m.focused = focused }

// MoveUp moves selection up.
func (m *RunsModel) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

// MoveDown moves selection down.
func (m *RunsModel) MoveDown() {
	if m.selectedIndex < len(m.runs)-1 {
		m.selectedIndex++
	}
}

// SelectedRun returns the run under the cursor.
func (m RunsModel) SelectedRun() (github.WorkflowRun, bool) {
	if len(m.runs) == 0 || m.selectedIndex >= len(m.runs) {
		return github.WorkflowRun{}, false
	}

	return m.runs[m.selectedIndex], true
}

// Summary counts how many runs passed, failed, and are still going, which is
// what the tab header reports without the tab being open.
func (m RunsModel) Summary() (int, int, int) {
	var passed, failed, active int

	for i := range m.runs {
		run := &m.runs[i]

		switch {
		case run.Status != github.StatusCompleted:
			active++
		case run.Conclusion == github.ConclusionSuccess:
			passed++
		case run.Conclusion == github.ConclusionFailure:
			failed++
		}
	}

	return passed, failed, active
}

// Update handles messages for the runs pane.
func (m RunsModel) Update(_ tea.Msg) (RunsModel, tea.Cmd) {
	return m, nil
}

// ViewContent renders the runs list without the pane border.
func (m RunsModel) ViewContent() string {
	if m.loading {
		return ui.SubtitleStyle.Render("Loading " + m.scope.Label() + "…")
	}

	if m.err != nil {
		return ui.SubtitleStyle.Render("Could not read runs") + "\n\n" +
			ui.NormalStyle.Render(m.err.Error())
	}

	if len(m.runs) == 0 {
		return m.viewEmpty()
	}

	var content strings.Builder

	layout := ui.FitColumns(runsColumns(m.scope), m.width, runsGutter)

	content.WriteString(ui.TableHeader(layout, runsGutter))
	content.WriteString("\n")

	first, last := ui.ScrollWindow(m.selectedIndex, len(m.runs), m.height-listPaneChrome)

	for i := first; i < last; i++ {
		run := &m.runs[i]

		indicator := "  "
		if i == m.selectedIndex {
			indicator = "> "
		}

		cells := ui.TableRow(layout, map[string]string{
			ui.ColKeyWorkflow: run.Name,
			ui.ColKeyBranch:   run.HeadBranch,
			ui.ColKeyTime:     formatTimeAgo(run.CreatedAt),
		})

		rowStyle := ui.TableRowStyle
		if i == m.selectedIndex {
			rowStyle = ui.TableSelectedStyle
		}

		content.WriteString(rowStyle.Render(indicator + runStatusIcon(run.Status, run.Conclusion) + " " + cells))

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

func (m RunsModel) viewEmpty() string {
	var content strings.Builder

	if !m.loaded {
		content.WriteString(ui.SubtitleStyle.Render("Not loaded"))
		content.WriteString("\n\n")
		content.WriteString(ui.HelpStyle.Render("[R] load  [s] scope"))

		return content.String()
	}

	content.WriteString(ui.SubtitleStyle.Render("No runs for " + m.scope.Label()))
	content.WriteString("\n\n")

	if m.scope == ScopeBranch && m.branch != "" {
		content.WriteString(ui.NormalStyle.Render("Nothing has run on " + m.branch + "."))
		content.WriteString("\n\n")
	}

	content.WriteString(ui.HelpStyle.Render("[s] scope  [R] reload"))

	return content.String()
}

// View renders the runs pane with its border.
func (m RunsModel) View() string {
	style := ui.PaneStyle(m.width, m.height, m.focused)

	return style.Render(ui.TitleStyle.Render("Runs ("+m.scope.Label()+")") + "\n" + m.ViewContent())
}

// RunsSelectedMsg is sent when a run in the pane is chosen.
type RunsSelectedMsg struct {
	Run github.WorkflowRun
}

// HandleSelect reports the run under the cursor.
func (m RunsModel) HandleSelect() tea.Cmd {
	run, ok := m.SelectedRun()
	if !ok {
		return nil
	}

	return func() tea.Msg {
		return RunsSelectedMsg{Run: run}
	}
}
