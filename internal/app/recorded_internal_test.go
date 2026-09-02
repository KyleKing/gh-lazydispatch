package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/cache"
	"github.com/kyleking/aragonite/ghcassette"

	"github.com/kyleking/gh-lazydispatch/internal/testrepo"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

func TestMain(m *testing.M) {
	code := m.Run()

	ghcassette.RemoveStub()
	os.Exit(code)
}

// The repository and run the cassette was recorded against. The run is a
// completed dispatch of a demo workflow that exists to be dispatched and is
// never removed, so re-recording reads history and creates nothing.
const (
	recordedRepo     = "KyleKing/gh-lazydispatch"
	recordedWorkflow = "demo-test.yml"
	recordedBranch   = "main"
	recordedRun      = 33467036043
)

// recordedApp builds the model the way main does, with a real client, and
// points this process's gh calls at the cassette.
func recordedApp(t *testing.T, name string) (Model, *ghcassette.Session) {
	t.Helper()

	path, err := filepath.Abs(filepath.Join("testdata", "cassettes", name+".golden"))
	if err != nil {
		t.Fatalf("resolving the cassette path: %v", err)
	}

	testrepo.RequirePublic(t, recordedRepo)

	s := ghcassette.Start(t, path)
	s.Apply(t)

	// A read this process already cached would never reach the cassette, so a
	// replay would pass while playing nothing.
	cache.ClearAll()

	m := New([]workflow.File{{
		Name: "Demo Test Suite", Filename: recordedWorkflow,
		On: workflow.OnTrigger{Dispatch: &workflow.Dispatch{}},
	}}, testHistory(), recordedRepo)

	m.branch = recordedBranch

	if m.ghClient == nil {
		t.Fatal("the model was built without a client, so nothing would reach the cassette")
	}

	return resize(t, m, 120, 40), s
}

// TestRecorded_TheRunsTabOpensARunAndItsLog is the path the tool exists for,
// end to end against the bytes GitHub sent: a branch's state, the run behind a
// row, and that run's log. Everything between the fetches is covered in
// process; what only a recording covers is the payload itself, where a moved
// field turns a green branch into an empty pane and a misread log prefix turns
// a failure into a blank viewer.
//
// One cassette holds the whole path because the branch listing is a megabyte
// of JSON: a second test wanting a second copy of it is worth restructuring to
// avoid.
//
//nolint:paralleltest // Apply sets the process environment
func TestRecorded_TheRunsTabOpensARunAndItsLog(t *testing.T) {
	m, s := recordedApp(t, "runs-journey")

	m.focused = PaneRight
	m.rightPanel.SetTab(panes.TabRuns)

	m = drainCmd(t, m, m.fetchRunsCmd(panes.ScopeBranch, recordedBranch))
	assertRunsTabAnswered(t, m)

	m = drainCmd(t, m, m.fetchTimeline(FetchTimelineMsg{RunID: recordedRun, Title: "Demo Test Suite"}))
	assertRunOpenedOnItsOwnTimings(t, m)

	m = drainCmd(t, m, m.fetchLogs(FetchLogsMsg{RunID: recordedRun, Workflow: recordedWorkflow}))
	assertLogOpenedInTheViewer(t, m)

	s.RequireAllPlayed(t)
}

// The question a reader opens with is whether their branch is green, and the
// rows answering it are laid out against widths resolved from the recorded
// names rather than from names chosen to fit.
func assertRunsTabAnswered(t *testing.T, m Model) {
	t.Helper()

	runs := m.rightPanel.Runs()
	if !runs.Loaded() {
		t.Fatal("the Runs tab never loaded")
	}

	if _, ok := runs.SelectedRun(); !ok {
		t.Fatal("the Runs tab loaded with no row under the cursor")
	}

	if total, _, _ := runs.Summary(); total == 0 {
		t.Error("the Runs tab summarized zero runs")
	}

	view := ansi.Strip(m.rightPanel.View())
	if !strings.Contains(view, "Runs") {
		t.Errorf("the tab bar does not name the tab it is drawing:\n%s", view)
	}

	assertFits(t, m.View().Content, m.width, m.height)
}

// A run is drawn on one time axis, which needs each job's own start and each
// step's. Drilling in is what makes a step's log reachable, and the name it
// hands back has to be GitHub's rather than the label drawn beside the bar.
func assertRunOpenedOnItsOwnTimings(t *testing.T, m Model) {
	t.Helper()

	detail := m.rightPanel.Detail()
	if detail == nil {
		t.Fatal("the run did not open")
	}

	if detail.Heading() == "" {
		t.Error("the open run has no heading")
	}

	detail.Drill()

	step, ok := detail.SelectedStep()
	if !ok {
		t.Fatal("a drilled job offered no step to open")
	}

	// timelineLabel truncates with an ellipsis, so one here means the label
	// was handed back where the step's own name belongs.
	if step == "" || strings.Contains(step, "\u2026") {
		t.Errorf("the step is named %q, which is the drawn label rather than GitHub's name", step)
	}
}

// gh prefixes every log line with "job\tstep\ttimestamp", which is the shape no
// hand-written fixture had and the reason the viewer once drew nothing.
func assertLogOpenedInTheViewer(t *testing.T, m Model) {
	t.Helper()

	viewer, ok := m.modalStack.Current().(*modal.LogsViewerModal)
	if !ok {
		t.Fatalf("the top of the stack is %T, want the log viewer", m.modalStack.Current())
	}

	view := ansi.Strip(viewer.View())
	if strings.TrimSpace(view) == "" {
		t.Fatal("the viewer opened empty")
	}

	if strings.Contains(view, "\t") {
		t.Errorf("the viewer is drawing gh's own prefix:\n%s", view)
	}
}
