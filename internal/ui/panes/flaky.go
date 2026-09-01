package panes

import (
	"sort"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/display"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// flakyColumns describes the pass-rate table, one row per workflow.
func flakyColumns() []table.Column {
	return []table.Column{
		{
			Key: ui.ColKeyWorkflow, Title: ui.ColTitleWorkflow, Min: ui.ColMinName, Max: ui.ColMaxName,
			Weight: ui.WeightHigh,
		},
		{
			Key: ui.ColKeyRuns, Title: ui.ColTitleRuns, Min: ui.ColMinCount, Max: ui.ColMaxCount,
			Align: table.AlignRight,
		},
		{
			Key: ui.ColKeyPass, Title: ui.ColTitlePass, Min: ui.ColMinShort, Max: ui.ColMinShort,
			Align: table.AlignRight,
		},
		{
			Key: ui.ColKeyTime, Title: "Last", Min: ui.ColMinLabel, Max: ui.ColMaxTime,
			Priority: ui.PrioFirstToGo,
		},
	}
}

// flakyRunColumns describes one workflow's own runs, which is where a pass rate
// stops being a number and names the branches it failed on.
func flakyRunColumns() []table.Column {
	return []table.Column{
		{
			Key: ui.ColKeyBranch, Title: ui.ColTitleBranch, Min: ui.ColMinName, Max: ui.ColMaxBranch,
			Weight: ui.WeightHigh,
		},
		{Key: "event", Title: "Event", Min: ui.ColMinLabel, Max: ui.ColMaxStatus, Priority: ui.PrioSecondToGo},
		{
			Key: ui.ColKeyTime, Title: ui.ColTitleTime, Min: ui.ColMinLabel, Max: ui.ColMaxTime,
			Priority: ui.PrioFirstToGo,
		},
	}
}

// workflowRate is one workflow's record over the runs the pane fetched.
type workflowRate struct {
	last     time.Time
	file     string
	name     string
	lastIcon string
	runs     int
	passed   int
	failed   int
	mixed    bool
}

// label names the row. A run's title is friendlier than its filename, but a
// workflow whose runs title themselves differently (dependabot names each run
// after the pull request it opened) has no one title, and the file is then the
// only honest name for the group.
func (r workflowRate) label() string {
	if r.mixed || r.name == "" {
		return r.file
	}

	return r.name
}

// rate is the share of finished runs that succeeded, and -1 when none finished.
// A workflow whose runs are all still going has no rate rather than a zero one.
func (r workflowRate) rate() int {
	finished := r.passed + r.failed
	if finished == 0 {
		return -1
	}

	return r.passed * percent / finished
}

const percent = 100

// FlakyModel answers whether a workflow is reliable, which is the opposite
// query to the Runs tab's: many runs of one workflow rather than one run of
// each. Both are served from a single listing, so selecting a workflow on the
// left re-derives the view rather than refetching.
type FlakyModel struct {
	err           error
	workflow      string
	runs          []github.WorkflowRun
	rates         []workflowRate
	selectedIndex int
	width         int
	height        int
	focused       bool
	loading       bool
	loaded        bool
}

// NewFlakyModel creates an empty pane, which loads nothing until the tab is
// opened.
func NewFlakyModel() FlakyModel {
	return FlakyModel{}
}

// SetRuns replaces the listing the pane derives both of its views from.
func (m *FlakyModel) SetRuns(runs []github.WorkflowRun) {
	m.runs = runs
	m.loading = false
	m.loaded = true
	m.err = nil
	m.rebuild()
}

// SetWorkflow narrows the pane to one workflow file, or widens it back to every
// workflow when path is empty. This follows the left column's selection, so
// walking the workflow list walks this pane with it.
func (m *FlakyModel) SetWorkflow(path string) {
	if path == m.workflow {
		return
	}

	m.workflow = path
	m.selectedIndex = 0
	m.rebuild()
}

// SetError records a failed load so the pane says so rather than reading empty.
func (m *FlakyModel) SetError(err error) {
	m.err = err
	m.loading = false
	m.loaded = true
}

// SetLoading marks a fetch in flight.
func (m *FlakyModel) SetLoading() {
	m.loading = true
	m.err = nil
}

// Loaded reports whether a fetch has already answered.
func (m FlakyModel) Loaded() bool { return m.loaded }

// Invalidate drops the loaded answer so the next open refetches.
func (m *FlakyModel) Invalidate() {
	m.loaded = false
	m.runs = nil
	m.rates = nil
}

// SetSize updates the pane dimensions.
func (m *FlakyModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused updates the focus state.
func (m *FlakyModel) SetFocused(focused bool) { m.focused = focused }

// MoveUp moves selection up.
func (m *FlakyModel) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

// MoveDown moves selection down.
func (m *FlakyModel) MoveDown() {
	if m.selectedIndex < m.rowCount()-1 {
		m.selectedIndex++
	}
}

// SelectedRun returns the run under the cursor, which only the per-workflow
// view has: an aggregate row is many runs.
func (m FlakyModel) SelectedRun() (github.WorkflowRun, bool) {
	filtered := m.filtered()
	if m.workflow == "" || m.selectedIndex >= len(filtered) {
		return github.WorkflowRun{}, false
	}

	return filtered[m.selectedIndex], true
}

// Summary reports the rows and the flakiest rate on them, which the tab header
// carries so the tab says whether it is worth opening.
func (m FlakyModel) Summary() (int, int) {
	worst := -1

	for _, r := range m.rates {
		if rate := r.rate(); rate >= 0 && (worst < 0 || rate < worst) {
			worst = rate
		}
	}

	return len(m.rates), worst
}

// filtered is the runs of the selected workflow, newest first as GitHub lists
// them.
func (m FlakyModel) filtered() []github.WorkflowRun {
	if m.workflow == "" {
		return nil
	}

	out := make([]github.WorkflowRun, 0, len(m.runs))

	for i := range m.runs {
		if workflowFile(m.runs[i].Path) == m.workflow {
			out = append(out, m.runs[i])
		}
	}

	return out
}

// workflowFile reduces a run's `.github/workflows/ci.yml` to `ci.yml`, which is
// how a workflow is named everywhere else in this tool.
func workflowFile(path string) string {
	if i := strings.LastIndex(path, "/"); i >= 0 {
		return path[i+1:]
	}

	return path
}

func (m FlakyModel) rowCount() int {
	if m.workflow != "" {
		return len(m.filtered())
	}

	return len(m.rates)
}

// rebuild groups the listing by workflow file, flakiest first, which is the
// order the question is asked in. Grouping by title instead would split a
// workflow that titles each run differently into one row per run.
func (m *FlakyModel) rebuild() {
	byName := make(map[string]*workflowRate)

	for i := range m.runs {
		run := &m.runs[i]
		file := workflowFile(run.Path)

		r, ok := byName[file]
		if !ok {
			r = &workflowRate{file: file, name: run.Name}
			byName[file] = r
		}

		if r.name != run.Name {
			r.mixed = true
		}

		r.runs++

		switch {
		case run.Status != github.StatusCompleted:
		case run.Conclusion == github.ConclusionSuccess:
			r.passed++
		case run.Conclusion == github.ConclusionFailure:
			r.failed++
		}

		if run.CreatedAt.After(r.last) {
			r.last = run.CreatedAt
			r.lastIcon = runStatusIcon(run.Status, run.Conclusion)
		}
	}

	m.rates = make([]workflowRate, 0, len(byName))
	for _, r := range byName {
		m.rates = append(m.rates, *r)
	}

	// Flakiest first. A workflow with nothing finished has no rate, and sorts
	// after every measured one rather than ahead of them: unknown is not the
	// same answer as zero.
	sort.Slice(m.rates, func(i, j int) bool {
		a, b := m.rates[i].rate(), m.rates[j].rate()
		if a != b {
			return b < 0 || (a >= 0 && a < b)
		}

		return m.rates[i].label() < m.rates[j].label()
	})

	if m.selectedIndex >= m.rowCount() {
		m.selectedIndex = 0
	}
}

// Update handles messages for the pane.
func (m FlakyModel) Update(_ tea.Msg) (FlakyModel, tea.Cmd) {
	return m, nil
}

// ViewContent renders the list without the pane border.
func (m FlakyModel) ViewContent() string {
	switch {
	case m.loading:
		return ui.SubtitleStyle.Render("Reading recent runs…")
	case m.err != nil:
		return ui.SubtitleStyle.Render("Could not read runs") + "\n\n" + ui.NormalStyle.Render(m.err.Error())
	case !m.loaded:
		return ui.SubtitleStyle.Render("Not loaded") + "\n\n" + ui.HelpStyle.Render("[R] load")
	case m.rowCount() == 0:
		return ui.SubtitleStyle.Render("No runs to measure") + "\n\n" +
			ui.NormalStyle.Render("GitHub has no recent runs of this workflow.")
	case m.workflow != "":
		return m.scopeLine() + m.viewRuns()
	}

	return m.scopeLine() + m.viewRates()
}

func (m FlakyModel) viewRates() string {
	layout := ui.FitColumns(flakyColumns(), m.width, runsGutter)

	return m.viewRows(layout, len(m.rates), func(l table.Layout, i int) (string, string) {
		r := m.rates[i]

		return r.lastIcon, ui.TableRow(l, map[string]string{
			ui.ColKeyWorkflow: r.label(),
			ui.ColKeyRuns:     strconv.Itoa(r.runs),
			ui.ColKeyPass:     rateLabel(r.rate()),
			ui.ColKeyTime:     display.RelativeTimeCompact(r.last),
		})
	})
}

func (m FlakyModel) viewRuns() string {
	runs := m.filtered()
	layout := ui.FitColumns(flakyRunColumns(), m.width, runsGutter)

	return m.viewRows(layout, len(runs), func(l table.Layout, i int) (string, string) {
		run := &runs[i]

		return runStatusIcon(run.Status, run.Conclusion), ui.TableRow(l, map[string]string{
			ui.ColKeyBranch: run.HeadBranch,
			"event":         run.Event,
			ui.ColKeyTime:   display.RelativeTimeCompact(run.CreatedAt),
		})
	})
}

// rateLabel spells a pass rate, and a workflow with nothing finished as "-"
// rather than as zero percent.
func rateLabel(rate int) string {
	if rate < 0 {
		return "-"
	}

	return strconv.Itoa(rate) + "%"
}

// scopeLine names what is being measured, which the tab bar leaves no title
// row for.
func (m FlakyModel) scopeLine() string {
	if m.workflow != "" {
		return ui.SubtitleStyle.Render(m.workflow+" · every recent run") + "\n"
	}

	return ui.SubtitleStyle.Render("every workflow · flakiest first") + "\n"
}

// viewRows draws the scrolled window of whichever row shape is active.
func (m FlakyModel) viewRows(
	layout table.Layout, total int, cells func(table.Layout, int) (string, string),
) string {
	var content strings.Builder

	content.WriteString(ui.TableHeader(layout, runsGutter))

	first, last := ui.ScrollWindow(m.selectedIndex, total, m.height-listPaneChrome-scopeLineHeight)

	for i := first; i < last; i++ {
		indicator := "  "
		rowStyle := ui.TableRowStyle

		if i == m.selectedIndex {
			indicator = "> "
			rowStyle = ui.TableSelectedStyle
		}

		glyph, row := cells(layout, i)

		content.WriteString("\n")
		content.WriteString(rowStyle.Render(indicator + glyph + " " + row))
	}

	if first > 0 || last < total {
		content.WriteString("\n")
		content.WriteString(ui.RenderScrollIndicator(last < total, first > 0))
	}

	return content.String()
}

// Title names what the pane is measuring.
func (m FlakyModel) Title() string {
	if m.workflow != "" {
		return m.workflow
	}

	return "every workflow"
}
