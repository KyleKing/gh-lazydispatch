package git

import (
	"context"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// FetchBranches retrieves all branches from the git repository.
// Returns both local and remote-tracking branches, with "origin/" prefix stripped.
// Falls back to default branches on error.
func FetchBranches(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "branch", "-r", "--list")
	output, err := cmd.Output()
	if err != nil {
		return _defaultBranches(), err
	}

	branches := _parseBranches(string(output))
	branches = _deduplicateBranches(branches)
	sort.Strings(branches)

	if len(branches) == 0 {
		return _defaultBranches(), nil
	}

	return branches, nil
}

// GetDefaultBranch attempts to determine the repository's default branch.
// Returns empty string if unable to determine.
func GetDefaultBranch(ctx context.Context) string {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", "symbolic-ref", "refs/remotes/origin/HEAD")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}

	ref := strings.TrimSpace(string(output))
	branch := strings.TrimPrefix(ref, "refs/remotes/origin/")
	return branch
}

func _defaultBranches() []string {
	return []string{"main", "master", "develop"}
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
