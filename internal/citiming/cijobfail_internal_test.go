package citiming

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

func TestRollUpJobFailuresCountsEveryRunAndPicksTheTopStep(t *testing.T) {
	t.Parallel()

	started := time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC)
	failedAt := func(step string) github.JobTiming {
		return github.JobTiming{
			Name: "smoke", Conclusion: github.ConclusionFailure, ID: 9,
			Steps: []github.StepTiming{
				{Name: "setup", Conclusion: github.ConclusionSuccess},
				{
					Name: step, Conclusion: github.ConclusionFailure,
					StartedAt: started, CompletedAt: started.Add(time.Minute),
				},
			},
		}
	}
	passed := github.JobTiming{Name: "smoke", Conclusion: github.ConclusionSuccess}
	skipped := github.JobTiming{Name: "smoke", Conclusion: "skipped"}

	jobs := runcache.Jobs{
		4: {failedAt("judge")},
		3: {passed},
		2: {failedAt("judge")},
		1: {failedAt("browser"), skipped},
	}

	report := rollUpJobFailures("smoke.yml", []int64{4, 3, 2, 1}, jobs)

	if len(report.Jobs) != 1 {
		t.Fatalf("jobs = %+v, want one", report.Jobs)
	}

	row := report.Jobs[0]
	if row.Runs != 4 || row.Failed != 3 || row.TopStepFailed != 2 {
		t.Errorf("row = %+v, want 4 runs, 3 failed, top step failed twice", row)
	}

	if row.TopStep != "judge" || row.LatestFailedRun != 4 || row.instance.jobID != 9 {
		t.Errorf("row = %+v, want top step judge and latest failed run 4", row)
	}

	if report.Failed != 3 {
		t.Errorf("failed runs = %d, want 3", report.Failed)
	}
}

func TestFirstErrorInStepStaysInsideTheStepAndSkipsTheEchoedScript(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, 11, 2, 10, 0, 5, 0, time.UTC)
	step := github.StepTiming{Name: "judge", StartedAt: start, CompletedAt: start.Add(10 * time.Second)}

	raw := strings.Join([]string{
		"2026-11-02T10:00:01.1Z error from the previous step",
		"2026-11-02T10:00:05.2Z ##[group]Run pnpm start",
		"2026-11-02T10:00:05.3Z echo \"::warning::infra failure\"",
		"2026-11-02T10:00:05.4Z ##[endgroup]",
		"2026-11-02T10:00:09.5Z Error: judge returned severity=error",
		"2026-11-02T10:00:15.5Z ##[error]Process completed with exit code 1.",
		"2026-11-02T10:00:30.1Z error from the next step",
	}, "\n")

	if got, want := firstErrorInStep(raw, step), "Error: judge returned severity=error"; got != want {
		t.Errorf("first error = %q, want %q", got, want)
	}
}
