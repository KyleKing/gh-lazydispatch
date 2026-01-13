package workflow

// WorkflowFile represents a parsed GitHub Actions workflow file.
type WorkflowFile struct {
	Name     string    `yaml:"name"`
	Filename string    `yaml:"-"`
	On       OnTrigger `yaml:"on"`
}

// OnTrigger represents the "on" field which can trigger workflows.
type OnTrigger struct {
	WorkflowDispatch *WorkflowDispatch `yaml:"workflow_dispatch"`
}

// WorkflowDispatch represents the workflow_dispatch trigger configuration.
type WorkflowDispatch struct {
	Inputs map[string]WorkflowInput `yaml:"inputs"`
}

// WorkflowInput represents a single input definition for workflow_dispatch.
type WorkflowInput struct {
	Description string   `yaml:"description"`
	Required    bool     `yaml:"required"`
	Default     string   `yaml:"default"`
	Type        string   `yaml:"type"`
	Options     []string `yaml:"options"`
}

// InputType returns the normalized input type, defaulting to "string".
func (i WorkflowInput) InputType() string {
	if i.Type == "" {
		return "string"
	}
	return i.Type
}

// IsDispatchable returns true if the workflow has workflow_dispatch trigger.
func (w WorkflowFile) IsDispatchable() bool {
	return w.On.WorkflowDispatch != nil
}

// GetInputs returns the workflow inputs, or empty map if none.
func (w WorkflowFile) GetInputs() map[string]WorkflowInput {
	if w.On.WorkflowDispatch == nil || w.On.WorkflowDispatch.Inputs == nil {
		return make(map[string]WorkflowInput)
	}
	return w.On.WorkflowDispatch.Inputs
}
