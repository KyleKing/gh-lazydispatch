package exec

import (
	"bytes"
	"os/exec"
)

// CommandExecutor defines an interface for executing external commands.
// This allows us to mock command execution in tests.
type CommandExecutor interface {
	// Execute runs a command with the given name and arguments.
	// Returns stdout, stderr, and any error.
	Execute(name string, args ...string) (stdout string, stderr string, err error)
}

// RealExecutor executes actual system commands.
type RealExecutor struct{}

// NewRealExecutor creates an executor that runs real commands.
func NewRealExecutor() *RealExecutor {
	return &RealExecutor{}
}

// Execute runs the actual command using os/exec.
func (e *RealExecutor) Execute(name string, args ...string) (string, string, error) {
	cmd := exec.Command(name, args...)

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}
