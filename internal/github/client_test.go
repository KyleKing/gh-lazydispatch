package github_test

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
)

func TestNewClientWithExecutor(t *testing.T) {
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
			mockExec := exec.NewMockExecutor()
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

func TestClient_GetWorkflowRun(t *testing.T) {
	tests := []struct {
		name        string
		runID       int64
		setupMock   func(*exec.MockExecutor)
		expectError bool
		wantStatus  string
	}{
		{
			name:  "successful fetch",
			runID: 12345,
			setupMock: func(m *exec.MockExecutor) {
				run := github.WorkflowRun{
					ID:         12345,
					Name:       "CI",
					Status:     github.StatusCompleted,
					Conclusion: github.ConclusionSuccess,
					HTMLURL:    "https://github.com/owner/repo/actions/runs/12345",
				}
				runJSON, _ := json.Marshal(run)
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/12345"}, string(runJSON), "", nil)
			},
			expectError: false,
			wantStatus:  github.StatusCompleted,
		},
		{
			name:  "run in progress",
			runID: 67890,
			setupMock: func(m *exec.MockExecutor) {
				run := github.WorkflowRun{
					ID:        67890,
					Name:      "Deploy",
					Status:    github.StatusInProgress,
					UpdatedAt: time.Now(),
				}
				runJSON, _ := json.Marshal(run)
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/67890"}, string(runJSON), "", nil)
			},
			expectError: false,
			wantStatus:  github.StatusInProgress,
		},
		{
			name:  "API error",
			runID: 99999,
			setupMock: func(m *exec.MockExecutor) {
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999"},
					"", "HTTP 404: Not Found", fmt.Errorf("exit status 1"))
			},
			expectError: true,
		},
		{
			name:  "invalid JSON response",
			runID: 11111,
			setupMock: func(m *exec.MockExecutor) {
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/11111"},
					"invalid json", "", nil)
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := exec.NewMockExecutor()
			tt.setupMock(mockExec)

			client, err := github.NewClientWithExecutor("owner/repo", mockExec)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			run, err := client.GetWorkflowRun(tt.runID)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if run.ID != tt.runID {
				t.Errorf("run.ID = %d, want %d", run.ID, tt.runID)
			}

			if run.Status != tt.wantStatus {
				t.Errorf("run.Status = %q, want %q", run.Status, tt.wantStatus)
			}
		})
	}
}

