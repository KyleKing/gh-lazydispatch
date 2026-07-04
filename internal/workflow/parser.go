package workflow

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/kyleking/gh-lazydispatch/internal/rule"
)

// workflowDispatchTrigger is the GitHub Actions trigger name that makes a workflow dispatchable.
const workflowDispatchTrigger = "workflow_dispatch"

// Parse parses workflow YAML content into a File struct.
func Parse(data []byte) (File, error) {
	var raw rawWorkflow
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return File{}, fmt.Errorf("parsing workflow YAML: %w", err)
	}

	wf := File{
		Name: raw.Name,
	}

	if raw.On.Dispatch != nil {
		wf.On.Dispatch = raw.On.Dispatch
	}

	inputComments, err := parseInputComments(data)
	if err != nil {
		return wf, err
	}

	if wf.On.Dispatch != nil && wf.On.Dispatch.Inputs != nil {
		for name, input := range wf.On.Dispatch.Inputs {
			if comments, ok := inputComments[name]; ok {
				rules, err := rule.ParseValidationComments(comments)
				if err != nil {
					continue
				}

				input.ValidationRules = rules
				wf.On.Dispatch.Inputs[name] = input
			}
		}
	}

	return wf, nil
}

// rawWorkflow handles the flexible "on" field parsing.
type rawWorkflow struct {
	On   rawOnTrigger `yaml:"on"`
	Name string       `yaml:"name"`
}

// rawOnTrigger handles "on" being either a string, list, or map.
type rawOnTrigger struct {
	Dispatch *Dispatch
}

func (t *rawOnTrigger) UnmarshalYAML(node *yaml.Node) error {
	switch node.Kind {
	case yaml.ScalarNode:
		if node.Value == workflowDispatchTrigger {
			t.Dispatch = &Dispatch{}
		}
	case yaml.SequenceNode:
		var triggers []string
		if err := node.Decode(&triggers); err == nil {
			for _, trigger := range triggers {
				if trigger == workflowDispatchTrigger {
					t.Dispatch = &Dispatch{}
					break
				}
			}
		}
	case yaml.MappingNode:
		var m struct {
			//nolint:tagliatelle // matches GitHub Actions workflow YAML schema
			Dispatch *Dispatch `yaml:"workflow_dispatch"`
		}

		if err := node.Decode(&m); err != nil {
			return fmt.Errorf("decoding workflow \"on\" trigger: %w", err)
		}

		t.Dispatch = m.Dispatch
	case yaml.DocumentNode, yaml.AliasNode:
		// not valid shapes for the "on" field; nothing to decode
	}

	return nil
}

// parseInputComments extracts comments from workflow input definitions.
// Returns a map of input name to associated comments.
func parseInputComments(data []byte) (map[string][]string, error) {
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("parsing workflow YAML for comments: %w", err)
	}

	result := make(map[string][]string)

	inputsNode := findInputsNode(&root)
	if inputsNode == nil {
		return result, nil
	}

	for i := 0; i < len(inputsNode.Content)-1; i += 2 {
		keyNode := inputsNode.Content[i]
		valueNode := inputsNode.Content[i+1]

		comments := commentsForInput(keyNode, valueNode)
		if len(comments) > 0 {
			result[keyNode.Value] = comments
		}
	}

	return result, nil
}

// commentsForInput collects all comments attached to a single workflow input:
// the input key's own head/line comments, plus any on its mapping properties.
func commentsForInput(keyNode, valueNode *yaml.Node) []string {
	var comments []string

	comments = append(comments, nodeComments(keyNode)...)

	if valueNode.Kind == yaml.MappingNode {
		for j := 0; j < len(valueNode.Content)-1; j += 2 {
			comments = append(comments, nodeComments(valueNode.Content[j])...)
		}
	}

	return comments
}

// nodeComments returns a node's head and line comments, split into individual lines.
func nodeComments(node *yaml.Node) []string {
	var comments []string

	if node.HeadComment != "" {
		comments = append(comments, splitCommentLines(node.HeadComment)...)
	}

	if node.LineComment != "" {
		comments = append(comments, splitCommentLines(node.LineComment)...)
	}

	return comments
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

		if key.Value == workflowDispatchTrigger && value.Kind == yaml.MappingNode {
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
