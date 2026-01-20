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

func TestChainExecutor_WaitForNone(t *testing.T) {
	// Setup mock executor to prevent real gh CLI calls
	mockExec := exec.NewMockExecutor()
	mockExec.AddCommand("gh", []string{"workflow", "run", "step1.yml", "--ref", "main"}, "", "", nil)
	runner.SetExecutor(mockExec)
	defer runner.SetExecutor(nil) // Reset after test

	client := &mockGitHubClient{
		latestID: 100,
		runs: map[int64]*github.WorkflowRun{
			100: {ID: 100, Status: github.StatusQueued},
		},
	}
	w := newMockWatcher()
	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "step1.yml", WaitFor: config.WaitNone},
		},
	}

	executor := chain.NewExecutor(client, w, "test-chain", chainDef)

	if err := executor.Start(map[string]string{}, "main"); err != nil {
		t.Fatalf("Start error: %v", err)
	}

	timeout := time.After(2 * time.Second)
	for {
		select {
		case _, ok := <-executor.Updates():
			if !ok {
				// Channel closed - check final state synchronously
				finalState := executor.State()
				if finalState.Status != chain.ChainCompleted {
					t.Errorf("expected ChainCompleted, got %v", finalState.Status)
				}
				return
			}
			// Continue draining updates
		case <-timeout:
			t.Fatal("timeout waiting for chain completion")
		}
	}
}
