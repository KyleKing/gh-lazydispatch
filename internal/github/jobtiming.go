package github

import (
	"encoding/json"
	"fmt"
	"time"
)

// StepTiming is one step of a job, with the timestamps a breakdown needs.
type StepTiming struct {
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at"`
	Name        string    `json:"name"`
	Conclusion  string    `json:"conclusion"`
	Number      int       `json:"number"`
}

// JobTiming is one job of a run. QueueSeconds is StartedAt - CreatedAt, the
// time GitHub took to hand the job to a runner.
//
// It is not aragonite's Job because that type carries neither created_at nor
// the runner labels.
type JobTiming struct {
	CreatedAt   time.Time    `json:"created_at"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt time.Time    `json:"completed_at"`
	Name        string       `json:"name"`
	Conclusion  string       `json:"conclusion"`
	Labels      []string     `json:"labels"`
	Steps       []StepTiming `json:"steps"`
	ID          int64        `json:"id"`
	RunID       int64        `json:"run_id"`
}

// ListRunJobTimings fetches every job of a run, with step timestamps and the
// labels of the runner each job asked for.
func (c *Client) ListRunJobTimings(runID int64) ([]JobTiming, error) {
	var jobs []JobTiming

	for page := 1; ; page++ {
		path := fmt.Sprintf("repos/%s/actions/runs/%d/jobs?per_page=%d&page=%d",
			c.fullName(), runID, maxRunTimingsPerPage, page)

		stdout, stderr, err := c.executor.Execute("gh", "api", path)
		if err != nil {
			return nil, fmt.Errorf("listing jobs of run %d: %w (stderr: %s)", runID, err, stderr)
		}

		var payload struct {
			Jobs []JobTiming `json:"jobs"`
		}

		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			return nil, fmt.Errorf("parsing the jobs of run %d: %w", runID, err)
		}

		for i := range payload.Jobs {
			payload.Jobs[i].RunID = runID
		}

		jobs = append(jobs, payload.Jobs...)

		if len(payload.Jobs) < maxRunTimingsPerPage {
			return jobs, nil
		}
	}
}
