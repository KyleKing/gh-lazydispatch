package panes

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/forge"
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

// runsColumns describes the runs table. Every row shares the ref named in the
// pane title, so there is no branch column.
func runsColumns() []table.Column {
	return []table.Column{
		{
			Key: ui.ColKeyWorkflow, Title: ui.ColTitleWorkflow, Min: ui.ColMinName, Max: ui.ColMaxName,
			Weight: ui.WeightHigh,
		},
		{
			Key: ui.ColKeyTime, Title: ui.ColTitleTime, Min: ui.ColMinLabel, Max: ui.ColMaxTime,
			Priority: ui.PrioFirstToGo,
		},
	}
}

// prColumns describes the pull request table. A pull request row reports its
// check rollup rather than one workflow, so it shares no column with a run row
// beyond the time.
func prColumns() []table.Column {
	return []table.Column{
		{Key: ui.ColKeyPR, Title: ui.ColTitlePR, Min: ui.ColMinShort, Max: ui.ColMinShort},
		{Key: ui.ColKeyName, Title: ui.ColTitleName, Min: ui.ColMinName, Max: ui.ColMaxName, Weight: ui.WeightHigh},
		{
			Key: ui.ColKeyChecks, Title: ui.ColTitleChecks, Min: ui.ColMinLabel, Max: ui.ColMaxStatus,
			Priority: ui.PrioSecondToGo,
		},
		{
			Key: ui.ColKeyTime, Title: ui.ColTitleTime, Min: ui.ColMinLabel, Max: ui.ColMaxTime,
			Priority: ui.PrioFirstToGo,
		},
	}
}

// checksLabel spells a pull request's rollup as passing, failing, and pending
// counts, dropping the zeroes.
func checksLabel(c forge.ChecksStatus) string {
	if c.Total == 0 {
		return "none"
	}

	var label strings.Builder

	for _, part := range []struct {
		glyph string
		count int
	}{{"+", c.Passing}, {"x", c.Failing}, {"*", c.Pending}} {
		if part.count > 0 {
			if label.Len() > 0 {
				label.WriteString(" ")
			}

			label.WriteString(strconv.Itoa(part.count) + part.glyph)
		}
	}

	if label.Len() == 0 {
		return "skipped"
	}

	return label.String()
}

// checksIcon reduces a rollup to the one glyph a row's gutter holds: a failure
// outranks a pending check, because it is the one a reader has to act on.
func checksIcon(c forge.ChecksStatus) string {
	switch {
	case c.Failing > 0:
		return "x"
	case c.Pending > 0:
		return "*"
	case c.Passing > 0:
		return "+"
	default:
		return "?"
	}
}

// RunsModel shows the current state of each workflow on GitHub, which is a
// different question from the local dispatch history the History tab keeps.
type RunsModel struct {
	err           error
	branch        string
	ref           string
	runs          []github.WorkflowRun
	prs           []github.PullRequest
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
	m.prs = nil
	m.loading = false
	m.loaded = true
	m.err = nil

	if m.selectedIndex >= len(runs) {
		m.selectedIndex = 0
	}
}

// SetPRs replaces what a pull request scope shows and marks it loaded.
func (m *RunsModel) SetPRs(scope RunsScope, prs []github.PullRequest) {
	m.scope = scope
	m.runs = nil
	m.prs = prs
	m.loading = false
	m.loaded = true
	m.err = nil

	if m.selectedIndex >= len(prs) {
		m.selectedIndex = 0
	}
}

// rowCount is how many rows the current scope holds, whichever shape they are.
func (m RunsModel) rowCount() int {
	if m.scope == ScopeBranch {
		return len(m.runs)
	}

	return len(m.prs)
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
	m.prs = nil
	m.ref = ""
	m.loaded = false
	m.selectedIndex = 0
}

// NextScope cycles to the next scope and invalidates what is on screen.
func (m *RunsModel) NextScope() RunsScope {
	m.scope = (m.scope + 1) % runsScopeCount
	m.runs = nil
	m.prs = nil
	m.ref = ""
	m.loaded = false
	m.selectedIndex = 0

	return m.scope
}

// Invalidate drops the loaded answer so the next open refetches.
func (m *RunsModel) Invalidate() {
	m.loaded = false
	m.runs = nil
	m.prs = nil
}

// DrillToBranch moves the pane to the branch scope for a ref the checkout is
// not on, which is how a pull request row reaches the runs behind its rollup.
func (m *RunsModel) DrillToBranch(ref string) {
	m.scope = ScopeBranch
	m.ref = ref
	m.runs = nil
	m.prs = nil
	m.loaded = false
	m.selectedIndex = 0
}

