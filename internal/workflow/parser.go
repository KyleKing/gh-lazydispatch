package workflow

import (
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/rule"
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

	inputComments, err := parseInputComments(data)
	if err != nil {
		return wf, err
	}

	if wf.On.WorkflowDispatch != nil && wf.On.WorkflowDispatch.Inputs != nil {
		for name, input := range wf.On.WorkflowDispatch.Inputs {
			if comments, ok := inputComments[name]; ok {
				rules, err := rule.ParseValidationComments(comments)
				if err != nil {
					continue
				}

				input.ValidationRules = rules
				wf.On.WorkflowDispatch.Inputs[name] = input
			}
		}
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

// parseInputComments extracts comments from workflow input definitions.
// Returns a map of input name to associated comments.
func parseInputComments(data []byte) (map[string][]string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	result := make(map[string][]string)

	inputsNode := findInputsNode(&root)
	if inputsNode == nil {
		return result, nil
	}

	for i := 0; i < len(inputsNode.Content)-1; i += 2 {
		keyNode := inputsNode.Content[i]
		valueNode := inputsNode.Content[i+1]

		inputName := keyNode.Value

		var comments []string

		if keyNode.HeadComment != "" {
			comments = append(comments, splitCommentLines(keyNode.HeadComment)...)
		}

		if keyNode.LineComment != "" {
			comments = append(comments, splitCommentLines(keyNode.LineComment)...)
		}

		if valueNode.Kind == yaml.MappingNode {
			for j := 0; j < len(valueNode.Content)-1; j += 2 {
				propNode := valueNode.Content[j]
				if propNode.HeadComment != "" {
					comments = append(comments, splitCommentLines(propNode.HeadComment)...)
				}

				if propNode.LineComment != "" {
					comments = append(comments, splitCommentLines(propNode.LineComment)...)
				}
			}
		}

		if len(comments) > 0 {
			result[inputName] = comments
		}
	}

	return result, nil
}

func findInputsNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}

	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		return findInputsNode(node.Content[0])
	}

	if node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]

		if key.Value == "on" {
			return findInputsInOnNode(value)
		}
	}

	return nil
}

func findInputsInOnNode(node *yaml.Node) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i]
		value := node.Content[i+1]

		if key.Value == "workflow_dispatch" && value.Kind == yaml.MappingNode {
			for j := 0; j < len(value.Content)-1; j += 2 {
				dispatchKey := value.Content[j]
				dispatchValue := value.Content[j+1]

				if dispatchKey.Value == "inputs" && dispatchValue.Kind == yaml.MappingNode {
					return dispatchValue
				}
			}
		}
	}

	return nil
}

func splitCommentLines(comment string) []string {
	var lines []string

	for _, line := range strings.Split(comment, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}

	return lines
}
