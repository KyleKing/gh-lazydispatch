package github

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"
)

// maxRunTimingsPerPage is the largest per_page the Actions API accepts.
const maxRunTimingsPerPage = 100

// RunTiming is one workflow run reduced to what a per-commit CI timing rollup
// needs: which commit it ran against and when it started and finished.
//
// It is not aragonite's WorkflowRun because that type carries no head_sha,
// which grouping runs by commit depends on.
type RunTiming struct {
	CommittedAt time.Time `json:"committed_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Name        string    `json:"name"`
	Status      string    `json:"status"`
	Conclusion  string    `json:"conclusion"`
	HeadSHA     string    `json:"head_sha"`
	ID          int64     `json:"id"`
}

// IsComplete reports whether the run has reached a conclusion.
func (r RunTiming) IsComplete() bool {
	return r.Status == StatusCompleted
}

// apiRunTiming is the Actions API's run shape, trimmed to the fields a timing
// rollup reads.
type apiRunTiming struct {
	HeadCommit struct {
		Timestamp time.Time `json:"timestamp"`
	} `json:"head_commit"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion"`
	HeadSHA    string    `json:"head_sha"`
	ID         int64     `json:"id"`
}

// maxFilteredResults is how many results the Actions API returns for one
// filtered query, however many pages are requested.
const maxFilteredResults = 1000

// RunTimingSample is the runs created in a time range, plus the slices where
// the API's result cap may have hidden some.
type RunTimingSample struct {
	Runs   []RunTiming
	Capped []time.Time
}

// dayFetchConcurrency bounds the day queries in flight, to stay clear of
// GitHub's secondary rate limit.
const (
	dayFetchConcurrency = 4
	dayLength           = 24 * time.Hour
)

// ListRunTimings fetches runs of event on branch created in [from, to), one
// day at a time because a filtered query returns at most 1000 results. A range
// that hits the cap is split until it fits, and any range that still does is
// listed in Capped. An empty event reads every event.
func (c *Client) ListRunTimings(branch, event string, from, to time.Time) (RunTimingSample, error) {
	var days []time.Time

	for day := from; day.Before(to); day = day.Add(dayLength) {
		days = append(days, day)
	}

	perDay := make([][]RunTiming, len(days))
	cappedPerDay := make([][]time.Time, len(days))

	var group errgroup.Group

	group.SetLimit(dayFetchConcurrency)

	for i, day := range days {
		group.Go(func() error {
			end := day.Add(dayLength)
			if end.After(to) {
				end = to
			}

			runs, capped, err := c.fetchRange(branch, event, day, end)
			perDay[i], cappedPerDay[i] = runs, capped

			return err
		})
	}

	if err := group.Wait(); err != nil {
		return RunTimingSample{}, fmt.Errorf("listing runs by day: %w", err)
	}

	var sample RunTimingSample

	seen := make(map[int64]bool)

	for i, runs := range perDay {
		sample.Capped = append(sample.Capped, cappedPerDay[i]...)

		for j := range runs {
			if !seen[runs[j].ID] {
				seen[runs[j].ID] = true
				sample.Runs = append(sample.Runs, runs[j])
			}
		}
	}

	return sample, nil
}

// minSplitSpan is the narrowest range worth splitting when a query hits the
// result cap; below it the range is reported as capped.
const (
	minSplitSpan = time.Hour
	halves       = 2
)

// fetchRange reads every run created in [from, to), halving the range while a
// query hits the result cap. It returns the runs and the ranges that still hit
// the cap at minSplitSpan.
func (c *Client) fetchRange(branch, event string, from, to time.Time) ([]RunTiming, []time.Time, error) {
	var runs []RunTiming

	for page := 1; ; page++ {
		batch, err := c.fetchRunTimingsPage(branch, event, from, to, page)
		if err != nil {
			return nil, nil, err
		}

		runs = append(runs, batch...)

		if len(batch) < maxRunTimingsPerPage || len(runs) >= maxFilteredResults {
			break
		}
	}

	if len(runs) < maxFilteredResults {
		return runs, nil, nil
	}

	span := to.Sub(from)
	if span <= minSplitSpan {
		return runs, []time.Time{from}, nil
	}

	mid := from.Add(span / halves)

	first, firstCapped, err := c.fetchRange(branch, event, from, mid)
	if err != nil {
		return nil, nil, err
	}

	second, secondCapped, err := c.fetchRange(branch, event, mid, to)
	if err != nil {
		return nil, nil, err
	}

	return append(first, second...), append(firstCapped, secondCapped...), nil
}

func (c *Client) fetchRunTimingsPage(branch, event string, from, to time.Time, page int) ([]RunTiming, error) {
	params := url.Values{}
	params.Set("per_page", strconv.Itoa(maxRunTimingsPerPage))
	params.Set("page", strconv.Itoa(page))

	if branch != "" {
		params.Set("branch", branch)
	}

	if event != "" {
		params.Set("event", event)
	}

	params.Set("created", from.UTC().Format(time.RFC3339)+".."+to.UTC().Format(time.RFC3339))

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
	for i := range payload.WorkflowRuns {
		r := &payload.WorkflowRuns[i]
		timings = append(timings, RunTiming{
			CommittedAt: r.HeadCommit.Timestamp,
			CreatedAt:   r.CreatedAt,
			UpdatedAt:   r.UpdatedAt,
			Name:        r.Name,
			Status:      r.Status,
			Conclusion:  r.Conclusion,
			HeadSHA:     r.HeadSHA,
			ID:          r.ID,
		})
	}

	return timings, nil
}
