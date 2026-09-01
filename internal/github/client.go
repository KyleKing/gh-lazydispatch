// Package github provides a client for interacting with GitHub API operations.
package github

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kyleking/aragonite/forge"
	ghforge "github.com/kyleking/aragonite/forge/github"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
)

// ErrInvalidRepositoryFormat indicates a repository string was not in "owner/repo" format.
var ErrInvalidRepositoryFormat = errors.New("invalid repository format (expected owner/repo)")

// ErrNoWorkflowRuns indicates no workflow runs matched the query.
var ErrNoWorkflowRuns = errors.New("no workflow runs found")

// Client wraps the GitHub API via gh CLI.
type Client struct {
	executor exec.CommandExecutor
	owner    string
	repo     string
}

// NewClient creates a new GitHub API client for the specified repository.
// Uses the real gh CLI executor by default.
func NewClient(repoFullName string) (*Client, error) {
	return NewClientWithExecutor(repoFullName, exec.NewRealExecutor())
}

// repoFullNameParts is the number of segments an "owner/repo" string splits into.
const repoFullNameParts = 2

// NewClientWithExecutor creates a new GitHub API client with a custom executor.
// This allows injecting a mock executor for testing.
func NewClientWithExecutor(repoFullName string, executor exec.CommandExecutor) (*Client, error) {
	parts := strings.SplitN(repoFullName, "/", repoFullNameParts)
	if len(parts) != repoFullNameParts {
		return nil, fmt.Errorf("%w: %s", ErrInvalidRepositoryFormat, repoFullName)
	}

	return &Client{
		executor: executor,
		owner:    parts[0],
		repo:     parts[1],
	}, nil
}

// GetWorkflowRun fetches a single workflow run by ID.
func (c *Client) GetWorkflowRun(runID int64) (*WorkflowRun, error) {
	run, err := ghforge.GetRun(c.runnerContext(context.Background()), ".", c.fullName(), runID)
	if err != nil {
		return nil, fmt.Errorf("fetching the run: %w", err)
	}

	return run, nil
}

// GetWorkflowRunJobs fetches the jobs for a workflow run, with the per-step
// timings the timeline lays out.
func (c *Client) GetWorkflowRunJobs(runID int64) ([]Job, error) {
	jobs, err := ghforge.RunJobs(c.runnerContext(context.Background()), ".", c.fullName(), runID)
	if err != nil {
		return nil, fmt.Errorf("fetching the run's jobs: %w", err)
	}

	return jobs, nil
}

// GetLatestRun fetches the most recent run of a workflow file, or the most
// recent run in the repository when workflowFile is empty.
func (c *Client) GetLatestRun(workflowFile string) (*WorkflowRun, error) {
	runs, err := ghforge.ListRuns(c.runnerContext(context.Background()), ".", c.fullName(), ghforge.RunQuery{
		Workflow: workflowFile,
		Limit:    1,
	})
	if err != nil {
		return nil, fmt.Errorf("reading the latest run: %w", err)
	}

	if len(runs) == 0 {
		return nil, ErrNoWorkflowRuns
	}

	return &runs[0], nil
}

// Owner returns the repository owner.
func (c *Client) Owner() string {
	return c.owner
}

// Repo returns the repository name.
func (c *Client) Repo() string {
	return c.repo
}

// RunQuery narrows a run listing. A zero value lists the most recent runs
// across every workflow.
type RunQuery struct {
	// Workflow is a workflow filename ("ci.yml"), not its display name.
	Workflow string
	Branch   string
	Status   string
	Event    string
	Limit    int
}

// ListRuns fetches recent workflow runs matching q, newest first.
func (c *Client) ListRuns(q RunQuery) ([]WorkflowRun, error) {
	runs, err := ghforge.ListRuns(c.runnerContext(context.Background()), ".", c.fullName(), ghforge.RunQuery{
		Workflow: q.Workflow,
		Branch:   q.Branch,
		Status:   q.Status,
		Event:    q.Event,
		Limit:    q.Limit,
	})
	if err != nil {
		return nil, fmt.Errorf("listing runs: %w", err)
	}

	return runs, nil
}

// LatestRunsOnBranch returns the newest run of each workflow on a branch, which
// is the branch's current state rather than its history. A run older than within
// is dropped unless it is still going; a zero within keeps every age.
func (c *Client) LatestRunsOnBranch(branch string, within time.Duration) ([]WorkflowRun, error) {
	runs, err := ghforge.LatestRunsOnBranch(
		c.runnerContext(context.Background()), ".", c.fullName(), branch, within,
	)
	if err != nil {
		return nil, fmt.Errorf("listing the branch's latest runs: %w", err)
	}

	return runs, nil
}

// PRScope names which pull requests a run listing should cover. Both are search
// queries rather than API filters, because "mine" and "awaiting my review" are
// questions only the search index answers.
type PRScope string

// Pull request scopes worth a saved view.
const (
	PRScopeMine      PRScope = "is:open author:@me"
	PRScopeReviewing PRScope = "is:open review-requested:@me"
)

// PullRequest is aragonite's pull request model, which already carries the
// check rollup that answers whether a pull request is green.
type PullRequest = forge.PullRequest

// PullRequestsInScope returns every pull request matching scope, each with its
// own check rollup.
//
// The rollup is the answer rather than a run listing because runs are keyed by
// branch: one page of a repository's recent runs is filled by whichever branch
// ran last, so every other pull request in it reports nothing.
func (c *Client) PullRequestsInScope(scope PRScope) ([]PullRequest, error) {
	ctx := c.runnerContext(context.Background())

	prs, err := ghforge.SearchPRsInRepo(ctx, ".", c.fullName(), string(scope))
	if err != nil {
		return nil, fmt.Errorf("searching pull requests: %w", err)
	}

	return prs, nil
}
