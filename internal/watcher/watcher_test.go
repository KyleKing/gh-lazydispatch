package watcher_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

type mockGitHubClient struct {
	runs map[int64]*github.WorkflowRun
	jobs map[int64][]github.Job
	err  error
}

func (m *mockGitHubClient) GetWorkflowRun(runID int64) (*github.WorkflowRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	if run, ok := m.runs[runID]; ok {
		return run, nil
	}
	return &github.WorkflowRun{ID: runID, Status: github.StatusQueued}, nil
}

func (m *mockGitHubClient) GetWorkflowRunJobs(runID int64) ([]github.Job, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.jobs[runID], nil
}

func TestNewWatcher(t *testing.T) {
	client := &mockGitHubClient{}
	w := watcher.NewWatcher(client)
	if w == nil {
		t.Fatal("expected non-nil watcher")
	}
	defer w.Stop()

	if w.TotalCount() != 0 {
		t.Errorf("TotalCount: got %d, want 0", w.TotalCount())
	}
	if w.ActiveCount() != 0 {
		t.Errorf("ActiveCount: got %d, want 0", w.ActiveCount())
	}
}

func TestWatch_AddsRun(t *testing.T) {
	client := &mockGitHubClient{
		runs: map[int64]*github.WorkflowRun{
			123: {ID: 123, Name: "test-workflow", Status: github.StatusQueued},
		},
	}
	w := watcher.NewWatcher(client)
	defer w.Stop()

	w.Watch(123, "test-workflow")

	if w.TotalCount() != 1 {
		t.Errorf("TotalCount after Watch: got %d, want 1", w.TotalCount())
	}

	run, ok := w.GetRun(123)
	if !ok {
		t.Fatal("expected to find run 123")
	}
	if run.RunID != 123 {
		t.Errorf("RunID: got %d, want 123", run.RunID)
	}
}

func TestUnwatch_RemovesRun(t *testing.T) {
	client := &mockGitHubClient{
		runs: map[int64]*github.WorkflowRun{
			123: {ID: 123, Name: "test-workflow", Status: github.StatusQueued},
		},
	}
	w := watcher.NewWatcher(client)
	defer w.Stop()

	w.Watch(123, "test-workflow")
	w.Unwatch(123)

	if w.TotalCount() != 0 {
		t.Errorf("TotalCount after Unwatch: got %d, want 0", w.TotalCount())
	}

	_, ok := w.GetRun(123)
	if ok {
		t.Error("expected run 123 to be removed")
	}
}

func TestPollRun_UpdatesState(t *testing.T) {
	client := &mockGitHubClient{
		runs: map[int64]*github.WorkflowRun{
			123: {ID: 123, Name: "test-workflow", Status: github.StatusInProgress},
		},
		jobs: map[int64][]github.Job{
			123: {{Name: "build", Status: github.StatusInProgress}},
		},
	}
	w := watcher.NewWatcher(client)
	defer w.Stop()

	w.Watch(123, "test-workflow")

	select {
	case update := <-w.Updates():
		if update.Error != nil {
			t.Fatalf("unexpected error: %v", update.Error)
		}
		if update.RunID != 123 {
			t.Errorf("RunID: got %d, want 123", update.RunID)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for update")
	}
}

func TestPollRun_SurfacesError(t *testing.T) {
	expectedErr := errors.New("API error")
	client := &mockGitHubClient{err: expectedErr}
	w := watcher.NewWatcher(client)
	defer w.Stop()

	w.Watch(123, "test-workflow")

	select {
	case update := <-w.Updates():
		if update.Error == nil {
			t.Error("expected error in update")
		}
		if !errors.Is(update.Error, expectedErr) {
			t.Errorf("error: got %v, want %v", update.Error, expectedErr)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for update")
	}
}

func TestClearCompleted(t *testing.T) {
	client := &mockGitHubClient{
		runs: map[int64]*github.WorkflowRun{
			1: {ID: 1, Name: "run1", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
			2: {ID: 2, Name: "run2", Status: github.StatusInProgress},
			3: {ID: 3, Name: "run3", Status: github.StatusCompleted, Conclusion: github.ConclusionFailure},
		},
	}
	w := watcher.NewWatcher(client)
	defer w.Stop()

	w.Watch(1, "run1")
	w.Watch(2, "run2")
	w.Watch(3, "run3")

	time.Sleep(100 * time.Millisecond)

	w.ClearCompleted()

	if w.TotalCount() != 1 {
		t.Errorf("TotalCount after ClearCompleted: got %d, want 1", w.TotalCount())
	}

	_, ok := w.GetRun(2)
	if !ok {
		t.Error("expected active run 2 to still exist")
	}
}

func TestActiveCount(t *testing.T) {
	client := &mockGitHubClient{
		runs: map[int64]*github.WorkflowRun{
			1: {ID: 1, Name: "run1", Status: github.StatusCompleted},
			2: {ID: 2, Name: "run2", Status: github.StatusInProgress},
			3: {ID: 3, Name: "run3", Status: github.StatusQueued},
		},
	}
	w := watcher.NewWatcher(client)
	defer w.Stop()

	w.Watch(1, "run1")
	w.Watch(2, "run2")
	w.Watch(3, "run3")

	time.Sleep(100 * time.Millisecond)

	if w.ActiveCount() != 2 {
		t.Errorf("ActiveCount: got %d, want 2", w.ActiveCount())
	}
}

func TestWatchedRun_IsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"queued", github.StatusQueued, true},
		{"in_progress", github.StatusInProgress, true},
		{"completed", github.StatusCompleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := watcher.WatchedRun{Status: tt.status}
			if run.IsActive() != tt.expected {
				t.Errorf("IsActive: got %v, want %v", run.IsActive(), tt.expected)
			}
		})
	}
}

func TestWatchedRun_IsSuccess(t *testing.T) {
	tests := []struct {
		name       string
		status     string
		conclusion string
		expected   bool
	}{
		{"completed success", github.StatusCompleted, github.ConclusionSuccess, true},
		{"completed failure", github.StatusCompleted, github.ConclusionFailure, false},
		{"in progress", github.StatusInProgress, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := watcher.WatchedRun{Status: tt.status, Conclusion: tt.conclusion}
			if run.IsSuccess() != tt.expected {
				t.Errorf("IsSuccess: got %v, want %v", run.IsSuccess(), tt.expected)
			}
		})
	}
}

func TestWatcher_DoubleStop(t *testing.T) {
	client := &mockGitHubClient{}
	w := watcher.NewWatcher(client)

	w.Stop()
	w.Stop() // Should not panic
}
