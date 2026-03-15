package panes

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// RightTab represents which tab is active in the right panel.
type RightTab int

const (
	TabHistory RightTab = iota
	TabChains
	TabLive
)

// TabbedRightModel manages the tabbed right panel.
type TabbedRightModel struct {
	activeTab RightTab
	width     int
	height    int
	focused   bool

	history HistoryModel
	chains  ChainListModel
	live    LiveRunsModel
}

// NewTabbedRight creates a new tabbed right panel.
func NewTabbedRight() TabbedRightModel {
	return TabbedRightModel{
		activeTab: TabHistory,
		history:   NewHistoryModel(),
		chains:    NewChainListModel(),
		live:      NewLiveRunsModel(),
	}
}

// SetSize updates the panel dimensions.
func (m *TabbedRightModel) SetSize(width, height int) {
	m.width = width
	m.height = height
	contentHeight := height - 4
	m.history.SetSize(width-2, contentHeight)
	m.chains.SetSize(width-2, contentHeight)
	m.live.SetSize(width-2, contentHeight)
}

// SetFocused updates the focus state.
func (m *TabbedRightModel) SetFocused(focused bool) {
	m.focused = focused
	m.history.SetFocused(focused && m.activeTab == TabHistory)
	m.chains.SetFocused(focused && m.activeTab == TabChains)
	m.live.SetFocused(focused && m.activeTab == TabLive)
}

// ActiveTab returns the currently active tab.
func (m TabbedRightModel) ActiveTab() RightTab {
	return m.activeTab
}

// NextTab switches to the next tab.
func (m *TabbedRightModel) NextTab() {
	m.activeTab = (m.activeTab + 1) % 3
	m.updateTabFocus()
}

// PrevTab switches to the previous tab.
func (m *TabbedRightModel) PrevTab() {
	m.activeTab = (m.activeTab + 2) % 3
	m.updateTabFocus()
}

func (m *TabbedRightModel) updateTabFocus() {
	m.history.SetFocused(m.focused && m.activeTab == TabHistory)
	m.chains.SetFocused(m.focused && m.activeTab == TabChains)
	m.live.SetFocused(m.focused && m.activeTab == TabLive)
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
	}

	return style.Render(tabs + "\n" + content)
}

func (m TabbedRightModel) renderTabHeader() string {
	tabs := []struct {
		name string
		tab  RightTab
	}{
		{"History", TabHistory},
		{"Chains", TabChains},
		{"Live", TabLive},
	}

	var parts []string

	for _, t := range tabs {
		if t.tab == m.activeTab {
			parts = append(parts, ui.SelectedStyle.Render("["+t.name+"]"))
		} else {
			parts = append(parts, ui.SubtitleStyle.Render(" "+t.name+" "))
		}
	}

	return strings.Join(parts, " ")
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
