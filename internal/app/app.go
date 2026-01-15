package app

import (
	"context"
	"os/exec"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-wfd/internal/frecency"
	"github.com/kyleking/gh-wfd/internal/git"
	"github.com/kyleking/gh-wfd/internal/runner"
	"github.com/kyleking/gh-wfd/internal/ui"
	"github.com/kyleking/gh-wfd/internal/ui/modal"
	"github.com/kyleking/gh-wfd/internal/workflow"
	"github.com/sahilm/fuzzy"
)

// FocusedPane represents which pane currently has focus.
type FocusedPane int

const (
	PaneWorkflows FocusedPane = iota
	PaneHistory
	PaneConfig
)

// Model is the root bubbletea model for the application.
type Model struct {
	focused   FocusedPane
	workflows []workflow.WorkflowFile
	history   *frecency.Store
	repo      string

	selectedWorkflow int
	selectedHistory  int
	branch           string
	inputs           map[string]string
	inputOrder       []string
	watchRun         bool

	modalStack *modal.Stack

	pendingInputName string

	// Config panel state
	selectedInput   int      // Currently selected input row (-1 = none)
	inputDetailMode bool     // When true, workflow pane shows input details
	filterText      string   // Current filter string
	filteredInputs  []string // Input names after filtering

	width  int
	height int
	keys   KeyMap
}

// New creates a new application model.
func New(workflows []workflow.WorkflowFile, history *frecency.Store, repo string) Model {
	m := Model{
		focused:       PaneWorkflows,
		workflows:     workflows,
		history:       history,
		repo:          repo,
		inputs:        make(map[string]string),
		modalStack:    modal.NewStack(),
		keys:          DefaultKeyMap(),
		selectedInput: -1,
	}

	if len(workflows) > 0 {
		m.initializeInputs(workflows[0])
	}

	return m
}

// Init implements tea.Model.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.modalStack.HasActive() {
		return m.updateModal(msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.modalStack.SetSize(msg.Width, msg.Height)
		return m, nil

	case modal.SelectResultMsg:
		return m.handleSelectResult(msg)

	case modal.BranchResultMsg:
		return m.handleBranchResult(msg)

	case modal.InputResultMsg:
		return m.handleInputResult(msg)

	case modal.ConfirmResultMsg:
		return m.handleConfirmResult(msg)

	case modal.FilterResultMsg:
		return m.handleFilterResult(msg)

	case modal.ResetResultMsg:
		return m.handleResetResult(msg)

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
}

func (m Model) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	cmd := m.modalStack.Update(msg)
	return m, cmd
}

func (m Model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit

	case key.Matches(msg, m.keys.Help):
		m.modalStack.Push(modal.NewHelpModal())
		return m, nil

	case key.Matches(msg, m.keys.Escape):
		if m.inputDetailMode {
			m.inputDetailMode = false
			m.selectedInput = -1
			return m, nil
		}
		return m, nil

	case key.Matches(msg, m.keys.Tab):
		m.focused = (m.focused + 1) % 3
		return m, nil

	case key.Matches(msg, m.keys.ShiftTab):
		m.focused = (m.focused + 2) % 3
		return m, nil

	case key.Matches(msg, m.keys.Up):
		m.handleUp()
		return m, nil

	case key.Matches(msg, m.keys.Down):
		m.handleDown()
		return m, nil

	case key.Matches(msg, m.keys.Enter):
		return m.handleEnter()

	case key.Matches(msg, m.keys.Watch):
		m.watchRun = !m.watchRun
		return m, nil

	case key.Matches(msg, m.keys.Branch):
		return m.openBranchModal()

	case key.Matches(msg, m.keys.Filter):
		if m.focused == PaneConfig {
			return m.openFilterModal()
		}
		return m, nil

	case key.Matches(msg, m.keys.Copy):
		if m.focused == PaneConfig {
			return m.copyCommandToClipboard()
		}
		return m, nil

	case key.Matches(msg, m.keys.Reset):
		if m.focused == PaneConfig {
			return m.openResetModal()
		}
		return m, nil

	case key.Matches(msg, m.keys.Input1):
		return m.openInputModalFiltered(0)
	case key.Matches(msg, m.keys.Input2):
		return m.openInputModalFiltered(1)
	case key.Matches(msg, m.keys.Input3):
		return m.openInputModalFiltered(2)
	case key.Matches(msg, m.keys.Input4):
		return m.openInputModalFiltered(3)
	case key.Matches(msg, m.keys.Input5):
		return m.openInputModalFiltered(4)
	case key.Matches(msg, m.keys.Input6):
		return m.openInputModalFiltered(5)
	case key.Matches(msg, m.keys.Input7):
		return m.openInputModalFiltered(6)
	case key.Matches(msg, m.keys.Input8):
		return m.openInputModalFiltered(7)
	case key.Matches(msg, m.keys.Input9):
		return m.openInputModalFiltered(8)
	case key.Matches(msg, m.keys.Input0):
		return m.openInputModalFiltered(9)
	}

	return m, nil
}

