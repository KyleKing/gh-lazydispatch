// Package watcher provides workflow run monitoring and status tracking functionality.
package watcher

import "github.com/kyleking/gh-lazydispatch/internal/github"

// GitHubClient defines the interface for GitHub API operations needed by the watcher.
type GitHubClient interface {
	GetWorkflowRun(runID int64) (*github.WorkflowRun, error)
	GetWorkflowRunJobs(runID int64) ([]github.Job, error)
}
