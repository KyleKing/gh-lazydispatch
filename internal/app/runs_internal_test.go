package app

import (
	"testing"
	"time"

	"github.com/kyleking/aragonite/forge"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

func run(id int64, name string, age time.Duration, status, conclusion string) github.WorkflowRun {
	return github.WorkflowRun{
		ID: id, Name: name, Status: status, Conclusion: conclusion,
		CreatedAt: time.Now().Add(-age),
	}
}

func TestApplyRunWindow(t *testing.T) {
	t.Parallel()

	fresh := run(1, "CI", time.Minute, github.StatusCompleted, github.ConclusionSuccess)
	stale := run(2, "Deploy", 6*time.Hour, github.StatusCompleted, github.ConclusionFailure)
	running := run(3, "Nightly", 3*24*time.Hour, github.StatusInProgress, "")

	tests := []struct {
		name string
		runs []github.WorkflowRun
		want []int64
	}{
		{
			name: "a run still going is current whatever its age",
			runs: []github.WorkflowRun{fresh, running, stale},
			want: []int64{1, 3},
		},
		{
			// A repository that dispatches twice a week would otherwise report
			// nothing at all.
			name: "a quiet repository keeps its newest few rather than nothing",
			runs: []github.WorkflowRun{stale, stale, stale, stale},
			want: []int64{2, 2, 2},
		},
		{
			name: "fewer runs than the fallback are all kept",
			runs: []github.WorkflowRun{stale},
			want: []int64{2},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := applyRunWindow(tt.runs)
			if len(got) != len(tt.want) {
				t.Fatalf("kept %d runs, want %d", len(got), len(tt.want))
			}

			for i, want := range tt.want {
				if got[i].ID != want {
					t.Errorf("run %d is %d, want %d", i, got[i].ID, want)
				}
			}
		})
	}
}

// The Runs tab costs an API call, so opening any other tab must not spend one.
func TestLoadRunsIfNeeded_OnlyOnTheRunsTab(t *testing.T) {
	t.Parallel()

	m := newRenderModel()
	m.focused = PaneHistory

	for _, tab := range tabsUpTo(panes.TabRuns) {
		m.rightPanel.SetTab(tab)

		if cmd := m.loadRunsIfNeeded(); cmd != nil {
			t.Errorf("tab %v asked for a run listing", tab)
		}
	}

	m.rightPanel.SetTab(panes.TabRuns)

	if cmd := m.loadRunsIfNeeded(); cmd == nil {
		t.Fatal("the Runs tab did not ask for a run listing")
	}

	// A second open reuses what the first loaded.
	m.rightPanel.Runs().SetRuns(panes.ScopeBranch, "main", nil)

	if cmd := m.loadRunsIfNeeded(); cmd != nil {
		t.Error("an already loaded Runs tab asked again")
	}
}

func tabsUpTo(limit panes.RightTab) []panes.RightTab {
	tabs := make([]panes.RightTab, 0, int(limit))
	for tab := panes.TabHistory; tab < limit; tab++ {
		tabs = append(tabs, tab)
	}

	return tabs
}

func TestRunsVerdict_ReportsTheBranchStateInTheStatusBar(t *testing.T) {
	t.Parallel()

	m := newRenderModel()

	if verdict := m.runsVerdict(); verdict != "" {
		t.Errorf("an unloaded Runs tab reported %q", verdict)
	}

	m.rightPanel.Runs().SetRuns(panes.ScopeBranch, "main", []github.WorkflowRun{
		run(1, "CI", time.Minute, github.StatusCompleted, github.ConclusionSuccess),
		run(2, "Deploy", time.Minute, github.StatusCompleted, github.ConclusionFailure),
		run(3, "Nightly", time.Minute, github.StatusInProgress, ""),
	})

	if verdict := m.runsVerdict(); verdict != " 1+ 1x 1*" {
		t.Errorf("verdict is %q, want one of each", verdict)
	}
}

// A pull request scope answers with rollups, and the row a reader picks has to
// reach the runs behind it.
func TestRunsPRScope_RollupsReportAndDrillIntoTheirBranch(t *testing.T) {
	t.Parallel()

	m := newRenderModel()
	m.rightPanel.SetTab(panes.TabRuns)
	m.rightPanel.Runs().SetPRs(panes.ScopeMine, []github.PullRequest{
		{
			Number: 7, Title: "Add the runs tab", HeadRef: "topic",
			Checks: forge.ChecksStatus{Total: 3, Passing: 2, Failing: 1},
		},
		{Number: 8, Title: "Bump the linter", HeadRef: "lint", Checks: forge.ChecksStatus{Total: 1, Pending: 1}},
	})

	if verdict := m.runsVerdict(); verdict != " 1x 1*" {
		t.Errorf("verdict is %q, want a failing and a pending pull request", verdict)
	}

	if got := m.runsTarget(); got != "#7" {
		t.Errorf("action target is %q, want the pull request under the cursor", got)
	}

	model, cmd := m.openSelectedRunsRow()
	if cmd == nil {
		t.Fatal("opening a pull request row asked for nothing")
	}

	drilled, ok := model.(Model)
	if !ok {
		t.Fatalf("opening a row returned %T, want a Model", model)
	}

	runs := drilled.rightPanel.Runs()
	if runs.Scope() != panes.ScopeBranch || runs.Ref() != "topic" {
		t.Errorf("drilled into %v/%q, want the branch scope on topic", runs.Scope(), runs.Ref())
	}

	if runs.Loaded() {
		t.Error("the drilled scope reported itself loaded before its runs arrived")
	}
}
