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
	sb.WriteString(fmt.Sprintf("# Chain: %s\n", chainName))
	if chain.Description != "" {
		sb.WriteString(fmt.Sprintf("# %s\n", chain.Description))
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
			sb.WriteString(fmt.Sprintf("#   %s = %s\n", k, v))
		}
		sb.WriteString("\n")
	}

	commands := resolveChainCommands(chain, variables, branch)

	for i, cmd := range commands {
		step := chain.Steps[i]
		sb.WriteString(fmt.Sprintf("# Step %d: %s\n", i+1, step.Workflow))

		switch step.WaitFor {
		case config.WaitSuccess:
			sb.WriteString("# (original: wait for success)\n")
		case config.WaitCompletion:
			sb.WriteString("# (original: wait for completion)\n")
		case config.WaitNone:
			sb.WriteString("# (original: no wait)\n")
		}

		sb.WriteString(cmd)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

func resolveChainCommands(chain *config.Chain, variables map[string]string, branch string) []string {
	commands := make([]string, len(chain.Steps))

	ctx := &InterpolationContext{
		Var:   variables,
		Steps: make(map[int]*StepResult),
	}

	for i, step := range chain.Steps {
		inputs, _ := InterpolateInputs(step.Inputs, ctx)

		cfg := runner.RunConfig{
			Workflow: step.Workflow,
			Branch:   branch,
			Inputs:   inputs,
		}
		args := runner.BuildArgs(cfg)
		commands[i] = runner.FormatCommand(args)

		ctx.Steps[i] = &StepResult{
			Workflow: step.Workflow,
			Inputs:   inputs,
		}
		if i > 0 {
			ctx.Previous = ctx.Steps[i-1]
		}
	}

	return commands
}
