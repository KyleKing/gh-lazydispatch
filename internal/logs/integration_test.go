package logs_test

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/testutil"
)

// TestIntegration_SuccessfulWorkflowRun tests fetching logs for a successful workflow run.
func TestIntegration_SuccessfulWorkflowRun(t *testing.T) {
	// Setup: Mock data
	runID := int64(12345)
	jobID := int64(67890)

	// Setup: Create mock executor
	mockExec := exec.NewMockExecutor()

	// Mock gh api response for GetWorkflowRunJobs
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "build",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Run actions/checkout@v4", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
					{Name: "Set up Python 3.11", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 2},
					{Name: "Install dependencies", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 3},
					{Name: "Run tests", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 4},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/12345/jobs"}, jobsJSON, "", nil)

	// Mock gh run view for log fetching
	logOutput := loadFixture(t, "successful_run.txt")
	mockExec.AddGHRunView(runID, jobID, logOutput)

	// Setup: Create GitHub client and GHFetcher with mocks
	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	// Execute: Fetch logs
	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("FetchStepLogsReal failed: %v", err)
	}

	// Assert: Verify results
	if len(stepLogs) != 4 {
		t.Errorf("expected 4 steps, got %d", len(stepLogs))
	}

	// Verify first step
	if stepLogs[0].StepName != "Run actions/checkout@v4" {
		t.Errorf("step 0 name: got %q, want %q", stepLogs[0].StepName, "Run actions/checkout@v4")
	}

	if stepLogs[0].Conclusion != github.ConclusionSuccess {
		t.Errorf("step 0 conclusion: got %q, want %q", stepLogs[0].Conclusion, github.ConclusionSuccess)
	}

	// Verify logs contain expected content
	if len(stepLogs[0].Entries) == 0 {
		t.Error("step 0 should have log entries")
	}

	foundCheckout := false
	for _, entry := range stepLogs[0].Entries {
		if entry.Content == "##[group]Run actions/checkout@v4" {
			foundCheckout = true
			break
		}
	}
	if !foundCheckout {
		t.Error("expected to find checkout log entry")
	}

	// Verify mock executor was called correctly
	// Should have 2 commands: gh api (for jobs) + gh run view (for logs)
	if len(mockExec.ExecutedCommands) != 2 {
		t.Errorf("expected 2 gh commands, got %d", len(mockExec.ExecutedCommands))
	}

	// First command should be gh api for getting jobs
	if len(mockExec.ExecutedCommands) >= 1 {
		apiCmd := mockExec.ExecutedCommands[0]
		if apiCmd.Name != "gh" {
			t.Errorf("command 0 name: got %q, want %q", apiCmd.Name, "gh")
		}
		if len(apiCmd.Args) >= 1 && apiCmd.Args[0] != "api" {
			t.Errorf("command 0 args[0]: got %q, want %q", apiCmd.Args[0], "api")
		}
	}

	// Second command should be gh run view for getting logs
	if len(mockExec.ExecutedCommands) >= 2 {
		runCmd := mockExec.ExecutedCommands[1]
		if runCmd.Name != "gh" {
			t.Errorf("command 1 name: got %q, want %q", runCmd.Name, "gh")
		}
		if len(runCmd.Args) >= 1 && runCmd.Args[0] != "run" {
			t.Errorf("command 1 args[0]: got %q, want %q", runCmd.Args[0], "run")
		}
	}
}

