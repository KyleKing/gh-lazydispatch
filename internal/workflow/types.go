package workflow

import "github.com/kyleking/gh-lazydispatch/internal/rule"

// File represents a parsed GitHub Actions workflow file.
type File struct {
	On       OnTrigger `yaml:"on"`
	Name     string    `yaml:"name"`
	Filename string    `yaml:"-"`
}

// OnTrigger represents the "on" field which can trigger workflows.
type OnTrigger struct {
	//nolint:tagliatelle // matches GitHub Actions workflow YAML schema
	Dispatch *Dispatch `yaml:"workflow_dispatch"`
}

// Dispatch represents the workflow_dispatch trigger configuration.
type Dispatch struct {
	Inputs map[string]Input `yaml:"inputs"`
}

// Input represents a single input definition for workflow_dispatch.
type Input struct {
	Description     string                `yaml:"description"`
	Default         string                `yaml:"default"`
	Type            string                `yaml:"type"`
	Options         []string              `yaml:"options"`
	ValidationRules []rule.ValidationRule `yaml:"-"`
	Required        bool                  `yaml:"required"`
}

// InputType returns the normalized input type, defaulting to "string".
func (i Input) InputType() string {
	if i.Type == "" {
		return "string"
	}

	return i.Type
}

// IsDispatchable returns true if the workflow has workflow_dispatch trigger.
func (w File) IsDispatchable() bool {
	return w.On.Dispatch != nil
}

// GetInputs returns the workflow inputs, or empty map if none.
func (w File) GetInputs() map[string]Input {
	if w.On.Dispatch == nil || w.On.Dispatch.Inputs == nil {
		return make(map[string]Input)
	}

	return w.On.Dispatch.Inputs
}
