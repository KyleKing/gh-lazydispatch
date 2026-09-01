package app

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
	"github.com/kyleking/gh-lazydispatch/internal/validation"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

const (
	// ViewsFixedChromeHeight is the vertical space reserved for the status and footer bars.
	viewsFixedChromeHeight = 2

	// FooterBarMargin is the space reserved between the left/right footer segments and the pane edges.
	footerBarMargin = 2

	// PaneContentMargin and cliPreviewMargin/tableColumn widths reserve space for borders and padding.
	paneContentMargin = 8
	cliPreviewMargin  = 10
)

// View implements tea.Model.
func (m Model) View() tea.View {
	if m.width == 0 || m.height == 0 {
		return tea.NewView("Loading...")
	}

	if m.width < MinTerminalWidth || m.height < MinTerminalHeight {
		v := tea.NewView(m.viewTooSmall())
		v.AltScreen = true

		return v
	}

	statusBar := m.viewTopStatusBar()
	footerBar := m.viewFooterBar()

	box := m.layout()
	topHeight, leftWidth := box.topHeight, box.leftWidth

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

	// The config pane's height follows the selected workflow's input count, so
	// the right panel is resized on every frame rather than only on a resize.
	m.rightPanel.SetSize(box.rightWidth, box.topHeight)
	m.rightPanel.SetFocused(m.focused == PaneHistory)
	rightPane := m.rightPanel.View()
	configPane := m.viewConfigPane(m.width, box.configHeight)

	top := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)
	main := lipgloss.JoinVertical(lipgloss.Left, statusBar, top, configPane, footerBar)

	var content string
	if m.modalStack.HasActive() {
		content = m.modalStack.Render(main)
	} else {
		content = main
	}

	v := tea.NewView(content)
	v.AltScreen = true

	return v
}

func (m Model) viewTooSmall() string {
	msg := fmt.Sprintf(
		"Terminal too small: %dx%d\nMinimum: %dx%d",
		m.width, m.height, MinTerminalWidth, MinTerminalHeight,
	)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, msg)
}

func (m Model) viewTopStatusBar() string {
	var parts []string

	if m.branch != "" {
		parts = append(parts, m.branch+m.runsVerdict())
	}

	if m.wfdConfig != nil && len(m.wfdConfig.Chains) > 0 {
		parts = append(parts, fmt.Sprintf("Chains(%d)", len(m.wfdConfig.Chains)))
	}

	if m.watcher != nil {
		runs := m.watcher.GetRuns()
		if len(runs) > 0 {
			active := m.rightPanel.Live().ActiveCount()
			if active > 0 {
				parts = append(parts, fmt.Sprintf("Live(%d*)", len(runs)))
			} else {
				parts = append(parts, fmt.Sprintf("Live(%d)", len(runs)))
			}
		}
	}

	if m.chainExecutor != nil {
		state := m.chainExecutor.State()
		if state.Status == chain.ChainRunning {
			parts = append(parts, fmt.Sprintf("Chain: %s (%d/%d)",
				state.ChainName,
				state.CurrentStep+1,
				len(state.StepStatuses)))
		}
	}

	left := strings.Join(parts, "  ")
	right := "lazydispatch"

	padding := m.width - ansi.StringWidth(left) - len(right) - footerBarMargin
	if padding < 1 {
		padding = 1
	}

	return ui.HelpStyle.Render(" " + left + strings.Repeat(" ", padding) + right + " ")
}

