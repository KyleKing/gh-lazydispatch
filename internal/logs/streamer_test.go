package logs

import (
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

func TestLogStreamer_detectNewLogs(t *testing.T) {
	tests := []struct {
		name          string
		initialState  map[int]int // stepIndex -> lineCount
		currentLogs   []*StepLogs
		expectedNew   int // number of steps with new logs
		expectedSteps []int
	}{
		{
			name:         "first poll - all logs are new",
			initialState: map[int]int{},
			currentLogs: []*StepLogs{
				{StepIndex: 0, StepName: "checkout", Entries: makeEntries(5)},
				{StepIndex: 1, StepName: "setup", Entries: makeEntries(3)},
			},
			expectedNew:   2,
			expectedSteps: []int{0, 1},
		},
		{
			name: "no new logs - same line counts",
			initialState: map[int]int{
				0: 5,
				1: 3,
			},
			currentLogs: []*StepLogs{
				{StepIndex: 0, StepName: "checkout", Entries: makeEntries(5)},
				{StepIndex: 1, StepName: "setup", Entries: makeEntries(3)},
			},
			expectedNew:   0,
			expectedSteps: []int{},
		},
		{
			name: "incremental update - step 1 has new lines",
			initialState: map[int]int{
				0: 5,
				1: 3,
			},
			currentLogs: []*StepLogs{
				{StepIndex: 0, StepName: "checkout", Entries: makeEntries(5)},
				{StepIndex: 1, StepName: "setup", Entries: makeEntries(7)},
			},
			expectedNew:   1,
			expectedSteps: []int{1},
		},
		{
			name: "new step appears",
			initialState: map[int]int{
				0: 5,
				1: 3,
			},
			currentLogs: []*StepLogs{
				{StepIndex: 0, StepName: "checkout", Entries: makeEntries(5)},
				{StepIndex: 1, StepName: "setup", Entries: makeEntries(3)},
				{StepIndex: 2, StepName: "build", Entries: makeEntries(10)},
			},
			expectedNew:   1,
			expectedSteps: []int{2},
		},
		{
			name: "multiple steps have updates",
			initialState: map[int]int{
				0: 5,
				1: 3,
				2: 10,
			},
			currentLogs: []*StepLogs{
				{StepIndex: 0, StepName: "checkout", Entries: makeEntries(5)},
				{StepIndex: 1, StepName: "setup", Entries: makeEntries(8)},
				{StepIndex: 2, StepName: "build", Entries: makeEntries(15)},
				{StepIndex: 3, StepName: "test", Entries: makeEntries(20)},
			},
			expectedNew:   3,
			expectedSteps: []int{1, 2, 3},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Make a copy of initial state for verification later
			initialStateCopy := make(map[int]int)
			for k, v := range tt.initialState {
				initialStateCopy[k] = v
			}

			streamer := &LogStreamer{
				state: &StreamState{
					StepLineCounts: tt.initialState,
				},
			}

			newSteps := streamer.detectNewLogs(tt.currentLogs)

			if len(newSteps) != tt.expectedNew {
				t.Errorf("expected %d new steps, got %d", tt.expectedNew, len(newSteps))
			}

			// Verify correct step indices were returned
			gotIndices := make([]int, len(newSteps))
			for i, step := range newSteps {
				gotIndices[i] = step.StepIndex
			}

			if len(gotIndices) != len(tt.expectedSteps) {
				t.Errorf("step indices: got %v, want %v", gotIndices, tt.expectedSteps)
				return
			}

			for i, expected := range tt.expectedSteps {
				if gotIndices[i] != expected {
					t.Errorf("step index %d: got %d, want %d", i, gotIndices[i], expected)
				}
			}

			// Verify only new entries are included (use copy of initial state)
			for _, newStep := range newSteps {
				originalStep := findStepByIndex(tt.currentLogs, newStep.StepIndex)
				if originalStep == nil {
					t.Fatalf("step %d not found in current logs", newStep.StepIndex)
				}

				lastCount, exists := initialStateCopy[newStep.StepIndex]
				if !exists {
					lastCount = 0
				}
				currentCount := len(originalStep.Entries)
				expectedNewEntries := currentCount - lastCount

				if len(newStep.Entries) != expectedNewEntries {
					t.Errorf("step %d: expected %d new entries (current %d - last %d), got %d",
						newStep.StepIndex, expectedNewEntries, currentCount, lastCount, len(newStep.Entries))
				}
			}

			// Verify state was updated
			for _, step := range tt.currentLogs {
				if streamer.state.StepLineCounts[step.StepIndex] != len(step.Entries) {
					t.Errorf("state not updated for step %d: got %d, want %d",
						step.StepIndex,
						streamer.state.StepLineCounts[step.StepIndex],
						len(step.Entries))
				}
			}
		})
	}
}

func TestStreamState_NewStreamState(t *testing.T) {
	state := NewStreamState()

	if state == nil {
		t.Fatal("expected non-nil state")
	}

	if state.StepLineCounts == nil {
		t.Fatal("expected initialized StepLineCounts map")
	}

	if len(state.StepLineCounts) != 0 {
		t.Errorf("expected empty StepLineCounts, got %d entries", len(state.StepLineCounts))
	}
}

func TestLogStreamer_Creation(t *testing.T) {
	// Create a simple mock client
	client := &mockGitHubClient{}

	streamer := NewLogStreamer(client, 12345, "test.yml")

	if streamer == nil {
		t.Fatal("expected non-nil streamer")
	}

	if streamer.runID != 12345 {
		t.Errorf("runID: got %d, want %d", streamer.runID, 12345)
	}

	if streamer.workflow != "test.yml" {
		t.Errorf("workflow: got %q, want %q", streamer.workflow, "test.yml")
	}

	if streamer.state == nil {
		t.Fatal("expected non-nil state")
	}

	if streamer.updates == nil {
		t.Fatal("expected non-nil updates channel")
	}

	// Clean up
	streamer.Stop()
}

// Helper functions

func makeEntries(count int) []LogEntry {
	entries := make([]LogEntry, count)
	for i := 0; i < count; i++ {
		entries[i] = LogEntry{
			Timestamp: time.Now(),
			Content:   "test log line",
			Level:     LogLevelInfo,
		}
	}
	return entries
}

func findStepByIndex(steps []*StepLogs, index int) *StepLogs {
	for _, step := range steps {
		if step.StepIndex == index {
			return step
		}
	}
	return nil
}

// mockGitHubClient is a minimal mock for testing
type mockGitHubClient struct{}

func (m *mockGitHubClient) GetWorkflowRun(runID int64) (*github.WorkflowRun, error) {
	return &github.WorkflowRun{
		ID:     runID,
		Status: "in_progress",
	}, nil
}

func (m *mockGitHubClient) GetWorkflowRunJobs(runID int64) ([]github.Job, error) {
	return []github.Job{}, nil
}
