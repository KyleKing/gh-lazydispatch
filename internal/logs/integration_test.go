package logs_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// TestIntegration_SuccessfulWorkflowRun tests fetching logs for a successful workflow run.
func TestIntegration_SuccessfulWorkflowRun(t *testing.T) {
	// Setup: Create mock GitHub client with httpmock
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	// Mock the GitHub API responses
	runID := int64(12345)
	jobID := int64(67890)

	// Mock GetWorkflowRunJobs response
	httpmock.RegisterResponder("GET", "https://api.github.com/repos/owner/repo/actions/runs/12345/jobs",
		httpmock.NewJsonResponderOrPanic(200, github.JobsResponse{
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
		}))

	// Setup: Create mock executor with test fixture
	mockExec := exec.NewMockExecutor()
	logOutput := loadFixture(t, "successful_run.txt")
	mockExec.AddGHRunView(runID, jobID, logOutput)

	// Setup: Create GitHub client and GHFetcher with mocks
	client, err := github.NewClient("owner/repo")
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
	if len(mockExec.ExecutedCommands) != 1 {
		t.Errorf("expected 1 gh command, got %d", len(mockExec.ExecutedCommands))
	}

	execCmd := mockExec.ExecutedCommands[0]
	if execCmd.Name != "gh" {
		t.Errorf("command name: got %q, want %q", execCmd.Name, "gh")
	}
}

// TestIntegration_FailedWorkflowRun tests fetching logs for a failed workflow run.
func TestIntegration_FailedWorkflowRun(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	runID := int64(12346)
	jobID := int64(67891)

	// Mock API response with failed job
	httpmock.RegisterResponder("GET", "https://api.github.com/repos/owner/repo/actions/runs/12346/jobs",
		httpmock.NewJsonResponderOrPanic(200, github.JobsResponse{
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
		}))

	mockExec := exec.NewMockExecutor()
	logOutput := loadFixture(t, "failed_run.txt")
	mockExec.AddGHRunView(runID, jobID, logOutput)

	client, err := github.NewClient("owner/repo")
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
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	runID := int64(12347)
	jobID := int64(67892)

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/owner/repo/actions/runs/12347/jobs",
		httpmock.NewJsonResponderOrPanic(200, github.JobsResponse{
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
		}))

	mockExec := exec.NewMockExecutor()
	logOutput := loadFixture(t, "run_with_warnings.txt")
	mockExec.AddGHRunView(runID, jobID, logOutput)

	client, err := github.NewClient("owner/repo")
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
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	runID := int64(12348)
	jobID := int64(67893)

	httpmock.RegisterResponder("GET", "https://api.github.com/repos/owner/repo/actions/runs/12348/jobs",
		httpmock.NewJsonResponderOrPanic(200, github.JobsResponse{
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
		}))

	mockExec := exec.NewMockExecutor()
	// Simulate gh CLI error (e.g., network timeout, auth failure)
	mockExec.AddGHRunViewError(runID, jobID, "HTTP 401: Bad credentials", fmt.Errorf("exit status 1"))

	client, err := github.NewClient("owner/repo")
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
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	runID := int64(12349)

	// Mock API error response (e.g., rate limiting, server error)
	httpmock.RegisterResponder("GET", "https://api.github.com/repos/owner/repo/actions/runs/12349/jobs",
		httpmock.NewJsonResponderOrPanic(500, map[string]string{
			"message": "Internal Server Error",
		}))

	mockExec := exec.NewMockExecutor()

	client, err := github.NewClient("owner/repo")
	if err != nil {
		t.Fatalf("failed to create GitHub client: %v", err)
	}

	fetcher := logs.NewGHFetcherWithExecutor(client, mockExec)

	// Execute
	_, err = fetcher.FetchStepLogsReal(runID, "ci.yml")

	// Assert: Should return error
	if err == nil {
		t.Fatal("expected error when GitHub API returns 500, got nil")
	}

	if err.Error() == "" {
		t.Error("expected non-empty error message")
	}
}

// NOTE: Rate limiting and other GitHub API error tests are skipped because
// the go-gh library creates its own HTTP client that doesn't use http.DefaultTransport,
// which prevents httpmock from intercepting requests. For true API integration testing,
// consider using a test GitHub instance or recording real API interactions.

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

// loadFixture loads a test fixture file from testdata/logs/.
func loadFixture(t *testing.T, filename string) string {
	t.Helper()
	data, err := os.ReadFile(fmt.Sprintf("../../testdata/logs/%s", filename))
	if err != nil {
		t.Fatalf("failed to load fixture %s: %v", filename, err)
	}
	return string(data)
}
