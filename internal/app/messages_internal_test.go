package app

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// routedMessages is one instance of every message Update dispatches by type.
// StartLogStreamMsg is deliberately absent: handling it starts a poller that
// reaches GitHub on its first tick, so it cannot be swept in-process.
func routedMessages() []struct {
	msg  tea.Msg
	name string
} {
	entry := frecency.HistoryEntry{
		Type: frecency.EntryTypeChain, ChainName: "demo", Branch: "main",
		StepResults: []frecency.ChainStepResult{
			{Workflow: "ci.yml", Status: "completed", Conclusion: "success", RunID: 1},
			{Workflow: "deploy.yml", Status: "failed", Conclusion: "failure", RunID: 2},
		},
	}

	return []struct {
		msg  tea.Msg
		name string
	}{
		{name: "ChainUpdateMsg", msg: ChainUpdateMsg{Update: chain.ChainUpdate{State: chain.ChainState{
			ChainName: "demo", Status: chain.ChainCompleted,
		}}}},
		{name: "EnvironmentsFetchedMsg", msg: EnvironmentsFetchedMsg{Names: []string{"staging"}}},
		{name: "FetchFlakyMsg", msg: FetchFlakyMsg{}},
		{name: "FetchLogsMsg", msg: FetchLogsMsg{RunID: 1}},
		{name: "FetchRunsMsg", msg: FetchRunsMsg{Scope: panes.ScopeBranch, Branch: "main"}},
		{name: "FetchTimelineMsg", msg: FetchTimelineMsg{RunID: 1, Title: "CI"}},
		{name: "FlakyFetchedMsg", msg: FlakyFetchedMsg{Runs: []github.WorkflowRun{
			run(1, "CI", time.Minute, github.StatusCompleted, github.ConclusionFailure),
		}}},
		{name: "FlakyFetchedMsg failed", msg: FlakyFetchedMsg{Error: errRefused}},
		{name: "HistoryViewLogsMsg", msg: panes.HistoryViewLogsMsg{Entry: entry}},
		{name: "LogStreamUpdateMsg", msg: LogStreamUpdateMsg{Update: logs.StreamUpdate{}}},
		{name: "LogsFetchedMsg", msg: LogsFetchedMsg{Logs: logsWithOneEntry(), RunID: 1}},
		{name: "LogsFetchedMsg failed", msg: LogsFetchedMsg{Error: errRefused}},
		{name: "RunMutationDoneMsg", msg: RunMutationDoneMsg{Kind: modal.RunActionCancel, RunID: 1}},
		{name: "RunUpdateMsg", msg: RunUpdateMsg{Update: watcher.RunUpdate{}}},
		{name: "RunsFetchedMsg", msg: RunsFetchedMsg{Scope: panes.ScopeBranch, Branch: "main"}},
		{name: "RunsFetchedMsg failed", msg: RunsFetchedMsg{Error: errRefused}},
		{name: "RunsFetchedMsg pull requests", msg: RunsFetchedMsg{Scope: panes.ScopeMine}},
		{name: "ShowLogsViewerMsg", msg: ShowLogsViewerMsg{Logs: logsWithOneEntry(), RunID: 1}},
		{name: "StopLogStreamMsg", msg: StopLogStreamMsg{RunID: 1}},
		{name: "TimelineFetchedMsg", msg: TimelineFetchedMsg{RunID: 1, Title: "CI", Jobs: detailJobs(false)}},
		{name: "TimelineFetchedMsg failed", msg: TimelineFetchedMsg{Error: errRefused}},
		{name: "TimelineTickMsg", msg: TimelineTickMsg{}},
		{name: "modal.ActionResultMsg", msg: modal.ActionResultMsg{Key: "w"}},
		{name: "modal.BranchResultMsg", msg: modal.BranchResultMsg{Value: "topic"}},
		{name: "modal.ChainConfirmResultMsg", msg: modal.ChainConfirmResultMsg{
			ChainName: testChainName, Branch: "main", Confirmed: true,
		}},
		{name: "modal.ChainSelectResultMsg", msg: modal.ChainSelectResultMsg{
			ChainName: testChainName, Chain: testChainConfig().Chains[testChainName],
		}},
		{name: "modal.ChainStatusStopMsg", msg: modal.ChainStatusStopMsg{}},
		{name: "modal.ChainStatusViewLogsMsg", msg: modal.ChainStatusViewLogsMsg{Branch: "main"}},
		{name: "modal.ChainVariableResultMsg", msg: modal.ChainVariableResultMsg{
			ChainName: testChainName, Variables: map[string]string{testChainVar: testValueStaging},
		}},
		{name: "modal.ChainVariableResultMsg canceled", msg: modal.ChainVariableResultMsg{Canceled: true}},
		{name: "modal.ConfirmResultMsg", msg: modal.ConfirmResultMsg{Value: true}},
		{name: "modal.FilterResultMsg", msg: modal.FilterResultMsg{Value: "env"}},
		{name: "modal.FilterResultMsg canceled", msg: modal.FilterResultMsg{Canceled: true}},
		{name: "modal.InputResultMsg", msg: modal.InputResultMsg{Value: "production"}},
		{name: "modal.LiveViewClearAllMsg", msg: modal.LiveViewClearAllMsg{}},
		{name: "modal.LiveViewClearMsg", msg: modal.LiveViewClearMsg{RunID: 1}},
		{name: "modal.RemapResultMsg", msg: modal.RemapResultMsg{Decisions: []modal.RemapDecision{
			{OriginalName: "env", Action: modal.RemapActionDrop},
		}}},
		{name: "modal.ResetResultMsg", msg: modal.ResetResultMsg{Confirmed: true}},
		{name: "modal.RunActionResultMsg", msg: modal.RunActionResultMsg{
			Kind: modal.RunActionRerun, RunID: 1, Confirmed: true,
		}},
		{name: "modal.RunConfirmResultMsg", msg: modal.RunConfirmResultMsg{
			Config: runner.RunConfig{Workflow: "ci.yml", Branch: "main"},
		}},
		{name: "modal.SelectResultMsg", msg: modal.SelectResultMsg{Value: "production"}},
		{name: "modal.ValidationErrorResultMsg", msg: modal.ValidationErrorResultMsg{Override: true}},
	}
}

