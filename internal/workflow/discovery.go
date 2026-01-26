// Package workflow provides discovery and parsing of GitHub Actions workflow files.
package workflow

import (
	"os"
	"path/filepath"
	"sort"
)

// Discover finds all workflow files in the .github/workflows directory
// and returns only those with workflow_dispatch triggers.
func Discover(repoRoot string) ([]WorkflowFile, error) {
	workflowDir := filepath.Join(repoRoot, ".github", "workflows")

	patterns := []string{
		filepath.Join(workflowDir, "*.yml"),
		filepath.Join(workflowDir, "*.yaml"),
	}

	var files []string

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, err
		}

		files = append(files, matches...)
	}

	var workflows []WorkflowFile

	for _, file := range files {
		wf, err := parseWorkflowFile(file)
		if err != nil {
			continue
		}

		if wf.IsDispatchable() {
			workflows = append(workflows, wf)
		}
	}

	sort.Slice(workflows, func(i, j int) bool {
		return workflows[i].Filename < workflows[j].Filename
	})

	return workflows, nil
}

func parseWorkflowFile(path string) (WorkflowFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return WorkflowFile{}, err
	}

	wf, err := Parse(data)
	if err != nil {
		return WorkflowFile{}, err
	}

	wf.Filename = filepath.Base(path)

	return wf, nil
}
