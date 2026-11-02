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

// ListRunsPaged fetches up to q.Limit runs matching q, newest first, reading as
// many pages as it takes. ListRuns stops at one page of 100.
func (c *Client) ListRunsPaged(q RunQuery) ([]WorkflowRun, error) {
	var runs []WorkflowRun

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

func (c *Client) runsPage(q RunQuery, page int) ([]WorkflowRun, error) {
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
			ID         int64     `json:"id"`
		} `json:"workflow_runs"`
	}

	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return nil, fmt.Errorf("parsing run list page %d: %w", page, err)
	}

	runs := make([]WorkflowRun, 0, len(payload.WorkflowRuns))
	for i := range payload.WorkflowRuns {
		r := &payload.WorkflowRuns[i]
		runs = append(runs, WorkflowRun{
			ID: r.ID, Name: r.Name, Status: r.Status, Conclusion: r.Conclusion,
			HeadBranch: r.HeadBranch, Event: r.Event, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
		})
	}

	return runs, nil
}
