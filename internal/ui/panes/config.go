package panes

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-wfd/internal/ui"
	"github.com/kyleking/gh-wfd/internal/workflow"
	"github.com/sahilm/fuzzy"
)

// ConfigModel manages the configuration pane.
type ConfigModel struct {
	workflow      *workflow.WorkflowFile
	branch        string
	inputs        map[string]string
	inputOrder    []string
	filteredOrder []string
	filterText    string
	watchRun      bool
	focused       bool
	width         int
	height        int

	selectedRow  int
	scrollOffset int
}

// NewConfigModel creates a new config pane model.
func NewConfigModel() ConfigModel {
	return ConfigModel{
		inputs:      make(map[string]string),
		selectedRow: -1,
	}
}

// SetWorkflow updates the current workflow.
func (m *ConfigModel) SetWorkflow(wf *workflow.WorkflowFile) {
	m.workflow = wf
	m.inputs = make(map[string]string)
	m.inputOrder = nil
	m.filteredOrder = nil
	m.filterText = ""
	m.selectedRow = -1
	m.scrollOffset = 0

	if wf != nil {
		wfInputs := wf.GetInputs()
		for name, input := range wfInputs {
			m.inputs[name] = input.Default
			m.inputOrder = append(m.inputOrder, name)
		}
		sort.Strings(m.inputOrder)
		m.filteredOrder = m.inputOrder
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

// SetFilter applies a fuzzy filter to the inputs.
func (m *ConfigModel) SetFilter(filter string) {
	m.filterText = filter
	if filter == "" {
		m.filteredOrder = m.inputOrder
	} else {
		matches := fuzzy.Find(filter, m.inputOrder)
		m.filteredOrder = make([]string, len(matches))
		for i, match := range matches {
			m.filteredOrder[i] = match.Str
		}
	}
	if m.selectedRow >= len(m.filteredOrder) {
		m.selectedRow = len(m.filteredOrder) - 1
	}
	m.scrollOffset = 0
}

// SelectUp moves selection up.
func (m *ConfigModel) SelectUp() {
	if m.selectedRow < 0 {
		m.selectedRow = 0
	} else if m.selectedRow > 0 {
		m.selectedRow--
	}
	m.adjustScroll()
}

// SelectDown moves selection down.
func (m *ConfigModel) SelectDown() {
	if m.selectedRow < 0 {
		m.selectedRow = 0
	} else if m.selectedRow < len(m.filteredOrder)-1 {
		m.selectedRow++
	}
	m.adjustScroll()
}

// ClearSelection deselects the current row.
func (m *ConfigModel) ClearSelection() {
	m.selectedRow = -1
}

// SelectedInput returns the currently selected input name.
func (m *ConfigModel) SelectedInput() string {
	if m.selectedRow < 0 || m.selectedRow >= len(m.filteredOrder) {
		return ""
	}
	return m.filteredOrder[m.selectedRow]
}

// HasSelection returns true if an input is selected.
func (m *ConfigModel) HasSelection() bool {
	return m.selectedRow >= 0 && m.selectedRow < len(m.filteredOrder)
}

// FilterText returns the current filter text.
func (m *ConfigModel) FilterText() string {
	return m.filterText
}

// ResetAllInputs resets all inputs to their default values.
func (m *ConfigModel) ResetAllInputs() {
	if m.workflow == nil {
		return
	}
	wfInputs := m.workflow.GetInputs()
	for name, input := range wfInputs {
		m.inputs[name] = input.Default
	}
}

func (m *ConfigModel) adjustScroll() {
	visibleRows := m.visibleRowCount()
	if m.selectedRow < m.scrollOffset {
		m.scrollOffset = m.selectedRow
	}
	if m.selectedRow >= m.scrollOffset+visibleRows {
		m.scrollOffset = m.selectedRow - visibleRows + 1
	}
}

func (m ConfigModel) visibleRowCount() int {
	return (m.height - 14)
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
		content.WriteString("\n\n")
		content.WriteString(ui.HelpStyle.Render("[Tab] pane  [q] quit"))
		return style.Render(content.String())
	}

	branch := m.branch
	if branch == "" {
		branch = "(not set)"
	}
	content.WriteString(ui.TitleStyle.Render("Branch"))
	content.WriteString(": ")
	content.WriteString(ui.HelpStyle.Render("[b]"))
	content.WriteString(" ")
	content.WriteString(branch)

	content.WriteString("    Watch: ")
	content.WriteString(ui.HelpStyle.Render("[w]"))
	content.WriteString(" ")
	if m.watchRun {
		content.WriteString("on")
	} else {
		content.WriteString("off")
	}
	content.WriteString("    ")
	content.WriteString(ui.HelpStyle.Render("[r]"))
	content.WriteString(" reset all")
	content.WriteString("\n")

	if m.filterText != "" {
		content.WriteString(ui.SubtitleStyle.Render("Filter: /" + m.filterText))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(m.renderTableHeader())
	content.WriteString("\n")
	content.WriteString(m.renderTableRows())

	content.WriteString("\n\n")
	content.WriteString(ui.SubtitleStyle.Render("Command:"))
	content.WriteString("\n")
	cliCmd := m.BuildCLIString()
	maxCmdWidth := m.width - 10
	if maxCmdWidth > 0 && len(cliCmd) > maxCmdWidth {
		cliCmd = "..." + cliCmd[len(cliCmd)-maxCmdWidth+3:]
	}
	content.WriteString(ui.CLIPreviewStyle.Render(cliCmd))
	content.WriteString(" ")
	content.WriteString(ui.HelpStyle.Render("[c]"))

	helpLine := "\n\n" + ui.HelpStyle.Render("[Tab] pane  [Enter] run  [j/k] select  [0-9] edit  [/] filter  [?] help  [q] quit")
	content.WriteString(helpLine)

	return style.Render(content.String())
}

func (m ConfigModel) renderTableHeader() string {
	return ui.TableHeaderStyle.Render(
		fmt.Sprintf(" %-2s  %-3s  %-15s  %-18s  %-15s", "#", "Req", "Name", "Value", "Default"),
	)
}

func (m ConfigModel) renderTableRows() string {
	var rows strings.Builder

	if m.workflow == nil {
		return ""
	}

	wfInputs := m.workflow.GetInputs()
	visibleRows := m.visibleRowCount()
	if visibleRows < 1 {
		visibleRows = 5
	}

	visibleStart := m.scrollOffset
	visibleEnd := m.scrollOffset + visibleRows
	if visibleEnd > len(m.filteredOrder) {
		visibleEnd = len(m.filteredOrder)
	}

	for i := visibleStart; i < visibleEnd; i++ {
		name := m.filteredOrder[i]
		input := wfInputs[name]
		val := m.inputs[name]

		numStr := " "
		displayIdx := i
		if displayIdx <= 9 {
			numStr = fmt.Sprintf("%d", displayIdx)
		}

		reqStr := " "
		if input.Required {
			reqStr = "x"
		}

		valueDisplay := val
		isSpecialValue := false
		if val == "" {
			valueDisplay = `("")`
			isSpecialValue = true
		}

		defaultDisplay := input.Default
		if defaultDisplay == "" {
			defaultDisplay = `("")`
		}

		isSelected := i == m.selectedRow
		isDimmed := val == input.Default

		displayName := name
		if len(displayName) > 15 {
			displayName = displayName[:12] + "..."
		}
		if len(valueDisplay) > 18 {
			valueDisplay = valueDisplay[:15] + "..."
		}
		if len(defaultDisplay) > 15 {
			defaultDisplay = defaultDisplay[:12] + "..."
		}

		indicator := "  "
		if isSelected {
			indicator = "> "
		}

		row := fmt.Sprintf("%s%-2s  %-3s  %-15s  %-18s  %-15s",
			indicator, numStr, reqStr, displayName, valueDisplay, defaultDisplay)

		var rowStyle = ui.TableRowStyle
		if isSelected {
			rowStyle = ui.TableSelectedStyle
		} else if isDimmed {
			rowStyle = ui.TableDefaultStyle
		} else if isSpecialValue {
			rowStyle = ui.TableItalicStyle
		}

		rows.WriteString(rowStyle.Render(row))
		if i < visibleEnd-1 {
			rows.WriteString("\n")
		}
	}

	if m.scrollOffset > 0 || visibleEnd < len(m.filteredOrder) {
		rows.WriteString("\n")
		scrollInfo := ""
		if m.scrollOffset > 0 {
			scrollInfo += "^"
		} else {
			scrollInfo += " "
		}
		scrollInfo += " "
		if visibleEnd < len(m.filteredOrder) {
			scrollInfo += "v"
		}
		rows.WriteString(ui.SubtitleStyle.Render(scrollInfo))
	}

	return rows.String()
}

// GetInputNames returns the ordered list of input names.
func (m ConfigModel) GetInputNames() []string {
	return m.inputOrder
}

// GetFilteredInputNames returns the filtered list of input names.
func (m ConfigModel) GetFilteredInputNames() []string {
	return m.filteredOrder
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

// BuildCommand returns the gh workflow run command as args.
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

// BuildCLIString returns the full CLI command string.
func (m ConfigModel) BuildCLIString() string {
	args := m.BuildCommand()
	if args == nil {
		return ""
	}
	return "gh " + strings.Join(args, " ")
}

// GetModifiedInputs returns inputs that differ from their defaults.
func (m ConfigModel) GetModifiedInputs() map[string]struct{ Current, Default string } {
	result := make(map[string]struct{ Current, Default string })
	if m.workflow == nil {
		return result
	}

	wfInputs := m.workflow.GetInputs()
	for name, input := range wfInputs {
		current := m.inputs[name]
		if current != input.Default {
			result[name] = struct{ Current, Default string }{current, input.Default}
		}
	}
	return result
}
