package internal_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

var errMockCommand = errors.New("mock command failed")

// TestEndToEnd_ChainExecutionWithLogs tests the full chain execution flow
// including workflow dispatch, status watching, and log retrieval.
// This covers Phases 1-3: Chain execution, log viewer, and real log fetching.
func TestEndToEnd_ChainExecutionWithLogs(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	setupChainExecutionMocks(mockExec)
	runner.SetExecutor(mockExec)
	defer runner.SetExecutor(nil)

	client := newMockGitHubClient()
	w := newMockRunWatcher()

	chainDef := &config.Chain{
		Description: "CI and Deploy pipeline",
		Steps: []config.ChainStep{
			{Workflow: "ci.yml", WaitFor: config.WaitNone, OnFailure: config.FailureAbort},
			{Workflow: "deploy.yml", WaitFor: config.WaitNone, OnFailure: config.FailureAbort,
				Inputs: map[string]string{"environment": "{{ var.env }}"}},
		},
	}

	executor := chain.NewExecutor(client, w, "ci-deploy", chainDef)
	variables := map[string]string{"env": "staging"}

	if err := executor.Start(variables, "main"); err != nil {
		t.Fatalf("chain start failed: %v", err)
	}

	drainUpdates(t, executor.Updates(), 2*time.Second)

	state := executor.State()
	if state.Status != chain.ChainCompleted {
		t.Errorf("chain status: got %v, want %v", state.Status, chain.ChainCompleted)
	}

	if len(mockExec.ExecutedCommands) != 2 {
		t.Errorf("executed commands: got %d, want 2", len(mockExec.ExecutedCommands))
	}

	verifyCommand(t, mockExec.ExecutedCommands[0], "gh", "workflow", "run", "ci.yml")
	verifyCommand(t, mockExec.ExecutedCommands[1], "gh", "workflow", "run", "deploy.yml")
}

// TestEndToEnd_LogFetchingWithGHCLI tests log fetching via mocked gh CLI.
func TestEndToEnd_LogFetchingWithGHCLI(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	setupLogFetchingMocks(mockExec)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)
	stepLogs, err := fetcher.FetchStepLogsReal(1001, "ci.yml")
	if err != nil {
		t.Fatalf("log fetch failed: %v", err)
	}

	if len(stepLogs) != 3 {
		t.Errorf("step count: got %d, want 3", len(stepLogs))
	}

	verifyStepLogs(t, stepLogs, []string{"Checkout", "Build", "Test"})

	hasError := false
	for _, step := range stepLogs {
		for _, entry := range step.Entries {
			if entry.Level == logs.LogLevelError {
				hasError = true
				break
			}
		}
	}
	if hasError {
		t.Error("unexpected error entries in successful run")
	}
}

// TestEndToEnd_FailedRunWithErrorLogs tests error detection in logs.
func TestEndToEnd_FailedRunWithErrorLogs(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	setupFailedRunMocks(mockExec)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)
	stepLogs, err := fetcher.FetchStepLogsReal(1002, "ci.yml")
	if err != nil {
		t.Fatalf("log fetch failed: %v", err)
	}

	hasFailedStep := false
	for _, step := range stepLogs {
		if step.Conclusion == github.ConclusionFailure {
			hasFailedStep = true
			break
		}
	}
	if !hasFailedStep {
		t.Error("expected at least one failed step")
	}
}

// TestEndToEnd_WatcherRegistration tests that chain execution registers runs with the watcher.
func TestEndToEnd_WatcherRegistration(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	mockExec.AddCommand("gh", []string{"workflow", "run", "test.yml", "--ref", "main"}, "", "", nil)
	runner.SetExecutor(mockExec)
	defer runner.SetExecutor(nil)

	client := newMockGitHubClient()
	w := newMockRunWatcher()

	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "test.yml", WaitFor: config.WaitNone},
		},
	}

	executor := chain.NewExecutor(client, w, "test-chain", chainDef)
	_ = executor.Start(map[string]string{}, "main")

	drainUpdates(t, executor.Updates(), 2*time.Second)

	if len(w.watched) != 1 {
		t.Errorf("watched runs: got %d, want 1", len(w.watched))
	}
}

// TestEndToEnd_ChainFailureHandling tests chain behavior when a step fails.
func TestEndToEnd_ChainFailureHandling(t *testing.T) {
	tests := []struct {
		name           string
		onFailure      config.FailureAction
		wantStatus     chain.ChainStatus
		wantCmdsCount  int
	}{
		{"abort", config.FailureAbort, chain.ChainFailed, 1},
		{"continue", config.FailureContinue, chain.ChainCompleted, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := exec.NewMockExecutor()
			mockExec.AddCommand("gh", []string{"workflow", "run", "step1.yml", "--ref", "main"},
				"", "dispatch failed", errMockCommand)
			mockExec.AddCommand("gh", []string{"workflow", "run", "step2.yml", "--ref", "main"}, "", "", nil)
			runner.SetExecutor(mockExec)
			defer runner.SetExecutor(nil)

			client := newMockGitHubClient()
			w := newMockRunWatcher()

			chainDef := &config.Chain{
				Steps: []config.ChainStep{
					{Workflow: "step1.yml", WaitFor: config.WaitNone, OnFailure: tt.onFailure},
					{Workflow: "step2.yml", WaitFor: config.WaitNone, OnFailure: config.FailureAbort},
				},
			}

			executor := chain.NewExecutor(client, w, "test", chainDef)
			_ = executor.Start(map[string]string{}, "main")

			drainUpdates(t, executor.Updates(), 2*time.Second)

			state := executor.State()
			if state.Status != tt.wantStatus {
				t.Errorf("status: got %v, want %v", state.Status, tt.wantStatus)
			}
			if len(mockExec.ExecutedCommands) != tt.wantCmdsCount {
				t.Errorf("commands: got %d, want %d", len(mockExec.ExecutedCommands), tt.wantCmdsCount)
			}
		})
	}
}

