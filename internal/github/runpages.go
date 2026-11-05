package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// runsPageSize is the largest per_page the Actions API accepts.
const runsPageSize = 100

// PagedRun is a run from ListRunsPaged, with the commit it ran on and how many
// times it was attempted.
type PagedRun struct {
	HeadSHA string
	WorkflowRun
	RunAttempt int
}

// ListRunsPaged fetches up to q.Limit runs matching q, newest first, reading as
// many pages as it takes. ListRuns stops at one page of 100.
func (c *Client) ListRunsPaged(q RunQuery) ([]PagedRun, error) {
	var runs []PagedRun

	for page := 1; q.Limit <= 0 || len(runs) < q.Limit; page++ {
		batch, err := c.runsPage(q, page)
		if err != nil {
			return nil, err
		}

		runs = append(runs, batch...)

		if len(batch) < runsPageSize {
			break
		}
	}

	if q.Limit > 0 && len(runs) > q.Limit {
		runs = runs[:q.Limit]
	}

	return runs, nil
}

func (c *Client) runsPage(q RunQuery, page int) ([]PagedRun, error) {
	params := url.Values{}
	params.Set("per_page", strconv.Itoa(runsPageSize))
	params.Set("page", strconv.Itoa(page))

	for key, value := range map[string]string{"branch": q.Branch, "event": q.Event, "status": q.Status} {
		if value != "" {
			params.Set(key, value)
		}
	}

	base := fmt.Sprintf("repos/%s/actions/runs", c.fullName())
	if q.Workflow != "" {
		base = fmt.Sprintf("repos/%s/actions/workflows/%s/runs", c.fullName(), url.PathEscape(q.Workflow))
	}

	stdout, stderr, err := c.executor.Execute("gh", "api", base+"?"+params.Encode())
	if err != nil {
		return nil, fmt.Errorf("listing runs, page %d: %w (stderr: %s)", page, err, stderr)
	}

	var payload struct {
		WorkflowRuns []struct {
			CreatedAt  time.Time `json:"created_at"`
			UpdatedAt  time.Time `json:"updated_at"`
			Name       string    `json:"name"`
			Status     string    `json:"status"`
			Conclusion string    `json:"conclusion"`
			HeadBranch string    `json:"head_branch"`
			Event      string    `json:"event"`
			HeadSHA    string    `json:"head_sha"`
			ID         int64     `json:"id"`
			RunAttempt int       `json:"run_attempt"`
		} `json:"workflow_runs"`
	}

	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return nil, fmt.Errorf("parsing run list page %d: %w", page, err)
	}

	runs := make([]PagedRun, 0, len(payload.WorkflowRuns))
	for i := range payload.WorkflowRuns {
		r := &payload.WorkflowRuns[i]
		runs = append(runs, PagedRun{
			HeadSHA: r.HeadSHA, RunAttempt: r.RunAttempt,
			WorkflowRun: WorkflowRun{
				ID: r.ID, Name: r.Name, Status: r.Status, Conclusion: r.Conclusion,
				HeadBranch: r.HeadBranch, Event: r.Event, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			},
		})
	}

	return runs, nil
}