func (m *Model) handleUp() {
	switch m.focused {
	case PaneWorkflows:
		if m.selectedWorkflow > 0 {
			m.selectedWorkflow--
			m.initializeInputs(m.workflows[m.selectedWorkflow])
		}
	case PaneHistory:
		if m.selectedHistory > 0 {
			m.selectedHistory--
		}
	case PaneConfig:
		if m.selectedInput < 0 {
			m.selectedInput = 0
		} else if m.selectedInput > 0 {
			m.selectedInput--
		}
		m.inputDetailMode = m.selectedInput >= 0
		m.syncFilteredInputs()
	}
}

func (m *Model) handleDown() {
	switch m.focused {
	case PaneWorkflows:
		if m.selectedWorkflow < len(m.workflows)-1 {
			m.selectedWorkflow++
			m.initializeInputs(m.workflows[m.selectedWorkflow])
		}
	case PaneHistory:
		entries := m.currentHistoryEntries()
		if m.selectedHistory < len(entries)-1 {
			m.selectedHistory++
		}
	case PaneConfig:
		if m.selectedInput < 0 {
			m.selectedInput = 0
		} else if m.selectedInput < len(m.filteredInputs)-1 {
			m.selectedInput++
		}
		m.inputDetailMode = m.selectedInput >= 0
		m.syncFilteredInputs()
	}
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focused {
	case PaneHistory:
		entries := m.currentHistoryEntries()
		if m.selectedHistory < len(entries) {
			entry := entries[m.selectedHistory]
			m.branch = entry.Branch
			m.inputs = make(map[string]string)
			for k, v := range entry.Inputs {
				m.inputs[k] = v
			}
		}
	case PaneConfig:
		return m.executeWorkflow()
	}
	return m, nil
}

func (m Model) executeWorkflow() (tea.Model, tea.Cmd) {
	if m.selectedWorkflow >= len(m.workflows) {
		return m, nil
	}

	wf := m.workflows[m.selectedWorkflow]
	cfg := runner.RunConfig{
		Workflow: wf.Filename,
		Branch:   m.branch,
		Inputs:   m.inputs,
		Watch:    m.watchRun,
	}

	m.history.Record(m.repo, wf.Filename, m.branch, m.inputs)
	m.history.Save()

	return m, tea.ExecProcess(exec.Command("gh", runner.BuildArgs(cfg)...), func(err error) tea.Msg {
		return executionDoneMsg{err: err}
	})
}

type executionDoneMsg struct {
	err error
}

func (m Model) openBranchModal() (tea.Model, tea.Cmd) {
	ctx := context.Background()

	branches, err := git.FetchBranches(ctx)
	if err != nil {
		branches = []string{"main", "master", "develop"}
	}

	if m.branch != "" && !_contains(branches, m.branch) {
		branches = append(branches, m.branch)
	}

	defaultBranch := git.GetDefaultBranch(ctx)

	branchModal := modal.NewSimpleBranchModal("Select Branch", branches, m.branch, defaultBranch)
	branchModal.SetSize(m.width, m.height)
	m.modalStack.Push(branchModal)
	return m, nil
}

