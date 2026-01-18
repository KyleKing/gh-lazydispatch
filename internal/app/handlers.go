package app

import (
	"context"
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/git"
	"github.com/kyleking/gh-lazydispatch/internal/rule"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/validation"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

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

	case key.Matches(msg, m.keys.LiveView):
		return m.openLiveViewModal()

	case key.Matches(msg, m.keys.Chain):
		return m.openChainSelectModal()

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
				m.branch = entry.Branch
				m.inputs = make(map[string]string)
				for k, v := range entry.Inputs {
					m.inputs[k] = v
				}
				m.viewMode = WorkflowListMode
				m.previewingHistoryEntry = nil
				return m.executeWorkflow()
			} else {
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

	validationErrors := m.validateAllInputs(wf)
	if len(validationErrors) > 0 {
		m.modalStack.Push(modal.NewValidationErrorModal(validationErrors))
		return m, nil
	}

	cfg := runner.RunConfig{
		Workflow: wf.Filename,
		Branch:   m.branch,
		Inputs:   m.inputs,
		Watch:    m.watchRun,
	}

	m.modalStack.Push(modal.NewRunConfirmModal(cfg))
	return m, nil
}

func (m Model) validateAllInputs(wf workflow.WorkflowFile) map[string][]string {
	errs := make(map[string][]string)
	inputs := wf.GetInputs()
	for name, input := range inputs {
		if rules := input.ValidationRules; len(rules) > 0 {
			if validationErrs := rule.ValidateValue(m.inputs[name], rules); len(validationErrs) > 0 {
				errs[name] = validationErrs
			}
		}
	}
	return errs
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

func (m Model) openLiveViewModal() (tea.Model, tea.Cmd) {
	if m.watcher == nil {
		return m, nil
	}
	runs := m.watcher.GetRuns()
	m.modalStack.Push(modal.NewLiveViewModal(runs))
	return m, nil
}

func (m Model) openChainSelectModal() (tea.Model, tea.Cmd) {
	if m.wfdConfig == nil || !m.wfdConfig.HasChains() {
		return m, nil
	}
	m.modalStack.Push(modal.NewChainSelectModal(m.wfdConfig))
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
		m.modalStack.Push(modal.NewInputModal(name, input.Description, input.Default, input.InputType(), currentVal, input.Options, input.ValidationRules))
	}

	return m, nil
}

func (m Model) openInputModalFiltered(index int) (tea.Model, tea.Cmd) {
	if index >= len(m.filteredInputs) {
		return m, nil
	}
	return m.openInputModalForName(m.filteredInputs[index])
}

func (m Model) openFilterModal() (tea.Model, tea.Cmd) {
	filterModal := modal.NewFilterModal("Filter Inputs", m.inputOrder, "")
	m.modalStack.Push(filterModal)
	return m, nil
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

func (m Model) openRemapModal() (tea.Model, tea.Cmd) {
	if m.previewingHistoryEntry == nil {
		return m, nil
	}

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

func (m Model) handleRemapResult(msg modal.RemapResultMsg) (tea.Model, tea.Cmd) {
	if m.previewingHistoryEntry == nil || len(msg.Decisions) == 0 {
		return m, nil
	}

	remappedInputs := make(map[string]string)

	for k, v := range m.previewingHistoryEntry.Inputs {
		remappedInputs[k] = v
	}

	for _, decision := range msg.Decisions {
		switch decision.Action {
		case modal.RemapActionDrop:
			delete(remappedInputs, decision.OriginalName)
		case modal.RemapActionKeep:
		case modal.RemapActionMap:
			if val, exists := remappedInputs[decision.OriginalName]; exists {
				delete(remappedInputs, decision.OriginalName)
				remappedInputs[decision.NewName] = val
			}
		}
	}

	m.previewingHistoryEntry.Inputs = remappedInputs

	return m, nil
}

func (m Model) handleChainSelectResult(msg modal.ChainSelectResultMsg) (tea.Model, tea.Cmd) {
	if m.ghClient == nil || m.watcher == nil {
		return m, nil
	}

	chainDef := msg.Chain
	executor := chain.NewExecutor(m.ghClient, m.watcher, msg.ChainName, &chainDef)
	m.chainExecutor = executor

	if err := executor.Start(m.inputs, m.branch); err != nil {
		return m, nil
	}

	m.modalStack.Push(modal.NewChainStatusModal(executor.State()))

	return m, m.chainSubscription()
}

func (m Model) handleChainUpdate(msg ChainUpdateMsg) (tea.Model, tea.Cmd) {
	if m.chainExecutor == nil {
		return m, nil
	}

	state := msg.Update.State
	if state.Status == chain.ChainCompleted || state.Status == chain.ChainFailed {
		m.chainExecutor = nil
		return m, nil
	}

	return m, m.chainSubscription()
}

func (m Model) handleValidationErrorResult(msg modal.ValidationErrorResultMsg) (tea.Model, tea.Cmd) {
	if msg.Override {
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
	}
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

func (m Model) watcherSubscription() tea.Cmd {
	if m.watcher == nil {
		return nil
	}
	return func() tea.Msg {
		update := <-m.watcher.Updates()
		return RunUpdateMsg{Update: update}
	}
}

func (m Model) chainSubscription() tea.Cmd {
	if m.chainExecutor == nil {
		return nil
	}
	return func() tea.Msg {
		update := <-m.chainExecutor.Updates()
		return ChainUpdateMsg{Update: update}
	}
}
