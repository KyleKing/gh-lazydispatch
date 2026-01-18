package chain

import (
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// GitHubClient defines the interface for GitHub API operations needed by the chain executor.
type GitHubClient interface {
	GetWorkflowRun(runID int64) (*github.WorkflowRun, error)
	GetWorkflowRunJobs(runID int64) ([]github.Job, error)
	GetLatestRun(workflowName string) (*github.WorkflowRun, error)
	Owner() string
	Repo() string
}

// RunWatcher defines the interface for watching workflow runs.
type RunWatcher interface {
	Watch(runID int64, workflowName string)
	Unwatch(runID int64)
	Updates() <-chan watcher.RunUpdate
}
