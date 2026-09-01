package internal_test

import (
	"errors"
	"fmt"
	"strings"
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
//
//nolint:paralleltest // mutates the package-level runner.SetExecutor mock; cannot run concurrent tests
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
			{
				Workflow: "deploy.yml", WaitFor: config.WaitNone, OnFailure: config.FailureAbort,
				Inputs: map[string]string{"environment": "{{ var.env }}"},
			},
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
//
//nolint:paralleltest // mutates the package-level runner.SetExecutor mock; cannot run concurrent tests
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
//
//nolint:paralleltest // mutates the package-level runner.SetExecutor mock; cannot run concurrent tests
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
//
//nolint:paralleltest // mutates the package-level runner.SetExecutor mock; cannot run concurrent tests
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
	if err := executor.Start(map[string]string{}, "main"); err != nil {
		t.Fatalf("failed to start chain: %v", err)
	}

	testutil.DrainChainUpdates(t, executor.Updates(), 2*time.Second)

	if len(w.Watched) != 1 {
		t.Errorf("watched runs: got %d, want 1", len(w.Watched))
	}
}

// TestEndToEnd_ChainFailureHandling tests chain behavior when a step fails.
//
//nolint:paralleltest // mutates the package-level runner.SetExecutor mock; cannot run concurrent tests
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
		//nolint:paralleltest // mutates the package-level runner.SetExecutor mock; cannot run concurrent subtests
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
			if err := executor.Start(map[string]string{}, "main"); err != nil {
				t.Fatalf("failed to start chain: %v", err)
			}

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
	m.AddCommand("gh",
		[]string{"workflow", "run", "deploy.yml", "--ref", "main", "-f", "environment=staging"}, "", "", nil)
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
	m.AddCommand("gh",
		[]string{"api", "repos/owner/repo/actions/runs/1001/jobs?per_page=100"},
		testutil.MustMarshalJSON(t, jobsResp), "", nil)

	logOutput := ghRunViewLog("build",
		ghStep{"Checkout", []string{"Cloning repository..."}},
		ghStep{"Build", []string{"Building project..."}},
		ghStep{"Test", []string{"Running tests...", "All tests passed"}},
	)
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
	m.AddCommand("gh",
		[]string{"api", "repos/owner/repo/actions/runs/1002/jobs?per_page=100"},
		testutil.MustMarshalJSON(t, jobsResp), "", nil)

	logOutput := ghRunViewLog("build",
		ghStep{"Checkout", []string{"Cloning repository..."}},
		ghStep{"Build", []string{"ERROR: Build failed", "##[error]Compilation error in main.go"}},
	)
	m.AddGHRunView(1002, 2002, logOutput)
}

// TestIntegration_ChainExecutionWithLogViewing tests the full end-to-end flow:
// 1. Execute a multi-step chain
// 2. Wait for completion
// 3. Retrieve logs for each step's workflow run
// 4. Verify log content and step results correlation
//
//nolint:paralleltest // mutates the package-level runner.SetExecutor mock; cannot run concurrent tests
func TestIntegration_ChainExecutionWithLogViewing(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	runner.SetExecutor(mockExec)

	defer runner.SetExecutor(nil)

	client := setupChainWithLogViewingMocks(t, mockExec)
	w := testutil.NewMockRunWatcher()

	chainDef := &config.Chain{
		Description: "CI and Deploy pipeline",
		Steps: []config.ChainStep{
			{Workflow: "ci.yml", WaitFor: config.WaitNone, OnFailure: config.FailureAbort},
			{
				Workflow: "deploy.yml", WaitFor: config.WaitNone, OnFailure: config.FailureAbort,
				Inputs: map[string]string{"env": "{{ var.env }}"},
			},
		},
	}

	executor := chain.NewExecutor(client, w, "ci-deploy", chainDef)
	variables := map[string]string{"env": "production"}

	if err := executor.Start(variables, "main"); err != nil {
		t.Fatalf("chain start failed: %v", err)
	}

	testutil.DrainChainUpdates(t, executor.Updates(), 3*time.Second)

	state := executor.State()
	if state.Status != chain.ChainCompleted {
		t.Fatalf("chain status: got %v, want %v", state.Status, chain.ChainCompleted)
	}

	if len(state.StepResults) != 2 {
		t.Fatalf("step results count: got %d, want 2", len(state.StepResults))
	}

	step1 := checkChainStepResult(t, state.StepResults[0], "step 1", 5001)
	step2 := checkChainStepResult(t, state.StepResults[1], "step 2", 5002)

	ghClient, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(ghClient, mockExec)

	ciStepLogs, err := fetcher.FetchStepLogsReal(step1.RunID, "ci.yml")
	if err != nil {
		t.Fatalf("failed to fetch ci.yml logs: %v", err)
	}

	checkStepLogsContain(t, "ci.yml", ciStepLogs, 2, "All tests passed (42 tests)")

	deployStepLogs, err := fetcher.FetchStepLogsReal(step2.RunID, "deploy.yml")
	if err != nil {
		t.Fatalf("failed to fetch deploy.yml logs: %v", err)
	}

	checkStepLogsContain(t, "deploy.yml", deployStepLogs, 2, "Deployment successful!")
	checkStepLogsNoErrors(t, "ci.yml", ciStepLogs)
	checkStepLogsNoErrors(t, "deploy.yml", deployStepLogs)
}

