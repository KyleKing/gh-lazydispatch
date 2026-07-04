package chainerr

import "fmt"

// APIError represents an error from a GitHub API operation.
type APIError struct {
	Err       error
	Operation string
	RunID     int64
}

func (e *APIError) Error() string {
	return fmt.Sprintf("%s for run %d: %v", e.Operation, e.RunID, e.Err)
}

func (e *APIError) Unwrap() error {
	return e.Err
}

// ChannelFullError indicates a channel update was dropped due to full buffer.
type ChannelFullError struct {
	Channel string
}

func (e *ChannelFullError) Error() string {
	return fmt.Sprintf("channel %s is full, update dropped", e.Channel)
}

// ValidationBlockedError indicates execution was blocked due to validation errors.
type ValidationBlockedError struct {
	Input  string
	Errors []string
}

func (e *ValidationBlockedError) Error() string {
	return fmt.Sprintf("validation failed for input %s: %v", e.Input, e.Errors)
}
