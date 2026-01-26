// Package errors defines custom error types for workflow chain execution and GitHub API operations.
package errors

import (
	"errors"
	"fmt"
)

// StepExecutionError represents an error that occurred during step execution.
type StepExecutionError struct {
	StepIndex int
	Workflow  string
	RunID     int64
	RunURL    string
	Cause     error
}

func (e *StepExecutionError) Error() string {
	if e.RunURL != "" {
		return fmt.Sprintf("step %d (%s) failed: %v [run: %s]", e.StepIndex+1, e.Workflow, e.Cause, e.RunURL)
	}

	return fmt.Sprintf("step %d (%s) failed: %v", e.StepIndex+1, e.Workflow, e.Cause)
}

func (e *StepExecutionError) Unwrap() error {
	return e.Cause
}

// StepDispatchError represents a failure to dispatch a workflow.
type StepDispatchError struct {
	Workflow   string
	Branch     string
	Cause      error
	Suggestion string
}

func (e *StepDispatchError) Error() string {
	return fmt.Sprintf("failed to dispatch workflow %q on branch %q: %v", e.Workflow, e.Branch, e.Cause)
}

func (e *StepDispatchError) Unwrap() error {
	return e.Cause
}

// InterpolationError represents a failure to interpolate template variables.
type InterpolationError struct {
	Field string
	Value string
	Cause error
}

func (e *InterpolationError) Error() string {
	return fmt.Sprintf("failed to interpolate %q with value %q: %v", e.Field, e.Value, e.Cause)
}

func (e *InterpolationError) Unwrap() error {
	return e.Cause
}

// RunWaitError represents an error while waiting for a run to complete.
type RunWaitError struct {
	RunID  int64
	RunURL string
	Cause  error
}

func (e *RunWaitError) Error() string {
	if e.RunURL != "" {
		return fmt.Sprintf("failed waiting for run %d: %v [url: %s]", e.RunID, e.Cause, e.RunURL)
	}

	return fmt.Sprintf("failed waiting for run %d: %v", e.RunID, e.Cause)
}

func (e *RunWaitError) Unwrap() error {
	return e.Cause
}

// GetRunURL extracts the run URL from an error chain if present.
func GetRunURL(err error) string {
	var stepErr *StepExecutionError
	if errors.As(err, &stepErr) && stepErr.RunURL != "" {
		return stepErr.RunURL
	}

	var waitErr *RunWaitError
	if errors.As(err, &waitErr) && waitErr.RunURL != "" {
		return waitErr.RunURL
	}

	return ""
}

// GetSuggestion extracts a suggestion from an error chain if present.
func GetSuggestion(err error) string {
	var dispatchErr *StepDispatchError
	if errors.As(err, &dispatchErr) && dispatchErr.Suggestion != "" {
		return dispatchErr.Suggestion
	}

	return ""
}
