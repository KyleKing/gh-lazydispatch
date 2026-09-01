package github_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/testutil"
)

func TestNewClientWithExecutor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		repoName    string
		expectError bool
		wantOwner   string
		wantRepo    string
	}{
		{
			name:        "valid repo format",
			repoName:    "owner/repo",
			expectError: false,
			wantOwner:   "owner",
			wantRepo:    "repo",
		},
		{
			name:        "with organization",
			repoName:    "my-org/my-repo",
			expectError: false,
			wantOwner:   "my-org",
			wantRepo:    "my-repo",
		},
		{
			name:        "invalid format - no slash",
			repoName:    "invalid",
			expectError: true,
		},
		{
			name:        "invalid format - empty",
			repoName:    "",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockExecutor()
			client, err := github.NewClientWithExecutor(tt.repoName, mockExec)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if client.Owner() != tt.wantOwner {
				t.Errorf("Owner() = %q, want %q", client.Owner(), tt.wantOwner)
			}

			if client.Repo() != tt.wantRepo {
				t.Errorf("Repo() = %q, want %q", client.Repo(), tt.wantRepo)
			}
		})
	}
}

// mockWorkflowRunSuccess registers a successful "gh api .../runs/<runID>" response for run.
func mockWorkflowRunSuccess(t *testing.T, m *testutil.MockExecutor, runID int64, run github.WorkflowRun) {
	t.Helper()

	runJSON, err := json.Marshal(run)
	if err != nil {
		t.Fatalf("failed to marshal run: %v", err)
	}

	path := fmt.Sprintf("repos/owner/repo/actions/runs/%d", runID)
	m.AddCommand("gh", []string{"api", path}, string(runJSON), "", nil)
}

func TestClient_GetWorkflowRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runID       int64
		setupMock   func(*testing.T, *testutil.MockExecutor)
		expectError bool
		wantStatus  string
	}{
		{
			name:  "successful fetch",
			runID: 12345,
			setupMock: func(t *testing.T, m *testutil.MockExecutor) {
				t.Helper()
				mockWorkflowRunSuccess(t, m, 12345, github.WorkflowRun{
					ID:         12345,
					Name:       "CI",
					Status:     github.StatusCompleted,
					Conclusion: github.ConclusionSuccess,
					HTMLURL:    "https://github.com/owner/repo/actions/runs/12345",
				})
			},
			expectError: false,
			wantStatus:  github.StatusCompleted,
		},
		{
			name:  "run in progress",
			runID: 67890,
			setupMock: func(t *testing.T, m *testutil.MockExecutor) {
				t.Helper()
				mockWorkflowRunSuccess(t, m, 67890, github.WorkflowRun{
					ID:        67890,
					Name:      "Deploy",
					Status:    github.StatusInProgress,
					UpdatedAt: time.Now(),
				})
			},
			expectError: false,
			wantStatus:  github.StatusInProgress,
		},
		{
			name:  "API error",
			runID: 99999,
			setupMock: func(_ *testing.T, m *testutil.MockExecutor) {
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999"},
					"", "HTTP 404: Not Found", testutil.ErrMockExitStatus1)
			},
			expectError: true,
		},
		{
			name:  "invalid JSON response",
			runID: 11111,
			setupMock: func(_ *testing.T, m *testutil.MockExecutor) {
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/11111"},
					"invalid json", "", nil)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockExecutor()
			tt.setupMock(t, mockExec)

			client, err := github.NewClientWithExecutor("owner/repo", mockExec)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			run, err := client.GetWorkflowRun(tt.runID)
			checkWorkflowRunResult(t, run, err, tt.expectError, tt.runID, tt.wantStatus)
		})
	}
}

