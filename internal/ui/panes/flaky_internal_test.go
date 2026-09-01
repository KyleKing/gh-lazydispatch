package panes

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

func flakyRun(name, path, branch string, age time.Duration, conclusion string) github.WorkflowRun {
	status := github.StatusCompleted
	if conclusion == "" {
		status = github.StatusInProgress
	}

	return github.WorkflowRun{
		Name: name, Path: path, HeadBranch: branch, Event: "push",
		Status: status, Conclusion: conclusion, CreatedAt: time.Now().Add(-age),
	}
}

func flakySample() []github.WorkflowRun {
	return []github.WorkflowRun{
		flakyRun("CI", ".github/workflows/ci.yml", "main", time.Minute, github.ConclusionSuccess),
		flakyRun("CI", ".github/workflows/ci.yml", "topic", 2*time.Minute, github.ConclusionFailure),
		flakyRun("CI", ".github/workflows/ci.yml", "main", 3*time.Minute, github.ConclusionSuccess),
		flakyRun("CI", ".github/workflows/ci.yml", "main", 4*time.Minute, github.ConclusionFailure),
		flakyRun("Release", ".github/workflows/release.yml", "main", 5*time.Minute, github.ConclusionSuccess),
		flakyRun("Nightly", ".github/workflows/nightly.yml", "main", 6*time.Minute, ""),
	}
}

// One listing answers both questions the pane asks: which workflow is least
// reliable, and which of one workflow's runs failed.
func TestFlakyModel_RatesTheListingAndNarrowsToOneWorkflow(t *testing.T) {
	t.Parallel()

	m := NewFlakyModel()
	m.SetSize(80, 20)
	m.SetRuns(flakySample())

	// Flakiest first, and a workflow with nothing finished has no rate rather
	// than a zero one, so it does not outrank a genuinely failing workflow.
	wantOrder := []string{"CI", "Release", "Nightly"}
	if len(m.rates) != len(wantOrder) {
		t.Fatalf("grouped into %d workflows, want %d", len(m.rates), len(wantOrder))
	}

	for i, want := range wantOrder {
		if got := m.rates[i].label(); got != want {
			t.Errorf("row %d is %s, want %s", i, got, want)
		}
	}

	if got := m.rates[0].rate(); got != 50 {
		t.Errorf("CI passed %d%% of its finished runs, want 50", got)
	}

	if got := m.rates[2].rate(); got != -1 {
		t.Errorf("a workflow with nothing finished reported %d%%, want no rate", got)
	}

	rows, worst := m.Summary()
	if rows != 3 || worst != 50 {
		t.Errorf("summary is %d rows worst %d, want 3 and 50", rows, worst)
	}

	// Narrowing costs no second fetch: the same listing is re-derived.
	m.SetWorkflow("ci.yml")

	if got := len(m.filtered()); got != 4 {
		t.Errorf("ci.yml has %d runs, want 4", got)
	}

	view := ansi.Strip(m.ViewContent())
	if !strings.Contains(view, "topic") {
		t.Errorf("the per-workflow view does not name the branch a run failed on:\n%s", view)
	}

	if !strings.Contains(view, "ci.yml") {
		t.Errorf("the per-workflow view does not say which workflow it measures:\n%s", view)
	}
}

// A workflow that titles each run after the pull request it opened is still one
// workflow, and is one row named by its file rather than one row per title.
func TestFlakyModel_GroupsByFileAndNamesAMixedGroupByIt(t *testing.T) {
	t.Parallel()

	m := NewFlakyModel()
	m.SetSize(80, 20)
	m.SetRuns([]github.WorkflowRun{
		flakyRun("Update foo", ".github/workflows/dependabot.yml", "dep/foo", time.Minute, github.ConclusionFailure),
		flakyRun("Update bar", ".github/workflows/dependabot.yml", "dep/bar", 2*time.Minute, github.ConclusionSuccess),
	})

	if len(m.rates) != 1 {
		t.Fatalf("one workflow grouped into %d rows", len(m.rates))
	}

	if got := m.rates[0].label(); got != "dependabot.yml" {
		t.Errorf("a group with no shared title is named %q, want its file", got)
	}
}

// An aggregate row is many runs, so no single run is selected there and the
// per-run verbs are not offered anything to act on.
func TestFlakyModel_OnlyThePerWorkflowViewSelectsARun(t *testing.T) {
	t.Parallel()

	m := NewFlakyModel()
	m.SetSize(80, 20)
	m.SetRuns(flakySample())

	if _, ok := m.SelectedRun(); ok {
		t.Error("an aggregate row reported a single run")
	}

	m.SetWorkflow("ci.yml")
	m.MoveDown()

	run, ok := m.SelectedRun()
	if !ok {
		t.Fatal("the per-workflow view selected no run")
	}

	if run.HeadBranch != "topic" {
		t.Errorf("selected the run on %s, want the second row", run.HeadBranch)
	}
}
