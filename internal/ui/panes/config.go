package panes

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-wfd/internal/ui"
	"github.com/kyleking/gh-wfd/internal/workflow"
)

// ConfigModel manages the configuration pane.
type ConfigModel struct {
	workflow   *workflow.WorkflowFile
	branch     string
	inputs     map[string]string
	inputOrder []string
	watchRun   bool
	focused    bool
	width      int
	height     int
}

// NewConfigModel creates a new config pane model.
func NewConfigModel() ConfigModel {
	return ConfigModel{
		inputs: make(map[string]string),
	}
}

// SetWorkflow updates the current workflow.
func (m *ConfigModel) SetWorkflow(wf *workflow.WorkflowFile) {
	m.workflow = wf
	m.inputs = make(map[string]string)
	m.inputOrder = nil

	if wf != nil {
		wfInputs := wf.GetInputs()
		for name, input := range wfInputs {
			m.inputs[name] = input.Default
			m.inputOrder = append(m.inputOrder, name)
		}
		sort.Strings(m.inputOrder)
	}
}

// SetBranch updates the current branch.
func (m *ConfigModel) SetBranch(branch string) {
	m.branch = branch
}

// SetInput updates a specific input value.
func (m *ConfigModel) SetInput(name, value string) {
	m.inputs[name] = value
}

// SetInputs updates all input values.
func (m *ConfigModel) SetInputs(inputs map[string]string) {
	if inputs == nil {
		return
	}
	for k, v := range inputs {
		m.inputs[k] = v
	}
}

// SetWatchRun updates the watch run flag.
func (m *ConfigModel) SetWatchRun(watch bool) {
	m.watchRun = watch
}

// ToggleWatchRun toggles the watch run flag.
func (m *ConfigModel) ToggleWatchRun() {
	m.watchRun = !m.watchRun
}

// SetSize updates the pane dimensions.
func (m *ConfigModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused updates the focus state.
func (m *ConfigModel) SetFocused(focused bool) {
	m.focused = focused
}

// Update handles messages for the config pane.
func (m ConfigModel) Update(msg tea.Msg) (ConfigModel, tea.Cmd) {
	return m, nil
}

// View renders the config pane.
func (m ConfigModel) View() string {
	style := ui.PaneStyle(m.width, m.height, m.focused)

	var content strings.Builder
	content.WriteString(ui.TitleStyle.Render("Configuration"))
	content.WriteString("\n\n")

	if m.workflow == nil {
		content.WriteString(ui.SubtitleStyle.Render("No workflow selected"))
		return style.Render(content.String())
	}

	wfName := m.workflow.Name
	if wfName == "" {
		wfName = m.workflow.Filename
	}
	content.WriteString(fmt.Sprintf("Workflow: %s\n", wfName))
	content.WriteString(fmt.Sprintf("File: %s\n", m.workflow.Filename))

	branch := m.branch
	if branch == "" {
		branch = "(not set)"
	}
	content.WriteString(fmt.Sprintf("Branch: [b] %s", branch))

	if m.watchRun {
		content.WriteString("  [w] watch: on")
	} else {
		content.WriteString("  [w] watch: off")
	}
	content.WriteString("\n")

	if len(m.inputOrder) > 0 {
		content.WriteString("\nInputs:\n")
		wfInputs := m.workflow.GetInputs()

		for i, name := range m.inputOrder {
			if i >= 9 {
				break
			}
			input := wfInputs[name]
			val := m.inputs[name]

			modified := ""
			if val != input.Default {
				modified = "*"
			}

			displayVal := val
			if displayVal == "" {
				displayVal = "(empty)"
			}

			typeHint := input.InputType()
			if typeHint == "choice" && len(input.Options) > 0 {
				typeHint = strings.Join(input.Options, "/")
			}

			required := ""
			if input.Required {
				required = "!"
			}

			content.WriteString(fmt.Sprintf("  [%d] %s%s%s: %s (%s)\n",
				i+1, name, required, modified, displayVal, typeHint))

			if input.Description != "" {
				content.WriteString(fmt.Sprintf("      %s\n", ui.SubtitleStyle.Render(input.Description)))
			}
		}
	}

	helpLine := "\n" + ui.HelpStyle.Render("[Tab] pane  [Enter] run  [b] branch  [1-9] input  [w] watch  [q] quit")
	content.WriteString(helpLine)

	return style.Render(content.String())
}

// GetInputNames returns the ordered list of input names.
func (m ConfigModel) GetInputNames() []string {
	return m.inputOrder
}

// GetInputValue returns the current value for an input.
func (m ConfigModel) GetInputValue(name string) string {
	return m.inputs[name]
}

// GetAllInputs returns all current input values.
func (m ConfigModel) GetAllInputs() map[string]string {
	result := make(map[string]string, len(m.inputs))
	for k, v := range m.inputs {
		result[k] = v
	}
	return result
}

// Branch returns the current branch.
func (m ConfigModel) Branch() string {
	return m.branch
}

// WatchRun returns the watch run flag.
func (m ConfigModel) WatchRun() bool {
	return m.watchRun
}

// Workflow returns the current workflow.
func (m ConfigModel) Workflow() *workflow.WorkflowFile {
	return m.workflow
}

// BuildCommand returns the gh workflow run command.
func (m ConfigModel) BuildCommand() []string {
	if m.workflow == nil {
		return nil
	}

	args := []string{"workflow", "run", m.workflow.Filename}

	if m.branch != "" {
		args = append(args, "--ref", m.branch)
	}

	for _, name := range m.inputOrder {
		val := m.inputs[name]
		if val != "" {
			args = append(args, "-f", name+"="+val)
		}
	}

	return args
}
