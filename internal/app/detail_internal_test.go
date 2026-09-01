package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

func detailJobs(running bool) []github.Job {
	start := time.Now().Add(-2 * time.Minute)

	done := start.Add(time.Minute)
	if running {
		done = time.Time{}
	}

	return []github.Job{
		{
			Name: "build", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			StartedAt: start, CompletedAt: start.Add(30 * time.Second),
			Steps: []github.Step{{
				Name: "Run actions/checkout@abc", Conclusion: github.ConclusionSuccess,
				StartedAt: start, CompletedAt: start.Add(5 * time.Second),
			}},
		},
		{Name: "test", Status: github.StatusInProgress, StartedAt: start, CompletedAt: done},
	}
}

// A run's timeline is a drill-down of the row that names it, not a peer tab, so
// it carries the list it came from in a breadcrumb and escape backs out of it.
func TestRunDetail_OpensFromARowAndBacksOutOfIt(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneRight
	m.rightPanel.SetTab(panes.TabRuns)
	m.rightPanel.Runs().SetRuns(panes.ScopeBranch, "main", []github.WorkflowRun{
		run(11, "CI", time.Minute, github.StatusCompleted, github.ConclusionFailure),
	})

	model, _, handled := m.handleTimelineMsg(TimelineFetchedMsg{RunID: 11, Title: "CI", Jobs: detailJobs(false)})
	if !handled {
		t.Fatal("the timeline message was not routed")
	}

	opened, ok := model.(Model)
	if !ok {
		t.Fatalf("routing returned %T, want a Model", model)
	}

	if opened.rightPanel.Detail() == nil {
		t.Fatal("the run did not open in the right panel")
	}

	view := opened.View().Content
	if !strings.Contains(view, "Runs › CI") {
		t.Errorf("the breadcrumb does not name the list the run came from:\n%s", view)
	}

	// Every per-run verb resolves against the open run rather than the row
	// under a cursor that is no longer on screen.
	runID, name, found := opened.selectedRun()
	if !found || runID != 11 || name != "CI" {
		t.Errorf("the open run resolves to %d/%q, want 11/CI", runID, name)
	}

	opened = pressSpecial(t, opened, tea.KeyEnter)
	if !opened.rightPanel.Detail().Drilled() {
		t.Fatal("enter did not drill into the selected job's steps")
	}

	opened = pressSpecial(t, opened, tea.KeyEscape)
	if opened.rightPanel.Detail() == nil || opened.rightPanel.Detail().Drilled() {
		t.Fatal("escape did not peel exactly the job's steps")
	}

	opened = pressSpecial(t, opened, tea.KeyEscape)
	if opened.rightPanel.Detail() != nil {
		t.Error("the second escape did not close the run")
	}
}

// The redraw tick is what makes an open bar grow, and it has to stop once every
// bar has an end, since a closed timeline draws the same picture forever.
func TestRunDetail_TicksOnlyWhileSomethingIsStillRunning(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneRight

	m.rightPanel.ShowDetail(nameRunsTab, "CI", detailJobs(true))

	if m.timelineTick() == nil {
		t.Error("a run still going scheduled no redraw")
	}

	m.rightPanel.ShowDetail(nameRunsTab, "CI", detailJobs(false))

	if m.timelineTick() != nil {
		t.Error("a finished run kept asking for redraws")
	}

	m.rightPanel.CloseDetail()

	if m.timelineTick() != nil {
		t.Error("a closed detail kept asking for redraws")
	}
}

// Tab walks the panes the layout actually draws. A repository with no chains
// draws no chains pane, so stopping on it would be a stop with nothing on it.
func TestStepPane_SkipsThePaneTheLayoutDoesNotDraw(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneWorkflows

	if got := m.stepPane(1); got != PaneChains {
		t.Errorf("tab landed on pane %d with chains configured, want the chains pane", got)
	}

	m.chains.SetChains(nil)

	if got := m.stepPane(1); got != PaneConfig {
		t.Errorf("tab landed on pane %d with no chains configured, want the config pane", got)
	}
}

// `l` and `h` cross between the columns, and `h` returns to the left pane that
// had focus rather than to the top of the column.
func TestPaneKeys_CrossBetweenTheColumnsAndComeBack(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focusPane(PaneConfig)

	m = pressRune(t, m, 'l')
	if m.focused != PaneRight {
		t.Fatalf("l left focus on pane %d, want the right panel", m.focused)
	}

	m = pressRune(t, m, 'h')
	if m.focused != PaneConfig {
		t.Errorf("h returned to pane %d, want the pane that had focus", m.focused)
	}
}

// The Flaky tab reads one page of runs and derives both of its views from it,
// so walking the workflow list on the left costs no further API call.
func TestFlakyTab_LoadsOnceAndFollowsTheWorkflowSelection(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneRight
	m.rightPanel.SetTab(panes.TabFlaky)

	if cmd := m.loadFlakyIfNeeded(); cmd == nil {
		t.Fatal("opening the Flaky tab asked for nothing")
	}

	model, _, handled := m.handleFlakyMsg(FlakyFetchedMsg{Runs: []github.WorkflowRun{
		{
			Name: "CI", Path: ".github/workflows/" + m.workflows[0].Filename, Status: github.StatusCompleted,
			Conclusion: github.ConclusionFailure, CreatedAt: time.Now(),
		},
	}})
	if !handled {
		t.Fatal("the fetched runs were not routed")
	}

	loaded, ok := model.(Model)
	if !ok {
		t.Fatalf("routing returned %T, want a Model", model)
	}

	if cmd := loaded.loadFlakyIfNeeded(); cmd != nil {
		t.Error("an already loaded Flaky tab asked again")
	}

	// The pane narrowed to the selected workflow without a second fetch.
	if _, found := loaded.rightPanel.Flaky().SelectedRun(); !found {
		t.Error("the pane did not narrow to the selected workflow's runs")
	}

	if _, cmd := loaded.reloadFlaky(); cmd == nil {
		t.Error("reload asked for nothing")
	}
}
