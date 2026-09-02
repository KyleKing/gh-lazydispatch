package app

import (
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// keyCopyCommand writes the machine's clipboard, so the sweep names it to skip
// it rather than clobbering whatever the person running the tests had copied.
const keyCopyCommand = "y"

// loadedModel has a row selected in every list, so an action resolves a target
// instead of reporting an empty pane. The verbs behave differently either way,
// and the empty side is what a fresh start already covers.
func loadedModel(t *testing.T) Model {
	t.Helper()

	m := resize(t, newRenderModel(), 120, 40)

	m.rightPanel.Runs().SetRuns(panes.ScopeBranch, "main", []github.WorkflowRun{
		run(11, "CI", time.Minute, github.StatusCompleted, github.ConclusionFailure),
	})
	m.rightPanel.Flaky().SetRuns([]github.WorkflowRun{
		run(12, "Nightly", time.Hour, github.StatusCompleted, github.ConclusionFailure),
		run(12, "Nightly", 2*time.Hour, github.StatusCompleted, github.ConclusionSuccess),
	})
	m.rightPanel.Live().SetRuns([]watcher.WatchedRun{
		{RunID: 13, Workflow: "Deploy", Status: github.StatusInProgress},
	})
	m.syncHistoryEntries()

	return m
}

// actionScope is one place the leader can be opened from: everything actionsFor
// distinguishes, since the menu it returns is the only thing that decides which
// verbs a pane offers.
type actionScope struct {
	place func(*Model)
	name  string
}

func actionScopes() []actionScope {
	focus := func(pane FocusedPane) func(*Model) {
		return func(m *Model) { m.focused = pane }
	}

	tab := func(t panes.RightTab) func(*Model) {
		return func(m *Model) {
			m.focused = PaneRight
			m.rightPanel.SetTab(t)
		}
	}

	return []actionScope{
		{name: paneWorkflows, place: focus(PaneWorkflows)},
		{name: "workflows with marks", place: func(m *Model) {
			m.focused = PaneWorkflows
			m.markedWorkflows.Toggle(m.workflows[0].Filename)
		}},
		{name: "chains", place: focus(PaneChains)},
		{name: "config", place: focus(PaneConfig)},
		{name: "history", place: tab(panes.TabHistory)},
		{name: "history preview", place: func(m *Model) {
			m.focused = PaneRight
			m.rightPanel.SetTab(panes.TabHistory)
			m.viewMode = HistoryPreviewMode
			m.previewingHistoryEntry = m.rightPanel.SelectedHistoryEntry()
		}},
		{name: "live", place: tab(panes.TabLive)},
		{name: "live with marks", place: func(m *Model) {
			m.focused = PaneRight
			m.rightPanel.SetTab(panes.TabLive)
			m.rightPanel.Live().ToggleMark()
		}},
		{name: "flaky", place: tab(panes.TabFlaky)},
		{name: nameRunsTab, place: tab(panes.TabRuns)},
		{name: "timeline", place: func(m *Model) {
			m.focused = PaneRight
			m.rightPanel.SetTab(panes.TabRuns)
			m.detailRunID = 11
			m.detailRunName = "CI"
			m.rightPanel.ShowDetail(nameRunsTab, "CI", detailJobs(false))
		}},
		{name: "drilled timeline", place: func(m *Model) {
			m.focused = PaneRight
			m.rightPanel.SetTab(panes.TabRuns)
			m.detailRunID = 11
			m.detailRunName = "CI"
			m.rightPanel.ShowDetail(nameRunsTab, "CI", detailJobs(false))
			m.rightPanel.Detail().Drill()
		}},
	}
}

// The leader is the only route to most of these verbs, so a menu that offers a
// key nothing answers to, or a verb that cannot run where it is offered, is
// silently a dead entry. Running each one is what catches that.
func TestActionMenu_EveryVerbRunsWhereItIsOffered(t *testing.T) {
	t.Parallel()

	for _, scope := range actionScopes() {
		t.Run(scope.name, func(t *testing.T) {
			t.Parallel()

			m := loadedModel(t)
			scope.place(&m)

			menu := m.actionsFor()
			if menu.title == "" || menu.target == "" {
				t.Fatalf("the menu is titled %q acting on %q", menu.title, menu.target)
			}

			seen := make(map[string]bool, len(menu.actions))

			for _, action := range menu.actions {
				if action.key == "" || action.name == "" || action.run == nil {
					t.Errorf("%+v is not a runnable entry", action)

					continue
				}

				if seen[action.key] {
					t.Errorf("%q is offered twice in one menu", action.key)
				}

				seen[action.key] = true

				if action.key == keyCopyCommand {
					continue
				}

				asModel(t, mustRun(t, m, action))
			}
		})
	}
}

// mustRun runs one verb through the menu's own dispatch, which is the path a
// keypress takes and the only thing that proves the key the menu drew resolves
// back to the verb it was drawn for.
func mustRun(t *testing.T, m Model, action paneAction) Model {
	t.Helper()

	m.pendingActions = []paneAction{action}

	model, _ := m.handleActionResult(modal.ActionResultMsg{Key: action.key})

	return asModel(t, model)
}

// The leader opens on the focused pane and hands its key back to the verb that
// was drawn beside it.
func TestActionMenu_LeaderOpensOnTheFocusedPaneAndRunsItsKey(t *testing.T) {
	t.Parallel()

	m := loadedModel(t)
	m.focused = PaneWorkflows

	m = pressRune(t, m, 'a')
	if !m.modalStack.HasActive() {
		t.Fatal("the leader opened no menu")
	}

	if len(m.pendingActions) == 0 {
		t.Fatal("the menu was drawn without remembering its verbs")
	}

	before := m.watchRun

	m = applyMsg(t, m, modal.ActionResultMsg{Key: "w"})
	if m.watchRun == before {
		t.Error("the key the menu drew did not run its verb")
	}

	if m.pendingActions != nil {
		t.Error("the menu's verbs outlived the menu")
	}
}

// A dismissed menu runs nothing, and a key no menu drew resolves to nothing
// rather than to whichever verb sorts first.
func TestActionMenu_DismissedOrUnknownKeysRunNothing(t *testing.T) {
	t.Parallel()

	m := loadedModel(t)
	m.focused = PaneWorkflows
	m.openActionMenu()

	before := m.watchRun

	for _, key := range []string{"", "Q"} {
		model, cmd := m.handleActionResult(modal.ActionResultMsg{Key: key})
		if cmd != nil {
			t.Errorf("key %q asked for work", key)
		}

		if asModel(t, model).watchRun != before {
			t.Errorf("key %q ran a verb", key)
		}
	}
}
