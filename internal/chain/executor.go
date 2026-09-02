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
//
//nolint:revive // stutters but renaming to Status would break call sites across the codebase
type ChainStatus string

// Overall chain execution statuses.
const (
	ChainPending   ChainStatus = "pending"
	ChainRunning   ChainStatus = "running"
	ChainCompleted ChainStatus = "completed"
	ChainFailed    ChainStatus = "failed"
)

// StepStatus represents the status of a single step.
type StepStatus string

// Per-step execution statuses.
const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepWaiting   StepStatus = "waiting"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

// ErrChainExecutionStopped indicates the chain was stopped while waiting for a run.
var ErrChainExecutionStopped = errors.New("chain execution stopped")

// StepResult represents the result of a completed step.
type StepResult struct {
	Inputs     map[string]string
	Workflow   string
	RunURL     string
	Status     StepStatus
	Conclusion string
	RunID      int64
}

// ChainState represents the current state of a chain execution.
//
//nolint:revive // stutters but renaming to State would break call sites across the codebase
type ChainState struct {
	Error        error
	StepResults  map[int]*StepResult
	ChainName    string
	Status       ChainStatus
	StepStatuses []StepStatus
	CurrentStep  int
}

// ChainUpdate is sent when the chain state changes.
//
//nolint:revive // stutters but renaming to Update would break call sites across the codebase
type ChainUpdate struct {
	State ChainState
}

// ChainExecutor manages the execution of a workflow chain.
//
//nolint:revive // stutters but renaming to Executor would break call sites across the codebase
type ChainExecutor struct {
	client          GitHubClient
	watcher         RunWatcher
	chain           *config.Chain
	state           *ChainState
	variables       map[string]string
	updates         chan ChainUpdate
	stopCh          chan struct{}
	dispatch        Dispatcher
	resolveExisting ExistingRunResolver
	chainName       string
	branch          string
	pollInterval    time.Duration
	mu              sync.RWMutex
	stopOnce        sync.Once
}

// Dispatcher starts one workflow and reports the run it started.
//
// The executor holds one rather than calling the runner directly, because the
// dispatch is the one step a test cannot take: internal/exec's mutation guard
// panics on `gh workflow run`, so the branch and failure handling around it were
// unreachable while the call was hard-wired.
type Dispatcher func(cfg runner.RunConfig, client GitHubClient) (int64, error)

// ErrNoExistingRun indicates a source: existing step found no queued or
// in-progress run of its workflow on the branch to adopt.
var ErrNoExistingRun = errors.New("no queued or in-progress run found")

// ExistingRunResolver finds the run a source: existing step should adopt,
// rather than dispatching a fresh one.
type ExistingRunResolver func(client GitHubClient, workflow, branch string) (*github.WorkflowRun, error)

// existingRunStatuses is the order ResolveExistingRun checks: a run already
// under way is what "existing" means, so an in-progress run wins over a
// merely queued one.
var existingRunStatuses = []string{github.StatusInProgress, github.StatusQueued}

// ResolveExistingRun is the real resolution: it asks GitHub for the newest
// in-progress or queued run of workflow on branch. It never falls back to
// dispatching, because a step that sometimes starts a run and sometimes
// adopts one is a step nobody can read.
func ResolveExistingRun(client GitHubClient, workflow, branch string) (*github.WorkflowRun, error) {
	for _, status := range existingRunStatuses {
		runs, err := client.ListRuns(github.RunQuery{Workflow: workflow, Branch: branch, Status: status, Limit: 1})
		if err != nil {
			return nil, fmt.Errorf("listing %s runs of %s: %w", status, workflow, err)
		}

		if len(runs) > 0 {
			return &runs[0], nil
		}
	}

	return nil, ErrNoExistingRun
}

// dispatchThroughRunner is the real dispatch: it shells out to
// `gh workflow run` and reports the run that appeared.
func dispatchThroughRunner(cfg runner.RunConfig, client GitHubClient) (int64, error) {
	runID, err := runner.ExecuteAndGetRunID(cfg, client)
	if err != nil {
		return 0, fmt.Errorf("dispatching %s: %w", cfg.Workflow, err)
	}

	return runID, nil
}

