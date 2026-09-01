package logs

import (
	"strconv"
	"strings"
	"testing"
)

type oneStepFetcher struct{}

func (oneStepFetcher) FetchStepLogs(_ int64, _ string) ([]*StepLogs, error) {
	return []*StepLogs{{StepName: "build", Conclusion: "success"}}, nil
}

// The log viewer titles itself from the name these logs carry, so a run reached
// by ID rather than through a dispatch must still name itself.
func TestGetLogsForRun_NamesLogsWhenNoWorkflowIsKnown(t *testing.T) {
	t.Parallel()

	const runID = 33423560774

	tests := []struct {
		name     string
		workflow string
		want     string
	}{
		{name: "a known workflow names the logs", workflow: "ci.yml", want: "ci.yml"},
		{name: "a bare run ID stands in for it", workflow: "", want: strconv.Itoa(runID)},
	}

	manager := &Manager{fetcher: oneStepFetcher{}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			runLogs, err := manager.GetLogsForRun(runID, tt.workflow)
			if err != nil {
				t.Fatal(err)
			}

			if !strings.Contains(runLogs.ChainName, tt.want) {
				t.Errorf("the logs are named %q, want it to carry %q", runLogs.ChainName, tt.want)
			}
		})
	}
}
