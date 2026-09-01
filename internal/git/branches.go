// Package git reads the refs a dispatch can target. Every read goes through
// aragonite's vcs layer, so a jj checkout answers the same questions a git one
// does.
package git

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/kyleking/aragonite/vcs"
)

const (
	fetchBranchesTimeout = 5 * time.Second
	gitCommandTimeout    = 2 * time.Second
)

// repoPath is the checkout every read runs against: the working directory,
// which is where the tool was started and where the workflows were found.
const repoPath = "."

// FetchBranches names the branches the remote holds, which are the refs a
// dispatch can target. It falls back to the conventional names when the remote
// answers with none.
func FetchBranches(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, fetchBranchesTimeout)
	defer cancel()

	branches, err := vcs.RemoteBranches(ctx, repoPath)
	if err != nil {
		return DefaultBranches(), fmt.Errorf("listing branches: %w", err)
	}

	if len(branches) == 0 {
		return DefaultBranches(), nil
	}

	return branches, nil
}

// GetCurrentBranch returns the checked out branch, empty on a detached HEAD.
func GetCurrentBranch(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()

	branch, err := vcs.GetOperations(repoPath).GetCurrentBranch(ctx, repoPath)
	if err != nil {
		return ""
	}

	return branch
}

// GetDefaultBranch returns the repository's default branch, empty when none can
// be determined.
func GetDefaultBranch(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, gitCommandTimeout)
	defer cancel()

	branch, _ := vcs.DefaultBranchName(ctx, repoPath)

	return branch
}

// DefaultBranches returns the fallback branch list used when none can be read.
// Callers append their own branch to it, so it hands back a copy.
func DefaultBranches() []string {
	return slices.Clone(vcs.DefaultBranchNames)
}
