package github

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"time"
)

// runsPerPageCap is the most runs the Actions API returns for one query.
const runsPerPageCap = 1000

var errRunCap = errors.New("a day of runs hit the 1000-result cap")

// ListRunAttemptJobs fetches the jobs of one attempt of a run. The plain jobs
// endpoint only serves the latest attempt.
func (c *Client) ListRunAttemptJobs(runID int64, attempt int) ([]JobTiming, error) {
	var jobs []JobTiming

	for page := 1; ; page++ {
		path := fmt.Sprintf("repos/%s/actions/runs/%d/attempts/%d/jobs?per_page=%d&page=%d",
			c.fullName(), runID, attempt, runsPageSize, page)

		stdout, stderr, err := c.executor.Execute("gh", "api", path)
		if err != nil {
			return nil, fmt.Errorf("listing jobs of run %d attempt %d: %w (stderr: %s)", runID, attempt, err, stderr)
		}

		var payload struct {
			Jobs []JobTiming `json:"jobs"`
		}

		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			return nil, fmt.Errorf("parsing the jobs of run %d attempt %d: %w", runID, attempt, err)
		}

		jobs = append(jobs, payload.Jobs...)

		if len(payload.Jobs) < runsPageSize {
			return jobs, nil
		}
	}
}

// WorkflowRunInfo is one run of a single workflow, with the event that started
// it and the commit time of its head.
type WorkflowRunInfo struct {
	CreatedAt    time.Time
	RunStartedAt time.Time
	UpdatedAt    time.Time
	CommittedAt  time.Time
	Event        string
	HeadSHA      string
	Conclusion   string
	Status       string
	ID           int64
	RunAttempt   int
}

// ListWorkflowRuns fetches the runs of one workflow file created in [from, to),
// a day at a time because one query returns at most 1000 results. A day that
// still hits the cap is an error, since the count would be wrong.
func (c *Client) ListWorkflowRuns(workflowFile string, from, to time.Time) ([]WorkflowRunInfo, error) {
	const day = 24 * time.Hour

	var runs []WorkflowRunInfo

	for start := from; start.Before(to); start = start.Add(day) {
		end := start.Add(day)
		if end.After(to) {
			end = to
		}

		batch, err := c.workflowRunsBetween(workflowFile, start, end)
		if err != nil {
			return nil, err
		}

		runs = append(runs, batch...)
	}

	return runs, nil
}

func (c *Client) workflowRunsBetween(workflowFile string, from, to time.Time) ([]WorkflowRunInfo, error) {
	var runs []WorkflowRunInfo

	created := from.UTC().Format(time.RFC3339) + ".." + to.UTC().Format(time.RFC3339)

	for page := 1; ; page++ {
		path := fmt.Sprintf("repos/%s/actions/workflows/%s/runs?created=%s&per_page=%d&page=%d",
			c.fullName(), url.PathEscape(workflowFile), url.QueryEscape(created), runsPageSize, page)

		stdout, stderr, err := c.executor.Execute("gh", "api", path)
		if err != nil {
			return nil, fmt.Errorf("listing runs of %s: %w (stderr: %s)", workflowFile, err, stderr)
		}

		var payload struct {
			WorkflowRuns []struct {
				CreatedAt    time.Time `json:"created_at"`
				RunStartedAt time.Time `json:"run_started_at"`
				UpdatedAt    time.Time `json:"updated_at"`
				HeadCommit   struct {
					Timestamp time.Time `json:"timestamp"`
				} `json:"head_commit"`
				Event      string `json:"event"`
				HeadSHA    string `json:"head_sha"`
				Conclusion string `json:"conclusion"`
				Status     string `json:"status"`
				ID         int64  `json:"id"`
				RunAttempt int    `json:"run_attempt"`
			} `json:"workflow_runs"`
		}

		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			return nil, fmt.Errorf("parsing the runs of %s: %w", workflowFile, err)
		}

		for i := range payload.WorkflowRuns {
			r := &payload.WorkflowRuns[i]
			runs = append(runs, WorkflowRunInfo{
				CreatedAt: r.CreatedAt, RunStartedAt: r.RunStartedAt, UpdatedAt: r.UpdatedAt,
				CommittedAt: r.HeadCommit.Timestamp, Event: r.Event, HeadSHA: r.HeadSHA,
				Conclusion: r.Conclusion, Status: r.Status, ID: r.ID, RunAttempt: r.RunAttempt,
			})
		}

		if len(payload.WorkflowRuns) < runsPageSize {
			return runs, nil
		}

		if len(runs) >= runsPerPageCap {
			return nil, fmt.Errorf("%w: %s from %s", errRunCap, workflowFile, from.Format(time.RFC3339))
		}
	}
}
