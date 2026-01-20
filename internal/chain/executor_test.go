package chain_test

import (
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

type mockGitHubClient struct {
	runs     map[int64]*github.WorkflowRun
	jobs     map[int64][]github.Job
	latestID int64
	err      error
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

func (m *mockGitHubClient) GetLatestRun(workflowName string) (*github.WorkflowRun, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &github.WorkflowRun{ID: m.latestID, Status: github.StatusQueued}, nil
}

func (m *mockGitHubClient) Owner() string { return "owner" }
func (m *mockGitHubClient) Repo() string  { return "repo" }

type mockRunWatcher struct {
	watched map[int64]string
	updates chan watcher.RunUpdate
}

func newMockWatcher() *mockRunWatcher {
	return &mockRunWatcher{
		watched: make(map[int64]string),
		updates: make(chan watcher.RunUpdate, 10),
	}
}

func (m *mockRunWatcher) Watch(runID int64, workflowName string) {
	m.watched[runID] = workflowName
}

func (m *mockRunWatcher) Unwatch(runID int64) {
	delete(m.watched, runID)
}

func (m *mockRunWatcher) Updates() <-chan watcher.RunUpdate {
	return m.updates
}

func TestNewExecutor(t *testing.T) {
	client := &mockGitHubClient{}
	w := newMockWatcher()
	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "step1.yml"},
			{Workflow: "step2.yml"},
		},
	}

	executor := chain.NewExecutor(client, w, "test-chain", chainDef)
	state := executor.State()

	if state.ChainName != "test-chain" {
		t.Errorf("ChainName: got %q, want %q", state.ChainName, "test-chain")
	}
	if state.Status != chain.ChainPending {
		t.Errorf("Status: got %v, want %v", state.Status, chain.ChainPending)
	}
	if len(state.StepStatuses) != 2 {
		t.Errorf("StepStatuses length: got %d, want 2", len(state.StepStatuses))
	}
	for i, status := range state.StepStatuses {
		if status != chain.StepPending {
			t.Errorf("StepStatuses[%d]: got %v, want %v", i, status, chain.StepPending)
		}
	}
}

func TestChainExecutor_Stop(t *testing.T) {
	// Setup mock executor to prevent real gh CLI calls
	mockExec := exec.NewMockExecutor()
	runner.SetExecutor(mockExec)
	defer runner.SetExecutor(nil) // Reset after test

	client := &mockGitHubClient{latestID: 123}
	w := newMockWatcher()
	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "step1.yml", WaitFor: config.WaitSuccess},
		},
	}

	executor := chain.NewExecutor(client, w, "test-chain", chainDef)
	executor.Stop()

	select {
	case <-executor.Updates():
	case <-time.After(100 * time.Millisecond):
	}
}

// Chain execution tests moved to internal/integration_test.go for E2E coverage.
// Kept here: unit tests for specific initialization and state functionality.

func TestNewExecutorFromHistory(t *testing.T) {
	client := &mockGitHubClient{}
	w := newMockWatcher()
	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "step1.yml"},
			{Workflow: "step2.yml"},
			{Workflow: "step3.yml"},
		},
	}

	previousResults := []chain.PreviousStepResult{
		{Workflow: "step1.yml", RunID: 100, Status: "completed", Conclusion: "success"},
	}

	executor := chain.NewExecutorFromHistory(client, w, "resume-chain", chainDef, previousResults, 1)
	state := executor.State()

	if state.CurrentStep != 1 {
		t.Errorf("CurrentStep: got %d, want 1", state.CurrentStep)
	}
	if state.StepStatuses[0] != chain.StepCompleted {
		t.Errorf("StepStatuses[0]: got %v, want %v", state.StepStatuses[0], chain.StepCompleted)
	}
	if state.StepStatuses[1] != chain.StepPending {
		t.Errorf("StepStatuses[1]: got %v, want %v", state.StepStatuses[1], chain.StepPending)
	}
}

// TestChainExecutor_WatcherIntegration moved to internal/integration_test.go
