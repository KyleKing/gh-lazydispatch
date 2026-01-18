package app

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/validation"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	statusBar := m.viewStatusBar()
	statusHeight := 0
	if statusBar != "" {
		statusHeight = 1
	}

	topHeight := (m.height - statusHeight) / 2
	bottomHeight := m.height - statusHeight - topHeight

	leftWidth := (m.width * 11) / 30
	rightWidth := m.width - leftWidth

	var leftPane string
	switch m.viewMode {
	case InputDetailMode:
		if m.getSelectedInputName() != "" {
			leftPane = m.viewInputDetailsPane(leftWidth, topHeight)
		} else {
			leftPane = m.viewWorkflowPane(leftWidth, topHeight)
		}
	case HistoryPreviewMode:
		leftPane = m.viewHistoryConfigPane(leftWidth, topHeight)
	default:
		leftPane = m.viewWorkflowPane(leftWidth, topHeight)
	}
	historyPane := m.viewHistoryPane(rightWidth, topHeight)
	configPane := m.viewConfigPane(m.width, bottomHeight)

	top := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, historyPane)
	var main string
	if statusBar != "" {
		main = lipgloss.JoinVertical(lipgloss.Left, top, configPane, statusBar)
	} else {
		main = lipgloss.JoinVertical(lipgloss.Left, top, configPane)
	}

	if m.modalStack.HasActive() {
		return m.modalStack.Render(main)
	}

	return main
}

func (m Model) viewStatusBar() string {
	if m.watcher == nil {
		return ""
	}
	runs := m.watcher.GetRuns()
	if len(runs) == 0 {
		return ""
	}
	return ui.HelpStyle.Render(modal.FormatStatusBar(runs) + " [l] view")
}

func (m Model) viewInputDetailsPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneWorkflows)

	selectedName := m.getSelectedInputName()
	if selectedName == "" {
		return m.viewWorkflowPane(width, height)
	}

	wf := m.workflows[m.selectedWorkflow]
	inputs := wf.GetInputs()
	input, ok := inputs[selectedName]
	if !ok {
		return m.viewWorkflowPane(width, height)
	}

	var content strings.Builder
	content.WriteString(ui.TitleStyle.Render("Input Details"))
	content.WriteString("\n\n")

	_renderInputHeader(&content, selectedName, input.Required)
	_renderInputType(&content, input.InputType())
	_renderInputOptions(&content, input.InputType(), input.Options)
	_renderInputDescription(&content, input.Description, width)
	_renderInputValues(&content, m.inputs[selectedName], input.Default)

	content.WriteString("\n\n")
	content.WriteString(ui.HelpStyle.Render("[Esc] back  [e] edit"))

	return style.Render(content.String())
}

func _renderInputHeader(content *strings.Builder, name string, required bool) {
	content.WriteString(ui.TitleStyle.Render(name))
	if required {
		content.WriteString(" ")
		content.WriteString(ui.SelectedStyle.Render("(required)"))
	}
	content.WriteString("\n\n")
}

func _renderInputType(content *strings.Builder, inputType string) {
	content.WriteString(ui.SubtitleStyle.Render("Type: "))
	content.WriteString(ui.NormalStyle.Render(inputType))
	content.WriteString("\n")
}

func _renderInputOptions(content *strings.Builder, inputType string, options []string) {
	if inputType != "choice" || len(options) == 0 {
		return
	}
	content.WriteString("\n")
	content.WriteString(ui.SubtitleStyle.Render("Options:"))
	content.WriteString("\n")
	for _, opt := range options {
		content.WriteString("  - ")
		content.WriteString(ui.NormalStyle.Render(opt))
		content.WriteString("\n")
	}
}

func _renderInputDescription(content *strings.Builder, description string, width int) {
	if description == "" {
		return
	}
	content.WriteString("\n")
	content.WriteString(ui.SubtitleStyle.Render("Description:"))
	content.WriteString("\n")
	wrapped := _wordWrap(description, width-8)
	content.WriteString(ui.NormalStyle.Render(wrapped))
	content.WriteString("\n")
}

func _renderInputValues(content *strings.Builder, current, defaultVal string) {
	content.WriteString("\n")
	content.WriteString(ui.SubtitleStyle.Render("Current: "))
	content.WriteString(ui.RenderEmptyValue(current))

	content.WriteString("\n")
	content.WriteString(ui.SubtitleStyle.Render("Default: "))
	content.WriteString(ui.RenderEmptyValue(defaultVal))
}

