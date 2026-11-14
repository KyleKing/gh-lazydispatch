package citiming

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/v2/internal/github"
	"github.com/kyleking/gh-lazydispatch/v2/internal/runcache"
)

func job(name, conclusion string, stepFailed bool) runcache.AttemptJob {
	return runcache.AttemptJob{Name: name, Conclusion: conclusion, StepFailed: stepFailed}
}

// A flake needs a failure and a later pass on one SHA. A run that failed and
// was never fixed is a real failure, one that only echoes a failure through an
// ignored aggregator job is left out, and classes come from conclusions alone.
func TestSummarizeFlakes_SeparatesSameSHAFlakesFromRealFailures(t *testing.T) {
	t.Parallel()

	commits := []ciTiming{
		{HeadSHA: "flaky", RunDetails: []ciRun{{Name: "Tests", ID: 1, Attempt: 2, Conclusion: "success"}}},
		{HeadSHA: "red", RunDetails: []ciRun{
			{Name: "Tests", ID: 2, Attempt: 1, Conclusion: "failure"},
			{Name: "Gate", ID: 3, Attempt: 1, Conclusion: "failure"},
		}},
		{HeadSHA: "echo", RunDetails: []ciRun{{Name: "Gate", ID: 4, Attempt: 1, Conclusion: "failure"}}},
		{HeadSHA: "green", RunDetails: []ciRun{{Name: "Tests", ID: 5, Attempt: 1, Conclusion: "success"}}},
		{HeadSHA: "superseded", RunDetails: []ciRun{{Name: "Tests", ID: 6, Conclusion: github.ConclusionCancelled}}},
	}

	attempts := runcache.Attempts{
		runcache.AttemptKey(1, 1): {job("unit", "timed_out", false), job("lint", "success", false)},
		runcache.AttemptKey(1, 2): {job("unit", "success", false), job("lint", "success", false)},
		runcache.AttemptKey(2, 1): {
			job("unit", "failure", true), job("api", "failure", false), job("web", github.ConclusionCancelled, false),
		},
		runcache.AttemptKey(3, 1): {job("CI Gate", "failure", true)},
		runcache.AttemptKey(4, 1): {job("CI Gate", "failure", true)},
	}

	got := summarizeFlakes(commits, attempts, []string{"CI Gate"})

	if got.SHAs != 4 || got.Flaked != 1 || got.Failed != 1 {
		t.Fatalf("SHAs/flaked/failed = %d/%d/%d, want 4/1/1: %+v", got.SHAs, got.Flaked, got.Failed, got)
	}

	if got.FlakedClasses[classTimedOut] != 1 {
		t.Errorf("flaked classes = %v, want one timed out", got.FlakedClasses)
	}

	wantFailed := map[string]int{classStepFailed: 1, classRunnerLost: 1, classCancelled: 1}
	for class, n := range wantFailed {
		if got.FailedClasses[class] != n {
			t.Errorf("failed class %q = %d, want %d (%v)", class, got.FailedClasses[class], n, got.FailedClasses)
		}
	}

	var md strings.Builder

	writeFlakes(&md, got, DefaultFlakeTarget)

	if tests := got.Workflows[0]; tests.Name != "Tests" || len(tests.FlakedMembers) != 1 ||
		tests.FlakedMembers[0] != "flaky" || len(tests.FailedMembers) != 1 || tests.FailedMembers[0] != "red" {
		t.Errorf("Tests row = %+v, want flaky and red as the member SHAs", tests)
	}

	for _, want := range []string{
		"25.00% of SHAs against a target of 0.50%", "| Tests / unit | 1 | 33.33% |", "| Tests | flaky |",
	} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("markdown missing %q:\n%s", want, md.String())
		}
	}
}

// With cuts at a date, the Targets block shows each side of it, a side under the
// minimum shows no percentiles, and the merge queue waits follow the same sides.
func TestWriteTargets_ReportsEachSideOfASplitAgainstItsTargets(t *testing.T) {
	t.Parallel()

	day := func(d int) time.Time { return time.Date(2026, 10, d, 12, 0, 0, 0, time.UTC) }

	var commits []ciTiming

	for d := 1; d <= 9; d++ {
		commits = append(commits, ciTiming{
			HeadSHA: "sha" + string(rune('a'+d)), StartedAt: day(d).Format(timeFormat), Seconds: 600,
		})
	}

	pr := reportEvent{
		filter: github.RunFilter{Event: "pull_request"},
		stats: ciTimingStats{
			Commits: commits, RangeFrom: day(1).Format(timeFormat), RangeTo: day(10).Format(timeFormat),
			Epochs: []ciEpoch{
				{Start: day(1).Format(timeFormat)},
				{Start: day(8).Format(timeFormat)},
			},
		},
	}
	queue := reportEvent{
		filter: github.RunFilter{Event: "merge_group"},
		stats:  ciTimingStats{Queue: []ciQueue{{waits: []float64{60, 90}}, {waits: []float64{300}}}},
	}

	var b strings.Builder

	writeTargets(&b, []reportEvent{pr, queue}, DefaultTargets(), true, 5)

	for _, want := range []string{
		"| Wall clock P50 | 2026-10-01 12:00 to 2026-10-08 12:00 | 7 | 10m0s | 5m0s | over by 5m0s |",
		"| Wall clock P50 | 2026-10-08 12:00 to 2026-10-10 12:00 | 2 | - | 5m0s | under the 5-push minimum |",
		"| Merge queue wait P50 | 2026-10-01 12:00 to 2026-10-08 12:00 | 2 | 1m15s | 2m0s | met |",
		"| Merge queue wait P50 | 2026-10-08 12:00 to 2026-10-10 12:00 | 1 | 5m0s | 2m0s | over by 3m0s |",
	} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("targets missing %q:\n%s", want, b.String())
		}
	}
}