// setupChainWithLogViewingMocks registers mock "gh workflow run", "gh api" (jobs),
// and "gh run view" (logs) responses for a ci.yml -> deploy.yml chain, and
// returns a GitHub client seeded with matching run metadata.
func setupChainWithLogViewingMocks(t *testing.T, mockExec *exec.MockExecutor) *testutil.MockGitHubClient {
	t.Helper()

	mockExec.AddCommand("gh", []string{"workflow", "run", "ci.yml", "--ref", "main"}, "", "", nil)
	mockExec.AddCommand("gh",
		[]string{"workflow", "run", "deploy.yml", "--ref", "main", "-f", "env=production"}, "", "", nil)

	jobsRespCI := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 6001, Name: "test", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			Steps: []github.Step{
				{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{Name: "Run tests", Number: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
			},
		}},
	}
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/5001/jobs?per_page=100"},
		testutil.MustMarshalJSON(t, jobsRespCI), "", nil)

	ciLogs := ghRunViewLog("test",
		ghStep{"Checkout", []string{"Checking out code..."}},
		ghStep{"Run tests", []string{"Running test suite...", "All tests passed (42 tests)"}},
	)
	mockExec.AddGHRunView(5001, 6001, ciLogs)

	jobsRespDeploy := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 6002, Name: "deploy", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			Steps: []github.Step{
				{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{
					Name: "Deploy to production", Number: 2,
					Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
				},
			},
		}},
	}
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/5002/jobs?per_page=100"},
		testutil.MustMarshalJSON(t, jobsRespDeploy), "", nil)

	deployLogs := ghRunViewLog("deploy",
		ghStep{"Checkout", []string{"Checking out code..."}},
		ghStep{"Deploy to production", []string{"Deploying application to production...", "Deployment successful!"}},
	)
	mockExec.AddGHRunView(5002, 6002, deployLogs)

	client := testutil.NewMockGitHubClient().
		WithRun(&github.WorkflowRun{
			ID: 5001, Name: "CI", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			HTMLURL: "https://github.com/owner/repo/actions/runs/5001",
		}).
		WithRun(&github.WorkflowRun{
			ID: 5002, Name: "Deploy", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			HTMLURL: "https://github.com/owner/repo/actions/runs/5002",
		})
	client.LatestByWorkflow["ci.yml"] = 5001
	client.LatestByWorkflow["deploy.yml"] = 5002

	return client
}

// checkChainStepResult asserts a chain step result is non-nil, completed, and
// has the expected run ID, returning it for further use.
func checkChainStepResult(t *testing.T, step *chain.StepResult, label string, wantRunID int64) *chain.StepResult {
	t.Helper()

	if step == nil {
		t.Fatalf("%s result is nil", label)
	}

	if step.RunID != wantRunID {
		t.Errorf("%s runID: got %d, want %d", label, step.RunID, wantRunID)
	}

	if step.Status != chain.StepCompleted {
		t.Errorf("%s status: got %v, want %v", label, step.Status, chain.StepCompleted)
	}

	return step
}

