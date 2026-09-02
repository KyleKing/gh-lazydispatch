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

// mutationSubcommands are the gh subcommands that can write, and so are
// blocked outright unless the operation after them is on readOperations.
//
//nolint:gochecknoglobals // a fixed table read by isMutationCommand
var mutationSubcommands = map[string]bool{
	"attestation":        true, // gh attestation verify can write
	"cache":              true, // gh cache delete
	"codespace":          true, // gh codespace create/delete
	"gist":               true, // gh gist create/delete
	"gpg-key":            true, // gh gpg-key add/delete
	"issue":              true, // gh issue create/edit/close
	"label":              true, // gh label create/delete
	"pr":                 true, // gh pr create/merge/close
	"project":            true, // gh project create/delete
	"release":            true, // gh release create/delete
	"repo":               true, // gh repo create/delete
	ghRunSubcommand:      true, // gh run cancel/rerun
	"secret":             true, // gh secret set/delete
	"ssh-key":            true, // gh ssh-key add/delete
	"variable":           true, // gh variable set/delete
	ghWorkflowSubcommand: true, // gh workflow run
}

// readOperations name a subcommand's read-only operations. The allowlist is
// per-operation rather than per-subcommand because the same noun both reads
// and writes: `gh run view` reads a run and `gh run cancel` ends one, and
// `gh pr list` is how this tool reads a pull request's checks. Anything absent
// stays blocked, so a gh release adding an operation is blocked until someone
// decides it reads.
//
//nolint:gochecknoglobals // a fixed table read by isMutationCommand
var readOperations = map[string]bool{
	"checks":        true,
	"diff":          true,
	"list":          true,
	"status":        true,
	ghViewOperation: true,
	"watch":         true,
}

// isMutationCommand reports whether a gh call could write to GitHub. It errs
// toward blocking: a subcommand that can write is a mutation unless its
// operation is a named read.
func isMutationCommand(name string, args []string) bool {
	if name != "gh" || len(args) == 0 {
		return false
	}

	if !mutationSubcommands[args[0]] {
		return false
	}

	return len(args) < 2 || !readOperations[args[1]]
}
