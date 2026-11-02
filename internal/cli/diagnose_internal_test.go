package cli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

var errLogNotFound = errors.New("gh command failed: HTTP 404")

func runLogsWithSteps(conclusions ...string) *logs.RunLogs {
	runLogs := logs.NewRunLogs("Orchestrator", "main")
	for i, conclusion := range conclusions {
		runLogs.AddStep(&logs.StepLogs{
			StepName:   "step",
			StepIndex:  i,
			Status:     github.StatusCompleted,
			Conclusion: conclusion,
			Entries:    []logs.LogEntry{{Content: "line", Level: logs.LogLevelInfo}},
		})
	}

	return runLogs
}

func TestBuildDiagnosisReportsFailedStepsByRunOutcome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		conclusion  string
		conclusions []string
		want        int
	}{
		{
			name:        "a successful run reports nothing",
			conclusion:  github.ConclusionSuccess,
			conclusions: []string{github.ConclusionSuccess, github.ConclusionSuccess},
			want:        0,
		},
		{
			name:        "a failed run whose steps all succeeded falls back to its last step",
			conclusion:  github.ConclusionFailure,
			conclusions: []string{github.ConclusionSuccess, github.ConclusionSuccess},
			want:        1,
		},
		{
			name:        "a failed step is reported whatever the run says",
			conclusion:  github.ConclusionFailure,
			conclusions: []string{github.ConclusionSuccess, github.ConclusionFailure},
			want:        1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			run := &github.WorkflowRun{
				ID:         1,
				Name:       "Orchestrator",
				Status:     github.StatusCompleted,
				Conclusion: tc.conclusion,
			}

			got := buildDiagnosis(run, runLogsWithSteps(tc.conclusions...), defaultTailLines)
			if len(got.FailedSteps) != tc.want {
				t.Errorf("failed_steps has %d entries, want %d", len(got.FailedSteps), tc.want)
			}
		})
	}
}

func TestBuildDiagnosisReportsTheFirstUnsuccessfulStepOfAFailedJob(t *testing.T) {
	t.Parallel()

	job := func(name, conclusion string) *logs.StepLogs {
		return &logs.StepLogs{
			StepName:      name,
			Conclusion:    conclusion,
			JobID:         7,
			JobName:       "deploy / pulumi",
			JobStatus:     github.StatusCompleted,
			JobConclusion: github.ConclusionFailure,
		}
	}

	runLogs := logs.NewRunLogs("run", "main")
	runLogs.AddStep(job("Set up job", github.ConclusionSuccess))

	died := job("Pulumi up (prod)", "")
	died.Error = errLogNotFound
	runLogs.AddStep(died)

	runLogs.AddStep(job("Post Run actions/checkout", github.ConclusionFailure))

	run := &github.WorkflowRun{ID: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionFailure}

	got := buildDiagnosis(run, runLogs, defaultTailLines)
	if len(got.FailedSteps) != 1 || got.FailedSteps[0].StepName != "Pulumi up (prod)" {
		t.Fatalf("failed_steps = %+v, want only the blank-conclusion step", got.FailedSteps)
	}

	if !strings.Contains(got.FailedSteps[0].LogNote, "log unavailable") {
		t.Errorf("log_note = %q, want it to say the log was unavailable", got.FailedSteps[0].LogNote)
	}

	empty := logNote(&logs.StepLogs{})
	if !strings.Contains(empty, "no log") {
		t.Errorf("an empty step's note = %q, want it to say there is no log", empty)
	}
}

func TestTailOfSummarizesWithTheFirstErrorOutsideTheEchoedScript(t *testing.T) {
	t.Parallel()

	line := func(content string, level logs.LogLevel) logs.LogEntry {
		return logs.LogEntry{Content: content, Level: level}
	}

	step := &logs.StepLogs{
		StepName: "Run smoke playbook",
		Duration: 6*time.Minute + 24*time.Second,
		Entries: []logs.LogEntry{
			line("##[group]Run set +e", logs.LogLevelInfo),
			line(`echo "::warning::infra failure"`, logs.LogLevelError),
			line("##[endgroup]", logs.LogLevelInfo),
			line("step.fail: judge returned severity=error", logs.LogLevelError),
			line("ELIFECYCLE Command failed", logs.LogLevelError),
		},
	}

	got := tailOf(step, defaultTailLines).Summary
	want := "Run smoke playbook (6m24s): step.fail: judge returned severity=error"

	if got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
}