// Option configures an executor.
type Option func(*ChainExecutor)

// WithDispatcher replaces how a step starts its workflow.
func WithDispatcher(d Dispatcher) Option {
	return func(e *ChainExecutor) { e.dispatch = d }
}

// WithPollInterval sets how often a step waiting on a run asks about it.
func WithPollInterval(d time.Duration) Option {
	return func(e *ChainExecutor) { e.pollInterval = d }
}

// WithExistingRunResolver replaces how a source: existing step finds the run
// to adopt.
func WithExistingRunResolver(r ExistingRunResolver) Option {
	return func(e *ChainExecutor) { e.resolveExisting = r }
}

// NewExecutor creates a new chain executor.
func NewExecutor(
	client GitHubClient, w RunWatcher, chainName string, chain *config.Chain, opts ...Option,
) *ChainExecutor {
	stepStatuses := make([]StepStatus, len(chain.Steps))
	for i := range stepStatuses {
		stepStatuses[i] = StepPending
	}

	executor := &ChainExecutor{
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
		updates:         make(chan ChainUpdate, 10),
		stopCh:          make(chan struct{}),
		dispatch:        dispatchThroughRunner,
		resolveExisting: ResolveExistingRun,
		pollInterval:    watcher.PollInterval,
	}

	for _, opt := range opts {
		opt(executor)
	}

	return executor
}

// PreviousStepResult contains the result of a previously completed step.
type PreviousStepResult struct {
	Workflow   string
	Status     string
	Conclusion string
	RunID      int64
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
//
//nolint:unparam // error return is part of the public API contract; kept for future validation without breaking callers
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

// startStep gets a step's run going, either by dispatching a fresh one or by
// adopting a run already in progress, and reports its ID and URL.
func (e *ChainExecutor) startStep(step config.ChainStep, inputs map[string]string) (int64, string, error) {
	if step.Source == config.SourceExisting {
		run, err := e.resolveExisting(e.client, step.Workflow, e.branch)
		if err != nil {
			return 0, "", &chainerr.StepAdoptionError{Workflow: step.Workflow, Branch: e.branch, Cause: err}
		}

		return run.ID, run.URL, nil
	}

	cfg := runner.RunConfig{
		Workflow: step.Workflow,
		Branch:   e.branch,
		Inputs:   inputs,
	}

	runID, err := e.dispatch(cfg, e.client)
	if err != nil {
		suggestion := ""
		if e.branch != "" {
			suggestion = fmt.Sprintf(
				"Verify workflow %q exists and supports workflow_dispatch on branch %q", step.Workflow, e.branch,
			)
		}

		return 0, "", &chainerr.StepDispatchError{
			Workflow:   step.Workflow,
			Branch:     e.branch,
			Cause:      err,
			Suggestion: suggestion,
		}
	}

	//nolint:errcheck // best-effort: run URL is optional display info, nil run is handled below
	run, _ := e.client.GetWorkflowRun(runID)
	if run != nil {
		return runID, run.URL, nil
	}

	return runID, "", nil
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
			Field: inputsSegment,
			Value: fmt.Sprintf("%v", step.Inputs),
			Cause: err,
		}
	}

	runID, runURL, err := e.startStep(step, inputs)
	if err != nil {
		return nil, err
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

func (e *ChainExecutor) waitForRun(runID int64, _ config.WaitCondition) (string, string, error) {
	ticker := time.NewTicker(e.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-e.stopCh:
			return "", "", ErrChainExecutionStopped
		case <-ticker.C:
			run, pollErr := e.client.GetWorkflowRun(runID)
			if pollErr != nil {
				return "", "", &chainerr.RunWaitError{
					RunID: runID,
					Cause: pollErr,
				}
			}

			if run.Status == github.StatusCompleted {
				return run.Conclusion, run.URL, nil
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

func (e *ChainExecutor) handleStepFailure(_ int, step config.ChainStep) bool {
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
