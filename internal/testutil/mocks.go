package testutil

import (
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// MockGitHubClient implements both chain.GitHubClient and watcher.GitHubClient interfaces.
type MockGitHubClient struct {
	Runs     map[int64]*github.WorkflowRun
	Jobs     map[int64][]github.Job
	LatestID int64
	Err      error
	owner    string
	repo     string
}

// NewMockGitHubClient creates a MockGitHubClient with sensible defaults.
func NewMockGitHubClient() *MockGitHubClient {
	return &MockGitHubClient{
		Runs:     make(map[int64]*github.WorkflowRun),
		Jobs:     make(map[int64][]github.Job),
		LatestID: 1000,
		owner:    "owner",
		repo:     "repo",
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

func (m *MockGitHubClient) GetWorkflowRun(runID int64) (*github.WorkflowRun, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	if run, ok := m.Runs[runID]; ok {
		return run, nil
	}
	return &github.WorkflowRun{ID: runID, Status: github.StatusQueued}, nil
}

func (m *MockGitHubClient) GetWorkflowRunJobs(runID int64) ([]github.Job, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return m.Jobs[runID], nil
}

func (m *MockGitHubClient) GetLatestRun(_ string) (*github.WorkflowRun, error) {
	if m.Err != nil {
		return nil, m.Err
	}
	return &github.WorkflowRun{ID: m.LatestID, Status: github.StatusQueued}, nil
}

func (m *MockGitHubClient) Owner() string { return m.owner }
func (m *MockGitHubClient) Repo() string  { return m.repo }

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

func (m *MockRunWatcher) Watch(runID int64, workflowName string) {
	m.Watched[runID] = workflowName
}

func (m *MockRunWatcher) Unwatch(runID int64) {
	delete(m.Watched, runID)
}

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
