package panes

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// RightTab represents which tab is active in the right panel.
type RightTab int

// Right panel tabs, in the order a reader wants them: what GitHub says now,
// what this session is watching, what this checkout dispatched, and how often
// any of it passes.
const (
	TabRuns RightTab = iota
	TabLive
	TabHistory
	TabFlaky
)

// TabCount is the number of tabs in the right panel, and the bound every
// RightTab is valid within.
const TabCount = 4

// TabbedRightModel manages the tabbed right panel and the run detail one of its
// rows drills into.
type TabbedRightModel struct {
	detail    *TimelineModel
	crumb     string
	live      LiveRunsModel
	history   HistoryModel
	flaky     FlakyModel
	runs      RunsModel
	activeTab RightTab
	width     int
	height    int
	focused   bool
}

// NewTabbedRight creates a new tabbed right panel.
func NewTabbedRight() TabbedRightModel {
	return TabbedRightModel{
		activeTab: TabRuns,
		history:   NewHistoryModel(),
		live:      NewLiveRunsModel(),
		runs:      NewRunsModel(),
		flaky:     NewFlakyModel(),
	}
}

// tabbedChromeHeight and tabbedChromeWidth reserve space for the tab bar, borders, and title.
const (
	tabbedChromeHeight = 4
	tabbedChromeWidth  = 2
)

// SetSize updates the panel dimensions.
func (m *TabbedRightModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	contentHeight := height - tabbedChromeHeight
	m.history.SetSize(width-tabbedChromeWidth, contentHeight)
	m.live.SetSize(width-tabbedChromeWidth, contentHeight)
	m.runs.SetSize(width-tabbedChromeWidth, contentHeight)
	m.flaky.SetSize(width-tabbedChromeWidth, contentHeight)

	if m.detail != nil {
		m.detail.SetSize(width-tabbedChromeWidth, contentHeight)
	}
}

// SetFocused updates the focus state.
func (m *TabbedRightModel) SetFocused(focused bool) {
	m.focused = focused
	m.updateTabFocus()
}

// ActiveTab returns the currently active tab.
func (m TabbedRightModel) ActiveTab() RightTab {
	return m.activeTab
}

// SetTab switches directly to one tab, for a caller that landed on the data
// rather than on the tab. It leaves any drilled-into detail, because naming a
// tab is asking for that tab.
func (m *TabbedRightModel) SetTab(tab RightTab) {
	if tab < 0 || tab >= TabCount {
		return
	}

	m.detail = nil
	m.activeTab = tab
	m.updateTabFocus()
}

// NextTab switches to the next tab.
func (m *TabbedRightModel) NextTab() {
	m.SetTab((m.activeTab + 1) % TabCount)
}

// PrevTab switches to the previous tab.
func (m *TabbedRightModel) PrevTab() {
	m.SetTab((m.activeTab + TabCount - 1) % TabCount)
}

func (m *TabbedRightModel) updateTabFocus() {
	inTab := m.focused && m.detail == nil
	m.history.SetFocused(inTab && m.activeTab == TabHistory)
	m.live.SetFocused(inTab && m.activeTab == TabLive)
	m.runs.SetFocused(inTab && m.activeTab == TabRuns)
	m.flaky.SetFocused(inTab && m.activeTab == TabFlaky)

	if m.detail != nil {
		m.detail.SetFocused(m.focused)
	}
}

// ShowDetail replaces the tab's list with one run drawn on a time axis, keeping
// the tab it came from in the breadcrumb. A run's timeline is a drill-down of a
// row rather than a peer of the list holding it, so it is reached by opening a
// row and left by backing out of one.
func (m *TabbedRightModel) ShowDetail(crumb, title string, jobs []github.Job) {
	detail := NewTimelineModel()
	detail.SetRun(title, jobs)
	detail.SetSize(m.width-tabbedChromeWidth, m.height-tabbedChromeHeight)

	m.detail = &detail
	m.crumb = crumb
	m.updateTabFocus()
}

// Detail returns the drilled-into run, or nil while a list is on screen.
func (m *TabbedRightModel) Detail() *TimelineModel { return m.detail }

// CloseDetail backs out to the list the detail was opened from, reporting
// whether there was one to close.
func (m *TabbedRightModel) CloseDetail() bool {
	if m.detail == nil {
		return false
	}

	m.detail = nil
	m.updateTabFocus()

	return true
}

// SetHistoryEntries updates the history entries.
func (m *TabbedRightModel) SetHistoryEntries(entries []frecency.HistoryEntry, workflowFilter string) {
	m.history.SetEntries(entries, workflowFilter)
}

// SetRuns updates the live runs.
func (m *TabbedRightModel) SetRuns(runs []watcher.WatchedRun) {
	m.live.SetRuns(runs)
}

// History returns the history model for direct access.
func (m *TabbedRightModel) History() *HistoryModel {
	return &m.history
}

// Live returns the live runs model for direct access.
func (m *TabbedRightModel) Live() *LiveRunsModel {
	return &m.live
}

// Runs returns the runs model for direct access.
func (m *TabbedRightModel) Runs() *RunsModel {
	return &m.runs
}

// Flaky returns the pass-rate model for direct access.
func (m *TabbedRightModel) Flaky() *FlakyModel {
	return &m.flaky
}

// Update handles messages for the active tab.
func (m TabbedRightModel) Update(msg tea.Msg) (TabbedRightModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	if m.detail != nil {
		detail, cmd := m.detail.Update(msg)
		*m.detail = detail

		return m, cmd
	}

	var cmd tea.Cmd

	switch m.activeTab {
	case TabHistory:
		m.history, cmd = m.history.Update(msg)
	case TabLive:
		m.live, cmd = m.live.Update(msg)
	case TabRuns:
		m.runs, cmd = m.runs.Update(msg)
	case TabFlaky:
		m.flaky, cmd = m.flaky.Update(msg)
	}

	return m, cmd
}

