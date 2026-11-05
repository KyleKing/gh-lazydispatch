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

	got, named := firstErrorInStep(raw, step)
	if want := "Error: judge returned severity=error"; got != want || named {
		t.Errorf("first error = %q (named %v), want %q unnamed", got, named, want)
	}
}

func TestFirstErrorInStepNamesTheFailedTestOverConsoleNoise(t *testing.T) {
	t.Parallel()

	vitestLines := []string{
		"2026-11-04T21:20:43.7218481Z ^[[22m^[[39m[Error: ResizeObserver loop completed with undeli" +
			"vered notifications.]",
		"2026-11-04T21:21:01.1612872Z    ^[[33m^[[2m✓^[[22m^[[39m Activity ^[[33m 434^[[2mms^[[22m^" +
			"[[39m",
		"2026-11-04T21:21:01.1613157Z    ^[[33m^[[2m✓^[[22m^[[39m Verify Trail ^[[33m 420^[[2mms^[[" +
			"22m^[[39m",
		"2026-11-04T21:21:01.1613496Z ^[[31m   ^[[31m×^[[31m Gateways^[[39m^[[33m 10300^[[2mms^[[22" +
			"m^[[39m^[[33m (retry x1)^[[39m",
		"2026-11-04T21:21:39.0731787Z ^[[31m⎯⎯⎯⎯⎯⎯⎯^[[39m^[[1m^[[41m Failed Tests 2 ^[[49m^[[22m^[[" +
			"31m⎯⎯⎯⎯⎯⎯⎯^[[39m",
		"2026-11-04T21:21:39.0731964Z ",
		"2026-11-04T21:21:39.0733244Z ^[[41m^[[1m FAIL ^[[22m^[[49m ^[[30m^[[45m storybook (chromiu" +
			"m) ^[[49m^[[39m src/pages/Inspect/InspectBrokerPage.stories.tsx^[[2m > ^[[22mGateways",
		"2026-11-04T21:21:39.0740373Z ^[[31m^[[1mTestingLibraryElementError^[[22m^[[39m: ",
		"2026-11-04T21:21:39.0741014Z ^[[34mClick to debug the error directly in Storybook: http://" +
			"localhost:6006/?path=/story/pages-inspect-inspectbrokerpage--gateways&addonPanel=storybook" +
			"/interacti",
		"2026-11-04T21:21:39.0741390Z ",
		"2026-11-04T21:21:39.0741395Z ",
	}
	pytestLines := []string{
		"2026-11-05T04:35:27.6642699Z [gw12] [ 53%] SUBPASSED(status=\"'FAILED'\", behavior=\"'USE_DEF" +
			"AULT_VALUE'\") tests/connectors/aws/test_ses_readiness.py::TestSesReadiness::test_configure" +
			"d_mail_fro",
		"2026-11-05T10:53:46.0872747Z 3.36s call     tests/routes/test_radar_event_exposure_vendors" +
			".py::TestRadarEventExposureVendors::test_bom_only_vendors_get_their_latest_saved_match",
		"2026-11-05T10:53:46.0873346Z ^[[36m^[[1m=========================== short test summary inf" +
			"o ============================^[[0m",
		"2026-11-05T10:53:46.0877158Z ^[[31mFAILED^[[0m tests/routes/test_agent_broker.py::^[[1mTes" +
			"tAgentBrokerRoutes::test_members_delegate_their_own_authority^[[0m - AssertionError: {\"det" +
			"ail\":\"The ",
		"2026-11-05T10:53:46.0878245Z assert 403 == 200",
		"2026-11-05T10:53:46.0879266Z  +  where 403 = <Response [403 Forbidden]>.status_code",
	}

	tests := []struct {
		name  string
		lines []string
		want  string
		named bool
	}{
		{
			name:  "vitest FAIL line names the file and story, not the logged console error",
			lines: vitestLines, named: true,
			want: "FAIL storybook (chromium) src/pages/Inspect/InspectBrokerPage.stories.tsx > Gateways",
		},
		{
			name:  "vitest without a FAIL line falls back to the failed-test marker",
			lines: vitestLines[:4], want: "× Gateways 10300ms (retry x1)", named: true,
		},
		{
			name:  "pytest summary entry gives the test id and skips SUBPASSED progress",
			lines: pytestLines, named: true,
			want: "FAILED tests/routes/test_agent_broker.py::TestAgentBrokerRoutes::" +
				"test_members_delegate_their_own_authority",
		},
		{
			name:  "no failed-test marker keeps the first error-level line",
			lines: vitestLines[:2], want: "[Error: ResizeObserver loop completed with undelivered notifications.]",
		},
	}

	start := time.Date(2026, 11, 4, 21, 0, 0, 0, time.UTC)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			step := github.StepTiming{Name: "Run tests", StartedAt: start, CompletedAt: start.Add(48 * time.Hour)}
			got, named := firstErrorInStep(strings.Join(tc.lines, "\n"), step)

			if got != tc.want || named != tc.named {
				t.Errorf("first error = %q (named %v), want %q (named %v)", got, named, tc.want, tc.named)
			}
		})
	}
}

func TestFailedSiblingsNamesTheShardsBehindAGateJob(t *testing.T) {
	t.Parallel()

	failed := func(name string) github.JobTiming {
		return github.JobTiming{Name: name, Conclusion: github.ConclusionFailure}
	}
	runJobs := []github.JobTiming{
		failed("Vitest"), failed("Vitest Storybook shard 4 of 6"), failed("Vitest Storybook shard 6 of 6"),
		{Name: "Vitest Unit shard 1 of 6", Conclusion: github.ConclusionSuccess},
		failed("Lint and Format"),
	}

	got := failedSiblings("Vitest", runJobs)
	if want := "failed jobs: Vitest Storybook shard 4 of 6, Vitest Storybook shard 6 of 6"; got != want {
		t.Errorf("siblings = %q, want %q", got, want)
	}

	if got := failedSiblings("Lint and Format", runJobs); got != "" {
		t.Errorf("siblings = %q, want none", got)
	}
}
