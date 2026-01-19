package chain_test

import (
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
)

func TestInterpolate_VarInputs(t *testing.T) {
	ctx := &chain.InterpolationContext{
		Var: map[string]string{
			"version": "1.0.0",
			"env":     "production",
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"simple key", "{{ var.version }}", "1.0.0"},
		{"with spaces", "{{  var.env  }}", "production"},
		{"in text", "Deploy version {{ var.version }} to {{ var.env }}", "Deploy version 1.0.0 to production"},
		{"missing key", "{{ var.missing }}", "{{ var.missing }}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := chain.Interpolate(tt.template, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestInterpolate_PreviousStep(t *testing.T) {
	ctx := &chain.InterpolationContext{
		Previous: &chain.StepResult{
			Workflow: "build.yml",
			Inputs: map[string]string{
				"version": "2.0.0",
				"tag":     "v2.0.0",
			},
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"simple key", "{{ previous.inputs.version }}", "2.0.0"},
		{"another key", "{{ previous.inputs.tag }}", "v2.0.0"},
		{"missing key", "{{ previous.inputs.missing }}", "{{ previous.inputs.missing }}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := chain.Interpolate(tt.template, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestInterpolate_StepsByIndex(t *testing.T) {
	ctx := &chain.InterpolationContext{
		Steps: map[int]*chain.StepResult{
			0: {
				Workflow: "step0.yml",
				Inputs:   map[string]string{"key": "value0"},
			},
			1: {
				Workflow: "step1.yml",
				Inputs:   map[string]string{"key": "value1"},
			},
		},
	}

	tests := []struct {
		name     string
		template string
		expected string
	}{
		{"step 0", "{{ steps.0.inputs.key }}", "value0"},
		{"step 1", "{{ steps.1.inputs.key }}", "value1"},
		{"missing step", "{{ steps.99.inputs.key }}", "{{ steps.99.inputs.key }}"},
		{"missing key", "{{ steps.0.inputs.missing }}", "{{ steps.0.inputs.missing }}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := chain.Interpolate(tt.template, ctx)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("got %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestInterpolate_MissingKey(t *testing.T) {
	ctx := &chain.InterpolationContext{
		Var: map[string]string{"key": "value"},
	}

	template := "{{ var.missing }}"
	result, err := chain.Interpolate(template, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != template {
		t.Errorf("expected original template for missing key, got %q", result)
	}
}

func TestInterpolate_NilContext(t *testing.T) {
	template := "{{ var.key }}"
	result, err := chain.Interpolate(template, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != template {
		t.Errorf("expected original template for nil context, got %q", result)
	}
}

func TestInterpolateInputs(t *testing.T) {
	ctx := &chain.InterpolationContext{
		Var: map[string]string{
			"version": "1.0.0",
			"env":     "prod",
		},
	}

	inputs := map[string]string{
		"version": "{{ var.version }}",
		"env":     "{{ var.env }}",
		"static":  "static-value",
	}

	result, err := chain.InterpolateInputs(inputs, ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]string{
		"version": "1.0.0",
		"env":     "prod",
		"static":  "static-value",
	}

	for k, v := range expected {
		if result[k] != v {
			t.Errorf("key %q: got %q, want %q", k, result[k], v)
		}
	}
}