func (m Model) viewWorkflowPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneWorkflows)

	title := ui.TitleStyle.Render("Workflows")
	maxLineWidth := width - 8
	var content string

	allLine := "all"
	if m.selectedWorkflow == -1 {
		content += ui.SelectedStyle.Render("> " + allLine)
	} else {
		content += ui.TableDefaultStyle.Render("  " + allLine)
	}
	if len(m.workflows) > 0 {
		content += "\n"
	}

	for i, wf := range m.workflows {
		name := wf.Name
		if name == "" {
			name = wf.Filename
		}
		line := name
		if len(line) > maxLineWidth {
			line = line[:maxLineWidth-3] + "..."
		}
		if i == m.selectedWorkflow {
			content += ui.SelectedStyle.Render("> " + line)
		} else {
			content += ui.NormalStyle.Render("  " + line)
		}
		if i < len(m.workflows)-1 {
			content += "\n"
		}
	}

	return style.Render(title + "\n" + content)
}

func (m Model) viewHistoryPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneHistory)

	var workflowName string
	if m.selectedWorkflow >= 0 && m.selectedWorkflow < len(m.workflows) {
		workflowName = m.workflows[m.selectedWorkflow].Filename
	} else {
		workflowName = "all"
	}
	title := ui.TitleStyle.Render("Recent Runs (" + workflowName + ")")

	entries := m.currentHistoryEntries()
	var content string
	if len(entries) == 0 {
		content = ui.SubtitleStyle.Render("No history")
	} else {
		for i, e := range entries {
			line := e.Branch
			if i == m.selectedHistory {
				content += ui.SelectedStyle.Render("> " + line)
			} else {
				content += ui.NormalStyle.Render("  " + line)
			}
			if i < len(entries)-1 {
				content += "\n"
			}
		}
	}

	return style.Render(title + "\n" + content)
}

func (m Model) viewHistoryConfigPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneWorkflows)

	var content strings.Builder
	content.WriteString(ui.TitleStyle.Render("Run Configuration Preview"))
	content.WriteString("\n\n")

	if m.previewingHistoryEntry == nil {
		content.WriteString(ui.SubtitleStyle.Render("No history entry selected"))
		return style.Render(content.String())
	}

	entry := m.previewingHistoryEntry

	content.WriteString(ui.SubtitleStyle.Render("Branch: "))
	content.WriteString(ui.NormalStyle.Render(entry.Branch))
	content.WriteString("\n\n")

	var currentWorkflow *workflow.WorkflowFile
	if m.selectedWorkflow >= 0 && m.selectedWorkflow < len(m.workflows) {
		currentWorkflow = &m.workflows[m.selectedWorkflow]
	}

	var validationErrors []validation.ConfigValidationError
	if currentWorkflow != nil {
		validationErrors = validation.ValidateHistoryConfig(entry, currentWorkflow)
	}

	errorMap := make(map[string]validation.ConfigValidationError)
	for _, err := range validationErrors {
		errorMap[err.HistoricalName] = err
	}

	if len(entry.Inputs) == 0 {
		content.WriteString(ui.SubtitleStyle.Render("No inputs"))
	} else {
		content.WriteString(ui.SubtitleStyle.Render("Inputs:"))
		content.WriteString("\n")
		for k, v := range entry.Inputs {
			content.WriteString("  ")

			if err, hasError := errorMap[k]; hasError {
				content.WriteString(ui.TableItalicStyle.Render("! "))
				content.WriteString(ui.TableDefaultStyle.Render(k))
				content.WriteString(": ")
				content.WriteString(ui.TableDefaultStyle.Render(ui.FormatEmptyValue(v)))
				content.WriteString(" ")
				content.WriteString(ui.SubtitleStyle.Render("("))
				switch err.Status {
				case validation.StatusMissing:
					content.WriteString(ui.SubtitleStyle.Render("missing"))
				case validation.StatusTypeChanged:
					content.WriteString(ui.SubtitleStyle.Render("type changed"))
				case validation.StatusOptionsChanged:
					content.WriteString(ui.SubtitleStyle.Render("invalid option"))
				}
				content.WriteString(ui.SubtitleStyle.Render(")"))
			} else {
				content.WriteString(ui.NormalStyle.Render(k))
				content.WriteString(": ")
				content.WriteString(ui.RenderEmptyValue(v))
			}
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	if len(validationErrors) > 0 {
		content.WriteString(ui.HelpStyle.Render("[Enter] apply & run  [a] remap  [Esc] back"))
	} else {
		content.WriteString(ui.HelpStyle.Render("[Enter] apply & run  [Esc] back"))
	}

	return style.Render(content.String())
}

func (m Model) viewConfigPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneConfig)

	var content strings.Builder
	content.WriteString(ui.TitleStyle.Render("Configuration"))
	content.WriteString("\n\n")

	if m.selectedWorkflow < 0 || m.selectedWorkflow >= len(m.workflows) {
		content.WriteString(ui.SubtitleStyle.Render("Select a workflow"))
		content.WriteString("\n\n")
		content.WriteString(ui.HelpStyle.Render("[Tab] pane  [1-9] select workflow  [q] quit"))
		return style.Render(content.String())
	}

	branch := m.branch
	if branch == "" {
		branch = "(not set)"
	}
	content.WriteString(ui.TitleStyle.Render("Branch"))
	content.WriteString(": [b] ")
	content.WriteString(branch)

	content.WriteString("    Watch: [w] ")
	if m.watchRun {
		content.WriteString("on")
	} else {
		content.WriteString("off")
	}
	content.WriteString("    [r] reset all")
	content.WriteString("\n")

	if m.filterText != "" {
		content.WriteString(ui.SubtitleStyle.Render("Filter: /" + m.filterText))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(m.renderTableHeader())
	content.WriteString("\n")
	content.WriteString(m.renderTableRows(height))

	content.WriteString("\n\n")
	content.WriteString(ui.SubtitleStyle.Render("Command ([c] copy):"))
	content.WriteString("\n")
	cliCmd := m.buildCLIString()
	maxCmdWidth := width - 10
	if maxCmdWidth > 0 && len(cliCmd) > maxCmdWidth {
		cliCmd = "..." + cliCmd[len(cliCmd)-maxCmdWidth+3:]
	}
	content.WriteString(ui.CLIPreviewStyle.Render(cliCmd))

	helpLine := "\n\n" + ui.HelpStyle.Render("[Tab] pane  [Enter] run  [j/k] select  [1-0] edit  [/] filter  [?] help  [q] quit")
	content.WriteString(helpLine)

	return style.Render(content.String())
}

