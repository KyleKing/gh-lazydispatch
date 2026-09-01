package app

import (
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// FetchRunsMsg asks for the current state of a scope's workflows.
type FetchRunsMsg struct {
	Branch string
	Scope  panes.RunsScope
}

// RunsFetchedMsg carries what GitHub said, or why it could not say.
type RunsFetchedMsg struct {
	Error  error
	Branch string
	Runs   []github.WorkflowRun
	Scope  panes.RunsScope
}

// runsWindow is how recent a run has to be to describe a branch's current state
// rather than its history.
const runsWindow = 4 * time.Hour

// runsFallback is how many runs are shown when nothing at all falls inside the
// window. A repository that dispatches twice a week would otherwise report
// nothing, and its last run is still the answer. One run inside the window is
// enough to make the window the answer, so this only fires on an empty one.
const runsFallback = 3

// applyRunWindow keeps the runs recent enough to describe the current state,
// falling back to the newest few when none are. Runs arrive newest first.
func applyRunWindow(runs []github.WorkflowRun) []github.WorkflowRun {
	cutoff := time.Now().Add(-runsWindow)
	kept := make([]github.WorkflowRun, 0, len(runs))

	for i := range runs {
		if runs[i].IsActive() || runs[i].CreatedAt.After(cutoff) {
			kept = append(kept, runs[i])
		}
	}

	if len(kept) > 0 {
		return kept
	}

	if len(runs) > runsFallback {
		return runs[:runsFallback]
	}

	return runs
}

// fetchRunsCmd reads a scope's runs off GitHub rather than out of the local
// dispatch history, which is the only way to answer whether a branch this
// checkout never dispatched from is green.
func (m Model) fetchRunsCmd(scope panes.RunsScope, branch string) tea.Cmd {
	client := m.ghClient
	if client == nil {
		return statusCmd("no GitHub client")
	}

	return func() tea.Msg {
		var (
			runs []github.WorkflowRun
			err  error
		)

		switch scope {
		case panes.ScopeMine:
			runs, err = client.LatestRunsForPRs(github.PRScopeMine, 0)
		case panes.ScopeReviewing:
			runs, err = client.LatestRunsForPRs(github.PRScopeReviewing, 0)
		default:
			runs, err = client.LatestRunsOnBranch(branch, 0)
		}

		if err != nil {
			return RunsFetchedMsg{Scope: scope, Branch: branch, Error: err}
		}

		return RunsFetchedMsg{Scope: scope, Branch: branch, Runs: applyRunWindow(runs)}
	}
}

// handleRunsMsg routes the Runs tab's background messages before the modal
// stack sees them.
func (m Model) handleRunsMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case FetchRunsMsg:
		m.rightPanel.Runs().SetLoading()

		return m, m.fetchRunsCmd(msg.Scope, msg.Branch), true

	case RunsFetchedMsg:
		if msg.Error != nil {
			m.rightPanel.Runs().SetError(msg.Error)
		} else {
			m.rightPanel.Runs().SetRuns(msg.Scope, msg.Branch, msg.Runs)
		}

		return m, nil, true
	}

	return m, nil, false
}

// loadRunsIfNeeded fetches the Runs tab's contents the first time it is opened,
// so a tab nobody visits costs no API call.
func (m Model) loadRunsIfNeeded() tea.Cmd {
	if m.rightPanel.ActiveTab() != panes.TabRuns {
		return nil
	}

	runs := m.rightPanel.Runs()
	if runs.Loaded() || runs.Loading() {
		return nil
	}

	return m.fetchRunsCmd(runs.Scope(), m.branch)
}

// cycleRunsScope moves the Runs tab to the next scope and loads it.
func (m Model) cycleRunsScope() (tea.Model, tea.Cmd) {
	scope := m.rightPanel.Runs().NextScope()

	return m, m.fetchRunsCmd(scope, m.branch)
}

// reloadRuns refetches the current scope.
func (m Model) reloadRuns() (tea.Model, tea.Cmd) {
	runs := m.rightPanel.Runs()
	runs.Invalidate()

	return m, m.fetchRunsCmd(runs.Scope(), m.branch)
}

// diagnoseSelectedRun opens the selected run's log filtered to the failure.
func (m Model) diagnoseSelectedRun() (tea.Model, tea.Cmd) {
	return m, m.selectedRunLogCmd(true)
}

// logsForSelectedRun opens the selected run's whole log.
func (m Model) logsForSelectedRun() (tea.Model, tea.Cmd) {
	return m, m.selectedRunLogCmd(false)
}

func (m Model) selectedRunLogCmd(errorsOnly bool) tea.Cmd {
	run, ok := m.rightPanel.SelectedGitHubRun()
	if !ok {
		return nil
	}

	return func() tea.Msg {
		return FetchLogsMsg{RunID: run.ID, Workflow: run.Name, ErrorsOnly: errorsOnly}
	}
}

func runsActions() []paneAction {
	return []paneAction{
		{key: "d", name: "diagnose the failure", run: Model.diagnoseSelectedRun},
		{key: "R", name: "reload", run: Model.reloadRuns},
		{key: "s", name: "switch scope", run: Model.cycleRunsScope},
		{key: "t", name: nameTimelineAction, run: Model.timelineForSelection},
		{key: "v", name: "view logs", run: Model.logsForSelectedRun},
	}
}

// runsVerdict spells the loaded scope's state for the status bar, so "is my
// branch green" is answered without the Runs tab being open.
func (m Model) runsVerdict() string {
	runs := m.rightPanel.Runs()
	if !runs.Loaded() {
		return ""
	}

	passed, failed, active := runs.Summary()
	if passed+failed+active == 0 {
		return ""
	}

	var verdict strings.Builder

	for _, part := range []struct {
		glyph string
		count int
	}{{"+", passed}, {"x", failed}, {"*", active}} {
		if part.count > 0 {
			verdict.WriteString(" " + strconv.Itoa(part.count) + part.glyph)
		}
	}

	return verdict.String()
}