func TestClient_GetWorkflowRunJobs(t *testing.T) {
	tests := []struct {
		name        string
		runID       int64
		setupMock   func(*exec.MockExecutor)
		expectError bool
		wantJobs    int
	}{
		{
			name:  "single job",
			runID: 12345,
			setupMock: func(m *exec.MockExecutor) {
				resp := github.JobsResponse{
					Jobs: []github.Job{
						{
							ID:         1,
							Name:       "build",
							Status:     github.StatusCompleted,
							Conclusion: github.ConclusionSuccess,
							Steps: []github.Step{
								{Name: "Checkout", Number: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
								{Name: "Build", Number: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
							},
						},
					},
				}
				respJSON, _ := json.Marshal(resp)
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/12345/jobs"}, string(respJSON), "", nil)
			},
			expectError: false,
			wantJobs:    1,
		},
		{
			name:  "multiple jobs",
			runID: 67890,
			setupMock: func(m *exec.MockExecutor) {
				resp := github.JobsResponse{
					Jobs: []github.Job{
						{ID: 1, Name: "lint", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
						{ID: 2, Name: "test", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
						{ID: 3, Name: "build", Status: github.StatusInProgress},
					},
				}
				respJSON, _ := json.Marshal(resp)
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/67890/jobs"}, string(respJSON), "", nil)
			},
			expectError: false,
			wantJobs:    3,
		},
		{
			name:  "no jobs",
			runID: 11111,
			setupMock: func(m *exec.MockExecutor) {
				resp := github.JobsResponse{Jobs: []github.Job{}}
				respJSON, _ := json.Marshal(resp)
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/11111/jobs"}, string(respJSON), "", nil)
			},
			expectError: false,
			wantJobs:    0,
		},
		{
			name:  "API error",
			runID: 99999,
			setupMock: func(m *exec.MockExecutor) {
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs/99999/jobs"},
					"", "HTTP 500: Internal Server Error", fmt.Errorf("exit status 1"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := exec.NewMockExecutor()
			tt.setupMock(mockExec)

			client, err := github.NewClientWithExecutor("owner/repo", mockExec)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			jobs, err := client.GetWorkflowRunJobs(tt.runID)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(jobs) != tt.wantJobs {
				t.Errorf("got %d jobs, want %d", len(jobs), tt.wantJobs)
			}
		})
	}
}

func TestClient_GetLatestRun(t *testing.T) {
	tests := []struct {
		name         string
		workflowName string
		setupMock    func(*exec.MockExecutor)
		expectError  bool
		expectNil    bool
		wantRunID    int64
	}{
		{
			name:         "latest run found",
			workflowName: "ci.yml",
			setupMock: func(m *exec.MockExecutor) {
				resp := github.RunsResponse{
					WorkflowRuns: []github.WorkflowRun{
						{ID: 12345, Name: "CI", Status: github.StatusQueued},
					},
				}
				respJSON, _ := json.Marshal(resp)
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs?per_page=1&workflow=ci.yml"}, string(respJSON), "", nil)
			},
			expectError: false,
			wantRunID:   12345,
		},
		{
			name:         "no workflow filter",
			workflowName: "",
			setupMock: func(m *exec.MockExecutor) {
				resp := github.RunsResponse{
					WorkflowRuns: []github.WorkflowRun{
						{ID: 99999, Name: "Any", Status: github.StatusInProgress},
					},
				}
				respJSON, _ := json.Marshal(resp)
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs?per_page=1"}, string(respJSON), "", nil)
			},
			expectError: false,
			wantRunID:   99999,
		},
		{
			name:         "no runs found",
			workflowName: "nonexistent.yml",
			setupMock: func(m *exec.MockExecutor) {
				resp := github.RunsResponse{WorkflowRuns: []github.WorkflowRun{}}
				respJSON, _ := json.Marshal(resp)
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs?per_page=1&workflow=nonexistent.yml"}, string(respJSON), "", nil)
			},
			expectError: false,
			expectNil:   true,
		},
		{
			name:         "API rate limit",
			workflowName: "ci.yml",
			setupMock: func(m *exec.MockExecutor) {
				m.AddCommand("gh", []string{"api", "repos/owner/repo/actions/runs?per_page=1&workflow=ci.yml"},
					"", "HTTP 403: rate limit exceeded", fmt.Errorf("exit status 1"))
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockExec := exec.NewMockExecutor()
			tt.setupMock(mockExec)

			client, err := github.NewClientWithExecutor("owner/repo", mockExec)
			if err != nil {
				t.Fatalf("failed to create client: %v", err)
			}

			run, err := client.GetLatestRun(tt.workflowName)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if tt.expectNil {
				if run != nil {
					t.Errorf("expected nil run, got %+v", run)
				}
				return
			}

			if run.ID != tt.wantRunID {
				t.Errorf("run.ID = %d, want %d", run.ID, tt.wantRunID)
			}
		})
	}
}

func TestClient_CommandsExecuted(t *testing.T) {
	mockExec := exec.NewMockExecutor()

	resp := github.RunsResponse{
		WorkflowRuns: []github.WorkflowRun{{ID: 1, Name: "CI"}},
	}
	respJSON, _ := json.Marshal(resp)
	mockExec.AddCommand("gh", []string{"api", "repos/test/project/actions/runs?per_page=1&workflow=build.yml"}, string(respJSON), "", nil)

	client, _ := github.NewClientWithExecutor("test/project", mockExec)
	_, _ = client.GetLatestRun("build.yml")

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
