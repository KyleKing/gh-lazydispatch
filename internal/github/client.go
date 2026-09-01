// Package github provides a client for interacting with GitHub API operations.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

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

	converted := fromForge(*run)

	return &converted, nil
}

// GetWorkflowRunJobs fetches the jobs for a workflow run, with the per-step
// timings the timeline lays out.
func (c *Client) GetWorkflowRunJobs(runID int64) ([]Job, error) {
	jobs, err := ghforge.RunJobs(c.runnerContext(context.Background()), ".", c.fullName(), runID)
	if err != nil {
		return nil, fmt.Errorf("fetching the run's jobs: %w", err)
	}

	return fromForgeJobs(jobs), nil
}

// GetLatestRun fetches the most recent workflow run, optionally filtered by workflow name.
func (c *Client) GetLatestRun(workflowName string) (*WorkflowRun, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs?per_page=1", c.owner, c.repo)
	if workflowName != "" {
		path += "&workflow=" + url.QueryEscape(workflowName)
	}

	stdout, stderr, err := c.executor.Execute("gh", "api", path)
	if err != nil {
		return nil, fmt.Errorf("gh api failed: %w (stderr: %s)", err, stderr)
	}

	var runsResp RunsResponse
	if err := json.Unmarshal([]byte(stdout), &runsResp); err != nil {
		return nil, fmt.Errorf("failed to parse runs: %w", err)
	}

	if len(runsResp.WorkflowRuns) == 0 {
		return nil, ErrNoWorkflowRuns
	}

	return &runsResp.WorkflowRuns[0], nil
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

	return fromForgeRuns(runs), nil
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

	return fromForgeRuns(runs), nil
}
