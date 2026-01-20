package internal_test

import (
	"errors"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/testutil"
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

	client := testutil.NewMockGitHubClient().
		WithRun(&github.WorkflowRun{ID: 1000, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess})
	w := testutil.NewMockRunWatcher()

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

	testutil.DrainChainUpdates(t, executor.Updates(), 2*time.Second)

	state := executor.State()
	if state.Status != chain.ChainCompleted {
		t.Errorf("chain status: got %v, want %v", state.Status, chain.ChainCompleted)
	}

	if len(mockExec.ExecutedCommands) != 2 {
		t.Errorf("executed commands: got %d, want 2", len(mockExec.ExecutedCommands))
	}

	testutil.AssertCommand(t, mockExec.ExecutedCommands[0], "gh", "workflow", "run", "ci.yml")
	testutil.AssertCommand(t, mockExec.ExecutedCommands[1], "gh", "workflow", "run", "deploy.yml")
}

// TestEndToEnd_LogFetchingWithGHCLI tests log fetching via mocked gh CLI.
func TestEndToEnd_LogFetchingWithGHCLI(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	setupLogFetchingMocks(t, mockExec)

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

	testutil.AssertStepLogNames(t, stepLogs, []string{"Checkout", "Build", "Test"})

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
	setupFailedRunMocks(t, mockExec)

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

	client := testutil.NewMockGitHubClient()
	w := testutil.NewMockRunWatcher()

	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "test.yml", WaitFor: config.WaitNone},
		},
	}

	executor := chain.NewExecutor(client, w, "test-chain", chainDef)
	_ = executor.Start(map[string]string{}, "main")

	testutil.DrainChainUpdates(t, executor.Updates(), 2*time.Second)

	if len(w.Watched) != 1 {
		t.Errorf("watched runs: got %d, want 1", len(w.Watched))
	}
}

// TestEndToEnd_ChainFailureHandling tests chain behavior when a step fails.
func TestEndToEnd_ChainFailureHandling(t *testing.T) {
	tests := []struct {
		name          string
		onFailure     config.FailureAction
		wantStatus    chain.ChainStatus
		wantCmdsCount int
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

			client := testutil.NewMockGitHubClient()
			w := testutil.NewMockRunWatcher()

			chainDef := &config.Chain{
				Steps: []config.ChainStep{
					{Workflow: "step1.yml", WaitFor: config.WaitNone, OnFailure: tt.onFailure},
					{Workflow: "step2.yml", WaitFor: config.WaitNone, OnFailure: config.FailureAbort},
				},
			}

			executor := chain.NewExecutor(client, w, "test", chainDef)
			_ = executor.Start(map[string]string{}, "main")

			testutil.DrainChainUpdates(t, executor.Updates(), 2*time.Second)

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

// Setup helpers

func setupChainExecutionMocks(m *exec.MockExecutor) {
	m.AddCommand("gh", []string{"workflow", "run", "ci.yml", "--ref", "main"}, "", "", nil)
	m.AddCommand("gh", []string{"workflow", "run", "deploy.yml", "--ref", "main", "-f", "environment=staging"}, "", "", nil)
}

func setupLogFetchingMocks(t *testing.T, m *exec.MockExecutor) {
	t.Helper()
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
	m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/1001/jobs"}, testutil.MustMarshalJSON(t, jobsResp), "", nil)

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

func setupFailedRunMocks(t *testing.T, m *exec.MockExecutor) {
	t.Helper()
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 2002, Name: "build", Status: github.StatusCompleted, Conclusion: github.ConclusionFailure,
			Steps: []github.Step{
				{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{Name: "Build", Number: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionFailure},
			},
		}},
	}
	m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/1002/jobs"}, testutil.MustMarshalJSON(t, jobsResp), "", nil)

	logOutput := `##[group]Checkout
Cloning repository...
##[endgroup]
##[group]Build
ERROR: Build failed
##[error]Compilation error in main.go
##[endgroup]`
	m.AddGHRunView(1002, 2002, logOutput)
}
