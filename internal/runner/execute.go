// Package runner provides workflow execution functionality using the GitHub CLI.
package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	execpkg "github.com/kyleking/gh-lazydispatch/internal/exec"
)

// RunConfig holds the configuration for running a workflow.
type RunConfig struct {
	Workflow string
	Branch   string
	Inputs   map[string]string
	Watch    bool
}

// defaultCommandExecutor wraps exec.CommandExecutor for interactive use.
type defaultCommandExecutor struct {
	executor execpkg.CommandExecutor
}

func (e defaultCommandExecutor) Execute(name string, args ...string) error {
	// For interactive execution, we want stdout/stderr to go directly to the terminal
	if e.executor == nil {
		cmd := exec.Command(name, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin

		return cmd.Run()
	}

	// When using an injected executor (e.g., for testing), use it
	_, _, err := e.executor.Execute(name, args...)

	return err
}

var executor = defaultCommandExecutor{executor: nil}

// SetExecutor sets the command executor for testing purposes.
// Pass nil to reset to default behavior.
func SetExecutor(exec execpkg.CommandExecutor) {
	executor = defaultCommandExecutor{executor: exec}
}

// BuildArgs constructs the gh workflow run arguments.
func BuildArgs(cfg RunConfig) []string {
	args := []string{"workflow", "run", cfg.Workflow}

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

func ExecuteWithExecutor(cfg RunConfig, exec CommandExecutor) error {
	args := BuildArgs(cfg)

	fmt.Println()
	fmt.Println("Running command:")
	fmt.Println("  " + FormatCommand(args))
	fmt.Println()

	if err := exec.Execute("gh", args...); err != nil {
		return fmt.Errorf("gh workflow run failed: %w", err)
	}

	if cfg.Watch {
		return watchLatestRunWithExecutor(cfg.Workflow, exec)
	}

	return nil
}

func watchLatestRun(workflow string) error {
	return watchLatestRunWithExecutor(workflow, executor)
}

func watchLatestRunWithExecutor(_ string, exec CommandExecutor) error {
	fmt.Println()
	fmt.Println("Watching run...")
	fmt.Println()

	return exec.Execute("gh", "run", "watch")
}

// DryRun prints the command that would be executed without running it.
func DryRun(cfg RunConfig) string {
	args := BuildArgs(cfg)
	return FormatCommand(args)
}

// ExecuteAndGetRunID runs the workflow and returns the run ID for watching.
// This polls the API shortly after dispatch to find the triggered run.
func ExecuteAndGetRunID(cfg RunConfig, client GitHubClient) (int64, error) {
	return ExecuteAndGetRunIDWithExecutor(cfg, client, executor)
}

func ExecuteAndGetRunIDWithExecutor(cfg RunConfig, client GitHubClient, exec CommandExecutor) (int64, error) {
	args := BuildArgs(cfg)

	fmt.Println()
	fmt.Println("Running command:")
	fmt.Println("  " + FormatCommand(args))
	fmt.Println()

	if err := exec.Execute("gh", args...); err != nil {
		return 0, fmt.Errorf("gh workflow run failed: %w", err)
	}

	run, err := client.GetLatestRun(cfg.Workflow)
	if err != nil {
		return 0, fmt.Errorf("failed to get run ID: %w", err)
	}

	if run == nil {
		return 0, fmt.Errorf("no run found for workflow: %s", cfg.Workflow)
	}

	return run.ID, nil
}