func (m Model) viewFooterBar() string {
	if m.commandMode {
		return m.viewCommandBar(m.width)
	}

	if m.status != "" {
		return ui.HelpStyle.Render(" " + m.status)
	}

	var hints []string

	switch m.focused {
	case PaneWorkflows:
		hints = append(hints, "[j/k] select", "[Enter] run")
	case PaneHistory:
		switch m.rightPanel.ActiveTab() {
		case panes.TabHistory:
			hints = append(hints, "[h/l] tab", "[j/k] select", "[Enter] apply")
		case panes.TabChains:
			hints = append(hints, "[h/l] tab", "[j/k] select", "[Enter] run chain")
		case panes.TabLive:
			hints = append(hints, "[h/l] tab", "[j/k] select")
		case panes.TabTimeline:
			hints = append(hints, "[h/l] tab", "[j/k] select", "[Enter] steps", "[Esc] jobs")
		case panes.TabRuns:
			open := "[Enter] logs"
			if m.rightPanel.Runs().Scope() != panes.ScopeBranch {
				open = "[Enter] runs"
			}

			hints = append(hints, "[h/l] tab", "[j/k] select", "[s] scope", "[R] reload", open)
		}
	case PaneConfig:
		hints = append(hints, "[Enter] run", "[1-0] edit", "[/] filter")
	}

	// The leaders and the way out are never dropped: everything else is
	// reachable through them, and they are what a first-timer needs.
	leaders := []string{"[Tab] pane", "[a] actions", "[:] command"}
	always := []string{"[?] help", "[q] quit"}

	return ui.HelpStyle.Render(" " + fitHints(m.width-1, leaders, hints, always))
}

// fitHints renders leaders, then as many contextual hints as fit, then always.
// Lipgloss widens every line of the frame to the longest one, so a footer one
// cell too long pushes the whole layout sideways rather than wrapping. Dropping
// a contextual hint is what keeps that from happening, and contextual hints are
// the droppable ones because the action menu lists them too.
func fitHints(width int, leaders, contextual, always []string) string {
	if width <= 0 {
		return ""
	}

	for keep := len(contextual); keep >= 0; keep-- {
		parts := make([]string, 0, len(leaders)+keep+len(always))
		parts = append(parts, leaders...)
		parts = append(parts, contextual[:keep]...)
		parts = append(parts, always...)

		line := strings.Join(parts, "  ")
		if ansi.StringWidth(line) <= width {
			return line
		}
	}

	return ansi.Truncate(strings.Join(append(leaders, always...), "  "), width, "…")
}

func (m Model) viewInputDetailsPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneWorkflows)

	selectedName := m.getSelectedInputName()
	if selectedName == "" {
		return m.viewWorkflowPane(width, height)
	}

	wf := m.SelectedWorkflow()
	if wf == nil {
		return m.viewWorkflowPane(width, height)
	}

	inputs := wf.GetInputs()

	input, ok := inputs[selectedName]
	if !ok {
		return m.viewWorkflowPane(width, height)
	}

	var content strings.Builder

	content.WriteString(ui.TitleStyle.Render(m.leftPaneTitle()))
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
	if inputType != inputTypeChoice || len(options) == 0 {
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

	wrapped := _wordWrap(description, width-paneContentMargin)
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

func (m Model) leftPaneTitle() string {
	switch m.viewMode {
	case InputDetailMode:
		return "Workflows > Input"
	case HistoryPreviewMode:
		return "Workflows > Preview"
	default:
		return paneWorkflows
	}
}

func (m Model) viewWorkflowPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneWorkflows)

	maxLineWidth := width - paneContentMargin
	first, last := ui.ScrollWindow(m.selectedWorkflow, len(m.workflows), height-workflowPaneChrome)

	title := ui.TitleStyle.Render(m.leftPaneTitle()) +
		ui.RenderScrollIndicator(last < len(m.workflows), first > 0)

	var content strings.Builder

	// The pseudo-entry above the workflows leaves history unfiltered; `0`
	// returns to it.
	allLine := "all workflows"
	if m.selectedWorkflow == -1 {
		content.WriteString(ui.SelectedStyle.Render("> " + allLine))
	} else {
		content.WriteString(ui.TableDefaultStyle.Render("  " + allLine))
	}

	for i := first; i < last; i++ {
		wf := m.workflows[i]

		name := wf.Name
		if name == "" {
			name = wf.Filename
		}

		line := table.Truncate(name, maxLineWidth)

		content.WriteString("\n")

		if i == m.selectedWorkflow {
			content.WriteString(ui.SelectedStyle.Render("> " + line))
		} else {
			content.WriteString(ui.NormalStyle.Render("  " + line))
		}
	}

	return style.Render(title + "\n" + content.String())
}