// Helper types and functions

type mockGitHubClient struct {
	runs     map[int64]*github.WorkflowRun
	jobs     map[int64][]github.Job
	latestID int64
}

func newMockGitHubClient() *mockGitHubClient {
	return &mockGitHubClient{
		latestID: 1000,
		runs: map[int64]*github.WorkflowRun{
			1000: {ID: 1000, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
		},
		jobs: map[int64][]github.Job{},
	}
}

func (m *mockGitHubClient) GetWorkflowRun(runID int64) (*github.WorkflowRun, error) {
	if run, ok := m.runs[runID]; ok {
		return run, nil
	}
	return &github.WorkflowRun{ID: runID, Status: github.StatusQueued}, nil
}

func (m *mockGitHubClient) GetWorkflowRunJobs(runID int64) ([]github.Job, error) {
	return m.jobs[runID], nil
}

func (m *mockGitHubClient) GetLatestRun(_ string) (*github.WorkflowRun, error) {
	return &github.WorkflowRun{ID: m.latestID, Status: github.StatusQueued}, nil
}

func (m *mockGitHubClient) Owner() string { return "owner" }
func (m *mockGitHubClient) Repo() string  { return "repo" }

type mockRunWatcher struct {
	watched map[int64]string
	updates chan watcher.RunUpdate
}

func newMockRunWatcher() *mockRunWatcher {
	return &mockRunWatcher{
		watched: make(map[int64]string),
		updates: make(chan watcher.RunUpdate, 10),
	}
}

func (m *mockRunWatcher) Watch(runID int64, workflow string) { m.watched[runID] = workflow }
func (m *mockRunWatcher) Unwatch(runID int64)                { delete(m.watched, runID) }
func (m *mockRunWatcher) Updates() <-chan watcher.RunUpdate  { return m.updates }

func setupChainExecutionMocks(m *exec.MockExecutor) {
	m.AddCommand("gh", []string{"workflow", "run", "ci.yml", "--ref", "main"}, "", "", nil)
	m.AddCommand("gh", []string{"workflow", "run", "deploy.yml", "--ref", "main", "-f", "environment=staging"}, "", "", nil)
}

func setupLogFetchingMocks(m *exec.MockExecutor) {
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 2001, Name: "build", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			Steps: []github.Step{
				{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{Name: "Build", Number: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{Name: "Test", Number: 3, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
			},
		}},
	}
	jobsJSON, _ := json.Marshal(jobsResp)
	m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/1001/jobs"}, string(jobsJSON), "", nil)

	logOutput := `##[group]Checkout
Cloning repository...
##[endgroup]
##[group]Build
Building project...
##[endgroup]
##[group]Test
Running tests...
All tests passed
##[endgroup]`
	m.AddGHRunView(1001, 2001, logOutput)
}

func setupFailedRunMocks(m *exec.MockExecutor) {
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 2002, Name: "build", Status: github.StatusCompleted, Conclusion: github.ConclusionFailure,
			Steps: []github.Step{
				{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{Name: "Build", Number: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionFailure},
			},
		}},
	}
	jobsJSON, _ := json.Marshal(jobsResp)
	m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/1002/jobs"}, string(jobsJSON), "", nil)

	logOutput := `##[group]Checkout
Cloning repository...
##[endgroup]
##[group]Build
ERROR: Build failed
##[error]Compilation error in main.go
##[endgroup]`
	m.AddGHRunView(1002, 2002, logOutput)
}

func drainUpdates(t *testing.T, updates <-chan chain.ChainUpdate, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for {
		select {
		case _, ok := <-updates:
			if !ok {
				return
			}
		case <-deadline:
			t.Fatal("timeout waiting for chain updates")
		}
	}
}

func verifyCommand(t *testing.T, cmd exec.ExecutedCommand, expectedArgs ...string) {
	t.Helper()
	if cmd.Name != expectedArgs[0] {
		t.Errorf("command name: got %q, want %q", cmd.Name, expectedArgs[0])
	}
	for i, arg := range expectedArgs[1:] {
		if i >= len(cmd.Args) || cmd.Args[i] != arg {
			found := ""
			if i < len(cmd.Args) {
				found = cmd.Args[i]
			}
			t.Errorf("command arg[%d]: got %q, want %q", i, found, arg)
		}
	}
}

func verifyStepLogs(t *testing.T, stepLogs []*logs.StepLogs, expectedNames []string) {
	t.Helper()
	if len(stepLogs) != len(expectedNames) {
		t.Errorf("step count: got %d, want %d", len(stepLogs), len(expectedNames))
		return
	}
	for i, name := range expectedNames {
		if stepLogs[i].StepName != name {
			t.Errorf("step[%d] name: got %q, want %q", i, stepLogs[i].StepName, name)
		}
	}
}
