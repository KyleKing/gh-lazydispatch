package cli

import (
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

// The last job to finish owns the push's wall clock, split into the wait for
// earlier jobs, the runner queue, and its own run, and its headroom is the gap
// to the job that finished before it.
func TestSummarizeCritical_SplitsTheLastJobsPathAndMeasuresHeadroom(t *testing.T) {
	t.Parallel()

	base := mustParse(t, "2026-01-01T00:00:00Z")
	at := func(seconds int) time.Time { return base.Add(time.Duration(seconds) * time.Second) }

	runs := []github.RunTiming{
		{
			HeadSHA: "a", ID: 1, Name: "CI", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			CreatedAt: at(0), UpdatedAt: at(310),
		},
		{
			HeadSHA: "a", ID: 2, Name: "Lint", Status: github.StatusCompleted, Conclusion: conclusionCanceled,
			CreatedAt: at(0), UpdatedAt: at(100),
		},
	}

	jobs := runcache.Jobs{1: {
		{
			Name: "gate", Labels: []string{"ubicloud"}, Conclusion: github.ConclusionSuccess,
			CreatedAt: at(10), StartedAt: at(40), CompletedAt: at(300),
			Steps: []github.StepTiming{
				{Name: "Cache deps", Conclusion: github.ConclusionSuccess, StartedAt: at(50), CompletedAt: at(58)},
				{Name: "Cache build", Conclusion: "skipped"},
			},
		},
		{
			Name: "unit", Labels: []string{"ubicloud"}, Conclusion: github.ConclusionSuccess,
			CreatedAt: at(0), StartedAt: at(5), CompletedAt: at(200),
		},
	}}

	stats := summarizeCITimings(runs)
	spans := []timeWindow{{from: at(-60), to: at(3600), label: "2026-01-01"}}
	pushes := sampledPushes(stats.Commits, spans, 10)
	got := summarizeCritical(pushes, spans, stats.Commits, jobs, nil)[0]

	type path struct{ upstream, queue, run, after float64 }

	have := path{got.MeanUpstream, got.MeanQueue, got.MeanRun, got.MeanAfterLast}
	if want := (path{10, 30, 260, 10}); have != want || got.Sampled != 1 {
		t.Errorf("path = %+v over %d pushes, want %+v over 1", have, got.Sampled, want)
	}

	if len(got.Jobs) != 1 || got.Jobs[0].Job != "gate" || got.Jobs[0].P50Headroom != 100 {
		t.Errorf("critical jobs = %+v, want gate with 100s of headroom", got.Jobs)
	}

	if len(got.Runners) != 1 || got.Runners[0].Jobs != 2 {
		t.Errorf("runners = %+v, want both jobs on one runner", got.Runners)
	}

	if len(got.Caches) != 2 || got.Caches[0].Name != "Cache deps" || got.Caches[1].SkippedRate != 1 {
		t.Errorf("cache steps = %+v, want Cache deps timed and Cache build skipped", got.Caches)
	}

	trends := summarizeTrends(stats.Commits, spans)
	if len(trends) != 2 || trends[0].Workflow != "CI" || trends[0].P90Seconds != 310 || trends[1].CancelRate != 1 {
		t.Errorf("trends = %+v, want CI at 310s then Lint with every run canceled", trends)
	}
}
