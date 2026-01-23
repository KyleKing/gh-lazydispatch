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

// TestIntegration_ChainExecutionWithLogViewing tests the full end-to-end flow:
// 1. Execute a multi-step chain
// 2. Wait for completion
// 3. Retrieve logs for each step's workflow run
// 4. Verify log content and step results correlation
func TestIntegration_ChainExecutionWithLogViewing(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	runner.SetExecutor(mockExec)
	defer runner.SetExecutor(nil)

	// Step 1: ci.yml (runID 5001)
	mockExec.AddCommand("gh", []string{"workflow", "run", "ci.yml", "--ref", "main"}, "", "", nil)

	// Step 2: deploy.yml (runID 5002)
	mockExec.AddCommand("gh", []string{"workflow", "run", "deploy.yml", "--ref", "main", "-f", "env=production"}, "", "", nil)

	// Setup log fetching for step 1 (ci.yml)
	jobsRespCI := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 6001, Name: "test", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			Steps: []github.Step{
				{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{Name: "Run tests", Number: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
			},
		}},
	}
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/5001/jobs"},
		testutil.MustMarshalJSON(t, jobsRespCI), "", nil)

	ciLogs := `##[group]Checkout
Checking out code...
##[endgroup]
##[group]Run tests
Running test suite...
All tests passed (42 tests)
##[endgroup]`
	mockExec.AddGHRunView(5001, 6001, ciLogs)

	// Setup log fetching for step 2 (deploy.yml)
	jobsRespDeploy := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 6002, Name: "deploy", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			Steps: []github.Step{
				{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{Name: "Deploy to production", Number: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
			},
		}},
	}
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/5002/jobs"},
		testutil.MustMarshalJSON(t, jobsRespDeploy), "", nil)

	deployLogs := `##[group]Checkout
Checking out code...
##[endgroup]
##[group]Deploy to production
Deploying application to production...
Deployment successful!
##[endgroup]`
	mockExec.AddGHRunView(5002, 6002, deployLogs)

	// Setup GitHub client with run metadata
	client := testutil.NewMockGitHubClient().
		WithRun(&github.WorkflowRun{
			ID:         5001,
			Name:       "CI",
			Status:     github.StatusCompleted,
			Conclusion: github.ConclusionSuccess,
			HTMLURL:    "https://github.com/owner/repo/actions/runs/5001",
		}).
		WithRun(&github.WorkflowRun{
			ID:         5002,
			Name:       "Deploy",
			Status:     github.StatusCompleted,
			Conclusion: github.ConclusionSuccess,
			HTMLURL:    "https://github.com/owner/repo/actions/runs/5002",
		})
	// Configure workflow-specific run IDs
	client.LatestByWorkflow["ci.yml"] = 5001
	client.LatestByWorkflow["deploy.yml"] = 5002

	w := testutil.NewMockRunWatcher()

	// Define chain
	chainDef := &config.Chain{
		Description: "CI and Deploy pipeline",
		Steps: []config.ChainStep{
			{
				Workflow:  "ci.yml",
				WaitFor:   config.WaitNone,
				OnFailure: config.FailureAbort,
			},
			{
				Workflow:  "deploy.yml",
				WaitFor:   config.WaitNone,
				OnFailure: config.FailureAbort,
				Inputs:    map[string]string{"env": "{{ var.env }}"},
			},
		},
	}

	// Execute chain
	executor := chain.NewExecutor(client, w, "ci-deploy", chainDef)
	variables := map[string]string{"env": "production"}

	if err := executor.Start(variables, "main"); err != nil {
		t.Fatalf("chain start failed: %v", err)
	}

	// Wait for chain completion
	testutil.DrainChainUpdates(t, executor.Updates(), 3*time.Second)

	// Verify chain completed successfully
	state := executor.State()
	if state.Status != chain.ChainCompleted {
		t.Fatalf("chain status: got %v, want %v", state.Status, chain.ChainCompleted)
	}

	if len(state.StepResults) != 2 {
		t.Fatalf("step results count: got %d, want 2", len(state.StepResults))
	}

	// Verify step 1 result
	step1 := state.StepResults[0]
	if step1 == nil {
		t.Fatal("step 1 result is nil")
	}
	if step1.RunID != 5001 {
		t.Errorf("step 1 runID: got %d, want 5001", step1.RunID)
	}
	if step1.Status != chain.StepCompleted {
		t.Errorf("step 1 status: got %v, want %v", step1.Status, chain.StepCompleted)
	}

	// Verify step 2 result
	step2 := state.StepResults[1]
	if step2 == nil {
		t.Fatal("step 2 result is nil")
	}
	if step2.RunID != 5002 {
		t.Errorf("step 2 runID: got %d, want 5002", step2.RunID)
	}
	if step2.Status != chain.StepCompleted {
		t.Errorf("step 2 status: got %v, want %v", step2.Status, chain.StepCompleted)
	}

	// Now test log viewing for both steps
	ghClient, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}
	fetcher := logs.NewGHFetcherWithExecutor(ghClient, mockExec)

	// Fetch logs for step 1 (ci.yml run 5001)
	ciStepLogs, err := fetcher.FetchStepLogsReal(step1.RunID, "ci.yml")
	if err != nil {
		t.Fatalf("failed to fetch ci.yml logs: %v", err)
	}

	if len(ciStepLogs) != 2 {
		t.Errorf("ci.yml step count: got %d, want 2", len(ciStepLogs))
	}

	// Verify CI log content
	foundTestsPass := false
	for _, step := range ciStepLogs {
		for _, entry := range step.Entries {
			if entry.Content == "All tests passed (42 tests)" {
				foundTestsPass = true
			}
		}
	}
	if !foundTestsPass {
		t.Error("expected to find 'All tests passed' in CI logs")
	}

	// Fetch logs for step 2 (deploy.yml run 5002)
	deployStepLogs, err := fetcher.FetchStepLogsReal(step2.RunID, "deploy.yml")
	if err != nil {
		t.Fatalf("failed to fetch deploy.yml logs: %v", err)
	}

	if len(deployStepLogs) != 2 {
		t.Errorf("deploy.yml step count: got %d, want 2", len(deployStepLogs))
	}

	// Verify deployment log content
	foundDeploySuccess := false
	for _, step := range deployStepLogs {
		for _, entry := range step.Entries {
			if entry.Content == "Deployment successful!" {
				foundDeploySuccess = true
			}
		}
	}
	if !foundDeploySuccess {
		t.Error("expected to find 'Deployment successful!' in deploy logs")
	}

	// Verify all logs are at appropriate log levels
	for _, stepLogs := range []struct {
		name string
		logs []*logs.StepLogs
	}{
		{"ci.yml", ciStepLogs},
		{"deploy.yml", deployStepLogs},
	} {
		hasError := false
		for _, step := range stepLogs.logs {
			for _, entry := range step.Entries {
				if entry.Level == logs.LogLevelError {
					hasError = true
					break
				}
			}
		}
		if hasError {
			t.Errorf("%s: unexpected error entries in successful run", stepLogs.name)
		}
	}
}