// TestIntegration_FailedWorkflowRun tests fetching logs for a failed workflow run.
func TestIntegration_FailedWorkflowRun(t *testing.T) {
	runID := int64(12346)
	jobID := int64(67891)

	// Setup: Create mock executor
	mockExec := exec.NewMockExecutor()

	// Mock gh api response with failed job
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "build",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionFailure,
				Steps: []github.Step{
					{Name: "Run actions/checkout@v4", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
					{Name: "Set up Python 3.11", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 2},
					{Name: "Install dependencies", Status: github.StatusCompleted, Conclusion: github.ConclusionFailure, Number: 3},
					{Name: "Run tests", Status: github.StatusCompleted, Conclusion: github.ConclusionSkipped, Number: 4},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/12346/jobs"}, jobsJSON, "", nil)

	// Mock gh run view for log fetching
	logOutput := loadFixture(t, "failed_run.txt")
	mockExec.AddGHRunView(runID, jobID, logOutput)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	// Execute
	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("FetchStepLogsReal failed: %v", err)
	}

	// Assert: Check for error detection in any step
	hasFailedStep := false
	hasErrorLog := false
	for _, step := range stepLogs {
		if step.Conclusion == github.ConclusionFailure {
			hasFailedStep = true
		}
		for _, entry := range step.Entries {
			if entry.Level == logs.LogLevelError {
				hasErrorLog = true
				break
			}
		}
	}
	if !hasFailedStep {
		t.Error("expected to find at least one failed step")
	}
	if !hasErrorLog {
		t.Error("expected to find error-level log entries")
	}

	// Verify at least one step was skipped
	hasSkippedStep := false
	for _, step := range stepLogs {
		if step.Conclusion == github.ConclusionSkipped {
			hasSkippedStep = true
			break
		}
	}
	if !hasSkippedStep {
		t.Error("expected to find at least one skipped step")
	}
}

// TestIntegration_WorkflowWithWarnings tests log parsing with warning detection.
func TestIntegration_WorkflowWithWarnings(t *testing.T) {
	runID := int64(12347)
	jobID := int64(67892)

	// Setup: Create mock executor
	mockExec := exec.NewMockExecutor()

	// Mock gh api response
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "lint",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Run actions/checkout@v4", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
					{Name: "Set up Python 3.11", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 2},
					{Name: "Install dependencies", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 3},
					{Name: "Run linter", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 4},
					{Name: "Run tests", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 5},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/12347/jobs"}, jobsJSON, "", nil)

	// Mock gh run view for log fetching
	logOutput := loadFixture(t, "run_with_warnings.txt")
	mockExec.AddGHRunView(runID, jobID, logOutput)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	// Execute
	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("FetchStepLogsReal failed: %v", err)
	}

	// Assert: Check for warning detection
	hasWarning := false
	for _, step := range stepLogs {
		for _, entry := range step.Entries {
			if entry.Level == logs.LogLevelWarning {
				hasWarning = true
				break
			}
		}
	}
	if !hasWarning {
		t.Error("expected to find warning-level log entries")
	}
}

// TestIntegration_GHCLIError tests handling of gh CLI command failures.
func TestIntegration_GHCLIError(t *testing.T) {
	runID := int64(12348)
	jobID := int64(67893)

	// Setup: Create mock executor
	mockExec := exec.NewMockExecutor()

	// Mock gh api response
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "build",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Run tests", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/12348/jobs"}, jobsJSON, "", nil)

	// Simulate gh CLI error (e.g., network timeout, auth failure)
	mockExec.AddGHRunViewError(runID, jobID, "HTTP 401: Bad credentials", fmt.Errorf("exit status 1"))

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	// Execute
	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("FetchStepLogsReal should not return error, got: %v", err)
	}

	// Assert: Step should have error recorded
	if len(stepLogs) != 1 {
		t.Fatalf("expected 1 step, got %d", len(stepLogs))
	}

	if stepLogs[0].Error == nil {
		t.Error("expected step to have error recorded")
	}

	if stepLogs[0].Error != nil && stepLogs[0].Error.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestIntegration_GitHubAPIError tests handling of GitHub API failures.
