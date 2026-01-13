package workflow

import (
	"testing"
)

func TestParse_WithInputs(t *testing.T) {
	data := []byte(`
name: Deploy
on:
  workflow_dispatch:
    inputs:
      environment:
        description: 'Target environment'
        required: true
        type: choice
        options:
          - production
          - staging
        default: staging
      dry_run:
        description: 'Dry run mode'
        type: boolean
        default: 'false'
`)

	wf, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if wf.Name != "Deploy" {
		t.Errorf("expected name 'Deploy', got %q", wf.Name)
	}

	if !wf.IsDispatchable() {
		t.Error("expected workflow to be dispatchable")
	}

	inputs := wf.GetInputs()
	if len(inputs) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(inputs))
	}

	env, ok := inputs["environment"]
	if !ok {
		t.Fatal("expected 'environment' input")
	}
	if env.InputType() != "choice" {
		t.Errorf("expected type 'choice', got %q", env.InputType())
	}
	if !env.Required {
		t.Error("expected 'environment' to be required")
	}
	if len(env.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(env.Options))
	}
	if env.Default != "staging" {
		t.Errorf("expected default 'staging', got %q", env.Default)
	}
}

func TestParse_SimpleDispatch(t *testing.T) {
	data := []byte(`
name: Simple
on: workflow_dispatch
`)

	wf, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if !wf.IsDispatchable() {
		t.Error("expected workflow to be dispatchable")
	}

	inputs := wf.GetInputs()
	if len(inputs) != 0 {
		t.Errorf("expected 0 inputs, got %d", len(inputs))
	}
}

func TestParse_NotDispatchable(t *testing.T) {
	data := []byte(`
name: CI
on:
  push:
    branches: [main]
  pull_request:
`)

	wf, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if wf.IsDispatchable() {
		t.Error("expected workflow to NOT be dispatchable")
	}
}

func TestParse_OnAsList(t *testing.T) {
	data := []byte(`
name: Multi Trigger
on: [push, pull_request]
`)

	wf, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if wf.IsDispatchable() {
		t.Error("expected workflow to NOT be dispatchable")
	}
}

func TestParse_NoName(t *testing.T) {
	data := []byte(`
on:
  workflow_dispatch:
    inputs:
      message:
        type: string
`)

	wf, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if wf.Name != "" {
		t.Errorf("expected empty name, got %q", wf.Name)
	}

	if !wf.IsDispatchable() {
		t.Error("expected workflow to be dispatchable")
	}
}

func TestWorkflowInput_InputType_Default(t *testing.T) {
	input := WorkflowInput{}
	if input.InputType() != "string" {
		t.Errorf("expected default type 'string', got %q", input.InputType())
	}
}

func TestWorkflowInput_InputType_Explicit(t *testing.T) {
	input := WorkflowInput{Type: "boolean"}
	if input.InputType() != "boolean" {
		t.Errorf("expected type 'boolean', got %q", input.InputType())
	}
}
