// Package github provides a client for interacting with GitHub API operations.
package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
)

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

// NewClientWithExecutor creates a new GitHub API client with a custom executor.
// This allows injecting a mock executor for testing.
func NewClientWithExecutor(repoFullName string, executor exec.CommandExecutor) (*Client, error) {
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s (expected owner/repo)", repoFullName)
	}

	return &Client{
		executor: executor,
		owner:    parts[0],
		repo:     parts[1],
	}, nil
}

// GetWorkflowRun fetches a single workflow run by ID.
func (c *Client) GetWorkflowRun(runID int64) (*WorkflowRun, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs/%d", c.owner, c.repo, runID)

	stdout, stderr, err := c.executor.Execute("gh", "api", path)
	if err != nil {
		return nil, fmt.Errorf("gh api failed: %w (stderr: %s)", err, stderr)
	}

	var run WorkflowRun
	if err := json.Unmarshal([]byte(stdout), &run); err != nil {
		return nil, fmt.Errorf("failed to parse workflow run: %w", err)
	}

	return &run, nil
}

// GetWorkflowRunJobs fetches the jobs for a workflow run.
func (c *Client) GetWorkflowRunJobs(runID int64) ([]Job, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs/%d/jobs", c.owner, c.repo, runID)

	stdout, stderr, err := c.executor.Execute("gh", "api", path)
	if err != nil {
		return nil, fmt.Errorf("gh api failed: %w (stderr: %s)", err, stderr)
	}

	var jobsResp JobsResponse
	if err := json.Unmarshal([]byte(stdout), &jobsResp); err != nil {
		return nil, fmt.Errorf("failed to parse jobs: %w", err)
	}

	return jobsResp.Jobs, nil
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
		return nil, nil
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
