package github

import (
	"context"
	"fmt"
	"time"

	ghforge "github.com/kyleking/aragonite/forge/github"
)

// RunTiming is aragonite's run timing row, which carries the head SHA, actor,
// attempt, and pull requests a per-commit rollup groups by.
type RunTiming = ghforge.RunTiming

// RunFilter narrows a run listing. An empty field matches everything.
type RunFilter = ghforge.RunTimingFilter

// RunTimingSample is the runs created in a time range, plus the slices where
// the API's result cap may have hidden some.
type RunTimingSample = ghforge.RunTimingSample

// IsComplete reports whether the run has reached a conclusion.
func IsComplete(run *RunTiming) bool {
	return run.Status == StatusCompleted
}

// ListRunTimings fetches runs matching filter created in [from, to), one day
// at a time because a filtered query returns at most 1000 results.
func (c *Client) ListRunTimings(filter RunFilter, from, to time.Time) (RunTimingSample, error) {
	sample, err := ghforge.ListRunTimings(c.runnerContext(context.Background()), "", c.fullName(), filter, from, to)
	if err != nil {
		return RunTimingSample{}, fmt.Errorf("listing run timings: %w", err)
	}

	return sample, nil
}