// Update routes a background message by type through a chain of handlers, so a
// message two of them claim is handled twice and one nobody claims is dropped
// in silence. The AST check proves a case exists; this proves it runs.
func TestUpdate_EveryRoutedMessageIsClaimedByExactlyOneHandler(t *testing.T) {
	t.Parallel()

	for _, tt := range routedMessages() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := loadedModel(t)
			m.wfdConfig = testChainConfig()

			claims := 0

			for _, handle := range m.asyncHandlers() {
				model, _, handled := handle(tt.msg)
				if !handled {
					continue
				}

				claims++

				asModel(t, model)
			}

			if claims != 1 {
				t.Errorf("%d handlers claimed the message, want exactly one", claims)
			}
		})
	}
}

// Every one of these also has to survive the full Update, which routes it past
// the modal stack and the key handlers before the async chain sees it.
func TestUpdate_RoutedMessagesSurviveTheWholeLoop(t *testing.T) {
	t.Parallel()

	for _, tt := range routedMessages() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := loadedModel(t)
			m.wfdConfig = testChainConfig()

			updated, _ := m.Update(tt.msg)
			asModel(t, updated)
		})
	}
}

// A history entry is a past dispatch, so opening one previews what would be
// sent and the second press sends it. Sending on the first press would make a
// list you scroll through a list you dispatch from by accident.
func TestHistoryEntry_PreviewsBeforeItRuns(t *testing.T) {
	t.Parallel()

	m := loadedModel(t)
	m.focused = PaneRight
	m.rightPanel.SetTab(panes.TabHistory)

	entry := m.rightPanel.SelectedHistoryEntry()
	if entry == nil {
		t.Fatal("the history pane has nothing selected")
	}

	model, _ := m.enterHistoryEntry()

	previewed := asModel(t, model)
	if previewed.viewMode != HistoryPreviewMode || previewed.modalStack.HasActive() {
		t.Fatal("the first press did not stop at a preview")
	}

	model, _ = previewed.enterHistoryEntry()

	ran := asModel(t, model)
	if ran.viewMode != WorkflowListMode || !ran.modalStack.HasActive() {
		t.Fatal("the second press did not ask to run it")
	}

	if ran.branch != entry.Branch {
		t.Errorf("the dispatch ref is %q, want the entry's %q", ran.branch, entry.Branch)
	}
}

