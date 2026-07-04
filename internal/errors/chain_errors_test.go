package chainerr_test

import (
	"errors"
	"testing"

	chainerr "github.com/kyleking/gh-lazydispatch/internal/errors"
)

func TestStepExecutionError(t *testing.T) {
	t.Parallel()

	cause := errors.New("run failed")
	err := &chainerr.StepExecutionError{
		StepIndex: 2,
		Workflow:  "deploy.yml",
		RunID:     12345,
		RunURL:    "https://github.com/owner/repo/actions/runs/12345",
		Cause:     cause,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}

	if !errors.Is(err, cause) {
		t.Error("expected error to unwrap to cause")
	}

	url := chainerr.GetRunURL(err)
	if url != "https://github.com/owner/repo/actions/runs/12345" {
		t.Errorf("GetRunURL: got %q, want url", url)
	}
}

func TestStepExecutionError_NoURL(t *testing.T) {
	t.Parallel()

	err := &chainerr.StepExecutionError{
		StepIndex: 0,
		Workflow:  "ci.yml",
		Cause:     errors.New("failed"),
	}

	url := chainerr.GetRunURL(err)
	if url != "" {
		t.Errorf("GetRunURL: got %q, want empty", url)
	}
}

func TestStepDispatchError(t *testing.T) {
	t.Parallel()

	cause := errors.New("workflow not found")
	err := &chainerr.StepDispatchError{
		Workflow:   "deploy.yml",
		Branch:     "main",
		Cause:      cause,
		Suggestion: "Check workflow file exists",
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}

	if !errors.Is(err, cause) {
		t.Error("expected error to unwrap to cause")
	}

	suggestion := chainerr.GetSuggestion(err)
	if suggestion != "Check workflow file exists" {
		t.Errorf("GetSuggestion: got %q, want suggestion", suggestion)
	}
}

func TestInterpolationError(t *testing.T) {
	t.Parallel()

	cause := errors.New("invalid template")
	err := &chainerr.InterpolationError{
		Field: "environment",
		Value: "{{ invalid }}",
		Cause: cause,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}

	if !errors.Is(err, cause) {
		t.Error("expected error to unwrap to cause")
	}
}

func TestRunWaitError(t *testing.T) {
	t.Parallel()

	cause := errors.New("API timeout")
	err := &chainerr.RunWaitError{
		RunID:  99999,
		RunURL: "https://github.com/owner/repo/actions/runs/99999",
		Cause:  cause,
	}

	msg := err.Error()
	if msg == "" {
		t.Error("expected non-empty error message")
	}

	if !errors.Is(err, cause) {
		t.Error("expected error to unwrap to cause")
	}

	url := chainerr.GetRunURL(err)
	if url != "https://github.com/owner/repo/actions/runs/99999" {
		t.Errorf("GetRunURL: got %q, want url", url)
	}
}

func TestGetRunURL_NestedError(t *testing.T) {
	t.Parallel()

	innerErr := &chainerr.RunWaitError{
		RunID:  123,
		RunURL: "https://example.com/run/123",
		Cause:  errors.New("timeout"),
	}
	outerErr := &chainerr.StepExecutionError{
		StepIndex: 1,
		Workflow:  "ci.yml",
		RunID:     123,
		Cause:     innerErr,
	}

	url := chainerr.GetRunURL(outerErr)
	if url == "" {
		t.Error("expected to find URL in error chain")
	}
}

func TestGetSuggestion_NoSuggestion(t *testing.T) {
	t.Parallel()

	err := &chainerr.StepExecutionError{
		StepIndex: 0,
		Workflow:  "ci.yml",
		Cause:     errors.New("failed"),
	}

	suggestion := chainerr.GetSuggestion(err)
	if suggestion != "" {
		t.Errorf("GetSuggestion: got %q, want empty", suggestion)
	}
}