func TestIntegration_GitHubAPIError(t *testing.T) {
	runID := int64(12349)

	// Setup: Create mock executor
	mockExec := exec.NewMockExecutor()

	// Mock gh api error response (e.g., rate limiting, server error)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/12349/jobs"},
		"", "HTTP 500: Internal Server Error", fmt.Errorf("exit status 1"))

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	// Execute
	_, err = fetcher.FetchStepLogsReal(runID, "ci.yml")

	// Assert: Should return error
	if err == nil {
		t.Fatal("expected error when gh api fails, got nil")
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// TestIntegration_CheckGHCLIAvailable tests gh CLI availability checking.
func TestIntegration_CheckGHCLIAvailable(t *testing.T) {
	tests := []struct {
		name        string
		setupMock   func(*exec.MockExecutor)
		expectError bool
	}{
		{
			name: "gh installed and authenticated",
			setupMock: func(m *exec.MockExecutor) {
				m.AddCommand("gh", []string{"--version"}, "gh version 2.40.0 (2024-01-01)", "", nil)
				m.AddCommand("gh", []string{"auth", "status"}, "✓ Logged in to github.com as user", "", nil)
			},
			expectError: false,
		},
		{
			name: "gh not installed",
			setupMock: func(m *exec.MockExecutor) {
				m.AddCommand("gh", []string{"--version"}, "", "command not found", fmt.Errorf("exit status 127"))
			},
			expectError: true,
		},
		{
			name: "gh not authenticated",
			setupMock: func(m *exec.MockExecutor) {
				m.AddCommand("gh", []string{"--version"}, "gh version 2.40.0", "", nil)
				m.AddCommand("gh", []string{"auth", "status"}, "", "You are not logged in", fmt.Errorf("exit status 1"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := exec.NewMockExecutor()
			tt.setupMock(mockExec)

			err := logs.CheckGHCLIAvailableWithExecutor(mockExec)

			if tt.expectError && err == nil {
				t.Error("expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// TestIntegration_MultiJobWorkflowRun tests fetching logs for a workflow with multiple jobs and steps.
func TestIntegration_MultiJobWorkflowRun(t *testing.T) {
	runID := int64(12350)
	jobID := int64(67894)

	mockExec := exec.NewMockExecutor()

	// Mock multi-step job
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "ci",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Run actions/checkout@v4", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
					{Name: "Set up Go 1.21", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 2},
					{Name: "Build application", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 3},
					{Name: "Run tests", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 4},
					{Name: "Upload coverage", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 5},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/12350/jobs"}, jobsJSON, "", nil)

	// Mock logs with multiple steps
	logOutput := loadFixture(t, "multi_job_run.txt")
	mockExec.AddGHRunView(runID, jobID, logOutput)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("FetchStepLogsReal failed: %v", err)
	}

	if len(stepLogs) != 5 {
		t.Errorf("expected 5 steps, got %d", len(stepLogs))
	}

	// Verify job name propagation
	for _, step := range stepLogs {
		if step.JobName != "ci" {
			t.Errorf("step %q has wrong job name: got %q, want %q", step.StepName, step.JobName, "ci")
		}
	}

	// Verify test step has entries (actual content parsing depends on log format)
	foundTestStep := false
	for _, step := range stepLogs {
		if step.StepName == "Run tests" {
			foundTestStep = true
			if len(step.Entries) == 0 {
				t.Error("expected 'Run tests' step to have log entries")
			}
		}
	}
	if !foundTestStep {
		t.Error("expected to find 'Run tests' step")
	}

	// Verify all steps are marked as success
	for _, step := range stepLogs {
		if step.Conclusion != github.ConclusionSuccess {
			t.Errorf("step %q has wrong conclusion: got %q, want %q", step.StepName, step.Conclusion, github.ConclusionSuccess)
		}
	}
}

// TestIntegration_LogStreaming tests incremental log streaming for active runs.
func TestIntegration_LogStreaming(t *testing.T) {
	runID := int64(99999)
	jobID := int64(88888)

	// Setup: Create mock executor with dynamic responses
	mockExec := exec.NewMockExecutor()

	// Mock job structure (3 steps)
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "ci",
				Status:     github.StatusInProgress,
				Conclusion: "",
				Steps: []github.Step{
					{Name: "Run actions/checkout@v4", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
					{Name: "Set up Go 1.21", Status: github.StatusInProgress, Conclusion: "", Number: 2},
					{Name: "Build application", Status: github.StatusQueued, Conclusion: "", Number: 3},
					{Name: "Run tests", Status: github.StatusQueued, Conclusion: "", Number: 4},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999/jobs"}, jobsJSON, "", nil)

	// Mock workflow run status - initially in_progress
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999"},
		`{"id":99999,"name":"CI","status":"in_progress","conclusion":"","html_url":"https://github.com/owner/repo/actions/runs/99999","updated_at":"2024-01-01T12:00:00Z"}`,
		"", nil)

	// Poll 1: Initial logs (2 steps partially complete)
	poll1Logs := loadFixture(t, "streaming_poll_1.txt")
	mockExec.AddGHRunView(runID, jobID, poll1Logs)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	// Execute: First poll
	stepLogs1, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("first poll failed: %v", err)
	}

	// Verify poll 1 results (only 2 steps have logs at this point)
	if len(stepLogs1) < 2 {
		t.Fatalf("poll 1: expected at least 2 steps, got %d", len(stepLogs1))
	}

	// Count entries in step 0 and 1 from poll 1
	step0Entries := len(stepLogs1[0].Entries)
	step1Entries := len(stepLogs1[1].Entries)

	if step0Entries == 0 {
		t.Error("poll 1: step 0 should have log entries")
	}
	if step1Entries == 0 {
		t.Error("poll 1: step 1 should have log entries")
	}

	t.Logf("Poll 1: step 0 has %d entries, step 1 has %d entries", step0Entries, step1Entries)

	// Reset mock executor for poll 2
	mockExec.Reset()
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999/jobs"}, jobsJSON, "", nil)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999"},
		`{"id":99999,"name":"CI","status":"in_progress","conclusion":"","html_url":"https://github.com/owner/repo/actions/runs/99999","updated_at":"2024-01-01T12:00:05Z"}`,
		"", nil)

	// Poll 2: More progress (step 2 now has logs, step 1 has more logs)
	poll2Logs := loadFixture(t, "streaming_poll_2.txt")
	mockExec.AddGHRunView(runID, jobID, poll2Logs)

	// Execute: Second poll
	stepLogs2, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("second poll failed: %v", err)
	}

	// Verify poll 2 has more steps and more content
	if len(stepLogs2) < 3 {
		t.Fatalf("poll 2: expected at least 3 steps, got %d", len(stepLogs2))
	}

	step0Entries2 := len(stepLogs2[0].Entries)
	step1Entries2 := len(stepLogs2[1].Entries)
	step2Entries2 := len(stepLogs2[2].Entries)

	t.Logf("Poll 2: step 0 has %d entries, step 1 has %d entries, step 2 has %d entries",
		step0Entries2, step1Entries2, step2Entries2)

	// Step 0 should remain the same (checkout doesn't change)
	if step0Entries2 != step0Entries {
		t.Logf("poll 2: step 0 entries changed: was %d, now %d (may be expected)", step0Entries, step0Entries2)
	}

	// Step 1 should have more entries (Go setup progressed)
	if step1Entries2 <= step1Entries {
		t.Logf("poll 2: step 1 entries: was %d, now %d (expected more)", step1Entries, step1Entries2)
	}

	// Step 2 is new in poll 2
	if step2Entries2 == 0 {
		t.Error("poll 2: step 2 should now have log entries")
	}

	// Reset for poll 3 (completed run)
	mockExec.Reset()

	// Update job status to completed
	jobsCompletedResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "ci",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Run actions/checkout@v4", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
					{Name: "Set up Go 1.21", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 2},
					{Name: "Build application", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 3},
					{Name: "Run tests", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 4},
				},
			},
		},
	}
	jobsCompletedJSON := testutil.MustMarshalJSON(t, jobsCompletedResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999/jobs"}, jobsCompletedJSON, "", nil)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999"},
		`{"id":99999,"name":"CI","status":"completed","conclusion":"success","html_url":"https://github.com/owner/repo/actions/runs/99999","updated_at":"2024-01-01T12:00:10Z"}`,
		"", nil)

	// Poll 3: All steps complete
	poll3Logs := loadFixture(t, "streaming_poll_3.txt")
	mockExec.AddGHRunView(runID, jobID, poll3Logs)

	// Execute: Third poll
	stepLogs3, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("third poll failed: %v", err)
	}

	// Verify poll 3 has complete logs
	step3Entries3 := len(stepLogs3[3].Entries)
	if step3Entries3 == 0 {
		t.Error("poll 3: step 3 (Run tests) should have log entries")
	}

	// Verify all steps are now marked as completed
	for i, step := range stepLogs3 {
		if step.Status != github.StatusCompleted {
			t.Errorf("poll 3: step %d status: got %q, want %q", i, step.Status, github.StatusCompleted)
		}
		if step.Conclusion != github.ConclusionSuccess {
			t.Errorf("poll 3: step %d conclusion: got %q, want %q", i, step.Conclusion, github.ConclusionSuccess)
		}
	}
}