// Remapping rewrites the entry being previewed, which is what makes a stale
// entry runnable: dropping an input that no longer exists and moving one that
// was renamed.
func TestRemapResult_RewritesThePreviewedEntry(t *testing.T) {
	t.Parallel()

	m := loadedModel(t)
	m.previewingHistoryEntry = &frecency.HistoryEntry{
		Workflow: "deploy.yml", Branch: "main",
		Inputs: map[string]string{"env": "prod", "gone": "1"},
	}

	model, _ := m.handleRemapResult(modal.RemapResultMsg{
		Decisions: []modal.RemapDecision{
			{OriginalName: "env", Action: modal.RemapActionMap, NewName: "environment"},
			{OriginalName: "gone", Action: modal.RemapActionDrop},
		},
	})

	after := asModel(t, model)

	inputs := after.previewingHistoryEntry.Inputs
	if inputs["environment"] != "prod" {
		t.Errorf("the renamed input is %q, want prod", inputs["environment"])
	}

	if _, still := inputs["gone"]; still {
		t.Error("the dropped input survived")
	}

	// With nothing being previewed there is nothing to rewrite, and the
	// decisions are dropped rather than applied to the live inputs.
	model, _ = loadedModel(t).handleRemapResult(modal.RemapResultMsg{
		Decisions: []modal.RemapDecision{{OriginalName: "env", Action: modal.RemapActionDrop}},
	})

	if asModel(t, model).previewingHistoryEntry != nil {
		t.Error("remapping invented an entry to rewrite")
	}
}

// A dispatch that failed used to look exactly like one that worked, because
// nothing handled the message that reported it.
func TestExecutionDone_ReportsAFailureAndOpensTheListItChanged(t *testing.T) {
	t.Parallel()

	model, _ := loadedModel(t).handleExecutionDone(executionDoneMsg{err: errRefused})
	if asModel(t, model).rightPanel.ActiveTab() == panes.TabLive {
		t.Error("a failed dispatch moved to the Live tab")
	}

	watched := loadedModel(t)
	watched.watchRun = true

	model, _ = watched.handleExecutionDone(executionDoneMsg{})

	after := asModel(t, model)
	if after.rightPanel.ActiveTab() != panes.TabLive || after.focused != PaneRight {
		t.Error("a watched dispatch did not open the Live tab")
	}

	model, _ = loadedModel(t).handleExecutionDone(executionDoneMsg{})
	if asModel(t, model).rightPanel.ActiveTab() != panes.TabHistory {
		t.Error("an unwatched dispatch did not open the History tab")
	}
}

// A chain read back out of history has to draw the same shape it drew while it
// ran, since that is what the log viewer reads to find each step's run.
func TestReconstructChainState_KeepsEachStepsOutcome(t *testing.T) {
	t.Parallel()

	state := reconstructChainStateFromHistory(frecency.HistoryEntry{
		ChainName: "demo",
		StepResults: []frecency.ChainStepResult{
			{Workflow: "ci.yml", Status: "completed", RunID: 1},
			{Workflow: "deploy.yml", Status: "failed", RunID: 2},
			{Workflow: "notify.yml", Status: "skipped"},
			{Workflow: "later.yml", Status: "pending"},
			{Workflow: "now.yml", Status: "running"},
			{Workflow: "held.yml", Status: "waiting"},
		},
	})

	want := []chain.StepStatus{
		chain.StepCompleted, chain.StepFailed, chain.StepSkipped,
		chain.StepPending, chain.StepRunning, chain.StepWaiting,
	}

	for i, status := range want {
		if state.StepStatuses[i] != status {
			t.Errorf("step %d reads as %v, want %v", i, state.StepStatuses[i], status)
		}
	}

	if state.StepResults[1].RunID != 2 || state.CurrentStep != len(want)-1 {
		t.Errorf("the reconstructed state is %+v", state)
	}
}

// These keys act on whatever the right panel lists, and each is inert outside
// the tab it means something in rather than silently doing nothing elsewhere.
func TestRunListKeys_ActOnlyWhereTheyMeanSomething(t *testing.T) {
	t.Parallel()

	tabs := map[string]panes.RightTab{
		"history": panes.TabHistory, "live": panes.TabLive,
		"flaky": panes.TabFlaky, "runs": panes.TabRuns,
	}

	for _, focus := range []FocusedPane{PaneWorkflows, PaneRight} {
		for name, tab := range tabs {
			t.Run(name, func(t *testing.T) {
				t.Parallel()

				m := loadedModel(t)
				m.focused = focus
				m.rightPanel.SetTab(tab)

				for _, code := range []rune{'s', 'R', 'd', 'D', 'v', 'e'} {
					m = pressRune(t, m, code)
				}
			})
		}
	}
}
