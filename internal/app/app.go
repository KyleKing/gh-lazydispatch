package app

import (
	"context"
	"os/exec"
	"sort"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-wfd/internal/frecency"
	"github.com/kyleking/gh-wfd/internal/git"
	"github.com/kyleking/gh-wfd/internal/runner"
	"github.com/kyleking/gh-wfd/internal/ui"
	"github.com/kyleking/gh-wfd/internal/ui/modal"
	"github.com/kyleking/gh-wfd/internal/workflow"
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

	width  int
	height int
	keys   KeyMap
}

// New creates a new application model.
func New(workflows []workflow.WorkflowFile, history *frecency.Store, repo string) Model {
	m := Model{
		focused:    PaneWorkflows,
		workflows:  workflows,
		history:    history,
		repo:       repo,
		inputs:     make(map[string]string),
		modalStack: modal.NewStack(),
		keys:       DefaultKeyMap(),
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

	case key.Matches(msg, m.keys.Input1):
		return m.openInputModal(0)
	case key.Matches(msg, m.keys.Input2):
		return m.openInputModal(1)
	case key.Matches(msg, m.keys.Input3):
		return m.openInputModal(2)
	case key.Matches(msg, m.keys.Input4):
		return m.openInputModal(3)
	case key.Matches(msg, m.keys.Input5):
		return m.openInputModal(4)
	case key.Matches(msg, m.keys.Input6):
		return m.openInputModal(5)
	case key.Matches(msg, m.keys.Input7):
		return m.openInputModal(6)
	case key.Matches(msg, m.keys.Input8):
		return m.openInputModal(7)
	case key.Matches(msg, m.keys.Input9):
		return m.openInputModal(8)
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
		m.modalStack.Push(modal.NewConfirmModal(name, input.Description, current))
	case "choice":
		m.modalStack.Push(modal.NewSelectModal(name, input.Options, currentVal))
	default:
		m.modalStack.Push(modal.NewInputModal(name, input.Description, input.Default, input.InputType(), currentVal))
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

func (m *Model) initializeInputs(wf workflow.WorkflowFile) {
	m.inputs = make(map[string]string)
	m.inputOrder = nil
	for name, input := range wf.GetInputs() {
		m.inputs[name] = input.Default
		m.inputOrder = append(m.inputOrder, name)
	}
	sort.Strings(m.inputOrder)
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

	workflowPane := m.viewWorkflowPane(leftWidth, topHeight)
	historyPane := m.viewHistoryPane(rightWidth, topHeight)
	configPane := m.viewConfigPane(m.width, bottomHeight)

	top := lipgloss.JoinHorizontal(lipgloss.Top, workflowPane, historyPane)
	main := lipgloss.JoinVertical(lipgloss.Left, top, configPane)

	if m.modalStack.HasActive() {
		return m.modalStack.Render(main)
	}

	return main
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

	title := ui.TitleStyle.Render("Configuration")

	var workflowLine, branchLine, inputsLine string

	if m.selectedWorkflow < len(m.workflows) {
		wf := m.workflows[m.selectedWorkflow]
		workflowLine = "Workflow: " + wf.Filename

		branch := m.branch
		if branch == "" {
			branch = "(not set)"
		}
		branchLine = "Branch: [b] " + branch

		if len(m.inputOrder) > 0 {
			inputs := wf.GetInputs()
			inputsLine = "\nInputs:"
			for i, name := range m.inputOrder {
				if i >= 9 {
					break
				}
				input := inputs[name]
				val := m.inputs[name]
				if val == "" {
					val = input.Default
				}
				if val == "" {
					val = "(empty)"
				}
				inputsLine += "\n  [" + string(rune('1'+i)) + "] " + name + ": " + val
			}
		}
	}

	watchStatus := ""
	if m.watchRun {
		watchStatus = " [w] watch: on"
	} else {
		watchStatus = " [w] watch: off"
	}

	content := workflowLine + "\n" + branchLine + watchStatus + inputsLine

	helpLine := "\n\n" + ui.HelpStyle.Render("[Tab] pane  [Enter] run  [b] branch  [1-9] input  [?] help  [q] quit")

	return style.Render(title + "\n" + content + helpLine)
}

func _contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