// checkStepLogsContain asserts stepLogs has wantSteps steps and that at least
// one log entry's content matches wantContent.
func checkStepLogsContain(t *testing.T, label string, stepLogs []*logs.StepLogs, wantSteps int, wantContent string) {
	t.Helper()

	if len(stepLogs) != wantSteps {
		t.Errorf("%s step count: got %d, want %d", label, len(stepLogs), wantSteps)
	}

	for _, step := range stepLogs {
		for _, entry := range step.Entries {
			if entry.Content == wantContent {
				return
			}
		}
	}

	t.Errorf("expected to find %q in %s logs", wantContent, label)
}

// checkStepLogsNoErrors asserts none of stepLogs' entries are at error level.
func checkStepLogsNoErrors(t *testing.T, label string, stepLogs []*logs.StepLogs) {
	t.Helper()

	for _, step := range stepLogs {
		for _, entry := range step.Entries {
			if entry.Level == logs.LogLevelError {
				t.Errorf("%s: unexpected error entries in successful run", label)
				return
			}
		}
	}
}

// TestIntegration_ChainWithErrorLogs tests log viewing for a chain with error-level logs.
// This verifies that error logs are properly captured even when steps complete.
//
//nolint:paralleltest // mutates the package-level runner.SetExecutor mock; cannot run concurrent tests
func TestIntegration_ChainWithErrorLogs(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	runner.SetExecutor(mockExec)

	defer runner.SetExecutor(nil)

	client := setupChainWithErrorLogsMocks(t, mockExec)
	w := testutil.NewMockRunWatcher()

	chainDef := &config.Chain{
		Description: "CI and Deploy with error logs",
		Steps: []config.ChainStep{
			{Workflow: "ci.yml", WaitFor: config.WaitNone, OnFailure: config.FailureAbort},
			{Workflow: "deploy.yml", WaitFor: config.WaitNone, OnFailure: config.FailureAbort},
		},
	}

	executor := chain.NewExecutor(client, w, "ci-deploy-warnings", chainDef)

	if err := executor.Start(map[string]string{}, "main"); err != nil {
		t.Fatalf("chain start failed: %v", err)
	}

	testutil.DrainChainUpdates(t, executor.Updates(), 3*time.Second)

	state := executor.State()
	if state.Status != chain.ChainCompleted {
		t.Errorf("chain status: got %v, want %v", state.Status, chain.ChainCompleted)
	}

	step1 := checkChainStepResult(t, state.StepResults[0], "step 1", 7001)
	step2 := checkChainStepResult(t, state.StepResults[1], "step 2", 7002)

	ghClient, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(ghClient, mockExec)

	deployStepLogs, err := fetcher.FetchStepLogsReal(step2.RunID, "deploy.yml")
	if err != nil {
		t.Fatalf("failed to fetch deploy.yml logs: %v", err)
	}

	checkDeployStepWarningsAndErrors(t, deployStepLogs)
	checkStepLogsContain(t, "deploy.yml", deployStepLogs, len(deployStepLogs), "Deployment successful despite warnings")

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

// setupChainWithErrorLogsMocks registers mock "gh workflow run", "gh api"
// (jobs), and "gh run view" (logs) responses for a ci.yml -> deploy.yml chain
// where the deploy step's logs contain non-fatal warnings/errors, and returns
// a GitHub client seeded with matching run metadata.
func setupChainWithErrorLogsMocks(t *testing.T, mockExec *exec.MockExecutor) *testutil.MockGitHubClient {
	t.Helper()

	// Step 1: ci.yml succeeds (runID 7001)
	mockExec.AddCommand("gh", []string{"workflow", "run", "ci.yml", "--ref", "main"}, "", "", nil)

	jobsRespCI := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 8001, Name: "test", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			Steps: []github.Step{
				{Name: "Run tests", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
			},
		}},
	}
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/7001/jobs?per_page=100"},
		testutil.MustMarshalJSON(t, jobsRespCI), "", nil)

	ciLogs := ghRunViewLog("test",
		ghStep{"Run tests", []string{"Running tests...", "All tests passed"}},
	)
	mockExec.AddGHRunView(7001, 8001, ciLogs)

	// Step 2: deploy.yml succeeds but has warnings/errors in logs (runID 7002)
	mockExec.AddCommand("gh", []string{"workflow", "run", "deploy.yml", "--ref", "main"}, "", "", nil)

	jobsRespDeploy := github.JobsResponse{
		Jobs: []github.Job{{
			ID: 8002, Name: "deploy", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			Steps: []github.Step{
				{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
				{Name: "Deploy", Number: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
			},
		}},
	}
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/7002/jobs?per_page=100"},
		testutil.MustMarshalJSON(t, jobsRespDeploy), "", nil)

	deployLogs := ghRunViewLog("deploy",
		ghStep{"Checkout", []string{"Checking out code..."}},
		ghStep{"Deploy", []string{
			"Deploying to production...",
			"##[warning]Deprecation notice: API v1 will be sunset in 6 months",
			"##[error]Non-critical error: Cache miss for dependency X",
			"Using fallback configuration",
			"Deployment successful despite warnings",
		}},
	)
	mockExec.AddGHRunView(7002, 8002, deployLogs)

	client := testutil.NewMockGitHubClient().
		WithRun(&github.WorkflowRun{
			ID: 7001, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			HTMLURL: "https://github.com/owner/repo/actions/runs/7001",
		}).
		WithRun(&github.WorkflowRun{
			ID: 7002, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			HTMLURL: "https://github.com/owner/repo/actions/runs/7002",
		})
	client.LatestByWorkflow["ci.yml"] = 7001
	client.LatestByWorkflow["deploy.yml"] = 7002

	return client
}

