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
	"github.com/kyleking/lazydispatch/internal/frecency"
	"github.com/kyleking/lazydispatch/internal/git"
	"github.com/kyleking/lazydispatch/internal/runner"
	"github.com/kyleking/lazydispatch/internal/ui"
	"github.com/kyleking/lazydispatch/internal/ui/modal"
	"github.com/kyleking/lazydispatch/internal/validation"
	"github.com/kyleking/lazydispatch/internal/workflow"
)

// FocusedPane represents which pane currently has focus.
type FocusedPane int

const (
	PaneWorkflows FocusedPane = iota
	PaneHistory
	PaneConfig
)

// ViewMode represents the current view mode.
type ViewMode int

const (
	WorkflowListMode ViewMode = iota
	HistoryPreviewMode
	InputDetailMode
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
	selectedInput        int                   // Currently selected input row (-1 = none)
	viewMode             ViewMode              // Current view mode
	filterText           string                // Current filter string
	filteredInputs       []string              // Input names after filtering
	previewingHistoryEntry *frecency.HistoryEntry // History entry being previewed

	width  int
	height int
	keys   KeyMap
}

// New creates a new application model.
func New(workflows []workflow.WorkflowFile, history *frecency.Store, repo string) Model {
	ctx := context.Background()
	currentBranch := git.GetCurrentBranch(ctx)

	m := Model{
		focused:          PaneWorkflows,
		workflows:        workflows,
		history:          history,
		repo:             repo,
		branch:           currentBranch,
		inputs:           make(map[string]string),
		modalStack:       modal.NewStack(),
		keys:             DefaultKeyMap(),
		selectedInput:    -1,
		selectedWorkflow: -1,
	}

	if len(workflows) > 0 {
		m.selectedWorkflow = 0
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

	case modal.RunConfirmResultMsg:
		return m.handleRunConfirmResult(msg)

	case modal.RemapResultMsg:
		return m.handleRemapResult(msg)

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
		if m.viewMode != WorkflowListMode {
			m.viewMode = WorkflowListMode
			m.selectedInput = -1
			m.previewingHistoryEntry = nil
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

	case key.Matches(msg, m.keys.Edit):
		if m.viewMode == InputDetailMode && m.selectedInput >= 0 {
			return m.openInputModalFiltered(m.selectedInput)
		}
		return m, nil

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

	case msg.String() == "a":
		if m.viewMode == HistoryPreviewMode && m.previewingHistoryEntry != nil {
			return m.openRemapModal()
		}
		return m, nil

	default:
		for i, k := range m.keys.InputKeys() {
			if key.Matches(msg, k) {
				return m.handleInputKey(i)
			}
		}
		for i, k := range m.keys.WorkflowKeys() {
			if key.Matches(msg, k) {
				return m.handleWorkflowKey(i)
			}
		}
	}

	return m, nil
}

func (m Model) handleInputKey(index int) (tea.Model, tea.Cmd) {
	if m.focused == PaneConfig {
		return m.openInputModalFiltered(index)
	}
	return m, nil
}

func (m Model) handleWorkflowKey(num int) (tea.Model, tea.Cmd) {
	if m.focused != PaneWorkflows {
		return m, nil
	}
	if num == 0 {
		m.selectedWorkflow = -1
		return m, nil
	}
	workflowIdx := num - 1
	if workflowIdx < len(m.workflows) {
		m.selectedWorkflow = workflowIdx
		m.initializeInputs(m.workflows[workflowIdx])
	}
	return m, nil
}

func (m *Model) handleUp() {
	switch m.focused {
	case PaneWorkflows:
		if m.selectedWorkflow > -1 {
			m.selectedWorkflow--
			if m.selectedWorkflow >= 0 {
				m.initializeInputs(m.workflows[m.selectedWorkflow])
			}
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
		m.viewMode = InputDetailMode
		if m.selectedInput < 0 {
			m.viewMode = WorkflowListMode
		}
		m.syncFilteredInputs()
	}
}

func (m *Model) handleDown() {
	switch m.focused {
	case PaneWorkflows:
		if m.selectedWorkflow < len(m.workflows)-1 {
			m.selectedWorkflow++
			if m.selectedWorkflow >= 0 {
				m.initializeInputs(m.workflows[m.selectedWorkflow])
			}
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
		m.viewMode = InputDetailMode
		if m.selectedInput < 0 {
			m.viewMode = WorkflowListMode
		}
		m.syncFilteredInputs()
	}
}

func (m Model) handleEnter() (tea.Model, tea.Cmd) {
	switch m.focused {
	case PaneHistory:
		entries := m.currentHistoryEntries()
		if m.selectedHistory < len(entries) {
			entry := entries[m.selectedHistory]
			if m.viewMode == HistoryPreviewMode {
				// Apply previewed config and run
				m.branch = entry.Branch
				m.inputs = make(map[string]string)
				for k, v := range entry.Inputs {
					m.inputs[k] = v
				}
				m.viewMode = WorkflowListMode
				m.previewingHistoryEntry = nil
				return m.executeWorkflow()
			} else {
				// Enter preview mode
				m.viewMode = HistoryPreviewMode
				m.previewingHistoryEntry = &entry
			}
		}
	case PaneConfig:
		return m.executeWorkflow()
	}
	return m, nil
}

func (m Model) executeWorkflow() (tea.Model, tea.Cmd) {
	if m.selectedWorkflow < 0 || m.selectedWorkflow >= len(m.workflows) {
		return m, nil
	}

	wf := m.workflows[m.selectedWorkflow]
	cfg := runner.RunConfig{
		Workflow: wf.Filename,
		Branch:   m.branch,
		Inputs:   m.inputs,
		Watch:    m.watchRun,
	}

	m.modalStack.Push(modal.NewRunConfirmModal(cfg))
	return m, nil
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
	return m.openInputModalForName(m.inputOrder[index])
}

func (m Model) openInputModalForName(name string) (tea.Model, tea.Cmd) {
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

func (m Model) handleRunConfirmResult(msg modal.RunConfirmResultMsg) (tea.Model, tea.Cmd) {
	if msg.Confirmed {
		return m.doExecuteWorkflow(msg.Config)
	}
	return m, nil
}

func (m Model) openRemapModal() (tea.Model, tea.Cmd) {
	if m.previewingHistoryEntry == nil {
		return m, nil
	}

	// Get current workflow for validation
	if m.selectedWorkflow < 0 || m.selectedWorkflow >= len(m.workflows) {
		return m, nil
	}

	currentWorkflow := &m.workflows[m.selectedWorkflow]
	validationErrors := validation.ValidateHistoryConfig(m.previewingHistoryEntry, currentWorkflow)

	if len(validationErrors) == 0 {
		return m, nil
	}

	currentInputs := currentWorkflow.GetInputs()
	remapModal := modal.NewRemapModal(validationErrors, currentInputs)
	m.modalStack.Push(remapModal)
	return m, nil
}

func (m Model) handleRemapResult(msg modal.RemapResultMsg) (tea.Model, tea.Cmd) {
	if m.previewingHistoryEntry == nil || len(msg.Decisions) == 0 {
		return m, nil
	}

	// Apply remapping decisions to the previewing entry
	remappedInputs := make(map[string]string)

	// Start with the historical inputs
	for k, v := range m.previewingHistoryEntry.Inputs {
		remappedInputs[k] = v
	}

	// Apply decisions
	for _, decision := range msg.Decisions {
		switch decision.Action {
		case modal.RemapActionDrop:
			// Remove the input
			delete(remappedInputs, decision.OriginalName)
		case modal.RemapActionKeep:
			// Keep as-is (already in remappedInputs)
		case modal.RemapActionMap:
			// Rename/remap the input
			if val, exists := remappedInputs[decision.OriginalName]; exists {
				delete(remappedInputs, decision.OriginalName)
				remappedInputs[decision.NewName] = val
			}
		}
	}

	// Update the previewing entry with remapped inputs
	m.previewingHistoryEntry.Inputs = remappedInputs

	return m, nil
}

func (m Model) doExecuteWorkflow(cfg runner.RunConfig) (tea.Model, tea.Cmd) {
	m.history.Record(m.repo, cfg.Workflow, cfg.Branch, cfg.Inputs)
	m.history.Save()

	return m, tea.ExecProcess(exec.Command("gh", runner.BuildArgs(cfg)...), func(err error) tea.Msg {
		return executionDoneMsg{err: err}
	})
}

func (m *Model) applyFilter() {
	m.filteredInputs = ui.ApplyFuzzyFilter(m.filterText, m.inputOrder)
	m.selectedInput = -1
	m.viewMode = WorkflowListMode
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
	filterModal := modal.NewFilterModal("Filter Inputs", m.inputOrder, "")
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
	return m.openInputModalForName(m.filteredInputs[index])
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
	m.viewMode = WorkflowListMode
	m.selectedHistory = 0
}

func (m Model) currentHistoryEntries() []frecency.HistoryEntry {
	if m.history == nil {
		return nil
	}
	var workflowFilter string
	if m.selectedWorkflow >= 0 && m.selectedWorkflow < len(m.workflows) {
		workflowFilter = m.workflows[m.selectedWorkflow].Filename
	}
	return m.history.TopForRepo(m.repo, workflowFilter, 10)
}

// SelectedWorkflow returns the currently selected workflow.
func (m Model) SelectedWorkflow() *workflow.WorkflowFile {
	if m.selectedWorkflow < 0 || m.selectedWorkflow >= len(m.workflows) {
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

	// Add "all" option
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

	// Get current workflow for validation
	var currentWorkflow *workflow.WorkflowFile
	if m.selectedWorkflow >= 0 && m.selectedWorkflow < len(m.workflows) {
		currentWorkflow = &m.workflows[m.selectedWorkflow]
	}

	// Validate configuration
	var validationErrors []validation.ConfigValidationError
	if currentWorkflow != nil {
		validationErrors = validation.ValidateHistoryConfig(entry, currentWorkflow)
	}

	// Build error map for quick lookup
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

			// Check if this input has a validation error
			if err, hasError := errorMap[k]; hasError {
				content.WriteString(ui.TableItalicStyle.Render("⚠ "))
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

func _padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

func _formatRowNumber(index int) string {
	displayIdx := index + 1
	if displayIdx <= 9 {
		return string(rune('0' + displayIdx))
	}
	if displayIdx == 10 {
		return "0"
	}
	return " "
}

func _contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
