package panes

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// RightTab represents which tab is active in the right panel.
type RightTab int

// Right panel tab values.
const (
	TabHistory RightTab = iota
	TabChains
	TabLive
	TabTimeline
	TabRuns
)

// TabbedRightModel manages the tabbed right panel.
type TabbedRightModel struct {
	history   HistoryModel
	chains    ChainListModel
	live      LiveRunsModel
	timeline  TimelineModel
	runs      RunsModel
	activeTab RightTab
	width     int
	height    int
	focused   bool
}

// NewTabbedRight creates a new tabbed right panel.
func NewTabbedRight() TabbedRightModel {
	return TabbedRightModel{
		activeTab: TabHistory,
		history:   NewHistoryModel(),
		chains:    NewChainListModel(),
		live:      NewLiveRunsModel(),
		timeline:  NewTimelineModel(),
		runs:      NewRunsModel(),
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
	m.chains.SetSize(width-tabbedChromeWidth, contentHeight)
	m.live.SetSize(width-tabbedChromeWidth, contentHeight)
	m.timeline.SetSize(width-tabbedChromeWidth, contentHeight)
	m.runs.SetSize(width-tabbedChromeWidth, contentHeight)
}

// SetFocused updates the focus state.
func (m *TabbedRightModel) SetFocused(focused bool) {
	m.focused = focused
	m.history.SetFocused(focused && m.activeTab == TabHistory)
	m.chains.SetFocused(focused && m.activeTab == TabChains)
	m.live.SetFocused(focused && m.activeTab == TabLive)
	m.timeline.SetFocused(focused && m.activeTab == TabTimeline)
	m.runs.SetFocused(focused && m.activeTab == TabRuns)
}

// ActiveTab returns the currently active tab.
func (m TabbedRightModel) ActiveTab() RightTab {
	return m.activeTab
}

// tabCount is the number of tabs in the right panel.
const tabCount = 5

// SetTab switches directly to one tab, for a caller that landed on the data
// rather than on the tab.
func (m *TabbedRightModel) SetTab(tab RightTab) {
	if tab < 0 || tab >= tabCount {
		return
	}

	m.activeTab = tab
	m.updateTabFocus()
}

// NextTab switches to the next tab.
func (m *TabbedRightModel) NextTab() {
	m.activeTab = (m.activeTab + 1) % tabCount
	m.updateTabFocus()
}

// PrevTab switches to the previous tab.
func (m *TabbedRightModel) PrevTab() {
	m.activeTab = (m.activeTab + tabCount - 1) % tabCount
	m.updateTabFocus()
}

func (m *TabbedRightModel) updateTabFocus() {
	m.history.SetFocused(m.focused && m.activeTab == TabHistory)
	m.chains.SetFocused(m.focused && m.activeTab == TabChains)
	m.live.SetFocused(m.focused && m.activeTab == TabLive)
	m.timeline.SetFocused(m.focused && m.activeTab == TabTimeline)
	m.runs.SetFocused(m.focused && m.activeTab == TabRuns)
}

// SetHistoryEntries updates the history entries.
func (m *TabbedRightModel) SetHistoryEntries(entries []frecency.HistoryEntry, workflowFilter string) {
	m.history.SetEntries(entries, workflowFilter)
}

// SetChains updates the chain definitions.
func (m *TabbedRightModel) SetChains(chains map[string]config.Chain) {
	m.chains.SetChains(chains)
}

// SetRuns updates the live runs.
func (m *TabbedRightModel) SetRuns(runs []watcher.WatchedRun) {
	m.live.SetRuns(runs)
}

// History returns the history model for direct access.
func (m *TabbedRightModel) History() *HistoryModel {
	return &m.history
}

// Chains returns the chain list model for direct access.
func (m *TabbedRightModel) Chains() *ChainListModel {
	return &m.chains
}

// Live returns the live runs model for direct access.
func (m *TabbedRightModel) Live() *LiveRunsModel {
	return &m.live
}

// Timeline returns the timeline model for direct access.
func (m *TabbedRightModel) Timeline() *TimelineModel {
	return &m.timeline
}

// Runs returns the runs model for direct access.
func (m *TabbedRightModel) Runs() *RunsModel {
	return &m.runs
}

// SetTimelineRun replaces what the Timeline tab draws.
func (m *TabbedRightModel) SetTimelineRun(title string, jobs []github.Job) {
	m.timeline.SetRun(title, jobs)
}

// Update handles messages for the active tab.
func (m TabbedRightModel) Update(msg tea.Msg) (TabbedRightModel, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	var cmd tea.Cmd

	switch m.activeTab {
	case TabHistory:
		m.history, cmd = m.history.Update(msg)
	case TabChains:
		m.chains, cmd = m.chains.Update(msg)
	case TabLive:
		m.live, cmd = m.live.Update(msg)
	case TabTimeline:
		m.timeline, cmd = m.timeline.Update(msg)
	case TabRuns:
		m.runs, cmd = m.runs.Update(msg)
	}

	return m, cmd
}

// View renders the tabbed panel.
func (m TabbedRightModel) View() string {
	style := ui.PaneStyle(m.width, m.height, m.focused)

	tabs := m.renderTabHeader()

	var content string

	switch m.activeTab {
	case TabHistory:
		content = m.history.ViewContent()
	case TabChains:
		content = m.chains.ViewContent()
	case TabLive:
		content = m.live.ViewContent()
	case TabTimeline:
		content = m.timeline.ViewContent()
	case TabRuns:
		content = m.runs.ViewContent()
	}

	return style.Render(tabs + "\n" + content)
}

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
// than having to visit four tabs to find them. The Runs tab reports a verdict
// instead of a count, since "three passed, one failed" is the answer and the
// row total is not.
func (m TabbedRightModel) tabLabels() []tabLabel {
	live := ""
	if n := len(m.live.runs); n > 0 {
		live = strconv.Itoa(n)
		if m.live.ActiveCount() > 0 {
			live += "*"
		}
	}

	return []tabLabel{
		{name: "History", count: countLabel(len(m.history.entries)), tab: TabHistory},
		{name: "Chains", count: countLabel(len(m.chains.chainNames)), tab: TabChains},
		{name: "Live", count: live, tab: TabLive},
		{name: "Timeline", count: countLabel(len(m.timeline.jobs)), tab: TabTimeline},
		{name: "Runs", count: m.runsVerdict(), tab: TabRuns},
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
			parts = append(parts, ui.SelectedStyle.Render("["+name+"]"))
		} else {
			parts = append(parts, ui.SubtitleStyle.Render(" "+name+" "))
		}
	}

	return strings.Join(parts, " ")
}

// SelectedGitHubRun returns the run selected in the Runs tab.
func (m TabbedRightModel) SelectedGitHubRun() (github.WorkflowRun, bool) {
	return m.runs.SelectedRun()
}

// SelectedHistoryEntry returns the currently selected history entry.
func (m TabbedRightModel) SelectedHistoryEntry() *frecency.HistoryEntry {
	return m.history.SelectedEntry()
}

// SelectedChain returns the currently selected chain.
func (m TabbedRightModel) SelectedChain() (string, config.Chain, bool) {
	return m.chains.SelectedChain()
}

// SelectedRun returns the currently selected run.
func (m TabbedRightModel) SelectedRun() (watcher.WatchedRun, bool) {
	return m.live.SelectedRun()
}