func (m Model) renderTableHeader() string {
	return ui.TableHeaderStyle.Render(
		"  #   Req  Name             Value              Default",
	)
}

func (m Model) renderTableRows(height int) string {
	var rows strings.Builder

	if m.selectedWorkflow >= len(m.workflows) {
		return ""
	}

	wf := m.workflows[m.selectedWorkflow]
	wfInputs := wf.GetInputs()
	visibleRows := height - TableHeaderHeight
	if visibleRows < 1 {
		visibleRows = 5
	}

	scrollOffset := 0
	if m.selectedInput >= visibleRows {
		scrollOffset = m.selectedInput - visibleRows + 1
	}

	visibleEnd := scrollOffset + visibleRows
	if visibleEnd > len(m.filteredInputs) {
		visibleEnd = len(m.filteredInputs)
	}

	for i := scrollOffset; i < visibleEnd; i++ {
		name := m.filteredInputs[i]
		input := wfInputs[name]
		val := m.inputs[name]

		numStr := _formatRowNumber(i)

		reqStr := " "
		if input.Required {
			reqStr = "x"
		}

		valueDisplay := ui.FormatEmptyValue(val)
		isSpecialValue := val == ""

		defaultDisplay := ui.FormatEmptyValue(input.Default)

		isSelected := i == m.selectedInput
		isDimmed := val == input.Default

		displayName := ui.TruncateWithEllipsis(name, 15)
		valueDisplay = ui.TruncateWithEllipsis(valueDisplay, 17)
		defaultDisplay = ui.TruncateWithEllipsis(defaultDisplay, 15)

		indicator := "  "
		if isSelected {
			indicator = "> "
		}

		row := indicator + numStr + "   " + reqStr + "    " +
			_padRight(displayName, 15) + "  " +
			_padRight(valueDisplay, 17) + "  " +
			defaultDisplay

		var rowStyle = ui.TableRowStyle
		if isSelected {
			rowStyle = ui.TableSelectedStyle
		} else if isDimmed {
			rowStyle = ui.TableDimmedStyle
		} else if isSpecialValue {
			rowStyle = ui.TableItalicStyle
		}

		rows.WriteString(rowStyle.Render(row))
		if i < visibleEnd-1 {
			rows.WriteString("\n")
		}
	}

	if scrollOffset > 0 || visibleEnd < len(m.filteredInputs) {
		rows.WriteString("\n")
		rows.WriteString(ui.RenderScrollIndicator(visibleEnd < len(m.filteredInputs), scrollOffset > 0))
	}

	return rows.String()
}