func (m Model) openInputModal(index int) (tea.Model, tea.Cmd) {
	if index >= len(m.inputOrder) {
		return m, nil
	}

	name := m.inputOrder[index]
	wf := m.workflows[m.selectedWorkflow]
	inputs := wf.GetInputs()
	input, ok := inputs[name]
	if !ok {
		return m, nil
	}

	m.pendingInputName = name
	currentVal := m.inputs[name]

	switch input.InputType() {
	case "boolean":
		current := currentVal == "true"
		defaultVal := input.Default == "true"
		m.modalStack.Push(modal.NewConfirmModal(name, input.Description, current, defaultVal))
	case "choice":
		m.modalStack.Push(modal.NewSelectModal(name, input.Options, currentVal, input.Default))
	default:
		m.modalStack.Push(modal.NewInputModal(name, input.Description, input.Default, input.InputType(), currentVal, input.Options))
	}

	return m, nil
}

func (m Model) handleSelectResult(msg modal.SelectResultMsg) (tea.Model, tea.Cmd) {
	if m.pendingInputName != "" {
		m.inputs[m.pendingInputName] = msg.Value
		m.pendingInputName = ""
	}
	return m, nil
}

func (m Model) handleBranchResult(msg modal.BranchResultMsg) (tea.Model, tea.Cmd) {
	m.branch = msg.Value
	return m, nil
}

func (m Model) handleInputResult(msg modal.InputResultMsg) (tea.Model, tea.Cmd) {
	if m.pendingInputName != "" {
		m.inputs[m.pendingInputName] = msg.Value
		m.pendingInputName = ""
	}
	return m, nil
}

func (m Model) handleConfirmResult(msg modal.ConfirmResultMsg) (tea.Model, tea.Cmd) {
	if m.pendingInputName != "" {
		if msg.Value {
			m.inputs[m.pendingInputName] = "true"
		} else {
			m.inputs[m.pendingInputName] = "false"
		}
		m.pendingInputName = ""
	}
	return m, nil
}

func (m Model) handleFilterResult(msg modal.FilterResultMsg) (tea.Model, tea.Cmd) {
	if !msg.Cancelled {
		m.filterText = msg.Value
		m.applyFilter()
	}
	return m, nil
}

func (m Model) handleResetResult(msg modal.ResetResultMsg) (tea.Model, tea.Cmd) {
	if msg.Confirmed {
		m.resetAllInputs()
	}
	return m, nil
}

func (m *Model) applyFilter() {
	if m.filterText == "" {
		m.filteredInputs = m.inputOrder
	} else {
		matches := fuzzy.Find(m.filterText, m.inputOrder)
		m.filteredInputs = make([]string, len(matches))
		for i, match := range matches {
			m.filteredInputs[i] = match.Str
		}
	}
	m.selectedInput = -1
	m.inputDetailMode = false
}

func (m *Model) resetAllInputs() {
	if m.selectedWorkflow >= len(m.workflows) {
		return
	}
	wf := m.workflows[m.selectedWorkflow]
	inputs := wf.GetInputs()
	for name, input := range inputs {
		m.inputs[name] = input.Default
	}
}

func (m *Model) syncFilteredInputs() {
	if m.filterText == "" {
		m.filteredInputs = m.inputOrder
	}
}

func (m Model) openFilterModal() (tea.Model, tea.Cmd) {
	filterModal := modal.NewFilterModal("Filter Inputs", m.inputOrder, m.filterText)
	m.modalStack.Push(filterModal)
	return m, nil
}

func (m Model) copyCommandToClipboard() (tea.Model, tea.Cmd) {
	if m.selectedWorkflow >= len(m.workflows) {
		return m, nil
	}
	cmd := m.buildCLIString()
	clipboard.WriteAll(cmd)
	return m, nil
}

func (m Model) buildCLIString() string {
	if m.selectedWorkflow >= len(m.workflows) {
		return ""
	}
	wf := m.workflows[m.selectedWorkflow]
	args := []string{"workflow", "run", wf.Filename}
	if m.branch != "" {
		args = append(args, "--ref", m.branch)
	}
	for _, name := range m.inputOrder {
		val := m.inputs[name]
		if val != "" {
			args = append(args, "-f", name+"="+val)
		}
	}
	return "gh " + strings.Join(args, " ")
}

