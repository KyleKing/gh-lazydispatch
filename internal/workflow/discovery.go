// Package workflow provides discovery and parsing of GitHub Actions workflow files.
package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// ParseFailure names a workflow file that could not be read or parsed. A
// repository holding only these looks identical to an empty one unless the
// failures are reported alongside the workflows.
type ParseFailure struct {
	Err      error
	Filename string
}

// Discover finds all workflow files in the .github/workflows directory and
// returns those with workflow_dispatch triggers, plus the files it could not
// parse. Only an unusable workflow directory is an error.
func Discover(repoRoot string) ([]File, []ParseFailure, error) {
	workflowDir := filepath.Join(repoRoot, ".github", "workflows")

	patterns := []string{
		filepath.Join(workflowDir, "*.yml"),
		filepath.Join(workflowDir, "*.yaml"),
	}

	var files []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, nil, fmt.Errorf("globbing workflow files with pattern %q: %w", pattern, err)
		}

		files = append(files, matches...)
	}

	var (
		workflows []File
		failures  []ParseFailure
	)

	for _, file := range files {
		wf, err := parseWorkflowFile(file)
		if err != nil {
			failures = append(failures, ParseFailure{Filename: filepath.Base(file), Err: err})
			continue
		}

		if wf.IsDispatchable() {
			workflows = append(workflows, wf)
		}
	}

	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].Filename < workflows[j].Filename
	})
	sort.Slice(failures, func(i, j int) bool {
		return failures[i].Filename < failures[j].Filename
	})

	return workflows, failures, nil
}

func parseWorkflowFile(path string) (File, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path from walking repo's .github/workflows dir
	if err != nil {
		return File{}, fmt.Errorf("reading workflow file %s: %w", path, err)
	}

	wf, err := Parse(data)
	if err != nil {
		return File{}, err
	}

	wf.Filename = filepath.Base(path)

	return wf, nil
}
