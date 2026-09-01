package timeline_test

import (
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/ui/timeline"
)

var base = time.Date(2026, 8, 31, 18, 10, 45, 0, time.UTC)

func at(seconds int) time.Time { return base.Add(time.Duration(seconds) * time.Second) }

// runSpans is the CI run the design was drawn from: six jobs, staggered
// starts, and a failure that did not set the wall time.
func runSpans() []timeline.Span {
	return []timeline.Span{
		{Label: "actionlint", Start: at(7), End: at(17), Conclusion: "success"},
		{Label: "project", Start: at(0), End: at(15), Conclusion: "success"},
		{Label: "benchmark", Start: at(18), End: at(127), Conclusion: "success"},
		{Label: "ci", Start: at(5), End: at(91), Conclusion: "failure"},
		{Label: "lint", Start: at(20), End: at(115), Conclusion: "success"},
		{Label: "hooks", Start: at(19), End: at(48), Conclusion: "success"},
	}
}

// TestLay_SharesOneWindowAcrossRows is the invariant the whole picture rests
// on: every row is measured against the run's own wall clock, so two bars of
// the same length took the same time. Laying each row out against its own
// duration would make the 10-second job look like the 109-second one.
func TestLay_SharesOneWindowAcrossRows(t *testing.T) {
	t.Parallel()

	layout := timeline.Lay(runSpans(), 60, time.Time{})

	if got := layout.Span; got != 127*time.Second {
		t.Fatalf("axis covers %v, want the run's full 127s", got)
	}

	byLabel := make(map[string]timeline.Row, len(layout.Rows))
	for _, row := range layout.Rows {
		byLabel[row.Label] = row
	}

	// project starts first, so it anchors the window at zero.
	if got := byLabel["project"].Offset; got != 0 {
		t.Errorf("the earliest job starts at cell %d, want 0", got)
	}

	// benchmark ends last, so it reaches the far edge.
	last := byLabel["benchmark"]
	if got := last.Offset + last.Width; got != layout.Track {
		t.Errorf("the last job ends at cell %d, want the track's %d", got, layout.Track)
	}

	// A job that ran ten times longer draws roughly ten times wider.
	if ratio := float64(last.Width) / float64(byLabel["actionlint"].Width); ratio < 8 || ratio > 14 {
		t.Errorf("109s is %.1fx the width of 10s, want about 11x", ratio)
	}
}

// TestLay_KeepsEveryRowInsideTheTrack guards the render: a row wider than the
// track, or starting past its end, corrupts the frame around it.
func TestLay_KeepsEveryRowInsideTheTrack(t *testing.T) {
	t.Parallel()

	for _, track := range []int{8, 20, 55, 200} {
		layout := timeline.Lay(runSpans(), track, time.Time{})

		for _, row := range layout.Rows {
			if row.Offset < 0 || row.Width < 1 || row.Offset+row.Width > track {
				t.Errorf("track %d: %q occupies [%d,%d), outside [0,%d)",
					track, row.Label, row.Offset, row.Offset+row.Width, track)
			}
		}
	}
}

// TestLay_DrawsAMomentaryStep keeps a step too short to measure from vanishing:
// a run's setup steps take under a second and still need to show they ran.
func TestLay_DrawsAMomentaryStep(t *testing.T) {
	t.Parallel()

	layout := timeline.Lay([]timeline.Span{
		{Label: "Set up job", Start: at(0), End: at(0), Conclusion: "success"},
		{Label: "mise run ci", Start: at(1), End: at(90), Conclusion: "failure"},
	}, 40, time.Time{})

	if got := layout.Rows[0].Width; got != 1 {
		t.Errorf("a zero-length step is %d cells wide, want 1", got)
	}
}

// TestLay_ClosesARunningSpanAtNow covers a run still in flight, where the
// axis has no end of its own yet.
func TestLay_ClosesARunningSpanAtNow(t *testing.T) {
	t.Parallel()

	layout := timeline.Lay([]timeline.Span{
		{Label: "done", Start: at(0), End: at(30), Conclusion: "success"},
		{Label: "running", Start: at(10)},
	}, 40, at(60))

	if got := layout.Span; got != 60*time.Second {
		t.Errorf("axis covers %v, want the 60s up to now", got)
	}

	running := layout.Rows[1]
	if !running.Running {
		t.Error("a span with no end is not marked running")
	}

	if got := running.Offset + running.Width; got != layout.Track {
		t.Errorf("the running bar ends at cell %d, want the track's %d", got, layout.Track)
	}

	if got := running.Duration; got != 50*time.Second {
		t.Errorf("the running bar reports %v, want 50s elapsed", got)
	}
}

func TestLay_RefusesATrackTooNarrowToSayAnything(t *testing.T) {
	t.Parallel()

	for _, track := range []int{0, 1, 7} {
		if layout := timeline.Lay(runSpans(), track, time.Time{}); len(layout.Rows) != 0 {
			t.Errorf("track %d laid out %d rows, want none", track, len(layout.Rows))
		}
	}

	if layout := timeline.Lay(nil, 60, time.Time{}); len(layout.Rows) != 0 {
		t.Error("no spans laid out rows")
	}
}