func (m Model) openResetModal() (tea.Model, tea.Cmd) {
	if m.selectedWorkflow >= len(m.workflows) {
		return m, nil
	}
	wf := m.workflows[m.selectedWorkflow]
	inputs := wf.GetInputs()
	var diffs []modal.ResetDiff

	for _, name := range m.inputOrder {
		input := inputs[name]
		current := m.inputs[name]
		if current != input.Default {
			diffs = append(diffs, modal.ResetDiff{
				Name:    name,
				Current: current,
				Default: input.Default,
			})
		}
	}

	resetModal := modal.NewResetModal(diffs)
	m.modalStack.Push(resetModal)
	return m, nil
}

func (m Model) openInputModalFiltered(index int) (tea.Model, tea.Cmd) {
	if index >= len(m.filteredInputs) {
		return m, nil
	}

	name := m.filteredInputs[index]
	if m.selectedWorkflow >= len(m.workflows) {
		return m, nil
	}
	wf := m.workflows[m.selectedWorkflow]
	inputs := wf.GetInputs()
	input, ok := inputs[name]
	if !ok {
		return m, nil
	}

	m.pendingInputName = name
	currentVal := m.inputs[name]

	switch input.InputType() {
	case "boolean":
		current := currentVal == "true"
		defaultVal := input.Default == "true"
		m.modalStack.Push(modal.NewConfirmModal(name, input.Description, current, defaultVal))
	case "choice":
		m.modalStack.Push(modal.NewSelectModal(name, input.Options, currentVal, input.Default))
	default:
		m.modalStack.Push(modal.NewInputModal(name, input.Description, input.Default, input.InputType(), currentVal, input.Options))
	}

	return m, nil
}

func (m *Model) initializeInputs(wf workflow.WorkflowFile) {
	m.inputs = make(map[string]string)
	m.inputOrder = nil
	for name, input := range wf.GetInputs() {
		m.inputs[name] = input.Default
		m.inputOrder = append(m.inputOrder, name)
	}
	sort.Strings(m.inputOrder)
	m.filteredInputs = m.inputOrder
	m.filterText = ""
	m.selectedInput = -1
	m.inputDetailMode = false
	m.selectedHistory = 0
}

func (m Model) currentHistoryEntries() []frecency.HistoryEntry {
	if m.history == nil {
		return nil
	}
	var workflowFilter string
	if m.selectedWorkflow < len(m.workflows) {
		workflowFilter = m.workflows[m.selectedWorkflow].Filename
	}
	return m.history.TopForRepo(m.repo, workflowFilter, 10)
}

// SelectedWorkflow returns the currently selected workflow.
func (m Model) SelectedWorkflow() *workflow.WorkflowFile {
	if m.selectedWorkflow >= len(m.workflows) {
		return nil
	}
	return &m.workflows[m.selectedWorkflow]
}

// View implements tea.Model.
func (m Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	topHeight := m.height / 2
	bottomHeight := m.height - topHeight

	leftWidth := (m.width * 11) / 30
	rightWidth := m.width - leftWidth

	var leftPane string
	if m.inputDetailMode && m.getSelectedInputName() != "" {
		leftPane = m.viewInputDetailsPane(leftWidth, topHeight)
	} else {
		leftPane = m.viewWorkflowPane(leftWidth, topHeight)
	}
	historyPane := m.viewHistoryPane(rightWidth, topHeight)
	configPane := m.viewConfigPane(m.width, bottomHeight)

	top := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, historyPane)
	main := lipgloss.JoinVertical(lipgloss.Left, top, configPane)

	if m.modalStack.HasActive() {
		return m.modalStack.Render(main)
	}

	return main
}