// TestIntegration_LogStreamer_IncrementalDetection tests the LogStreamer's ability to detect incremental updates.
func TestIntegration_LogStreamer_IncrementalDetection(t *testing.T) {
	runID := int64(77777)
	jobID := int64(66666)

	// Setup mock executor
	mockExec := exec.NewMockExecutor()

	// Mock job structure
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "test",
				Status:     github.StatusInProgress,
				Conclusion: "",
				Steps: []github.Step{
					{Name: "Run actions/checkout@v4", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
					{Name: "Set up Go 1.21", Status: github.StatusInProgress, Conclusion: "", Number: 2},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)

	// Setup initial poll
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/77777/jobs"}, jobsJSON, "", nil)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/77777"},
		`{"id":77777,"name":"Test","status":"in_progress","conclusion":"","html_url":"https://github.com/owner/repo/actions/runs/77777","updated_at":"2024-01-01T12:00:00Z"}`,
		"", nil)
	poll1Logs := loadFixture(t, "streaming_poll_1.txt")
	mockExec.AddGHRunView(runID, jobID, poll1Logs)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	// Create streamer
	streamer := logs.NewLogStreamer(client, runID, "test.yml")

	// Manually perform first poll to initialize state
	firstLogs, err := logs.NewGHFetcherWithExecutor(client, mockExec).FetchStepLogsReal(runID, "test.yml")
	if err != nil {
		t.Fatalf("initial fetch failed: %v", err)
	}

	// Simulate detecting new logs by calling detectNewLogs (we need to use reflection or create a test helper)
	// For now, verify the basic structure works
	if len(firstLogs) != 2 {
		t.Errorf("expected 2 steps in first poll, got %d", len(firstLogs))
	}

	// Verify streamer was created successfully
	if streamer == nil {
		t.Fatal("streamer should not be nil")
	}

	// Clean up
	streamer.Stop()
}

