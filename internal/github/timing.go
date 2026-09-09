package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// maxRunTimingsPerPage is the largest per_page the Actions API accepts.
const maxRunTimingsPerPage = 100

// RunTiming is one workflow run reduced to what a per-commit CI timing rollup
// needs: which commit it ran against and when it started and finished.
//
// It is not aragonite's WorkflowRun because that type carries no head_sha,
// which grouping runs by commit depends on.
type RunTiming struct {
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Name       string
	Status     string
	Conclusion string
	HeadSHA    string
	ID         int64
}

// IsComplete reports whether the run has reached a conclusion.
func (r RunTiming) IsComplete() bool {
	return r.Status == StatusCompleted
}

// apiRunTiming is the Actions API's run shape, trimmed to the fields a timing
// rollup reads.
type apiRunTiming struct {
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	HeadSHA    string    `json:"head_sha"`
	ID         int64     `json:"id"`
}

// ListRunTimings fetches recent runs of event on branch, newest first, up to
// limit, paging through the Actions API as needed. A non-positive limit reads
// one page.
func (c *Client) ListRunTimings(branch, event string, limit int) ([]RunTiming, error) {
	if limit <= 0 {
		limit = maxRunTimingsPerPage
	}

	var timings []RunTiming

	for page := 1; len(timings) < limit; page++ {
		perPage := maxRunTimingsPerPage
		if remaining := limit - len(timings); remaining < perPage {
			perPage = remaining
		}

		batch, err := c.fetchRunTimingsPage(branch, event, page, perPage)
		if err != nil {
			return nil, err
		}

		timings = append(timings, batch...)

		if len(batch) < perPage {
			break
		}
	}

	return timings, nil
}

func (c *Client) fetchRunTimingsPage(branch, event string, page, perPage int) ([]RunTiming, error) {
	params := url.Values{}
	params.Set("per_page", strconv.Itoa(perPage))
	params.Set("page", strconv.Itoa(page))

	if branch != "" {
		params.Set("branch", branch)
	}

	if event != "" {
		params.Set("event", event)
	}

	path := "repos/" + c.fullName() + "/actions/runs?" + params.Encode()

	stdout, stderr, err := c.executor.Execute("gh", "api", path)
	if err != nil {
		return nil, fmt.Errorf("listing runs for %s: %w (stderr: %s)", c.fullName(), err, stderr)
	}

	var payload struct {
		WorkflowRuns []apiRunTiming `json:"workflow_runs"`
	}

	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		return nil, fmt.Errorf("parsing the run list: %w", err)
	}

	timings := make([]RunTiming, 0, len(payload.WorkflowRuns))
	for _, r := range payload.WorkflowRuns {
		timings = append(timings, RunTiming(r))
	}

	return timings, nil
}
