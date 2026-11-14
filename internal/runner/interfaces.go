package runner

import "github.com/kyleking/gh-lazydispatch/v2/internal/github"

// GitHubClient defines the interface for GitHub API operations needed by the runner.
type GitHubClient interface {
	GetLatestRun(workflowName string) (*github.WorkflowRun, error)
}
