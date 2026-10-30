package listener_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/listener"
)

// runQueue answers GetWorkflowRun from a queue of runs, one per call, so a poll
// loop's retries are exercised without a live repository or a real sleep.
type runQueue struct {
	runs  []*github.WorkflowRun
	calls int
}

func (q *runQueue) GetWorkflowRun(int64) (*github.WorkflowRun, error) {
	run := q.runs[min(q.calls, len(q.runs)-1)]
	q.calls++

	return run, nil
}

type checkQueue struct {
	polls [][]github.PRCheck
	calls int
}

func (q *checkQueue) GetPRChecks(int) ([]github.PRCheck, error) {
	checks := q.polls[min(q.calls, len(q.polls)-1)]
	q.calls++

	return checks, nil
}

// A run that is already done settles on the first check without sleeping, and a
// running one is polled until it finishes, reporting progress each time.
func TestWait_ReturnsAtOnceWhenSettledAndPollsUntilARunFinishes(t *testing.T) {
	t.Parallel()

	finished := &github.WorkflowRun{ID: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess}
	done := &runQueue{runs: []*github.WorkflowRun{finished}}

	outcome, err := listener.Wait(t.Context(), listener.RunDone(done, 1), time.Hour, func(string) {})
	if err != nil || outcome.Failed || done.calls != 1 {
		t.Fatalf("done run: outcome %+v, err %v, %d calls; want a success after one call", outcome, err, done.calls)
	}

	running := &runQueue{runs: []*github.WorkflowRun{
		{ID: 2, Status: github.StatusInProgress},
		{ID: 2, Status: github.StatusInProgress},
		{ID: 2, Status: github.StatusCompleted, Conclusion: github.ConclusionFailure},
	}}

	var progress []string

	outcome, err = listener.Wait(t.Context(), listener.RunDone(running, 2), time.Millisecond,
		func(line string) { progress = append(progress, line) })
	if err != nil || !outcome.Failed || len(outcome.Runs) != 1 || running.calls != 3 || len(progress) != 2 {
		t.Fatalf("running run: outcome %+v, err %v, %d calls, %d progress", outcome, err, running.calls, len(progress))
	}
}

// Checks that have not appeared, or are pending, keep the wait open. Once all
// have settled the failing ones name their runs, and a check from another app
// (no run link) names none.
func TestChecksSettled_WaitsForEveryCheckAndNamesTheFailingRuns(t *testing.T) {
	t.Parallel()

	queue := &checkQueue{polls: [][]github.PRCheck{
		nil,
		{{Name: "a", Bucket: github.CheckPass}, {Name: "b", Bucket: github.CheckPending}},
		{
			{Name: "a", Bucket: github.CheckPass},
			{Name: "b", Bucket: github.CheckFail, Link: "https://github.com/o/r/actions/runs/77/job/9"},
			{Name: "c", Bucket: github.CheckFail, Link: "https://example.com/other"},
		},
	}}

	outcome, err := listener.Wait(t.Context(), listener.ChecksSettled(queue, 5), time.Millisecond, func(string) {})
	if err != nil {
		t.Fatal(err)
	}

	if queue.calls != 3 || !outcome.Failed || len(outcome.Runs) != 1 || outcome.Runs[0] != 77 ||
		!strings.Contains(outcome.Summary, "2 of 3 failed") {
		t.Errorf("outcome %+v after %d calls, want a failure naming run 77 after 3 calls", outcome, queue.calls)
	}
}

func TestWait_StopsWhenTheContextEnds(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	running := &runQueue{runs: []*github.WorkflowRun{{ID: 1, Status: github.StatusInProgress}}}
	if _, err := listener.Wait(ctx, listener.RunDone(running, 1), time.Hour, func(string) {}); err == nil {
		t.Error("a canceled wait returned no error")
	}
}

func TestNotify_RingsTheBellAndRunsTheCommandWithTheSummary(t *testing.T) {
	t.Parallel()

	var out bytes.Buffer
	if err := listener.Notify(t.Context(), &out, "true", listener.Outcome{Summary: "done"}); err != nil {
		t.Fatal(err)
	}

	if out.String() != "\a" {
		t.Errorf("wrote %q, want the terminal bell", out.String())
	}

	if err := listener.Notify(t.Context(), &out, "/nonexistent-notifier", listener.Outcome{}); err == nil {
		t.Error("a notifier that cannot run was not reported")
	}
}
