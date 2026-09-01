// Package exec provides command execution with mocking support for testing.
package exec

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

const (
	ghAPISubcommand      = "api"
	ghLogFlag            = "--log"
	ghRunSubcommand      = "run"
	ghViewOperation      = "view"
	ghWorkflowSubcommand = "workflow"
)

// CommandExecutor defines an interface for executing external commands.
// This allows us to mock command execution in tests.
type CommandExecutor interface {
	// Execute runs a command with the given name and arguments.
	// Returns stdout, stderr, and any error.
	Execute(name string, args ...string) (stdout, stderr string, err error)
}

// RealExecutor executes actual system commands.
type RealExecutor struct{}

// NewRealExecutor creates an executor that runs real commands.
func NewRealExecutor() *RealExecutor {
	return &RealExecutor{}
}

// Execute runs the actual command using os/exec.
// It includes a safety check to prevent accidental mutation of GitHub resources during tests.
//
//nolint:nonamedreturns // gocritic wants named returns matching the CommandExecutor interface
func (*RealExecutor) Execute(name string, args ...string) (stdout, stderr string, err error) {
	// Safety check: Prevent mutation commands during tests
	if testing.Testing() && isMutationCommand(name, args) {
		panic(fmt.Sprintf(
			"SAFETY VIOLATION: Attempted to run mutation command during test: %s %s\n"+
				"This could modify real GitHub resources!\n"+
				"Use testutil.MockExecutor or runner.SetExecutor() in your test instead.",
			name, strings.Join(args, " "),
		))
	}

	// #nosec G204 -- deliberate exec wrapper; callers pass fixed binaries with internal args
	cmd := exec.CommandContext(context.Background(), name, args...)

	var stdoutBuf, stderrBuf bytes.Buffer

	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	err = cmd.Run()

	return stdoutBuf.String(), stderrBuf.String(), err
}

// isMutationCommand checks if a command could mutate GitHub resources.
func isMutationCommand(name string, args []string) bool {
	if name != "gh" {
		return false
	}

	if len(args) == 0 {
		return false
	}

	// Block commands that can mutate GitHub state
	mutationCommands := map[string]bool{
		ghWorkflowSubcommand: true, // gh workflow run
		"issue":              true, // gh issue create/edit/close
		"pr":                 true, // gh pr create/merge/close
		"release":            true, // gh release create/delete
		"repo":               true, // gh repo create/delete
		"secret":             true, // gh secret set/delete
		"variable":           true, // gh variable set/delete
		"label":              true, // gh label create/delete
		ghRunSubcommand:      true, // gh run cancel/rerun (but not "run view")
		"gist":               true, // gh gist create/delete
		"project":            true, // gh project create/delete
		"cache":              true, // gh cache delete
		"attestation":        true, // gh attestation verify can write
		"codespace":          true, // gh codespace create/delete
		"gpg-key":            true, // gh gpg-key add/delete
		"ssh-key":            true, // gh ssh-key add/delete
	}

	subcommand := args[0]

	// Special case: "gh run view" is read-only, but "gh run cancel/rerun" are mutations
	if subcommand == ghRunSubcommand && len(args) > 1 {
		operation := args[1]
		// Allow read-only run operations
		if operation == ghViewOperation || operation == "list" || operation == "watch" {
			return false
		}

		return true // Block cancel, rerun, etc.
	}

	return mutationCommands[subcommand]
}
