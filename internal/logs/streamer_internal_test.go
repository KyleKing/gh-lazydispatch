package logs

import (
	"errors"
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
	for i := range count {
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

func TestLogStreamer_StartStop(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping streamer test in short mode")
	}

	client := &mockGitHubClient{}
	streamer := NewLogStreamer(client, 12345, "test.yml")

	streamer.Start()

	// Wait a bit for initial poll (poll interval is 2s)
	// Give it time to at least attempt the initial poll
	time.Sleep(100 * time.Millisecond)

	streamer.Stop()

	// Drain any buffered updates and verify channel eventually closes
	for {
		_, ok := <-streamer.Updates()
		if !ok {
			break // Channel closed as expected
		}
	}

	// Verify Stop is idempotent
	streamer.Stop() // Should not panic
}

func TestLogStreamer_PollingBehavior(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping polling test in short mode")
	}

	client := &mockGitHubClient{}
	streamer := NewLogStreamer(client, 12345, "test.yml")

	streamer.Start()
	defer streamer.Stop()

	// Count updates received in first second
	updateCount := 0
	timeout := time.After(1 * time.Second)

loop:
	for {
		select {
		case <-streamer.Updates():
			updateCount++
		case <-timeout:
			break loop
		}
	}

	// With 2s poll interval, expect at least 1 update (initial poll)
	if updateCount < 1 {
		t.Errorf("expected at least 1 update, got %d", updateCount)
	}

	t.Logf("received %d updates in 1 second", updateCount)
}

func TestLogStreamer_RunCompletion(t *testing.T) {
	client := &completedRunMockClient{}
	streamer := NewLogStreamer(client, 12345, "test.yml")

	streamer.Start()

	var completionUpdate StreamUpdate

	timeout := time.After(1 * time.Second)

	for {
		select {
		case update, ok := <-streamer.Updates():
			if !ok {
				// Channel closed, verify we got completion
				if completionUpdate.Status != "completed" {
					t.Error("expected completion update before channel closed")
				}

				return
			}

			if update.Status == "completed" {
				completionUpdate = update
			}
		case <-timeout:
			t.Fatal("timeout waiting for completion")
		}
	}
}

func TestLogStreamer_ErrorHandling(t *testing.T) {
	tests := []struct {
		name   string
		client GitHubClient
	}{
		{
			name:   "GetWorkflowRun error",
			client: &errorMockClient{errorOnGetRun: true},
		},
		{
			name:   "GetWorkflowRunJobs error",
			client: &errorMockClient{errorOnGetJobs: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			streamer := NewLogStreamer(tt.client, 12345, "test.yml")
			streamer.Start()
			defer streamer.Stop()

			// Wait for error update
			select {
			case update := <-streamer.Updates():
				if update.Error == nil {
					t.Error("expected error in update")
				}
			case <-time.After(500 * time.Millisecond):
				t.Error("timeout waiting for error update")
			}
		})
	}
}

func TestLogStreamer_ChannelFull(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping channel full test in short mode")
	}

	client := &mockGitHubClient{}
	streamer := NewLogStreamer(client, 12345, "test.yml")

	// Don't start the streamer, manually send updates to fill the channel
	// This tests the warning logging behavior

	// Fill the channel completely
	for range 50 {
		streamer.updates <- StreamUpdate{RunID: 12345}
	}

	// Attempt to send one more (should trigger warning log)
	select {
	case streamer.updates <- StreamUpdate{RunID: 12345}:
		t.Error("expected channel to be full")
	default:
		// Channel is full as expected
	}

	// Clean up
	streamer.Stop()
}

func TestLogStreamer_ConcurrentStop(t *testing.T) {
	client := &mockGitHubClient{}
	streamer := NewLogStreamer(client, 12345, "test.yml")

	streamer.Start()

	// Call Stop from multiple goroutines
	done := make(chan bool, 3)

	for range 3 {
		go func() {
			streamer.Stop()
			done <- true
		}()
	}

	// Wait for all goroutines
	for range 3 {
		<-done
	}

	// Drain any buffered updates and verify channel eventually closes
	for {
		_, ok := <-streamer.Updates()
		if !ok {
			break // Channel closed as expected
		}
	}
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

// completedRunMockClient returns a completed run status
type completedRunMockClient struct{}

func (c *completedRunMockClient) GetWorkflowRun(runID int64) (*github.WorkflowRun, error) {
	return &github.WorkflowRun{
		ID:         runID,
		Status:     "completed",
		Conclusion: "success",
	}, nil
}

func (c *completedRunMockClient) GetWorkflowRunJobs(runID int64) ([]github.Job, error) {
	return []github.Job{}, nil
}

// errorMockClient returns errors based on configuration
type errorMockClient struct {
	errorOnGetRun  bool
	errorOnGetJobs bool
}

func (e *errorMockClient) GetWorkflowRun(runID int64) (*github.WorkflowRun, error) {
	if e.errorOnGetRun {
		return nil, errors.New("mock error: failed to get workflow run")
	}

	return &github.WorkflowRun{
		ID:     runID,
		Status: "in_progress",
	}, nil
}

func (e *errorMockClient) GetWorkflowRunJobs(runID int64) ([]github.Job, error) {
	if e.errorOnGetJobs {
		return nil, errors.New("mock error: failed to get jobs")
	}

	return []github.Job{}, nil
}
