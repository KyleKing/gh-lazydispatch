package chain_test

import (
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/testutil"
)

func TestNewExecutor(t *testing.T) {
	client := testutil.NewMockGitHubClient()
	w := testutil.NewMockRunWatcher()
	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "step1.yml"},
			{Workflow: "step2.yml"},
		},
	}

	executor := chain.NewExecutor(client, w, "test-chain", chainDef)
	state := executor.State()

	if state.ChainName != "test-chain" {
		t.Errorf("ChainName: got %q, want %q", state.ChainName, "test-chain")
	}
	if state.Status != chain.ChainPending {
		t.Errorf("Status: got %v, want %v", state.Status, chain.ChainPending)
	}
	if len(state.StepStatuses) != 2 {
		t.Errorf("StepStatuses length: got %d, want 2", len(state.StepStatuses))
	}
	for i, status := range state.StepStatuses {
		if status != chain.StepPending {
			t.Errorf("StepStatuses[%d]: got %v, want %v", i, status, chain.StepPending)
		}
	}
}

func TestChainExecutor_Stop(t *testing.T) {
	mockExec := exec.NewMockExecutor()
	runner.SetExecutor(mockExec)
	defer runner.SetExecutor(nil)

	client := testutil.NewMockGitHubClient()
	client.LatestID = 123
	w := testutil.NewMockRunWatcher()
	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "step1.yml", WaitFor: config.WaitSuccess},
		},
	}

	executor := chain.NewExecutor(client, w, "test-chain", chainDef)
	executor.Stop()

	select {
	case <-executor.Updates():
	case <-time.After(100 * time.Millisecond):
	}
}

func TestChainExecutor_DoubleStop(t *testing.T) {
	client := testutil.NewMockGitHubClient()
	w := testutil.NewMockRunWatcher()
	chainDef := &config.Chain{
		Steps: []config.ChainStep{{Workflow: "test.yml"}},
	}

	executor := chain.NewExecutor(client, w, "test", chainDef)
	executor.Stop()
	executor.Stop() // Should not panic
}

// Chain execution tests moved to internal/integration_test.go for E2E coverage.
// Kept here: unit tests for specific initialization and state functionality.

func TestNewExecutorFromHistory(t *testing.T) {
	client := testutil.NewMockGitHubClient()
	w := testutil.NewMockRunWatcher()
	chainDef := &config.Chain{
		Steps: []config.ChainStep{
			{Workflow: "step1.yml"},
			{Workflow: "step2.yml"},
			{Workflow: "step3.yml"},
		},
	}

	previousResults := []chain.PreviousStepResult{
		{Workflow: "step1.yml", RunID: 100, Status: "completed", Conclusion: "success"},
	}

	executor := chain.NewExecutorFromHistory(client, w, "resume-chain", chainDef, previousResults, 1)
	state := executor.State()

	if state.CurrentStep != 1 {
		t.Errorf("CurrentStep: got %d, want 1", state.CurrentStep)
	}
	if state.StepStatuses[0] != chain.StepCompleted {
		t.Errorf("StepStatuses[0]: got %v, want %v", state.StepStatuses[0], chain.StepCompleted)
	}
	if state.StepStatuses[1] != chain.StepPending {
		t.Errorf("StepStatuses[1]: got %v, want %v", state.StepStatuses[1], chain.StepPending)
	}
}