// checkWorkflowRunResult asserts the result of GetWorkflowRun against expectations.
func checkWorkflowRunResult(
	t *testing.T, run *github.WorkflowRun, err error, expectError bool, wantRunID int64, wantStatus string,
) {
	t.Helper()

	if expectError {
		if err == nil {
			t.Error("expected error, got nil")
		}

		return
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if run.ID != wantRunID {
		t.Errorf("run.ID = %d, want %d", run.ID, wantRunID)
	}

	if run.Status != wantStatus {
		t.Errorf("run.Status = %q, want %q", run.Status, wantStatus)
	}
}

// mockWorkflowRunJobs registers a successful "gh api .../runs/<runID>/jobs" response for jobs.
func mockWorkflowRunJobs(t *testing.T, m *testutil.MockExecutor, runID int64, jobs []github.Job) {
	t.Helper()

	respJSON, err := json.Marshal(github.JobsResponse{Jobs: jobs})
	if err != nil {
		t.Fatalf("failed to marshal resp: %v", err)
	}

	path := fmt.Sprintf("repos/owner/repo/actions/runs/%d/jobs?per_page=100", runID)
	m.AddCommand("gh", []string{"api", path}, string(respJSON), "", nil)
}

func TestClient_GetWorkflowRunJobs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		runID       int64
		setupMock   func(*testing.T, *testutil.MockExecutor)
		expectError bool
		wantJobs    int
	}{
		{
			name:  "single job",
			runID: 12345,
			setupMock: func(t *testing.T, m *testutil.MockExecutor) {
				t.Helper()
				mockWorkflowRunJobs(t, m, 12345, []github.Job{
					{
						ID:         1,
						Name:       "build",
						Status:     github.StatusCompleted,
						Conclusion: github.ConclusionSuccess,
						Steps: []github.Step{
							{
								Name: "Checkout", Number: 1,
								Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
							},
							{
								Name: "Build", Number: 2,
								Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
							},
						},
					},
				})
			},
			expectError: false,
			wantJobs:    1,
		},
		{
			name:  "multiple jobs",
			runID: 67890,
			setupMock: func(t *testing.T, m *testutil.MockExecutor) {
				t.Helper()
				mockWorkflowRunJobs(t, m, 67890, []github.Job{
					{ID: 1, Name: "lint", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
					{ID: 2, Name: "test", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
					{ID: 3, Name: "build", Status: github.StatusInProgress},
				})
			},
			expectError: false,
			wantJobs:    3,
		},
		{
			name:  "no jobs",
			runID: 11111,
			setupMock: func(t *testing.T, m *testutil.MockExecutor) {
				t.Helper()
				mockWorkflowRunJobs(t, m, 11111, []github.Job{})
			},
			expectError: false,
			wantJobs:    0,
		},
		{
			name:  "API error",
			runID: 99999,
			setupMock: func(_ *testing.T, m *testutil.MockExecutor) {
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999/jobs?per_page=100"},
					"", "HTTP 500: Internal Server Error", testutil.ErrMockExitStatus1)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockExecutor()
			tt.setupMock(t, mockExec)

			client, err := github.NewClientWithExecutor("owner/repo", mockExec)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			jobs, err := client.GetWorkflowRunJobs(tt.runID)
			checkWorkflowRunJobsResult(t, jobs, err, tt.expectError, tt.wantJobs)
		})
	}
}

// checkWorkflowRunJobsResult asserts the result of GetWorkflowRunJobs against expectations.
func checkWorkflowRunJobsResult(t *testing.T, jobs []github.Job, err error, expectError bool, wantJobs int) {
	t.Helper()

	if expectError {
		if err == nil {
			t.Error("expected error, got nil")
		}

		return
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(jobs) != wantJobs {
		t.Errorf("got %d jobs, want %d", len(jobs), wantJobs)
	}
}

// mockLatestRunResponse registers a "gh api .../runs?..." response listing runs for a workflow query.
func mockLatestRunResponse(t *testing.T, m *testutil.MockExecutor, query string, runs []github.WorkflowRun) {
	t.Helper()

	respJSON, err := json.Marshal(github.RunsResponse{WorkflowRuns: runs})
	if err != nil {
		t.Fatalf("failed to marshal resp: %v", err)
	}

	m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs?" + query}, string(respJSON), "", nil)
}

func TestClient_GetLatestRun(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		workflowName string
		setupMock    func(*testing.T, *testutil.MockExecutor)
		expectError  bool
		expectNil    bool
		wantRunID    int64
	}{
		{
			name:         "latest run found",
			workflowName: "ci.yml",
			setupMock: func(t *testing.T, m *testutil.MockExecutor) {
				t.Helper()
				mockLatestRunResponse(t, m, "per_page=1&workflow=ci.yml", []github.WorkflowRun{
					{ID: 12345, Name: "CI", Status: github.StatusQueued},
				})
			},
			expectError: false,
			wantRunID:   12345,
		},
		{
			name:         "no workflow filter",
			workflowName: "",
			setupMock: func(t *testing.T, m *testutil.MockExecutor) {
				t.Helper()
				mockLatestRunResponse(t, m, "per_page=1", []github.WorkflowRun{
					{ID: 99999, Name: "Any", Status: github.StatusInProgress},
				})
			},
			expectError: false,
			wantRunID:   99999,
		},
		{
			name:         "no runs found",
			workflowName: "nonexistent.yml",
			setupMock: func(t *testing.T, m *testutil.MockExecutor) {
				t.Helper()
				mockLatestRunResponse(t, m, "per_page=1&workflow=nonexistent.yml", []github.WorkflowRun{})
			},
			expectError: true,
		},
		{
			name:         "API rate limit",
			workflowName: "ci.yml",
			setupMock: func(_ *testing.T, m *testutil.MockExecutor) {
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs?per_page=1&workflow=ci.yml"},
					"", "HTTP 403: rate limit exceeded", testutil.ErrMockExitStatus1)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockExec := testutil.NewMockExecutor()
			tt.setupMock(t, mockExec)

			client, err := github.NewClientWithExecutor("owner/repo", mockExec)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			run, err := client.GetLatestRun(tt.workflowName)
			checkLatestRunResult(t, run, err, tt.expectError, tt.expectNil, tt.wantRunID)
		})
	}
}

