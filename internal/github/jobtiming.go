package github

import (
	"context"
	"fmt"

	ghforge "github.com/kyleking/aragonite/forge/github"
)

// StepTiming is one step of a job, with the timestamps a breakdown needs.
type StepTiming = ghforge.StepTiming

// JobTiming is one job of a run with its creation time and runner labels, which
// aragonite's forge.Job does not carry.
type JobTiming = ghforge.JobTiming

// ListRunJobTimings fetches every job of a run, with step timestamps and the
// labels of the runner each job asked for.
func (c *Client) ListRunJobTimings(runID int64) ([]JobTiming, error) {
	jobs, err := ghforge.RunJobTimings(c.runnerContext(context.Background()), "", c.fullName(), runID)
	if err != nil {
		return nil, fmt.Errorf("listing run jobs: %w", err)
	}

	return jobs, nil
}
