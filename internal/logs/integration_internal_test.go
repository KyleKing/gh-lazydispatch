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

	manager := &Manager{fetcher: oneStepFetcher{}, cache: newLogCache()}

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

type countingFetcher struct{ calls int }

func (f *countingFetcher) FetchStepLogs(_ int64, _ string) ([]*StepLogs, error) {
	f.calls++

	return []*StepLogs{{StepName: "build", Conclusion: "success"}}, nil
}

// Reopening a run is the common move, and a log is megabytes: the second read
// must not download it again.
func TestGetLogsForRun_ReadsOneRunOnce(t *testing.T) {
	t.Parallel()

	fetcher := &countingFetcher{}
	manager := &Manager{fetcher: fetcher, cache: newLogCache()}

	for range 3 {
		if _, err := manager.GetLogsForRun(42, "ci.yml"); err != nil {
			t.Fatal(err)
		}
	}

	if fetcher.calls != 1 {
		t.Errorf("three reads of one run cost %d fetches, want 1", fetcher.calls)
	}

	if _, err := manager.GetLogsForRun(43, "ci.yml"); err != nil {
		t.Fatal(err)
	}

	if fetcher.calls != 2 {
		t.Errorf("another run cost %d fetches in total, want 2", fetcher.calls)
	}
}