// MoveUp moves the selection up in whatever the panel is showing.
func (m *TabbedRightModel) MoveUp() {
	if m.detail != nil {
		m.detail.MoveUp()
		return
	}

	switch m.activeTab {
	case TabHistory:
		m.history.MoveUp()
	case TabLive:
		m.live.MoveUp()
	case TabRuns:
		m.runs.MoveUp()
	case TabFlaky:
		m.flaky.MoveUp()
	}
}

// MoveDown moves the selection down in whatever the panel is showing.
func (m *TabbedRightModel) MoveDown() {
	if m.detail != nil {
		m.detail.MoveDown()
		return
	}

	switch m.activeTab {
	case TabHistory:
		m.history.MoveDown()
	case TabLive:
		m.live.MoveDown()
	case TabRuns:
		m.runs.MoveDown()
	case TabFlaky:
		m.flaky.MoveDown()
	}
}

// View renders the tabbed panel.
func (m TabbedRightModel) View() string {
	if m.detail != nil {
		return ui.PaneBox(m.width, m.height, m.focused, m.renderCrumb()+"\n"+m.detail.ViewContent())
	}

	var content string

	switch m.activeTab {
	case TabHistory:
		content = m.history.ViewContent()
	case TabLive:
		content = m.live.ViewContent()
	case TabRuns:
		content = m.runs.ViewContent()
	case TabFlaky:
		content = m.flaky.ViewContent()
	}

	return ui.PaneBox(m.width, m.height, m.focused, m.renderTabHeader()+"\n"+content)
}

// renderCrumb names the path into the detail, so backing out is obviously a
// direction rather than a guess.
func (m TabbedRightModel) renderCrumb() string {
	trail := m.crumb
	if m.detail != nil {
		trail += " › " + m.detail.Heading()
	}

	line := ui.TitleStyle.Render(ansi.Truncate(trail, m.width-tabbedChromeWidth-backHintWidth, "…")) +
		ui.HelpStyle.Render("  [esc] back")

	return line
}

// backHintWidth is the room renderCrumb leaves for its own hint.
const backHintWidth = 12

func (m TabbedRightModel) renderTabHeader() string {
	tabs := m.tabLabels()

	full := m.joinTabs(tabs, false)
	if ansi.StringWidth(ansi.Strip(full)) <= m.width-tabbedChromeWidth {
		return full
	}

	return m.joinTabs(tabs, true)
}

// tabLabel is one tab's name and the count that makes its contents visible
// without opening it.
type tabLabel struct {
	name  string
	count string
	tab   RightTab
}

// tabLabels reports what each tab holds, so a reader sees the counts rather
// than having to visit them all. The Runs tab reports a verdict instead of a
// count, since "three passed, one failed" is the answer and the row total is
// not, and the Flaky tab reports its worst pass rate for the same reason.
func (m TabbedRightModel) tabLabels() []tabLabel {
	live := ""
	if n := len(m.live.runs); n > 0 {
		live = strconv.Itoa(n)
		if m.live.ActiveCount() > 0 {
			live += "*"
		}
	}

	return []tabLabel{
		{name: "Runs", count: m.runsVerdict(), tab: TabRuns},
		{name: "Live", count: live, tab: TabLive},
		{name: "History", count: countLabel(len(m.history.entries)), tab: TabHistory},
		{name: "Flaky", count: m.flakyVerdict(), tab: TabFlaky},
	}
}

func countLabel(n int) string {
	if n == 0 {
		return ""
	}

	return strconv.Itoa(n)
}

// runsVerdict spells the branch's state in the glyphs the run lists already
// use, and stays empty until something has been loaded.
func (m TabbedRightModel) runsVerdict() string {
	passed, failed, active := m.runs.Summary()

	var parts []string

	for _, part := range []struct {
		glyph string
		count int
	}{{"+", passed}, {"x", failed}, {"*", active}} {
		if part.count > 0 {
			parts = append(parts, strconv.Itoa(part.count)+part.glyph)
		}
	}

	return strings.Join(parts, "")
}

// flakyVerdict reports the worst pass rate measured, which is the number that
// decides whether the tab is worth opening.
func (m TabbedRightModel) flakyVerdict() string {
	rows, worst := m.flaky.Summary()
	if rows == 0 || worst < 0 {
		return ""
	}

	return strconv.Itoa(worst) + "%"
}

// joinTabs renders the tab bar, abbreviating each name to its initial when the
// full names do not fit. The counts survive abbreviation, because they are what
// the bar is for.
func (m TabbedRightModel) joinTabs(tabs []tabLabel, abbreviated bool) string {
	parts := make([]string, 0, len(tabs))

	for _, t := range tabs {
		name := t.name
		if abbreviated {
			name = name[:1]
		}

		if t.count != "" {
			name += " " + t.count
		}

		if t.tab == m.activeTab {
			parts = append(parts, ui.TabActiveStyle.Render(name))
		} else {
			parts = append(parts, ui.TabInactiveStyle.Render(name))
		}
	}

	return strings.Join(parts, "")
}

// SelectedGitHubRun returns the run selected in the Runs tab.
func (m TabbedRightModel) SelectedGitHubRun() (github.WorkflowRun, bool) {
	return m.runs.SelectedRun()
}

// SelectedHistoryEntry returns the currently selected history entry.
func (m TabbedRightModel) SelectedHistoryEntry() *frecency.HistoryEntry {
	return m.history.SelectedEntry()
}

// SelectedRun returns the currently selected run.
func (m TabbedRightModel) SelectedRun() (watcher.WatchedRun, bool) {
	return m.live.SelectedRun()
}