// checkLatestRunResult asserts the result of GetLatestRun against expectations.
func checkLatestRunResult(
	t *testing.T, run *github.WorkflowRun, err error, expectError, expectNil bool, wantRunID int64,
) {
	t.Helper()

	if expectError {
		if err == nil {
			t.Error("expected error, got nil")
		}

		return
	}

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if expectNil {
		if run != nil {
			t.Errorf("expected nil run, got %+v", run)
		}

		return
	}

	if run.ID != wantRunID {
		t.Errorf("run.ID = %d, want %d", run.ID, wantRunID)
	}
}

func TestClient_CommandsExecuted(t *testing.T) {
	t.Parallel()

	mockExec := testutil.NewMockExecutor()

	resp := github.RunsResponse{
		WorkflowRuns: []github.WorkflowRun{{ID: 1, Name: "CI"}},
	}
	respJSON, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("failed to marshal resp: %v", err)
	}

	mockExec.AddCommand("gh", []string{"api", "repos/test/project/actions/runs?per_page=1&workflow=build.yml"},
		string(respJSON), "", nil)

	client, err := github.NewClientWithExecutor("test/project", mockExec)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}

	if _, err := client.GetLatestRun("build.yml"); err != nil {
		t.Fatalf("failed to get latest run: %v", err)
	}

	if len(mockExec.ExecutedCommands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(mockExec.ExecutedCommands))
	}

	cmd := mockExec.ExecutedCommands[0]
	if cmd.Name != "gh" {
		t.Errorf("command name = %q, want %q", cmd.Name, "gh")
	}

	if len(cmd.Args) < 2 || cmd.Args[0] != "api" {
		t.Errorf("expected 'gh api ...' command, got %v", cmd.Args)
	}
}

// A branch listing must report each workflow's current state rather than its
// history, and a workflow reporting its mode in the title has one state per mode.
func TestLatestRunsOnBranch(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC()
	payload := fmt.Sprintf(`{"workflow_runs":[
		{"id":1,"path":".github/workflows/ci.yml","name":"CI","status":"completed",
		 "conclusion":"success","head_branch":"topic","created_at":%q},
		{"id":2,"path":".github/workflows/ci.yml","name":"CI","status":"completed",
		 "conclusion":"failure","head_branch":"topic","created_at":%q},
		{"id":3,"path":".github/workflows/deploy.yml","name":"Deploy (preview)","status":"completed",
		 "conclusion":"success","head_branch":"topic","created_at":%q}
	]}`,
		now.Format(time.RFC3339),
		now.Add(-time.Hour).Format(time.RFC3339),
		now.Format(time.RFC3339),
	)

	mockExec := testutil.NewMockExecutor()
	mockExec.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs?branch=topic&per_page=100"},
		payload, "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatal(err)
	}

	runs, err := client.LatestRunsOnBranch("topic", 0)
	if err != nil {
		t.Fatal(err)
	}

	if len(runs) != 2 {
		t.Fatalf("got %d runs, want the newest CI plus the Deploy mode: %+v", len(runs), runs)
	}

	for _, run := range runs {
		if run.ID == 2 {
			t.Error("the superseded CI run survived")
		}

		if run.Path != "ci.yml" && run.Path != "deploy.yml" {
			t.Errorf("run %d kept the API's full path %q", run.ID, run.Path)
		}
	}
}
