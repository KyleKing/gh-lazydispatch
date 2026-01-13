package main

import (
	"fmt"
	"os"

	"github.com/kyleking/gh-workflow-runner/internal/workflow"
)

func main() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting current directory: %v\n", err)
		os.Exit(1)
	}

	workflows, err := workflow.Discover(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error discovering workflows: %v\n", err)
		os.Exit(1)
	}

	if len(workflows) == 0 {
		fmt.Println("No dispatchable workflows found in .github/workflows/")
		os.Exit(0)
	}

	fmt.Printf("Found %d dispatchable workflow(s):\n", len(workflows))
	for _, wf := range workflows {
		name := wf.Name
		if name == "" {
			name = "(unnamed)"
		}
		fmt.Printf("  - %s (%s)\n", wf.Filename, name)
		for key, input := range wf.GetInputs() {
			fmt.Printf("      %s: %s (type: %s)\n", key, input.Description, input.InputType())
		}
	}
}
