package chain_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/testutil"
)

// chainOf builds a three-step chain whose middle step fails, so what onFailure
// does with it is the only thing the case varies.
func chainOf(onFailure config.FailureAction) *config.Chain {
	steps := make([]config.ChainStep, 0, 3)
	for _, name := range []string{"first.yml", "second.yml", "third.yml"} {
		steps = append(steps, config.ChainStep{
			Workflow: name, WaitFor: config.WaitSuccess, OnFailure: onFailure,
		})
	}

	return &config.Chain{Steps: steps}
}

// countingDispatcher hands out run IDs 1, 2, 3 in dispatch order, so a run's ID
// says which step started it.
func countingDispatcher(dispatched *atomic.Int64) chain.Dispatcher {
	return func(_ runner.RunConfig, _ chain.GitHubClient) (int64, error) {
		return dispatched.Add(1), nil
	}
}

func completedRun(id int64, conclusion string) *github.WorkflowRun {
	return &github.WorkflowRun{ID: id, Status: github.StatusCompleted, Conclusion: conclusion}
}

// The chain engine's whole job is deciding what a failed step means for the
// steps after it, and it could not be tested at all while the dispatch was
// hard-wired to `gh workflow run`.
func TestChainExecution_AFailedStepDecidesWhatFollows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		onFailure      config.FailureAction
		wantStatus     chain.ChainStatus
		wantDispatched int64
		wantStatuses   []chain.StepStatus
	}{
		{
			name:           "abort stops the chain at the failure",
			onFailure:      config.FailureAbort,
			wantStatus:     chain.ChainFailed,
			wantDispatched: 2,
			wantStatuses:   []chain.StepStatus{chain.StepCompleted, chain.StepFailed, chain.StepPending},
		},
		{
			name:           "continue runs the rest and still completes",
			onFailure:      config.FailureContinue,
			wantStatus:     chain.ChainCompleted,
			wantDispatched: 3,
			wantStatuses:   []chain.StepStatus{chain.StepCompleted, chain.StepFailed, chain.StepCompleted},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := testutil.NewMockGitHubClient().
				WithRun(completedRun(1, github.ConclusionSuccess)).
				WithRun(completedRun(2, github.ConclusionFailure)).
				WithRun(completedRun(3, github.ConclusionSuccess))

			w := testutil.NewMockRunWatcher()
			defer w.Close()

			var dispatched atomic.Int64

			executor := chain.NewExecutor(client, w, "release", chainOf(tt.onFailure),
				chain.WithDispatcher(countingDispatcher(&dispatched)),
				chain.WithPollInterval(time.Millisecond))

			if err := executor.Start(nil, "main"); err != nil {
				t.Fatal(err)
			}

			state := finalState(t, executor)

			if state.Status != tt.wantStatus {
				t.Errorf("the chain ended %q, want %q", state.Status, tt.wantStatus)
			}

			if got := dispatched.Load(); got != tt.wantDispatched {
				t.Errorf("%d steps dispatched, want %d", got, tt.wantDispatched)
			}

			for i, want := range tt.wantStatuses {
				if state.StepStatuses[i] != want {
					t.Errorf("step %d is %q, want %q", i, state.StepStatuses[i], want)
				}
			}
		})
	}
}

// Stopping mid-chain must not leave the goroutine dispatching the rest.
func TestChainExecution_StopHaltsBeforeTheNextStep(t *testing.T) {
	t.Parallel()

	client := testutil.NewMockGitHubClient()
	w := testutil.NewMockRunWatcher()

	defer w.Close()

	var dispatched atomic.Int64

	// Every run stays queued, so the first step waits until the stop reaches it.
	executor := chain.NewExecutor(client, w, "release", chainOf(config.FailureAbort),
		chain.WithDispatcher(countingDispatcher(&dispatched)),
		chain.WithPollInterval(time.Millisecond))

	if err := executor.Start(nil, "main"); err != nil {
		t.Fatal(err)
	}

	executor.Stop()
	finalState(t, executor)

	if got := dispatched.Load(); got > 1 {
		t.Errorf("%d steps dispatched after the stop, want at most the one already running", got)
	}
}

// finalState drains the update channel and returns the state the chain settled
// on. The executor closes the channel when it is done, which is the signal.
func finalState(t *testing.T, executor *chain.ChainExecutor) chain.ChainState {
	t.Helper()

	deadline := time.After(5 * time.Second)

	for {
		select {
		case _, ok := <-executor.Updates():
			if !ok {
				return executor.State()
			}
		case <-deadline:
			t.Fatal("the chain never finished")
		}
	}
}
