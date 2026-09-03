package cli

import (
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

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
