package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

func mustParse(t *testing.T, s string) time.Time {
	t.Helper()

	ts, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parsing %q: %v", s, err)
	}

	return ts
}

// A commit's CI timing is the span from the earliest triggered run's start to
// the last one's finish, and a push still in flight stays in the raw rows
// but out of the percentiles, since its span is not the real one yet.
func TestSummarizeCITimings_SpansAcrossACommitsRunsAndExcludesInFlightPushes(t *testing.T) {
	t.Parallel()

	runs := []github.RunTiming{
		{
			HeadSHA: "done", Status: github.StatusCompleted,
			CreatedAt: mustParse(t, "2026-01-01T00:00:00Z"), UpdatedAt: mustParse(t, "2026-01-01T00:04:00Z"),
		},
		{
			HeadSHA: "done", Status: github.StatusCompleted,
			CreatedAt: mustParse(t, "2026-01-01T00:01:00Z"), UpdatedAt: mustParse(t, "2026-01-01T00:10:00Z"),
		},
		{
			HeadSHA: "inflight", Status: github.StatusCompleted,
			CreatedAt: mustParse(t, "2026-01-02T00:00:00Z"), UpdatedAt: mustParse(t, "2026-01-02T00:04:00Z"),
		},
		{
			HeadSHA: "inflight", Status: "in_progress",
			CreatedAt: mustParse(t, "2026-01-02T00:01:00Z"), UpdatedAt: mustParse(t, "2026-01-02T00:01:00Z"),
		},
	}

	stats := summarizeCITimings(runs)

	if len(stats.Commits) != 2 {
		t.Fatalf("got %d commits, want both rows: %+v", len(stats.Commits), stats.Commits)
	}

	commit := stats.Commits[0]
	if commit.HeadSHA != "done" || commit.Excluded != "" {
		t.Fatalf("first commit is %+v, want %q counted", commit, "done")
	}

	if got := stats.Commits[1]; got.HeadSHA != "inflight" || got.Excluded != excludedIncomplete {
		t.Fatalf("second commit is %+v, want %q excluded as incomplete", got, "inflight")
	}

	// Earliest CreatedAt (00:00) to latest UpdatedAt (00:10) is 600 seconds,
	// not either individual run's own span.
	const wantSeconds = 600

	if commit.Seconds != wantSeconds {
		t.Errorf("span is %v seconds, want %v", commit.Seconds, wantSeconds)
	}

	if stats.MinSeconds != wantSeconds || stats.MaxSeconds != wantSeconds || stats.P50Seconds != wantSeconds {
		t.Errorf("stats are %+v, want every stat to equal the single commit's span", stats)
	}
}

// Percentiles read off the sorted durations, not commit order, so a fast
// commit sorted last by start time must not skew the reported median.
func TestSummarizeCITimings_PercentilesIgnoreCommitOrder(t *testing.T) {
	t.Parallel()

	makeRun := func(sha string, start time.Time, seconds int) github.RunTiming {
		return github.RunTiming{
			HeadSHA: sha, Status: github.StatusCompleted,
			CreatedAt: start, UpdatedAt: start.Add(time.Duration(seconds) * time.Second),
		}
	}

	day := func(n int) time.Time {
		return mustParse(t, "2026-01-01T00:00:00Z").AddDate(0, 0, n)
	}

	// Chronological order is 300s, 100s, 200s: not sorted by duration.
	runs := []github.RunTiming{
		makeRun("a", day(0), 300),
		makeRun("b", day(1), 100),
		makeRun("c", day(2), 200),
	}

	stats := summarizeCITimings(runs)

	if stats.MinSeconds != 100 || stats.P50Seconds != 200 || stats.MaxSeconds != 300 {
		t.Errorf("stats are %+v, want min=100 p50=200 max=300", stats)
	}
}

// Each commit lands in the window its CI began in, and the table puts the
// baseline above the current window with the P90 change between them.
func TestSummarizeWindows_BucketsByStartAndRendersTheComparison(t *testing.T) {
	t.Parallel()

	now := mustParse(t, "2026-01-15T00:00:00Z")
	windows := compareWindows(now, 7, 7)

	makeRun := func(sha string, daysAgo, seconds int) github.RunTiming {
		start := now.AddDate(0, 0, -daysAgo)

		return github.RunTiming{
			HeadSHA: sha, Status: github.StatusCompleted, Name: "CI", Conclusion: "success",
			CreatedAt: start, UpdatedAt: start.Add(time.Duration(seconds) * time.Second),
		}
	}

	canceled := makeRun("new-canceled", 2, 9000)
	canceled.Conclusion = conclusionCanceled

	supersededEarly := []github.RunTiming{makeRun("new-superseded", 1, 400), makeRun("new-superseded", 1, 100)}
	supersededEarly[1].Conclusion = conclusionCanceled

	stats := summarizeCITimings([]github.RunTiming{
		makeRun("old-a", 10, 100),
		makeRun("old-b", 9, 200),
		makeRun("new-a", 3, 300),
		canceled,
		supersededEarly[0],
		supersededEarly[1],
		makeRun("ancient", 30, 999),
	})

	got := summarizeWindows(stats.Commits, windows)

	if got[0].Commits != 2 || got[0].P90Seconds != 190 {
		t.Errorf("baseline is %+v, want 2 commits with p90 190", got[0])
	}

	if got[1].Commits != 2 || got[1].P90Seconds != 390 {
		t.Errorf("current is %+v, want 2 commits with p90 390", got[1])
	}

	var out strings.Builder
	if err := writeCITimingMarkdown(&out, "main", got); err != nil {
		t.Fatal(err)
	}

	wantRows := []string{
		"| baseline (2026-01-01 to 2026-01-08) | 2 | 0 | 2m30s | 3m10s | 3m20s |",
		"| current (2026-01-08 to 2026-01-15) | 2 | 1 of 3 (33%: 1 canceled) | 5m50s | 6m30s | 6m40s |",
		"P90 change: +105%",
		"| CI | 100% | 5m0s | 6m20s | 3m10s | +100% | 6m20s |",
	}

	for _, want := range wantRows {
		if !strings.Contains(out.String(), want) {
			t.Errorf("table lacks %q:\n%s", want, out.String())
		}
	}
}
