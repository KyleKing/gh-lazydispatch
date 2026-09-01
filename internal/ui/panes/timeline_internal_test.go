package panes

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

var timelineBase = time.Date(2026, 8, 31, 18, 10, 45, 0, time.UTC)

func timelineAt(seconds int) time.Time {
	return timelineBase.Add(time.Duration(seconds) * time.Second)
}

// timelineJobs is the CI run the design was drawn from: the failure at 91s did
// not set the run's 127s wall time, which is the thing a status list hides.
func timelineJobs() []github.Job {
	return []github.Job{
		{
			Name: "actionlint", StartedAt: timelineAt(7),
			CompletedAt: timelineAt(17), Conclusion: "success",
		},
		{
			Name: "benchmark", StartedAt: timelineAt(18),
			CompletedAt: timelineAt(127), Conclusion: "success",
		},
		{
			Name: "ci", StartedAt: timelineAt(5), CompletedAt: timelineAt(91), Conclusion: "failure",
			Steps: []github.Step{
				{
					Name: "Set up job", Number: 1,
					StartedAt: timelineAt(6), CompletedAt: timelineAt(7), Conclusion: "success",
				},
				{
					Name: "Run actions/checkout@3d3c42e5", Number: 2,
					StartedAt: timelineAt(7), CompletedAt: timelineAt(8), Conclusion: "success",
				},
				{
					Name: "Run mise run ci", Number: 3,
					StartedAt: timelineAt(12), CompletedAt: timelineAt(88), Conclusion: "failure",
				},
			},
		},
	}
}

func newTimeline(t *testing.T, width int) TimelineModel {
	t.Helper()

	m := NewTimelineModel()
	m.SetSize(width, 20)
	m.SetRun("CI  main  failure", timelineJobs())

	return m
}

// TestTimelinePane_DrawsEveryJobOnOneAxis is the reading the pane exists for:
// the failing job and the job that actually set the wall time are both on
// screen, measured against the same clock.
func TestTimelinePane_DrawsEveryJobOnOneAxis(t *testing.T) {
	t.Parallel()

	view := ansi.Strip(newTimeline(t, 70).ViewContent())

	// The axis spans the earliest start to the latest end, which here is ci's
	// 18:10:50 to benchmark's 18:12:52.
	for _, want := range []string{"actionlint", "benchmark", "ci", "2m02s"} {
		if !strings.Contains(view, want) {
			t.Errorf("the timeline is missing %q:\n%s", want, view)
		}
	}

	if !strings.Contains(view, "✗") || !strings.Contains(view, "✓") {
		t.Errorf("the timeline does not mark outcomes:\n%s", view)
	}
}

// TestTimelinePane_DrillsIntoAJobAndBackOut covers the navigation: enter opens
// the selected job's steps, esc returns to the jobs with that job reselected.
func TestTimelinePane_DrillsIntoAJobAndBackOut(t *testing.T) {
	t.Parallel()

	m := newTimeline(t, 70)
	m.MoveDown()
	m.MoveDown()

	if m.Drilled() {
		t.Fatal("the pane starts drilled in")
	}

	m.Drill()

	if !m.Drilled() {
		t.Fatal("enter did not drill into the selected job")
	}

	view := ansi.Strip(m.ViewContent())
	if !strings.Contains(view, "› ci") {
		t.Errorf("the heading does not name the job drilled into:\n%s", view)
	}

	// The action-reference noise GitHub puts in a step name is stripped.
	if !strings.Contains(view, "actions/checkout") || strings.Contains(view, "3d3c42e5") {
		t.Errorf("step labels still carry the action reference:\n%s", view)
	}

	m.Undrill()

	if m.Drilled() {
		t.Error("esc did not back out to the jobs")
	}

	if !strings.Contains(ansi.Strip(m.ViewContent()), "benchmark") {
		t.Error("backing out did not restore the jobs")
	}
}

// TestTimelinePane_FitsTheWidthItIsGiven guards the frame: lipgloss widens
// every line to the longest, so one row past the pane pushes the layout
// sideways.
func TestTimelinePane_FitsTheWidthItIsGiven(t *testing.T) {
	t.Parallel()

	for _, width := range []int{40, 55, 70, 120} {
		m := newTimeline(t, width)

		for _, drilled := range []bool{false, true} {
			if drilled {
				m.MoveDown()
				m.MoveDown()
				m.Drill()
			}

			for i, line := range strings.Split(m.ViewContent(), "\n") {
				if got := ansi.StringWidth(line); got > width {
					t.Errorf("width %d drilled=%v: line %d is %d cells: %q",
						width, drilled, i+1, got, ansi.Strip(line))
				}
			}
		}
	}
}

func TestTimelinePane_SaysWhenItHasNothingToDraw(t *testing.T) {
	t.Parallel()

	m := NewTimelineModel()
	m.SetSize(70, 20)

	if view := ansi.Strip(m.ViewContent()); !strings.Contains(view, "No run selected") {
		t.Errorf("an empty timeline renders no explanation:\n%s", view)
	}

	// A pane too narrow to draw says so rather than rendering a misleading bar.
	m.SetRun("CI", timelineJobs())
	m.SetSize(20, 20)

	if view := ansi.Strip(m.ViewContent()); !strings.Contains(view, "Not enough room") {
		t.Errorf("a pane too narrow drew something anyway:\n%s", view)
	}
}

// TestTimelinePane_ForgetsADrillDownWhenTheRunChanges keeps a stale index from
// pointing into a different run's jobs.
func TestTimelinePane_ForgetsADrillDownWhenTheRunChanges(t *testing.T) {
	t.Parallel()

	m := newTimeline(t, 70)
	m.MoveDown()
	m.MoveDown()
	m.Drill()

	m.SetRun("other", timelineJobs()[:1])

	if m.Drilled() {
		t.Error("a shorter run kept the old drill-down")
	}

	if strings.TrimSpace(ansi.Strip(m.ViewContent())) == "" {
		t.Error("the pane rendered nothing after the run changed")
	}
}