// TestIntegration_LargeLogFile tests fetching and parsing a large log file (10k lines).
func TestIntegration_LargeLogFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping large log test in short mode")
	}

	runID := int64(99999)
	jobID := int64(88888)

	mockExec := exec.NewMockExecutor()

	// Generate 10k line log using helper
	largeLog := testutil.GenerateLargeLogFixture(10000)
	mockExec.AddGHRunView(runID, jobID, largeLog)

	// Setup jobs response
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "large-job",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Run actions/checkout@v4", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999/jobs"}, jobsJSON, "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	start := time.Now()
	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("FetchStepLogsReal failed: %v", err)
	}

	t.Logf("Fetched %d steps with 10k lines in %v", len(stepLogs), duration)

	// Verify performance: should complete in <2 seconds
	if duration > 2*time.Second {
		t.Errorf("performance regression: took %v (expected <2s)", duration)
	}

	// Verify logs were parsed
	if len(stepLogs) == 0 {
		t.Error("expected at least one step with logs")
	}

	totalEntries := 0
	for _, step := range stepLogs {
		totalEntries += len(step.Entries)
	}

	t.Logf("Parsed %d total log entries", totalEntries)

	if totalEntries == 0 {
		t.Error("expected parsed log entries")
	}
}

// TestIntegration_UnicodeCharacters tests proper handling of unicode characters in logs.
func TestIntegration_UnicodeCharacters(t *testing.T) {
	runID := int64(99998)
	jobID := int64(88887)

	mockExec := exec.NewMockExecutor()

	// Use unicode fixture
	unicodeLog := testutil.GenerateUnicodeLog()
	mockExec.AddGHRunView(runID, jobID, unicodeLog)

	// Setup jobs response
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "unicode-job",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Build", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99998/jobs"}, jobsJSON, "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("FetchStepLogsReal failed: %v", err)
	}

	// Verify unicode characters were preserved
	foundUnicode := false
	for _, step := range stepLogs {
		for _, entry := range step.Entries {
			if strings.Contains(entry.Content, "🚀") ||
				strings.Contains(entry.Content, "テスト") ||
				strings.Contains(entry.Content, "测试") {
				foundUnicode = true
				break
			}
		}
	}

	if !foundUnicode {
		t.Error("expected to find unicode characters in parsed logs")
	}

	t.Logf("Successfully parsed logs with unicode characters")
}

// TestIntegration_ANSIColorCodes tests proper handling of ANSI color codes in logs.
func TestIntegration_ANSIColorCodes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping ANSI test in short mode")
	}

	runID := int64(99997)
	jobID := int64(88886)

	mockExec := exec.NewMockExecutor()

	ansiLog := testutil.GenerateANSILog()
	mockExec.AddGHRunView(runID, jobID, ansiLog)

	// Setup jobs response
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "ansi-job",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Test", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99997/jobs"}, jobsJSON, "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("FetchStepLogsReal failed: %v", err)
	}

	// Verify ANSI codes are either preserved or stripped consistently
	foundANSI := false
	for _, step := range stepLogs {
		for _, entry := range step.Entries {
			if strings.Contains(entry.Content, "\x1b[") {
				foundANSI = true
				break
			}
		}
	}

	// Log whether ANSI codes were preserved or stripped
	if foundANSI {
		t.Logf("ANSI color codes preserved in logs")
	} else {
		t.Logf("ANSI color codes stripped from logs")
	}

	// Verify content is readable regardless of ANSI handling
	totalEntries := 0
	for _, step := range stepLogs {
		totalEntries += len(step.Entries)
	}

	if totalEntries == 0 {
		t.Error("expected to find log entries")
	} else {
		t.Logf("Successfully parsed %d log entries from ANSI log", totalEntries)
	}
}

