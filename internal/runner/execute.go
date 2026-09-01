// Package runner provides workflow execution functionality using the GitHub CLI.
package runner

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	execpkg "github.com/kyleking/gh-lazydispatch/internal/exec"
)

const (
	ghRunArg      = "run"
	ghWorkflowArg = "workflow"
)

// ErrNoRunFound indicates no workflow run was found after dispatch.
var ErrNoRunFound = errors.New("no run found for workflow")

// RunConfig holds the configuration for running a workflow.
type RunConfig struct {
	Inputs   map[string]string
	Workflow string
	Branch   string
	Watch    bool
}

// defaultCommandExecutor wraps exec.CommandExecutor for interactive use.
type defaultCommandExecutor struct {
	executor execpkg.CommandExecutor
}

func (e defaultCommandExecutor) Execute(name string, args ...string) error {
	// For interactive execution, we want stdout/stderr to go directly to the terminal
	if e.executor == nil {
		// #nosec G204 -- deliberate exec wrapper; callers pass fixed binaries with internal args
		cmd := exec.CommandContext(context.Background(), name, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("running %s: %w", name, err)
		}

		return nil
	}

	// When using an injected executor (e.g., for testing), use it
	if _, _, err := e.executor.Execute(name, args...); err != nil {
		return fmt.Errorf("executing %s: %w", name, err)
	}

	return nil
}

// The default executor writes gh's output straight to the terminal, which is
// right only once Bubble Tea has released it (tea.ExecProcess). The quiet one
// captures that output instead, for the paths that run while the TUI draws.
var (
	executor      CommandExecutor = defaultCommandExecutor{executor: nil}
	quietExecutor CommandExecutor = capturingExecutor{executor: nil}
)

// capturingExecutor swallows a command's output so it cannot reach a terminal
// the TUI is drawing to.
type capturingExecutor struct {
	executor execpkg.CommandExecutor
}

func (e capturingExecutor) Execute(name string, args ...string) error {
	cmdExec := e.executor
	if cmdExec == nil {
		cmdExec = execpkg.NewRealExecutor()
	}

	if _, _, err := cmdExec.Execute(name, args...); err != nil {
		return fmt.Errorf("executing %s: %w", name, err)
	}

	return nil
}

// SetExecutor sets the command executor for testing purposes.
// Pass nil to reset to default behavior.
func SetExecutor(cmdExec execpkg.CommandExecutor) {
	executor = defaultCommandExecutor{executor: cmdExec}
	quietExecutor = capturingExecutor{executor: cmdExec}
}

// BuildArgs constructs the gh workflow run arguments.
func BuildArgs(cfg RunConfig) []string {
	args := []string{ghWorkflowArg, ghRunArg, cfg.Workflow}

	if cfg.Branch != "" {
		args = append(args, "--ref", cfg.Branch)
	}

	for k, v := range cfg.Inputs {
		if v != "" {
			args = append(args, "-f", k+"="+v)
		}
	}

	return args
}

// FormatCommand returns a human-readable command string.
func FormatCommand(args []string) string {
	quoted := make([]string, len(args))

	for i, arg := range args {
		if strings.Contains(arg, " ") || strings.Contains(arg, "=") {
			quoted[i] = fmt.Sprintf("%q", arg)
		} else {
			quoted[i] = arg
		}
	}

	return "gh " + strings.Join(quoted, " ")
}

// CommandExecutor executes shell commands (for testing compatibility).
type CommandExecutor interface {
	Execute(name string, args ...string) error
}

// Execute runs the workflow using gh CLI.
// It prints the command being run (like lazygit) then executes it.
func Execute(cfg RunConfig) error {
	return ExecuteWithExecutor(cfg, executor)
}

// ExecuteWithExecutor runs the workflow using the given executor instead of the package default.
func ExecuteWithExecutor(cfg RunConfig, cmdExec CommandExecutor) error {
	args := BuildArgs(cfg)

	fmt.Println()
	fmt.Println("Running command:")
	fmt.Println("  " + FormatCommand(args))
	fmt.Println()

	if err := cmdExec.Execute("gh", args...); err != nil {
		return fmt.Errorf("gh workflow run failed: %w", err)
	}

	if cfg.Watch {
		return watchLatestRunWithExecutor(cfg.Workflow, cmdExec)
	}

	return nil
}

func watchLatestRunWithExecutor(_ string, cmdExec CommandExecutor) error {
	fmt.Println()
	fmt.Println("Watching run...")
	fmt.Println()

	if err := cmdExec.Execute("gh", ghRunArg, "watch"); err != nil {
		return fmt.Errorf("gh run watch failed: %w", err)
	}

	return nil
}

// DryRun prints the command that would be executed without running it.
func DryRun(cfg RunConfig) string {
	args := BuildArgs(cfg)
	return FormatCommand(args)
}

// ExecuteAndGetRunID runs the workflow and returns the run ID for watching.
// This polls the API shortly after dispatch to find the triggered run.
//
// It writes nothing to the terminal: a chain calls it from a goroutine while
// the TUI still owns the alt screen, so anything gh or this package printed
// would land on top of the rendered frame.
func ExecuteAndGetRunID(cfg RunConfig, client GitHubClient) (int64, error) {
	return ExecuteAndGetRunIDWithExecutor(cfg, client, quietExecutor)
}

// ExecuteAndGetRunIDWithExecutor runs the workflow using the given executor and returns the triggered run ID.
func ExecuteAndGetRunIDWithExecutor(cfg RunConfig, client GitHubClient, cmdExec CommandExecutor) (int64, error) {
	args := BuildArgs(cfg)

	if err := cmdExec.Execute("gh", args...); err != nil {
		return 0, fmt.Errorf("gh workflow run failed: %w", err)
	}

	run, err := client.GetLatestRun(cfg.Workflow)
	if err != nil {
		return 0, fmt.Errorf("failed to get run ID: %w", err)
	}

	if run == nil {
		return 0, fmt.Errorf("%w: %s", ErrNoRunFound, cfg.Workflow)
	}

	return run.ID, nil
}
