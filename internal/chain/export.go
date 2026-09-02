package chain

import (
	"fmt"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
)

// ExportAsBash generates a bash script from a chain definition.
// The script is lossy: it does not include wait conditions or failure handling.
func ExportAsBash(chainName string, chain *config.Chain, variables map[string]string, branch string) string {
	var sb strings.Builder

	sb.WriteString("#!/bin/bash\n")
	fmt.Fprintf(&sb, "# Chain: %s\n", chainName)

	if chain.Description != "" {
		fmt.Fprintf(&sb, "# %s\n", chain.Description)
	}

	sb.WriteString("#\n")
	sb.WriteString("# WARNING: This is a simplified export.\n")
	sb.WriteString("# Wait conditions and failure handling are not included.\n")
	sb.WriteString("# Steps are executed sequentially without monitoring.\n")
	sb.WriteString("\n")
	sb.WriteString("set -e\n")
	sb.WriteString("\n")

	if len(variables) > 0 {
		sb.WriteString("# Variables:\n")

		for k, v := range variables {
			fmt.Fprintf(&sb, "#   %s = %s\n", k, v)
		}

		sb.WriteString("\n")
	}

	for i, resolved := range ResolveSteps(chain, variables, branch) {
		step := chain.Steps[i]
		fmt.Fprintf(&sb, "# Step %d: %s\n", i+1, step.Workflow)

		switch step.WaitFor {
		case config.WaitSuccess:
			sb.WriteString("# (original: wait for success)\n")
		case config.WaitCompletion:
			sb.WriteString("# (original: wait for completion)\n")
		case config.WaitNone:
			sb.WriteString("# (original: no wait)\n")
		}

		if resolved.Err != nil {
			fmt.Fprintf(&sb, "# ERROR: failed to interpolate inputs for step %d (%s): %v\n\n",
				i+1, step.Workflow, resolved.Err)

			continue
		}

		if step.Source == config.SourceExisting {
			fmt.Fprintf(&sb, "# SKIPPED: source: existing adopts a run already going on the"+
				" branch, which a script cannot express\n\n")

			continue
		}

		sb.WriteString(resolved.Command)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// ResolvedStep is one chain step with its templates interpolated and the gh
// command it dispatches. Building it once is what keeps the preview modal, the
// bash export, and the command list a chain records in history agreeing with
// what ChainExecutor actually runs.
type ResolvedStep struct {
	Inputs map[string]string
	// Err is the interpolation failure, if the step had one. A preview renders
	// it and carries on; only the executor treats it as fatal.
	Err      error
	Workflow string
	// Command is the gh dispatch command a dispatch-source step runs. Empty
	// for a source: existing step, which adopts a run instead of starting one.
	Command string
	Source  config.SourceKind
}

// ResolveSteps interpolates every step's inputs in order, so a step's
// `previous` names the step directly above it.
func ResolveSteps(chain *config.Chain, variables map[string]string, branch string) []ResolvedStep {
	resolved := make([]ResolvedStep, len(chain.Steps))

	ctx := &InterpolationContext{
		Var:   variables,
		Steps: make(map[int]*StepResult),
	}

	for i, step := range chain.Steps {
		if i > 0 {
			ctx.Previous = ctx.Steps[i-1]
		}

		inputs, err := InterpolateInputs(step.Inputs, ctx)

		command := ""
		if step.Source != config.SourceExisting {
			cfg := runner.RunConfig{
				Workflow: step.Workflow,
				Branch:   branch,
				Inputs:   inputs,
			}
			command = runner.FormatCommand(runner.BuildArgs(cfg))
		}

		resolved[i] = ResolvedStep{
			Workflow: step.Workflow,
			Inputs:   inputs,
			Command:  command,
			Source:   step.Source,
			Err:      err,
		}

		ctx.Steps[i] = &StepResult{
			Workflow: step.Workflow,
			Inputs:   inputs,
		}
	}

	return resolved
}
