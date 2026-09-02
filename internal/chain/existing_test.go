package chain_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/testutil"
)

// TestChainExecution_SourceExisting_AdoptsWithoutDispatching pins the case
// docs/design-listeners.md calls the narrow fix: a step whose source is
// "existing" attaches to a run already going on the branch instead of
// starting a fresh one.
func TestChainExecution_SourceExisting_AdoptsWithoutDispatching(t *testing.T) {
	t.Parallel()

	adopted := completedRun(42, github.ConclusionSuccess)
	adopted.URL = "https://github.com/kyleking/gh-lazydispatch/actions/runs/42"

	client := testutil.NewMockGitHubClient().WithRun(adopted)
	client.ListRunsFunc = func(q github.RunQuery) ([]github.WorkflowRun, error) {
		if q.Workflow == "build.yml" && q.Status == github.StatusInProgress {
			return []github.WorkflowRun{*adopted}, nil
		}

		return nil, nil
	}

	w := testutil.NewMockRunWatcher()
	defer w.Close()

	var dispatched atomic.Int64

	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "build.yml", Source: config.SourceExisting, WaitFor: config.WaitSuccess},
		},
	}

	executor := chain.NewExecutor(client, w, "adopt", chainDef,
		chain.WithDispatcher(countingDispatcher(&dispatched)),
		chain.WithPollInterval(time.Millisecond))

	if err := executor.Start(nil, "main"); err != nil {
		t.Fatal(err)
	}

	state := finalState(t, executor)

	if got := dispatched.Load(); got != 0 {
		t.Errorf("dispatched %d workflows, want 0: an existing step must never dispatch", got)
	}

	if state.Status != chain.ChainCompleted {
		t.Errorf("chain ended %q, want %q", state.Status, chain.ChainCompleted)
	}

	result := state.StepResults[0]
	if result == nil || result.RunID != adopted.ID {
		t.Errorf("step adopted run %v, want run %d", result, adopted.ID)
	}

	if _, watched := w.Watched[adopted.ID]; !watched {
		t.Errorf("adopted run %d was not watched", adopted.ID)
	}
}

// TestChainExecution_SourceExisting_FailsLoudWithNothingToAdopt pins that a
// step with no run to adopt fails rather than silently dispatching one: a
// step that sometimes starts a production build and sometimes adopts one is
// a step nobody can read.
func TestChainExecution_SourceExisting_FailsLoudWithNothingToAdopt(t *testing.T) {
	t.Parallel()

	client := testutil.NewMockGitHubClient()

	w := testutil.NewMockRunWatcher()
	defer w.Close()

	var dispatched atomic.Int64

	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "build.yml", Source: config.SourceExisting, OnFailure: config.FailureAbort},
		},
	}

	executor := chain.NewExecutor(client, w, "adopt-nothing", chainDef,
		chain.WithDispatcher(countingDispatcher(&dispatched)),
		chain.WithPollInterval(time.Millisecond))

	if err := executor.Start(nil, "main"); err != nil {
		t.Fatal(err)
	}

	state := finalState(t, executor)

	if got := dispatched.Load(); got != 0 {
		t.Errorf("dispatched %d workflows, want 0", got)
	}

	if state.Status != chain.ChainFailed {
		t.Errorf("chain ended %q, want %q", state.Status, chain.ChainFailed)
	}

	if state.StepStatuses[0] != chain.StepFailed {
		t.Errorf("step status: got %v, want %v", state.StepStatuses[0], chain.StepFailed)
	}
}
