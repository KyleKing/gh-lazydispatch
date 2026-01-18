package github

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
)

// Client wraps the GitHub REST API client.
type Client struct {
	rest  *api.RESTClient
	owner string
	repo  string
}

// NewClient creates a new GitHub API client for the specified repository.
func NewClient(repoFullName string) (*Client, error) {
	parts := strings.SplitN(repoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository format: %s (expected owner/repo)", repoFullName)
	}

	rest, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w", err)
	}

	return &Client{
		rest:  rest,
		owner: parts[0],
		repo:  parts[1],
	}, nil
}

// GetWorkflowRun fetches a single workflow run by ID.
func (c *Client) GetWorkflowRun(runID int64) (*WorkflowRun, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs/%d", c.owner, c.repo, runID)

	resp, err := c.rest.Request("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow run: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var run WorkflowRun
	if err := json.Unmarshal(body, &run); err != nil {
		return nil, fmt.Errorf("failed to parse workflow run: %w", err)
	}

	return &run, nil
}

// GetWorkflowRunJobs fetches the jobs for a workflow run.
func (c *Client) GetWorkflowRunJobs(runID int64) ([]Job, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs/%d/jobs", c.owner, c.repo, runID)

	resp, err := c.rest.Request("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow jobs: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var jobsResp JobsResponse
	if err := json.Unmarshal(body, &jobsResp); err != nil {
		return nil, fmt.Errorf("failed to parse jobs: %w", err)
	}

	return jobsResp.Jobs, nil
}

// GetLatestRun fetches the most recent workflow run, optionally filtered by workflow name.
func (c *Client) GetLatestRun(workflowName string) (*WorkflowRun, error) {
	path := fmt.Sprintf("repos/%s/%s/actions/runs?per_page=1", c.owner, c.repo)
	if workflowName != "" {
		path += "&workflow=" + workflowName
	}

	resp, err := c.rest.Request("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get workflow runs: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var runsResp RunsResponse
	if err := json.Unmarshal(body, &runsResp); err != nil {
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
