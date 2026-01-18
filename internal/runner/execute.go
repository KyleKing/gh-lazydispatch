package runner

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kyleking/lazydispatch/internal/github"
)

// RunConfig holds the configuration for running a workflow.
type RunConfig struct {
	Workflow string
	Branch   string
	Inputs   map[string]string
	Watch    bool
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

// Execute runs the workflow using gh CLI.
// It prints the command being run (like lazygit) then executes it.
func Execute(cfg RunConfig) error {
	args := BuildArgs(cfg)

	fmt.Println()
	fmt.Println("Running command:")
	fmt.Println("  " + FormatCommand(args))
	fmt.Println()

	cmd := exec.Command("gh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gh workflow run failed: %w", err)
	}

	if cfg.Watch {
		return watchLatestRun(cfg.Workflow)
	}

	return nil
}

func watchLatestRun(workflow string) error {
	fmt.Println()
	fmt.Println("Watching run...")
	fmt.Println()

	cmd := exec.Command("gh", "run", "watch")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	return cmd.Run()
}

// DryRun prints the command that would be executed without running it.
func DryRun(cfg RunConfig) string {
	args := BuildArgs(cfg)
	return FormatCommand(args)
}

// ExecuteAndGetRunID runs the workflow and returns the run ID for watching.
// This polls the API shortly after dispatch to find the triggered run.
func ExecuteAndGetRunID(cfg RunConfig, client *github.Client) (int64, error) {
	args := BuildArgs(cfg)

	fmt.Println()
	fmt.Println("Running command:")
	fmt.Println("  " + FormatCommand(args))
	fmt.Println()

	cmd := exec.Command("gh", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
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