// deployLogCounts tallies warning/error entries and whether specific expected
// messages were found, across a deploy step's logs.
type deployLogCounts struct {
	warningCount     int
	errorCount       int
	foundDeprecation bool
	foundCacheMiss   bool
}

// countDeployStepLevels tallies warning/error entries in deployStepLogs and
// notes whether the expected deprecation warning and cache-miss error appear.
func countDeployStepLevels(deployStepLogs []*logs.StepLogs) deployLogCounts {
	var counts deployLogCounts

	for _, step := range deployStepLogs {
		for _, entry := range step.Entries {
			switch entry.Level {
			case logs.LogLevelWarning:
				counts.warningCount++

				if entry.Content == "##[warning]Deprecation notice: API v1 will be sunset in 6 months" {
					counts.foundDeprecation = true
				}
			case logs.LogLevelError:
				counts.errorCount++

				if entry.Content == "##[error]Non-critical error: Cache miss for dependency X" {
					counts.foundCacheMiss = true
				}
			case logs.LogLevelInfo, logs.LogLevelDebug, logs.LogLevelUnknown:
				// not counted; only warnings/errors are tallied here
			}
		}
	}

	return counts
}

// checkDeployStepWarningsAndErrors asserts the deploy step's logs contain the
// expected deprecation warning and non-critical cache-miss error.
func checkDeployStepWarningsAndErrors(t *testing.T, deployStepLogs []*logs.StepLogs) {
	t.Helper()

	counts := countDeployStepLevels(deployStepLogs)

	if counts.warningCount == 0 {
		t.Error("expected warning-level log entries in deploy step")
	}

	if counts.errorCount == 0 {
		t.Error("expected error-level log entries in deploy step (non-critical)")
	}

	if !counts.foundDeprecation {
		t.Error("expected to find deprecation warning in logs")
	}

	if !counts.foundCacheMiss {
		t.Error("expected to find cache miss error in logs")
	}
}

// ghStep is one step of a fabricated `gh run view --log` transcript.
type ghStep struct {
	name  string
	lines []string
}

// ghRunViewLog renders steps the way `gh run view --log` does: every line
// prefixed with the job name, the step name, and a timestamp, all tab
// separated. Fixtures written as bare ##[group] blocks do not exercise the
// parser, because gh never emits that shape.
func ghRunViewLog(job string, steps ...ghStep) string {
	var b strings.Builder

	stamp := time.Date(2026, time.September, 1, 3, 17, 43, 0, time.UTC)

	for _, step := range steps {
		for _, line := range step.lines {
			fmt.Fprintf(&b, "%s\t%s\t%s %s\n", job, step.name, stamp.Format(time.RFC3339Nano), line)
			stamp = stamp.Add(time.Second)
		}
	}

	return b.String()
}