func (m Model) getSelectedInputName() string {
	if len(m.filteredInputs) == 0 {
		return ""
	}
	if m.selectedInput < 0 || m.selectedInput >= len(m.filteredInputs) {
		return ""
	}
	return m.filteredInputs[m.selectedInput]
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

	content.WriteString(ui.TitleStyle.Render(selectedName))
	if input.Required {
		content.WriteString(" ")
		content.WriteString(ui.SelectedStyle.Render("(required)"))
	}
	content.WriteString("\n\n")

	content.WriteString(ui.SubtitleStyle.Render("Type: "))
	content.WriteString(ui.NormalStyle.Render(input.InputType()))
	content.WriteString("\n")

	if input.InputType() == "choice" && len(input.Options) > 0 {
		content.WriteString("\n")
		content.WriteString(ui.SubtitleStyle.Render("Options:"))
		content.WriteString("\n")
		for _, opt := range input.Options {
			content.WriteString("  - ")
			content.WriteString(ui.NormalStyle.Render(opt))
			content.WriteString("\n")
		}
	}

	if input.Description != "" {
		content.WriteString("\n")
		content.WriteString(ui.SubtitleStyle.Render("Description:"))
		content.WriteString("\n")
		wrapped := _wordWrap(input.Description, width-8)
		content.WriteString(ui.NormalStyle.Render(wrapped))
		content.WriteString("\n")
	}

	content.WriteString("\n")
	content.WriteString(ui.SubtitleStyle.Render("Current: "))
	currentVal := m.inputs[selectedName]
	if currentVal == "" {
		content.WriteString(ui.TableItalicStyle.Render(`("")`))
	} else {
		content.WriteString(ui.NormalStyle.Render(currentVal))
	}

	content.WriteString("\n")
	content.WriteString(ui.SubtitleStyle.Render("Default: "))
	if input.Default == "" {
		content.WriteString(ui.TableItalicStyle.Render(`("")`))
	} else {
		content.WriteString(ui.NormalStyle.Render(input.Default))
	}

	content.WriteString("\n\n")
	content.WriteString(ui.HelpStyle.Render("[Esc] back  [Enter] edit"))

	return style.Render(content.String())
}

func _wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0
	for i, word := range words {
		if i > 0 && lineLen+1+len(word) > width {
			result.WriteString("\n")
			lineLen = 0
		} else if i > 0 {
			result.WriteString(" ")
			lineLen++
		}
		result.WriteString(word)
		lineLen += len(word)
	}
	return result.String()
}

func (m Model) viewWorkflowPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneWorkflows)

	title := ui.TitleStyle.Render("Workflows")
	maxLineWidth := width - 8
	var content string
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
	if m.selectedWorkflow < len(m.workflows) {
		workflowName = m.workflows[m.selectedWorkflow].Filename
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

func (m Model) viewConfigPane(width, height int) string {
	style := ui.PaneStyle(width, height, m.focused == PaneConfig)

	var content strings.Builder
	content.WriteString(ui.TitleStyle.Render("Configuration"))
	content.WriteString("\n\n")

	if m.selectedWorkflow >= len(m.workflows) {
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
	content.WriteString(ui.SubtitleStyle.Render("Command:"))
	content.WriteString("\n")
	cliCmd := m.buildCLIString()
	maxCmdWidth := width - 10
	if maxCmdWidth > 0 && len(cliCmd) > maxCmdWidth {
		cliCmd = "..." + cliCmd[len(cliCmd)-maxCmdWidth+3:]
	}
	content.WriteString(ui.CLIPreviewStyle.Render(cliCmd))
	content.WriteString(" [c]")

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
	visibleRows := height - 14
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

		numStr := " "
		displayIdx := i + 1
		if displayIdx <= 9 {
			numStr = string(rune('0' + displayIdx))
		} else if displayIdx == 10 {
			numStr = "0"
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

		isSelected := i == m.selectedInput
		isDimmed := val == input.Default

		displayName := name
		if len(displayName) > 15 {
			displayName = displayName[:12] + "..."
		}
		if len(valueDisplay) > 17 {
			valueDisplay = valueDisplay[:14] + "..."
		}
		if len(defaultDisplay) > 15 {
			defaultDisplay = defaultDisplay[:12] + "..."
		}

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
		scrollInfo := ""
		if scrollOffset > 0 {
			scrollInfo += "^"
		} else {
			scrollInfo += " "
		}
		scrollInfo += " "
		if visibleEnd < len(m.filteredInputs) {
			scrollInfo += "v"
		}
		rows.WriteString(ui.SubtitleStyle.Render(scrollInfo))
	}

	return rows.String()
}

func _padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

func _contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
