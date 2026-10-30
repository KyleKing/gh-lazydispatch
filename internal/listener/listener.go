// Package listener waits for a condition over state the tool already reads (a
// run finishing, a pull request's checks settling) and hands the outcome to an
// action. It holds the poll loop and the conditions, so the watch command and a
// future TUI arming drive the same code.
package listener

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// Outcome is what a condition found when it settled.
type Outcome struct {
	Summary string
	// Runs are the Actions runs behind a failure, where the condition can name them.
	Runs   []int64
	Failed bool
}

// Condition reports whether it has settled. Until it has, Summary is the
// progress line to show.
type Condition func() (Outcome, bool, error)

// Wait polls cond every interval until it settles or ctx ends. A condition
// that is already settled returns without sleeping.
func Wait(ctx context.Context, cond Condition, interval time.Duration, progress func(string)) (Outcome, error) {
	for {
		outcome, done, err := cond()
		if err != nil {
			return Outcome{}, err
		}

		if done {
			return outcome, nil
		}

		progress(outcome.Summary)

		select {
		case <-ctx.Done():
			return Outcome{}, fmt.Errorf("watch stopped: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
}

// RunGetter fetches one workflow run by ID.
type RunGetter interface {
	GetWorkflowRun(runID int64) (*github.WorkflowRun, error)
}

// CheckGetter lists the checks on a pull request.
type CheckGetter interface {
	GetPRChecks(number int) ([]github.PRCheck, error)
}

// RunDone settles when the run completes. Anything but success is a failure.
func RunDone(client RunGetter, runID int64) Condition {
	return func() (Outcome, bool, error) {
		run, err := client.GetWorkflowRun(runID)
		if err != nil {
			return Outcome{}, false, fmt.Errorf("fetching run %d: %w", runID, err)
		}

		if run.Status != github.StatusCompleted {
			return Outcome{Summary: fmt.Sprintf("run %d: %s...", runID, run.Status)}, false, nil
		}

		return Outcome{
			Summary: fmt.Sprintf("run %d finished: %s (%s)", runID, run.Status, run.Conclusion),
			Failed:  run.Conclusion != github.ConclusionSuccess,
			Runs:    failedRuns(run.Conclusion, runID),
		}, true, nil
	}
}

func failedRuns(conclusion string, runID int64) []int64 {
	if conclusion == github.ConclusionSuccess {
		return nil
	}

	return []int64{runID}
}

// ErrNoChecks is returned by ChecksSettled when a pull request has no checks at
// all, which a watcher cannot tell from checks that have not started yet.
var ErrNoChecks = errors.New("no checks reported yet")

// ChecksSettled settles when no check on the pull request is pending. A pull
// request whose checks have not appeared yet is still pending, not settled.
func ChecksSettled(client CheckGetter, number int) Condition {
	return func() (Outcome, bool, error) {
		checks, err := client.GetPRChecks(number)
		if err != nil {
			return Outcome{}, false, fmt.Errorf("pull request %d: %w", number, err)
		}

		if len(checks) == 0 {
			return Outcome{Summary: fmt.Sprintf("pull request %d: %v", number, ErrNoChecks)}, false, nil
		}

		var pending, failed int

		runs := make(map[int64]bool)

		for i := range checks {
			switch checks[i].Bucket {
			case github.CheckPending:
				pending++
			case github.CheckFail, github.CheckCancel:
				failed++

				if id := checks[i].RunID(); id != 0 {
					runs[id] = true
				}
			}
		}

		if pending > 0 {
			summary := fmt.Sprintf("pull request %d: %d of %d checks pending...", number, pending, len(checks))

			return Outcome{Summary: summary}, false, nil
		}

		return Outcome{
			Summary: fmt.Sprintf("pull request %d checks settled: %d of %d failed", number, failed, len(checks)),
			Failed:  failed > 0,
			Runs:    sortedKeys(runs),
		}, true, nil
	}
}

func sortedKeys(set map[int64]bool) []int64 {
	keys := make([]int64, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return keys
}
