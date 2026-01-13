package workflow

import (
	"gopkg.in/yaml.v3"
)

// Parse parses workflow YAML content into a WorkflowFile struct.
func Parse(data []byte) (WorkflowFile, error) {
	var raw rawWorkflow
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return WorkflowFile{}, err
	}

	wf := WorkflowFile{
		Name: raw.Name,
	}

	if raw.On.WorkflowDispatch != nil {
		wf.On.WorkflowDispatch = raw.On.WorkflowDispatch
	}

	return wf, nil
}

// rawWorkflow handles the flexible "on" field parsing.
type rawWorkflow struct {
	Name string       `yaml:"name"`
	On   rawOnTrigger `yaml:"on"`
}

// rawOnTrigger handles "on" being either a string, list, or map.
type rawOnTrigger struct {
	WorkflowDispatch *WorkflowDispatch
}

func (t *rawOnTrigger) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Value == "workflow_dispatch" {
			t.WorkflowDispatch = &WorkflowDispatch{}
		}
	case yaml.SequenceNode:
		var triggers []string
		if err := node.Decode(&triggers); err == nil {
			for _, trigger := range triggers {
				if trigger == "workflow_dispatch" {
					t.WorkflowDispatch = &WorkflowDispatch{}
					break
				}
			}
		}
	case yaml.MappingNode:
		var m struct {
			WorkflowDispatch *WorkflowDispatch `yaml:"workflow_dispatch"`
		}
		if err := node.Decode(&m); err != nil {
			return err
		}
		t.WorkflowDispatch = m.WorkflowDispatch
	}
	return nil
}
