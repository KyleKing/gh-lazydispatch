// Package git provides Git operations for branch discovery and management.
package git

import (
	"context"
	"fmt"
	"os/exec"
	"sort"
	"strings"
	"time"
)

const (
	fetchBranchesTimeout = 5 * time.Second
	gitCommandTimeout    = 2 * time.Second
)

const (
	branchDevelop = "develop"
	branchMain    = "main"
	branchMaster  = "master"
)

// CommandRunner executes git commands and returns their output.
type CommandRunner interface {
	RunCommand(ctx context.Context, args ...string) ([]byte, error)
}

type defaultCommandRunner struct{}

func (*defaultCommandRunner) RunCommand(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", args...) // #nosec G204 -- fixed git binary; args are built internally

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running git %s: %w", strings.Join(args, " "), err)
	}

	return out, nil
}

var runner CommandRunner = &defaultCommandRunner{}

// FetchBranches retrieves all branches from the git repository.
// Returns both local and remote-tracking branches, with "origin/" prefix stripped.
// Falls back to default branches on error.
func FetchBranches(ctx context.Context) ([]string, error) {
	return fetchBranchesWithRunner(ctx, runner)
}

func fetchBranchesWithRunner(ctx context.Context, r CommandRunner) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchBranchesTimeout)
	defer cancel()

	output, err := r.RunCommand(ctx, "branch", "-r", "--list")
	if err != nil {
		return DefaultBranches(), err
	}

	branches := _parseBranches(string(output))
	branches = _deduplicateBranches(branches)
	sort.Strings(branches)

	if len(branches) == 0 {
		return DefaultBranches(), nil
	}

	return branches, nil
}

// GetCurrentBranch returns the currently checked out branch.
// Returns empty string if unable to determine (e.g., detached HEAD).
func GetCurrentBranch(ctx context.Context) string {
	return getCurrentBranchWithRunner(ctx, runner)
}

func getCurrentBranchWithRunner(ctx context.Context, r CommandRunner) string {
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()

	output, err := r.RunCommand(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return ""
	}

	branch := strings.TrimSpace(string(output))
	if branch == "HEAD" {
		return ""
	}

	return branch
}

// GetDefaultBranch attempts to determine the repository's default branch.
// Returns empty string if unable to determine.
func GetDefaultBranch(ctx context.Context) string {
	return getDefaultBranchWithRunner(ctx, runner)
}

func getDefaultBranchWithRunner(ctx context.Context, r CommandRunner) string {
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()

	output, err := r.RunCommand(ctx, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err != nil {
		return ""
	}

	ref := strings.TrimSpace(string(output))
	branch := strings.TrimPrefix(ref, "refs/remotes/origin/")

	return branch
}

// DefaultBranches returns the fallback branch list used when none can be fetched.
func DefaultBranches() []string {
	return []string{branchMain, branchMaster, branchDevelop}
}

func _parseBranches(output string) []string {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	branches := make([]string, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "origin/HEAD") {
			continue
		}

		branch := strings.TrimPrefix(line, "origin/")
		branches = append(branches, branch)
	}

	return branches
}

func _deduplicateBranches(branches []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(branches))

	for _, branch := range branches {
		if !seen[branch] {
			seen[branch] = true

			result = append(result, branch)
		}
	}

	return result
}
