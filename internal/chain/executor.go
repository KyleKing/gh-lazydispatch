package chain

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// ChainStatus represents the overall status of a chain execution.
type ChainStatus string

const (
	ChainPending   ChainStatus = "pending"
	ChainRunning   ChainStatus = "running"
	ChainCompleted ChainStatus = "completed"
	ChainFailed    ChainStatus = "failed"
)

// StepStatus represents the status of a single step.
type StepStatus string

const (
	StepPending    StepStatus = "pending"
	StepRunning    StepStatus = "running"
	StepWaiting    StepStatus = "waiting"
	StepCompleted  StepStatus = "completed"
	StepFailed     StepStatus = "failed"
	StepSkipped    StepStatus = "skipped"
)

// StepResult represents the result of a completed step.
type StepResult struct {
	Workflow   string
	Inputs     map[string]string
	RunID      int64
	Status     StepStatus
	Conclusion string
}

// ChainState represents the current state of a chain execution.
type ChainState struct {
	ChainName    string
	CurrentStep  int
	StepResults  map[int]*StepResult
	StepStatuses []StepStatus
	Status       ChainStatus
	Error        error
}

// ChainUpdate is sent when the chain state changes.
type ChainUpdate struct {
	State ChainState
}

// ChainExecutor manages the execution of a workflow chain.
type ChainExecutor struct {
	client        GitHubClient
	watcher       RunWatcher
	chain         *config.Chain
	chainName     string
	state         *ChainState
	triggerInputs map[string]string
	branch        string
	updates       chan ChainUpdate
	mu            sync.RWMutex
	stopCh        chan struct{}
}

// NewExecutor creates a new chain executor.
func NewExecutor(client GitHubClient, w RunWatcher, chainName string, chain *config.Chain) *ChainExecutor {
	stepStatuses := make([]StepStatus, len(chain.Steps))
	for i := range stepStatuses {
		stepStatuses[i] = StepPending
	}

	return &ChainExecutor{
		client:    client,
		watcher:   w,
		chain:     chain,
		chainName: chainName,
		state: &ChainState{
			ChainName:    chainName,
			CurrentStep:  0,
			StepResults:  make(map[int]*StepResult),
			StepStatuses: stepStatuses,
			Status:       ChainPending,
		},
		updates: make(chan ChainUpdate, 10),
		stopCh:  make(chan struct{}),
	}
}

// Start begins executing the chain with the given trigger inputs.
func (e *ChainExecutor) Start(triggerInputs map[string]string, branch string) error {
	e.mu.Lock()
	e.triggerInputs = triggerInputs
	e.branch = branch
	e.state.Status = ChainRunning
	e.mu.Unlock()

	go e.runChain()
	return nil
}

// State returns the current chain state.
func (e *ChainExecutor) State() ChainState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return *e.state
}

// Updates returns the channel for receiving chain updates.
func (e *ChainExecutor) Updates() <-chan ChainUpdate {
	return e.updates
}

// Stop stops the chain execution.
func (e *ChainExecutor) Stop() {
	close(e.stopCh)
}

func (e *ChainExecutor) runChain() {
	defer close(e.updates)

	for i, step := range e.chain.Steps {
		select {
		case <-e.stopCh:
			return
		default:
		}

		e.mu.Lock()
		e.state.CurrentStep = i
		e.state.StepStatuses[i] = StepRunning
		e.mu.Unlock()
		e.sendUpdate()

		result, err := e.runStep(i, step)
		if err != nil {
			e.handleStepError(i, step, err)
			if e.state.Status == ChainFailed {
				return
			}
			continue
		}

		e.mu.Lock()
		e.state.StepResults[i] = result
		e.state.StepStatuses[i] = result.Status
		e.mu.Unlock()
		e.sendUpdate()

		if result.Status == StepFailed {
			if !e.handleStepFailure(i, step) {
				return
			}
		}
	}

	e.mu.Lock()
	e.state.Status = ChainCompleted
	e.mu.Unlock()
	e.sendUpdate()
}

func (e *ChainExecutor) runStep(idx int, step config.ChainStep) (*StepResult, error) {
	ctx := &InterpolationContext{
		Trigger: e.triggerInputs,
		Steps:   e.state.StepResults,
	}
	if idx > 0 {
		ctx.Previous = e.state.StepResults[idx-1]
	}

	inputs, err := InterpolateInputs(step.Inputs, ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to interpolate inputs: %w", err)
	}

	cfg := runner.RunConfig{
		Workflow: step.Workflow,
		Branch:   e.branch,
		Inputs:   inputs,
	}

	runID, err := runner.ExecuteAndGetRunID(cfg, e.client)
	if err != nil {
		return nil, fmt.Errorf("failed to dispatch workflow: %w", err)
	}

	e.watcher.Watch(runID, step.Workflow)

	e.mu.Lock()
	e.state.StepStatuses[idx] = StepWaiting
	e.mu.Unlock()
	e.sendUpdate()

	if step.WaitFor == config.WaitNone {
		return &StepResult{
			Workflow: step.Workflow,
			Inputs:   inputs,
			RunID:    runID,
			Status:   StepCompleted,
		}, nil
	}

	conclusion, err := e.waitForRun(runID, step.WaitFor)
	if err != nil {
		return nil, err
	}

	status := StepCompleted
	if conclusion != github.ConclusionSuccess && step.WaitFor == config.WaitSuccess {
		status = StepFailed
	}

	return &StepResult{
		Workflow:   step.Workflow,
		Inputs:     inputs,
		RunID:      runID,
		Status:     status,
		Conclusion: conclusion,
	}, nil
}

func (e *ChainExecutor) waitForRun(runID int64, waitFor config.WaitCondition) (string, error) {
	ticker := time.NewTicker(watcher.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return "", fmt.Errorf("chain execution stopped")
		case <-ticker.C:
			run, err := e.client.GetWorkflowRun(runID)
			if err != nil {
				return "", fmt.Errorf("failed to poll run %d: %w", runID, err)
			}

			if run.Status == github.StatusCompleted {
				return run.Conclusion, nil
			}
		}
	}
}

func (e *ChainExecutor) handleStepError(idx int, step config.ChainStep, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch step.OnFailure {
	case config.FailureAbort:
		e.state.StepStatuses[idx] = StepFailed
		e.state.Status = ChainFailed
		e.state.Error = err
	case config.FailureSkip:
		e.state.StepStatuses[idx] = StepSkipped
	case config.FailureContinue:
		e.state.StepStatuses[idx] = StepFailed
	}
}

func (e *ChainExecutor) handleStepFailure(idx int, step config.ChainStep) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	switch step.OnFailure {
	case config.FailureAbort:
		e.state.Status = ChainFailed
		return false
	case config.FailureSkip, config.FailureContinue:
		return true
	}
	return false
}

func (e *ChainExecutor) sendUpdate() {
	e.mu.RLock()
	state := *e.state
	e.mu.RUnlock()

	select {
	case e.updates <- ChainUpdate{State: state}:
	default:
		log.Printf("warning: chain update channel full, update dropped for step %d", state.CurrentStep)
	}
}
