package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// FetchFlakyMsg asks for the run history a pass rate is measured over.
type FetchFlakyMsg struct{}

// FlakyFetchedMsg carries the listing, or why it could not be read.
type FlakyFetchedMsg struct {
	Error error
	Runs  []github.WorkflowRun
}

// flakySampleSize is how many of the repository's recent runs a pass rate is
// measured over. One page is one API call and covers weeks in most
// repositories; a larger sample would answer a question about last quarter
// rather than about whether a workflow is flaky now.
const flakySampleSize = 100

// fetchFlakyCmd reads one page of the repository's runs across every workflow.
// Both of the tab's views come out of the same listing, so narrowing to the
// selected workflow costs no second call.
func (m Model) fetchFlakyCmd() tea.Cmd {
	client := m.ghClient
	if client == nil {
		return statusCmd("no GitHub client")
	}

	return func() tea.Msg {
		runs, err := client.ListRuns(github.RunQuery{Limit: flakySampleSize})
		if err != nil {
			return FlakyFetchedMsg{Error: err}
		}

		return FlakyFetchedMsg{Runs: runs}
	}
}

// handleFlakyMsg routes the Flaky tab's background work.
func (m Model) handleFlakyMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case FetchFlakyMsg:
		m.rightPanel.Flaky().SetLoading()

		return m, m.fetchFlakyCmd(), true

	case FlakyFetchedMsg:
		if msg.Error != nil {
			m.rightPanel.Flaky().SetError(msg.Error)

			return m, nil, true
		}

		m.rightPanel.Flaky().SetRuns(msg.Runs)
		m.syncSelectedWorkflow()

		return m, nil, true
	}

	return m, nil, false
}

// loadFlakyIfNeeded fetches the pass rates the first time the tab is opened.
func (m Model) loadFlakyIfNeeded() tea.Cmd {
	if m.rightPanel.ActiveTab() != panes.TabFlaky || m.rightPanel.Flaky().Loaded() {
		return nil
	}

	return m.fetchFlakyCmd()
}

// reloadFlaky refetches the run history.
func (m Model) reloadFlaky() (tea.Model, tea.Cmd) {
	m.rightPanel.Flaky().Invalidate()

	return m, m.fetchFlakyCmd()
}

// syncSelectedWorkflow points the pass-rate pane at whatever the workflow list
// has selected, which is what makes walking the left column walk the right
// panel with it.
func (m *Model) syncSelectedWorkflow() {
	file := ""
	if m.selectedWorkflow >= 0 && m.selectedWorkflow < len(m.workflows) {
		file = m.workflows[m.selectedWorkflow].Filename
	}

	m.rightPanel.Flaky().SetWorkflow(file)
}
