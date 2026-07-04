// Package demo provides demo mode functionality for branch selection workflows.
package demo

import (
	"encoding/json"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// Demo run and job IDs used to populate mock GitHub API responses.
const (
	demoRunIDCI      = 1001
	demoRunIDDeploy  = 1002
	demoRunIDRelease = 1003

	demoJobIDCIBuild          = 2001
	demoJobIDCITest           = 2002
	demoJobIDDeployStaging    = 2003
	demoJobIDDeployProduction = 2004

	demoLatestRunIDCI      = 1004
	demoLatestRunIDDeploy  = 1005
	demoLatestRunIDRelease = 1006
)

// MockConfig holds configuration for demo mode.
type MockConfig struct {
	Owner    string
	Repo     string
	Executor *exec.MockExecutor
}

// NewMockConfig creates a default mock configuration.
func NewMockConfig() *MockConfig {
	return &MockConfig{
		Owner:    "demo-org",
		Repo:     "demo-repo",
		Executor: exec.NewMockExecutor(),
	}
}

// SetupMockExecutor configures the mock executor with realistic demo data.
func (c *MockConfig) SetupMockExecutor() {
	c.setupGHCLI()
	c.setupWorkflowRuns()
	c.setupWorkflowDispatch()
}

// Install installs the mock executor globally for runner operations.
func (c *MockConfig) Install() {
	runner.SetExecutor(c.Executor)
}

// Uninstall removes the mock executor.
func (c *MockConfig) Uninstall() {
	runner.SetExecutor(nil)
}

func (c *MockConfig) setupGHCLI() {
	c.Executor.AddGHVersion("2.45.0")
	c.Executor.AddGHAuthStatus(true, "demo-user")
}

func (c *MockConfig) setupWorkflowRuns() {
	// Mock API responses for workflow runs
	c.addWorkflowRun(demoRunIDCI, "CI", github.StatusCompleted, github.ConclusionSuccess)
	c.addWorkflowRun(demoRunIDDeploy, "Deploy", github.StatusInProgress, "")
	c.addWorkflowRun(demoRunIDRelease, "Release", github.StatusQueued, "")

	// Mock jobs for each run
	c.addRunJobs(demoRunIDCI, []github.Job{
		{ID: demoJobIDCIBuild, Name: "build", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
		{ID: demoJobIDCITest, Name: "test", Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
	})

	c.addRunJobs(demoRunIDDeploy, []github.Job{
		{
			ID:         demoJobIDDeployStaging,
			Name:       "deploy-staging",
			Status:     github.StatusCompleted,
			Conclusion: github.ConclusionSuccess,
		},
		{ID: demoJobIDDeployProduction, Name: "deploy-production", Status: github.StatusInProgress, Conclusion: ""},
	})

	// Mock logs for runs
	c.addRunLogs(demoRunIDCI, demoJobIDCIBuild, buildJobLogs())
	c.addRunLogs(demoRunIDCI, demoJobIDCITest, testJobLogs())
}

func (c *MockConfig) setupWorkflowDispatch() {
	// Mock workflow dispatch commands
	c.Executor.AddGHWorkflowRun("ci.yml", "main", nil)
	c.Executor.AddGHWorkflowRun("ci.yml", "develop", nil)
	c.Executor.AddGHWorkflowRun("deploy.yml", "main", map[string]string{"environment": "staging"})
	c.Executor.AddGHWorkflowRun("deploy.yml", "main", map[string]string{"environment": "production"})
	c.Executor.AddGHWorkflowRun("release.yml", "main", map[string]string{"version": "1.0.0"})

	// Mock latest run lookup after dispatch
	c.Executor.AddGHAPILatestRun(c.Owner, c.Repo, "ci.yml", demoLatestRunIDCI, github.StatusQueued)
	c.Executor.AddGHAPILatestRun(c.Owner, c.Repo, "deploy.yml", demoLatestRunIDDeploy, github.StatusQueued)
	c.Executor.AddGHAPILatestRun(c.Owner, c.Repo, "release.yml", demoLatestRunIDRelease, github.StatusQueued)
}

func (c *MockConfig) addWorkflowRun(runID int64, name, status, conclusion string) {
	c.Executor.AddGHAPIRun(c.Owner, c.Repo, runID, status, conclusion)
}

func (c *MockConfig) addRunJobs(runID int64, jobs []github.Job) {
	resp := github.JobsResponse{Jobs: jobs}

	jobsJSON, err := json.Marshal(resp)
	if err != nil {
		panic("demo: failed to marshal jobs: " + err.Error())
	}

	c.Executor.AddGHAPIJobs(c.Owner, c.Repo, runID, string(jobsJSON))
}

func (c *MockConfig) addRunLogs(runID, jobID int64, logs string) {
	c.Executor.AddGHRunView(runID, jobID, logs)
}

// DemoWorkflows returns a set of demo workflows for testing the UI.
func DemoWorkflows() []workflow.WorkflowFile {
	return []workflow.WorkflowFile{
		{
			Name:     "CI",
			Filename: "ci.yml",
			On: workflow.OnTrigger{
				WorkflowDispatch: &workflow.WorkflowDispatch{
					Inputs: map[string]workflow.WorkflowInput{},
				},
			},
		},
		{
			Name:     "Deploy",
			Filename: "deploy.yml",
			On: workflow.OnTrigger{
				WorkflowDispatch: &workflow.WorkflowDispatch{
					Inputs: map[string]workflow.WorkflowInput{
						"environment": {
							Type: "choice", Description: "Target environment", Required: true,
							Options: []string{"staging", "production"}, Default: "staging",
						},
						"dry_run": {
							Type: "boolean", Description: "Perform dry run only", Required: false, Default: "false",
						},
					},
				},
			},
		},
		{
			Name:     "Release",
			Filename: "release.yml",
			On: workflow.OnTrigger{
				WorkflowDispatch: &workflow.WorkflowDispatch{
					Inputs: map[string]workflow.WorkflowInput{
						"version": {
							Type: "string", Description: "Version to release (e.g., 1.0.0)",
							Required: true, Default: "",
						},
						"prerelease": {
							Type: "boolean", Description: "Mark as prerelease", Required: false, Default: "false",
						},
					},
				},
			},
		},
		{
			Name:     "Benchmark",
			Filename: "benchmark.yml",
			On: workflow.OnTrigger{
				WorkflowDispatch: &workflow.WorkflowDispatch{
					Inputs: map[string]workflow.WorkflowInput{
						"iterations": {
							Type: "string", Description: "Number of iterations", Required: false, Default: "100",
						},
						"profile": {
							Type: "choice", Description: "Performance profile", Required: false,
							Options: []string{"quick", "standard", "thorough"}, Default: "standard",
						},
					},
				},
			},
		},
	}
}

func buildJobLogs() string {
	return `##[group]Run actions/checkout@v4
Syncing repository: demo-org/demo-repo
##[endgroup]
##[group]Set up Go 1.21
go version go1.21.6 linux/amd64
##[endgroup]
##[group]Build
go build -v ./...
demo-org/demo-repo/cmd
Build completed successfully
##[endgroup]
`
}

func testJobLogs() string {
	return `##[group]Run actions/checkout@v4
Syncing repository: demo-org/demo-repo
##[endgroup]
##[group]Run tests
go test -v -race ./...
=== RUN   TestMain
--- PASS: TestMain (0.01s)
=== RUN   TestConfig
--- PASS: TestConfig (0.00s)
=== RUN   TestWorkflow
--- PASS: TestWorkflow (0.02s)
PASS
coverage: 85.3% of statements
##[endgroup]
`
}