// Ref returns the ref a drilled branch scope is showing, empty when the pane is
// showing the checkout's own branch.
func (m RunsModel) Ref() string { return m.ref }

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
	if m.selectedIndex < m.rowCount()-1 {
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

// SelectedPR returns the pull request under the cursor.
func (m RunsModel) SelectedPR() (github.PullRequest, bool) {
	if m.scope == ScopeBranch || m.selectedIndex >= len(m.prs) {
		return github.PullRequest{}, false
	}

	return m.prs[m.selectedIndex], true
}

// Summary counts how many runs passed, failed, and are still going, which is
// what the tab header reports without the tab being open.
func (m RunsModel) Summary() (int, int, int) {
	var passed, failed, active int

	for i := range m.prs {
		switch checksIcon(m.prs[i].Checks) {
		case "x":
			failed++
		case "*":
			active++
		case "+":
			passed++
		}
	}

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

// ViewContent renders the list without the pane border.
func (m RunsModel) ViewContent() string {
	if m.loading {
		return ui.SubtitleStyle.Render("Loading " + m.scope.Label() + "…")
	}

	if m.err != nil {
		return ui.SubtitleStyle.Render("Could not read runs") + "\n\n" +
			ui.NormalStyle.Render(m.err.Error())
	}

	if m.rowCount() == 0 {
		return m.viewEmpty()
	}

	if m.scope == ScopeBranch {
		return m.scopeLine() + m.viewRows(ui.FitColumns(runsColumns(), m.width, runsGutter), len(m.runs), m.runCells)
	}

	return m.scopeLine() + m.viewRows(ui.FitColumns(prColumns(), m.width, runsGutter), len(m.prs), m.prCells)
}

func (m RunsModel) runCells(layout table.Layout, i int) (string, string) {
	run := &m.runs[i]

	return runStatusIcon(run.Status, run.Conclusion), ui.TableRow(layout, map[string]string{
		ui.ColKeyWorkflow: run.Name,
		ui.ColKeyTime:     display.RelativeTimeCompact(run.CreatedAt),
	})
}

func (m RunsModel) prCells(layout table.Layout, i int) (string, string) {
	pr := &m.prs[i]

	return checksIcon(pr.Checks), ui.TableRow(layout, map[string]string{
		ui.ColKeyPR:     "#" + strconv.Itoa(pr.Number),
		ui.ColKeyName:   pr.Title,
		ui.ColKeyChecks: checksLabel(pr.Checks),
		ui.ColKeyTime:   display.RelativeTimeCompact(pr.UpdatedAt),
	})
}

// scopeLine names what the rows describe. The tab bar takes the pane's title
// row, so without this a scope drilled into a pull request's head branch reads
// as the checkout's own.
func (m RunsModel) scopeLine() string {
	return ui.SubtitleStyle.Render(ansi.Truncate(m.title(), m.width, "…")) + "\n"
}

// scopeLineHeight is the row scopeLine spends.
const scopeLineHeight = 1

// viewRows draws the scrolled window of whichever row shape the scope holds.
func (m RunsModel) viewRows(
	layout table.Layout, total int, cells func(table.Layout, int) (string, string),
) string {
	var content strings.Builder

	content.WriteString(ui.TableHeader(layout, runsGutter))
	content.WriteString("\n")

	first, last := ui.ScrollWindow(m.selectedIndex, total, m.height-listPaneChrome-scopeLineHeight)

	for i := first; i < last; i++ {
		indicator := "  "
		if i == m.selectedIndex {
			indicator = "> "
		}

		rowStyle := ui.TableRowStyle
		if i == m.selectedIndex {
			rowStyle = ui.TableSelectedStyle
		}

		glyph, row := cells(layout, i)
		content.WriteString(rowStyle.Render(indicator + glyph + " " + row))

		if i < last-1 {
			content.WriteString("\n")
		}
	}

	if first > 0 || last < total {
		content.WriteString("\n")
		content.WriteString(ui.RenderScrollIndicator(last < total, first > 0))
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

	if ref := m.title(); m.scope == ScopeBranch && ref != "" {
		content.WriteString(ui.NormalStyle.Render("Nothing has run on " + ref + "."))
		content.WriteString("\n\n")
	}

	content.WriteString(ui.HelpStyle.Render("[s] scope  [R] reload"))

	return content.String()
}

// title names what the pane is showing: the scope, or the ref a pull request
// row was drilled into.
func (m RunsModel) title() string {
	if m.scope != ScopeBranch {
		return m.scope.Label()
	}

	if m.ref != "" {
		return m.ref
	}

	return m.branch
}

// View renders the runs pane with its border.
func (m RunsModel) View() string {
	style := ui.PaneStyle(m.width, m.height, m.focused)

	return style.Render(ui.TitleStyle.Render("Runs ("+m.title()+")") + "\n" + m.ViewContent())
}
