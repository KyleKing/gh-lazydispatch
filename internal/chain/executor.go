// Package chain provides workflow chain execution and template interpolation.
package chain

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	chainerr "github.com/kyleking/gh-lazydispatch/internal/errors"
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
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepWaiting   StepStatus = "waiting"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

// StepResult represents the result of a completed step.
type StepResult struct {
	Workflow   string
	Inputs     map[string]string
	RunID      int64
	RunURL     string
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
	client    GitHubClient
	watcher   RunWatcher
	chain     *config.Chain
	chainName string
	state     *ChainState
	variables map[string]string // chain-level variables
	branch    string
	updates   chan ChainUpdate
	mu        sync.RWMutex
	stopCh    chan struct{}
	stopOnce  sync.Once
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

// PreviousStepResult contains the result of a previously completed step.
type PreviousStepResult struct {
	Workflow   string
	RunID      int64
	Status     string
	Conclusion string
}

// NewExecutorFromHistory creates a chain executor that resumes from a specific step.
// Steps 0..resumeFromStep-1 are pre-populated from previousResults.
func NewExecutorFromHistory(
	client GitHubClient,
	w RunWatcher,
	chainName string,
	chain *config.Chain,
	previousResults []PreviousStepResult,
	resumeFromStep int,
) *ChainExecutor {
	stepStatuses := make([]StepStatus, len(chain.Steps))
	stepResults := make(map[int]*StepResult)

	for i := range len(chain.Steps) {
		if i < resumeFromStep && i < len(previousResults) {
			prev := previousResults[i]
			status := StepCompleted

			if prev.Status == "failed" || prev.Conclusion == "failure" {
				status = StepFailed
			} else if prev.Status == "skipped" {
				status = StepSkipped
			}

			stepStatuses[i] = status
			stepResults[i] = &StepResult{
				Workflow:   prev.Workflow,
				RunID:      prev.RunID,
				Status:     status,
				Conclusion: prev.Conclusion,
			}
		} else {
			stepStatuses[i] = StepPending
		}
	}

	return &ChainExecutor{
		client:    client,
		watcher:   w,
		chain:     chain,
		chainName: chainName,
		state: &ChainState{
			ChainName:    chainName,
			CurrentStep:  resumeFromStep,
			StepResults:  stepResults,
			StepStatuses: stepStatuses,
			Status:       ChainPending,
		},
		updates: make(chan ChainUpdate, 10),
		stopCh:  make(chan struct{}),
	}
}

// Start begins executing the chain with the given variables.
func (e *ChainExecutor) Start(variables map[string]string, branch string) error {
	e.mu.Lock()
	e.variables = variables
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
// Safe to call multiple times.
func (e *ChainExecutor) Stop() {
	e.stopOnce.Do(func() {
		close(e.stopCh)
	})
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
		Var:   e.variables,
		Steps: e.state.StepResults,
	}
	if idx > 0 {
		ctx.Previous = e.state.StepResults[idx-1]
	}

	inputs, err := InterpolateInputs(step.Inputs, ctx)
	if err != nil {
		return nil, &chainerr.InterpolationError{
			Field: "inputs",
			Value: fmt.Sprintf("%v", step.Inputs),
			Cause: err,
		}
	}

	cfg := runner.RunConfig{
		Workflow: step.Workflow,
		Branch:   e.branch,
		Inputs:   inputs,
	}

	runID, err := runner.ExecuteAndGetRunID(cfg, e.client)
	if err != nil {
		suggestion := ""
		if e.branch != "" {
			suggestion = fmt.Sprintf("Verify workflow %q exists and supports workflow_dispatch on branch %q", step.Workflow, e.branch)
		}

		return nil, &chainerr.StepDispatchError{
			Workflow:   step.Workflow,
			Branch:     e.branch,
			Cause:      err,
			Suggestion: suggestion,
		}
	}

	e.watcher.Watch(runID, step.Workflow)

	run, _ := e.client.GetWorkflowRun(runID)
	runURL := ""

	if run != nil {
		runURL = run.HTMLURL
	}

	e.mu.Lock()
	e.state.StepStatuses[idx] = StepWaiting
	e.mu.Unlock()
	e.sendUpdate()

	if step.WaitFor == config.WaitNone {
		return &StepResult{
			Workflow: step.Workflow,
			Inputs:   inputs,
			RunID:    runID,
			RunURL:   runURL,
			Status:   StepCompleted,
		}, nil
	}

	conclusion, waitRunURL, err := e.waitForRun(runID, step.WaitFor)
	if waitRunURL != "" {
		runURL = waitRunURL
	}

	if err != nil {
		return nil, &chainerr.StepExecutionError{
			StepIndex: idx,
			Workflow:  step.Workflow,
			RunID:     runID,
			RunURL:    runURL,
			Cause:     err,
		}
	}

	status := StepCompleted
	if conclusion != github.ConclusionSuccess && step.WaitFor == config.WaitSuccess {
		status = StepFailed
	}

	return &StepResult{
		Workflow:   step.Workflow,
		Inputs:     inputs,
		RunID:      runID,
		RunURL:     runURL,
		Status:     status,
		Conclusion: conclusion,
	}, nil
}

func (e *ChainExecutor) waitForRun(runID int64, waitFor config.WaitCondition) (conclusion, runURL string, err error) {
	ticker := time.NewTicker(watcher.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return "", "", errors.New("chain execution stopped")
		case <-ticker.C:
			run, pollErr := e.client.GetWorkflowRun(runID)
			if pollErr != nil {
				return "", "", &chainerr.RunWaitError{
					RunID: runID,
					Cause: pollErr,
				}
			}

			runURL = run.HTMLURL
			if run.Status == github.StatusCompleted {
				return run.Conclusion, runURL, nil
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
	case <-e.stopCh:
		return
	case e.updates <- ChainUpdate{State: state}:
	default:
		log.Printf("warning: chain update channel full, update dropped for step %d", state.CurrentStep)
	}
}