// TestIntegration_ChainWithErrorLogs tests log viewing for a chain with error-level logs.
// This verifies that error logs are properly captured even when steps complete.
func TestIntegration_ChainWithErrorLogs(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	runner.SetExecutor(mockExec)
	defer runner.SetExecutor(nil)

	// Step 1: ci.yml succeeds (runID 7001)
	mockExec.AddCommand("gh", []string{"workflow", "run", "ci.yml", "--ref", "main"}, "", "", nil)

	// Setup successful CI logs
	jobsRespCI := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 8001, Name: "test", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			Steps: []github.Step{
				{Name: "Run tests", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
			},
		}},
	}
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/7001/jobs"},
		testutil.MustMarshalJSON(t, jobsRespCI), "", nil)

	ciLogs := `##[group]Run tests
Running tests...
All tests passed
##[endgroup]`
	mockExec.AddGHRunView(7001, 8001, ciLogs)

	// Step 2: deploy.yml succeeds but has warnings/errors in logs (runID 7002)
	mockExec.AddCommand("gh", []string{"workflow", "run", "deploy.yml", "--ref", "main"}, "", "", nil)

	// Setup deployment with warnings
	jobsRespDeploy := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 8002, Name: "deploy", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			Steps: []github.Step{
				{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{Name: "Deploy", Number: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
			},
		}},
	}
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/7002/jobs"},
		testutil.MustMarshalJSON(t, jobsRespDeploy), "", nil)

	deployLogs := `##[group]Checkout
Checking out code...
##[endgroup]
##[group]Deploy
Deploying to production...
##[warning]Deprecation notice: API v1 will be sunset in 6 months
##[error]Non-critical error: Cache miss for dependency X
Using fallback configuration
Deployment successful despite warnings
##[endgroup]`
	mockExec.AddGHRunView(7002, 8002, deployLogs)

	// Setup GitHub client with run metadata
	client := testutil.NewMockGitHubClient().
		WithRun(&github.WorkflowRun{
			ID:         7001,
			Status:     github.StatusCompleted,
			Conclusion: github.ConclusionSuccess,
			HTMLURL:    "https://github.com/owner/repo/actions/runs/7001",
		}).
		WithRun(&github.WorkflowRun{
			ID:         7002,
			Status:     github.StatusCompleted,
			Conclusion: github.ConclusionSuccess,
			HTMLURL:    "https://github.com/owner/repo/actions/runs/7002",
		})
	// Configure workflow-specific run IDs
	client.LatestByWorkflow["ci.yml"] = 7001
	client.LatestByWorkflow["deploy.yml"] = 7002

	w := testutil.NewMockRunWatcher()

	// Define chain
	chainDef := &config.Chain{
		Description: "CI and Deploy with error logs",
		Steps: []config.ChainStep{
			{
				Workflow:  "ci.yml",
				WaitFor:   config.WaitNone,
				OnFailure: config.FailureAbort,
			},
			{
				Workflow:  "deploy.yml",
				WaitFor:   config.WaitNone,
				OnFailure: config.FailureAbort,
			},
		},
	}

	// Execute chain
	executor := chain.NewExecutor(client, w, "ci-deploy-warnings", chainDef)

	if err := executor.Start(map[string]string{}, "main"); err != nil {
		t.Fatalf("chain start failed: %v", err)
	}

	// Wait for chain to complete
	testutil.DrainChainUpdates(t, executor.Updates(), 3*time.Second)

	// Verify chain completed successfully
	state := executor.State()
	if state.Status != chain.ChainCompleted {
		t.Errorf("chain status: got %v, want %v", state.Status, chain.ChainCompleted)
	}

	// Verify both steps succeeded
	step1 := state.StepResults[0]
	if step1 == nil {
		t.Fatal("step 1 result is nil")
	}
	if step1.Status != chain.StepCompleted {
		t.Errorf("step 1 status: got %v, want %v", step1.Status, chain.StepCompleted)
	}

	step2 := state.StepResults[1]
	if step2 == nil {
		t.Fatal("step 2 result is nil")
	}
	if step2.Status != chain.StepCompleted {
		t.Errorf("step 2 status: got %v, want %v", step2.Status, chain.StepCompleted)
	}

	// Now test log viewing for the deploy step with warnings/errors
	ghClient, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}
	fetcher := logs.NewGHFetcherWithExecutor(ghClient, mockExec)

	// Fetch logs for deploy step
	deployStepLogs, err := fetcher.FetchStepLogsReal(step2.RunID, "deploy.yml")
	if err != nil {
		t.Fatalf("failed to fetch deploy.yml logs: %v", err)
	}

	// Verify we captured various log levels
	warningCount := 0
	errorCount := 0
	foundDeprecation := false
	foundCacheMiss := false

	for _, step := range deployStepLogs {
		for _, entry := range step.Entries {
			if entry.Level == logs.LogLevelWarning {
				warningCount++
				if entry.Content == "##[warning]Deprecation notice: API v1 will be sunset in 6 months" {
					foundDeprecation = true
				}
			}
			if entry.Level == logs.LogLevelError {
				errorCount++
				if entry.Content == "##[error]Non-critical error: Cache miss for dependency X" {
					foundCacheMiss = true
				}
			}
		}
	}

	if warningCount == 0 {
		t.Error("expected warning-level log entries in deploy step")
	}

	if errorCount == 0 {
		t.Error("expected error-level log entries in deploy step (non-critical)")
	}

	if !foundDeprecation {
		t.Error("expected to find deprecation warning in logs")
	}

	if !foundCacheMiss {
		t.Error("expected to find cache miss error in logs")
	}

	// Verify successful step completes despite errors/warnings
	foundSuccess := false
	for _, step := range deployStepLogs {
		for _, entry := range step.Entries {
			if entry.Content == "Deployment successful despite warnings" {
				foundSuccess = true
			}
		}
	}
	if !foundSuccess {
		t.Error("expected to find success message in logs")
	}

	// Verify CI step has clean logs
	ciStepLogs, err := fetcher.FetchStepLogsReal(step1.RunID, "ci.yml")
	if err != nil {
		t.Fatalf("failed to fetch ci.yml logs: %v", err)
	}

	for _, step := range ciStepLogs {
		for _, entry := range step.Entries {
			if entry.Level == logs.LogLevelError || entry.Level == logs.LogLevelWarning {
				t.Errorf("unexpected error/warning entries in clean CI step: %s", entry.Content)
			}
		}
	}
}
