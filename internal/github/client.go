// Package github provides a client for interacting with GitHub API operations.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/aragonite/cache"
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
	jobs     *cache.TTLCache[[]Job]
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
		jobs:     cache.NewTTLCache[[]Job](jobsReuse),
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

// jobsReuse is how long a finished run's jobs are served from memory. Opening a
// log from an open timeline reads the same run's jobs again, and a finished
// run's jobs do not change. A run still in progress is never kept, because the
// log poller reads it again to see the next step finish.
const jobsReuse = 5 * time.Minute

// GetWorkflowRunJobs fetches the jobs for a workflow run, with the per-step
// timings the timeline lays out.
func (c *Client) GetWorkflowRunJobs(runID int64) ([]Job, error) {
	key := strconv.FormatInt(runID, 10)
	if cached, ok := c.jobs.Get(key, cache.NoStamp); ok {
		return cached, nil
	}

	jobs, err := ghforge.RunJobs(c.runnerContext(context.Background()), ".", c.fullName(), runID)
	if err != nil {
		return nil, fmt.Errorf("fetching the run's jobs: %w", err)
	}

	if allJobsCompleted(jobs) {
		c.jobs.Set(key, cache.NoStamp, jobs)
	}

	return jobs, nil
}

// Annotation is one check-run annotation, which is where GitHub records why a
// runner died when it left no step log behind.
type Annotation struct {
	Level   string `json:"annotation_level"`
	Message string `json:"message"`
	Title   string `json:"title"`
	Path    string `json:"path"`
}

// GetJobAnnotations fetches the annotations of a job, which GitHub serves as
// the check run of the same ID.
func (c *Client) GetJobAnnotations(jobID int64) ([]Annotation, error) {
	stdout, stderr, err := c.executor.Execute("gh", "api",
		fmt.Sprintf("repos/%s/check-runs/%d/annotations?per_page=100", c.fullName(), jobID))
	if err != nil {
		return nil, fmt.Errorf("reading the job's annotations: %w (stderr: %s)", err, stderr)
	}

	var annotations []Annotation
	if err := json.Unmarshal([]byte(stdout), &annotations); err != nil {
		return nil, fmt.Errorf("parsing the job's annotations: %w", err)
	}

	return annotations, nil
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

// ListEnvironments names the repository's deployment environments, which is
// what a workflow input of type "environment" has to be one of. A repository
// that declares none answers with an empty list rather than an error.
func (c *Client) ListEnvironments() ([]string, error) {
	stdout, stderr, err := c.executor.Execute("gh", "api", "--paginate",
		"repos/"+c.fullName()+"/environments", "--jq", ".environments[].name")
	if err != nil {
		return nil, fmt.Errorf("reading the repository's environments: %w (stderr: %s)", err, stderr)
	}

	var names []string

	for _, line := range strings.Split(stdout, "\n") {
		if name := strings.TrimSpace(line); name != "" {
			names = append(names, name)
		}
	}

	return names, nil
}

// RerunFailedJobs re-runs only the failed jobs of a run, which is what a run
// that failed on one flaky job needs rather than a whole second run.
func (c *Client) RerunFailedJobs(runID int64) error {
	if _, stderr, err := c.executor.Execute("gh", "run", "rerun",
		strconv.FormatInt(runID, 10), "--failed", "--repo", c.fullName()); err != nil {
		return fmt.Errorf("re-running the failed jobs: %w (stderr: %s)", err, stderr)
	}

	return nil
}

// CancelRun stops a run that is still going.
func (c *Client) CancelRun(runID int64) error {
	if _, stderr, err := c.executor.Execute("gh", "run", "cancel",
		strconv.FormatInt(runID, 10), "--repo", c.fullName()); err != nil {
		return fmt.Errorf("canceling the run: %w (stderr: %s)", err, stderr)
	}

	return nil
}

// CoreBudget reads what is left of the hourly REST allowance for the
// authenticated user. The read is free, so a caller can ask before a burst of calls.
func CoreBudget(ctx context.Context) (ghforge.Allowance, error) {
	budget, err := ghforge.Budgets(ctx, "")
	if err != nil {
		return ghforge.Allowance{}, fmt.Errorf("reading the rate limit: %w", err)
	}

	return budget.Core, nil
}

func allJobsCompleted(jobs []Job) bool {
	for i := range jobs {
		if jobs[i].Status != StatusCompleted {
			return false
		}
	}

	return len(jobs) > 0
}