// workflowPaneChrome is the vertical space the workflow pane spends on its
// border, title, and the "all workflows" pseudo-entry.
const workflowPaneChrome = 4

func (m Model) viewHistoryConfigPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneWorkflows)

	var content strings.Builder

	content.WriteString(ui.TitleStyle.Render(m.leftPaneTitle()))
	content.WriteString("\n\n")

	if m.previewingHistoryEntry == nil {
		content.WriteString(ui.SubtitleStyle.Render("No history entry selected"))
		return style.Render(content.String())
	}

	entry := m.previewingHistoryEntry

	content.WriteString(ui.SubtitleStyle.Render("Branch: "))
	content.WriteString(ui.NormalStyle.Render(entry.Branch))
	content.WriteString("\n\n")

	var currentWorkflow *workflow.File
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
				case validation.StatusValid:
					// unreachable: errorMap only contains entries with a non-valid status
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
		return style.Render(content.String())
	}

	content.WriteString(m.configHeaderLine(width - ui.PaneBorderSize))
	content.WriteString("\n")

	if m.filterText != "" {
		content.WriteString(ui.SubtitleStyle.Render("Filter: /" + m.filterText))
		content.WriteString("\n")
	}

	layout := ui.FitColumns(ui.ConfigColumns(), width-paneContentMargin, ui.RowGutterWidth)

	content.WriteString("\n")
	content.WriteString(m.renderTableHeader(layout))
	content.WriteString("\n")
	content.WriteString(m.renderTableRows(layout, height))

	content.WriteString("\n\n")
	content.WriteString(ui.SubtitleStyle.Render("Command ([c] copy):"))
	content.WriteString("\n")

	cliCmd := m.buildCLIString()
	maxCmdWidth := width - cliPreviewMargin

	// The command's tail carries the inputs, which is the half a reader checks.
	if maxCmdWidth > 0 {
		cliCmd = table.TruncateLeft(cliCmd, maxCmdWidth)
	}

	content.WriteString(ui.CLIPreviewStyle.Render(cliCmd))

	return style.Render(content.String())
}

// configHeaderLine spells the branch and the toggles on one line. A long branch
// name gives way rather than wrapping, since a wrapped line costs the config
// pane a row it did not budget for and the toggles are what the keys act on.
func (m Model) configHeaderLine(width int) string {
	branch := m.branch
	if branch == "" {
		branch = "(not set)"
	}

	watch := "off"
	if m.watchRun {
		watch = "on"
	}

	tail := "    Watch: [w] " + watch + "    [r] reset all"
	head := ": [b] "

	if room := width - ansi.StringWidth(head+tail) - len("Branch"); room > 0 {
		branch = table.TruncateLeft(branch, room)
	}

	return ui.TitleStyle.Render("Branch") + head + branch + tail
}

func (Model) renderTableHeader(layout table.Layout) string {
	return ui.TableHeader(layout, ui.RowGutterWidth)
}

func (m Model) renderTableRows(layout table.Layout, height int) string {
	var rows strings.Builder

	wf := m.SelectedWorkflow()
	if wf == nil {
		return ""
	}

	wfInputs := wf.GetInputs()

	visibleRows := height - configPaneChrome
	if visibleRows < len(m.filteredInputs) {
		// The scroll indicator costs a row, and it only appears once there is
		// something off screen to point at.
		visibleRows--
	}

	if visibleRows < 1 {
		visibleRows = 1
	}

	scrollOffset, visibleEnd := ui.ScrollWindow(m.selectedInput, len(m.filteredInputs), visibleRows)

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

		indicator := "  "
		if isSelected {
			indicator = "> "
		}

		row := indicator + ui.TableRow(layout, map[string]string{
			"num":         numStr,
			"req":         reqStr,
			ui.ColKeyName: name,
			"value":       valueDisplay,
			"default":     defaultDisplay,
		})

		rowStyle := ui.TableRowStyle

		switch {
		case isSelected:
			rowStyle = ui.TableSelectedStyle
		case isDimmed:
			rowStyle = ui.TableDimmedStyle
		case isSpecialValue:
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