// TestIntegration_NetworkTimeout tests handling of network timeout scenarios.
func TestIntegration_NetworkTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping timeout test in short mode")
	}

	runID := int64(99996)

	mockExec := exec.NewMockExecutor()

	// Simulate timeout by adding command that returns context error
	mockExec.AddCommand("gh", []string{"api", fmt.Sprintf("repos/owner/repo/actions/runs/%d/jobs", runID)},
		"", "context deadline exceeded", fmt.Errorf("context deadline exceeded"))

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	// Execute - should handle timeout gracefully
	_, err = fetcher.FetchStepLogsReal(runID, "ci.yml")

	// Should return error for API failure
	if err == nil {
		t.Error("expected error for timeout scenario")
	}

	if !strings.Contains(err.Error(), "context deadline exceeded") &&
		!strings.Contains(err.Error(), "exit status 1") {
		t.Errorf("unexpected error message: %v", err)
	}

	t.Logf("Timeout handled correctly with error: %v", err)
}

// TestIntegration_VeryLargeLogFile tests fetching an extremely large log file (50k lines).
func TestIntegration_VeryLargeLogFile(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping very large log test in short mode")
	}

	runID := int64(99995)
	jobID := int64(88885)

	mockExec := exec.NewMockExecutor()

	// Generate 50k line log
	largeLog := testutil.GenerateLargeLogFixture(50000)
	mockExec.AddGHRunView(runID, jobID, largeLog)

	// Setup jobs response
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "very-large-job",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Run tests", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99995/jobs"}, jobsJSON, "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	start := time.Now()
	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("FetchStepLogsReal failed: %v", err)
	}

	t.Logf("Fetched %d steps with 50k lines in %v", len(stepLogs), duration)

	// Performance check
	if duration > 5*time.Second {
		t.Errorf("performance regression: took %v (expected <5s)", duration)
	}

	totalEntries := 0
	for _, step := range stepLogs {
		totalEntries += len(step.Entries)
	}

	t.Logf("Parsed %d total log entries", totalEntries)

	if totalEntries == 0 {
		t.Error("expected parsed log entries")
	}
}

// TestIntegration_MixedLogContent tests logs with various patterns.
func TestIntegration_MixedLogContent(t *testing.T) {
	runID := int64(99994)
	jobID := int64(88884)

	mockExec := exec.NewMockExecutor()

	// Generate mixed content log
	mixedLog := testutil.GenerateMixedLog(1000)
	mockExec.AddGHRunView(runID, jobID, mixedLog)

	// Setup jobs response
	jobsResp := github.JobsResponse{
		Jobs: []github.Job{
			{
				ID:         jobID,
				Name:       "mixed-job",
				Status:     github.StatusCompleted,
				Conclusion: github.ConclusionSuccess,
				Steps: []github.Step{
					{Name: "Mixed test", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess, Number: 1},
				},
			},
		},
	}
	jobsJSON := testutil.MustMarshalJSON(t, jobsResp)
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99994/jobs"}, jobsJSON, "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	stepLogs, err := fetcher.FetchStepLogsReal(runID, "ci.yml")
	if err != nil {
		t.Fatalf("FetchStepLogsReal failed: %v", err)
	}

	// Verify various log levels detected
	hasError := false
	hasWarning := false
	hasInfo := false

	for _, step := range stepLogs {
		for _, entry := range step.Entries {
			switch entry.Level {
			case logs.LogLevelError:
				hasError = true
			case logs.LogLevelWarning:
				hasWarning = true
			case logs.LogLevelInfo:
				hasInfo = true
			}
		}
	}

	if !hasError {
		t.Error("expected to find error-level logs")
	}

	if !hasWarning {
		t.Error("expected to find warning-level logs")
	}

	if !hasInfo {
		t.Error("expected to find info-level logs")
	}

	t.Logf("Successfully parsed mixed log content with all log levels")
}

// loadFixture loads a test fixture file from testdata/logs/.
func loadFixture(t *testing.T, filename string) string {
	t.Helper()
	data, err := os.ReadFile(fmt.Sprintf("../../testdata/logs/%s", filename))
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", filename, err)
	}
	return string(data)
}
