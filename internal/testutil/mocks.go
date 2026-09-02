package testutil

import (
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// MockGitHubClient implements both chain.GitHubClient and watcher.GitHubClient interfaces.
type MockGitHubClient struct {
	Err              error
	ListRunsFunc     func(q github.RunQuery) ([]github.WorkflowRun, error)
	Runs             map[int64]*github.WorkflowRun
	Jobs             map[int64][]github.Job
	LatestByWorkflow map[string]int64
	owner            string
	repo             string
	LatestID         int64
}

// defaultMockLatestID is an arbitrary starting run ID for mock-generated runs.
const defaultMockLatestID = 1000

// NewMockGitHubClient creates a MockGitHubClient with sensible defaults.
func NewMockGitHubClient() *MockGitHubClient {
	return &MockGitHubClient{
		Runs:             make(map[int64]*github.WorkflowRun),
		Jobs:             make(map[int64][]github.Job),
		LatestByWorkflow: make(map[string]int64),
		LatestID:         defaultMockLatestID,
		owner:            "owner",
		repo:             "repo",
	}
}

// WithOwnerRepo sets the owner and repo for the mock client.
func (m *MockGitHubClient) WithOwnerRepo(owner, repo string) *MockGitHubClient {
	m.owner = owner
	m.repo = repo

	return m
}

// WithRun adds a workflow run to the mock.
func (m *MockGitHubClient) WithRun(run *github.WorkflowRun) *MockGitHubClient {
	m.Runs[run.ID] = run
	return m
}

// WithJobs adds jobs for a run to the mock.
func (m *MockGitHubClient) WithJobs(runID int64, jobs []github.Job) *MockGitHubClient {
	m.Jobs[runID] = jobs
	return m
}

// WithError sets the error to return from all methods.
func (m *MockGitHubClient) WithError(err error) *MockGitHubClient {
	m.Err = err
	return m
}

// GetWorkflowRun returns the mocked run for runID, or a queued stub if none is configured.
func (m *MockGitHubClient) GetWorkflowRun(runID int64) (*github.WorkflowRun, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	if run, ok := m.Runs[runID]; ok {
		return run, nil
	}

	return &github.WorkflowRun{ID: runID, Status: github.StatusQueued}, nil
}

// GetWorkflowRunJobs returns the mocked jobs for runID.
func (m *MockGitHubClient) GetWorkflowRunJobs(runID int64) ([]github.Job, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	return m.Jobs[runID], nil
}

// GetLatestRun returns the mocked latest run for workflow.
func (m *MockGitHubClient) GetLatestRun(workflow string) (*github.WorkflowRun, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	// Check if there's a specific run ID for this workflow
	if runID, ok := m.LatestByWorkflow[workflow]; ok {
		return &github.WorkflowRun{ID: runID, Status: github.StatusQueued}, nil
	}
	// Fall back to default LatestID
	return &github.WorkflowRun{ID: m.LatestID, Status: github.StatusQueued}, nil
}

// ListRuns returns ListRunsFunc's result, or an empty listing if unset.
func (m *MockGitHubClient) ListRuns(q github.RunQuery) ([]github.WorkflowRun, error) {
	if m.Err != nil {
		return nil, m.Err
	}

	if m.ListRunsFunc != nil {
		return m.ListRunsFunc(q)
	}

	return nil, nil
}

// Owner returns the mocked repository owner.
func (m *MockGitHubClient) Owner() string { return m.owner }

// Repo returns the mocked repository name.
func (m *MockGitHubClient) Repo() string { return m.repo }

// MockRunWatcher implements chain.RunWatcher interface.
type MockRunWatcher struct {
	Watched map[int64]string
	updates chan watcher.RunUpdate
}

// NewMockRunWatcher creates a new MockRunWatcher.
func NewMockRunWatcher() *MockRunWatcher {
	return &MockRunWatcher{
		Watched: make(map[int64]string),
		updates: make(chan watcher.RunUpdate, 10),
	}
}

// Watch records runID as watched for workflowName.
func (m *MockRunWatcher) Watch(runID int64, workflowName string) {
	m.Watched[runID] = workflowName
}

// Unwatch removes runID from the watched set.
func (m *MockRunWatcher) Unwatch(runID int64) {
	delete(m.Watched, runID)
}

// Updates returns the mocked run update channel.
func (m *MockRunWatcher) Updates() <-chan watcher.RunUpdate {
	return m.updates
}

// SendUpdate sends an update to the mock watcher's channel (for testing).
func (m *MockRunWatcher) SendUpdate(update watcher.RunUpdate) {
	m.updates <- update
}

// Close closes the updates channel.
func (m *MockRunWatcher) Close() {
	close(m.updates)
}
