package app

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/git"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/rule"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
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

	case key.Matches(msg, m.keys.Space):
		if m.focused == PaneWorkflows {
			m.focused = PaneConfig
			return m, nil
		}
		return m, nil

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

	case key.Matches(msg, m.keys.TabNext):
		if m.focused == PaneHistory {
			m.rightPanel.NextTab()
			return m, nil
		}
		return m, nil

	case key.Matches(msg, m.keys.TabPrev):
		if m.focused == PaneHistory {
			m.rightPanel.PrevTab()
			return m, nil
		}
		return m, nil

	case key.Matches(msg, m.keys.Clear):
		if m.focused == PaneHistory && m.rightPanel.ActiveTab() == panes.TabLive {
			if run, ok := m.rightPanel.SelectedRun(); ok {
				if m.watcher != nil {
					m.watcher.Unwatch(run.RunID)
					m.rightPanel.SetRuns(m.watcher.GetRuns())
				}
			}
			return m, nil
		}
		return m, nil

	case key.Matches(msg, m.keys.ClearAll):
		if m.focused == PaneHistory && m.rightPanel.ActiveTab() == panes.TabLive {
			if m.watcher != nil {
				m.watcher.ClearCompleted()
				m.rightPanel.SetRuns(m.watcher.GetRuns())
			}
			return m, nil
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

	case msg.String() == "l":
		if m.focused == PaneHistory && m.rightPanel.ActiveTab() == panes.TabHistory {
			return m, m.rightPanel.History().HandleViewLogs()
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
		switch m.rightPanel.ActiveTab() {
		case panes.TabHistory:
			m.rightPanel.History().MoveUp()
		case panes.TabChains:
			m.rightPanel.Chains().MoveUp()
		case panes.TabLive:
			m.rightPanel.Live().MoveUp()
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
		switch m.rightPanel.ActiveTab() {
		case panes.TabHistory:
			m.rightPanel.History().MoveDown()
		case panes.TabChains:
			m.rightPanel.Chains().MoveDown()
		case panes.TabLive:
			m.rightPanel.Live().MoveDown()
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
		switch m.rightPanel.ActiveTab() {
		case panes.TabHistory:
			entry := m.rightPanel.SelectedHistoryEntry()
			if entry != nil {
				if m.viewMode == HistoryPreviewMode {
					m.branch = entry.Branch
					m.inputs = make(map[string]string)
					for k, v := range entry.Inputs {
						m.inputs[k] = v
					}
					m.viewMode = WorkflowListMode
					m.previewingHistoryEntry = nil
					return m.executeWorkflow()
				}
				m.viewMode = HistoryPreviewMode
				m.previewingHistoryEntry = entry
			}
		case panes.TabChains:
			if name, chainDef, ok := m.rightPanel.SelectedChain(); ok {
				return m.startChainFlow(name, chainDef)
			}
		}
	case PaneConfig:
		return m.executeWorkflow()
	}
	return m, nil
}

func (m Model) startChainFlow(name string, chainDef config.Chain) (tea.Model, tea.Cmd) {
	m.pendingChainName = name
	m.pendingChain = &chainDef

	if len(chainDef.Variables) > 0 {
		m.modalStack.Push(modal.NewChainVariableModal(name, &chainDef))
		return m, nil
	}

	m.pendingChainVariables = nil
	m.modalStack.Push(modal.NewChainConfirmModal(name, &chainDef, nil, m.branch, m.watchRun))
	return m, nil
}

func (m Model) handleChainVariableResult(msg modal.ChainVariableResultMsg) (tea.Model, tea.Cmd) {
	if msg.Cancelled || m.pendingChain == nil {
		m.pendingChainName = ""
		m.pendingChain = nil
		return m, nil
	}

	m.pendingChainVariables = msg.Variables
	m.modalStack.Push(modal.NewChainConfirmModal(
		m.pendingChainName,
		m.pendingChain,
		msg.Variables,
		m.branch,
		m.watchRun,
	))
	return m, nil
}

func (m Model) handleChainConfirmResult(msg modal.ChainConfirmResultMsg) (tea.Model, tea.Cmd) {
	if !msg.Confirmed || m.pendingChain == nil {
		m.pendingChainName = ""
		m.pendingChain = nil
		m.pendingChainVariables = nil
		return m, nil
	}

	if m.ghClient == nil || m.watcher == nil {
		return m, nil
	}

	chainDef := m.pendingChain
	chainName := msg.ChainName
	variables := msg.Variables
	branch := msg.Branch

	commands := m.buildChainCommands(chainDef, variables, branch)
	m.pendingChainCommands = commands

	executor := chain.NewExecutor(m.ghClient, m.watcher, chainName, chainDef)
	m.chainExecutor = executor

	if err := executor.Start(variables, branch); err != nil {
		return m, nil
	}

	// Store executing chain metadata for history update on completion
	m.executingChainName = chainName
	m.executingChainBranch = branch
	m.executingChainVariables = variables

	m.history.RecordChain(m.repo, chainName, branch, variables, nil)
	m.history.Save()

	statusModal := modal.NewChainStatusModalWithCommands(executor.State(), commands, branch)
	m.modalStack.Push(statusModal)

	m.pendingChainName = ""
	m.pendingChain = nil
	m.pendingChainVariables = nil

	return m, m.chainSubscription()
}

func (m Model) buildChainCommands(chainDef *config.Chain, variables map[string]string, branch string) []string {
	commands := make([]string, len(chainDef.Steps))

	ctx := &chain.InterpolationContext{
		Var:   variables,
		Steps: make(map[int]*chain.StepResult),
	}

	for i, step := range chainDef.Steps {
		inputs, _ := chain.InterpolateInputs(step.Inputs, ctx)

		cfg := runner.RunConfig{
			Workflow: step.Workflow,
			Branch:   branch,
			Inputs:   inputs,
		}
		args := runner.BuildArgs(cfg)
		commands[i] = runner.FormatCommand(args)

		ctx.Steps[i] = &chain.StepResult{
			Workflow: step.Workflow,
			Inputs:   inputs,
		}
		if i > 0 {
			ctx.Previous = ctx.Steps[i-1]
		}
	}

	return commands
}

func (m Model) handleChainStatusStop() (tea.Model, tea.Cmd) {
	if m.chainExecutor != nil {
		m.chainExecutor.Stop()
		m.chainExecutor = nil
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
	return m.startChainFlow(msg.ChainName, msg.Chain)
}

func (m Model) handleChainUpdate(msg ChainUpdateMsg) (tea.Model, tea.Cmd) {
	if m.chainExecutor == nil {
		return m, nil
	}

	state := msg.Update.State
	if state.Status == chain.ChainCompleted || state.Status == chain.ChainFailed {
		// Convert chain step results to frecency step results for history
		stepResults := convertToFrecencyStepResults(state.StepResults)

		// Update history with step results
		m.history.RecordChain(m.repo, m.executingChainName, m.executingChainBranch, m.executingChainVariables, stepResults)
		m.history.Save()

		// Clear executing chain metadata
		m.executingChainName = ""
		m.executingChainBranch = ""
		m.executingChainVariables = nil
		m.chainExecutor = nil
		return m, nil
	}

	return m, m.chainSubscription()
}

// convertToFrecencyStepResults converts chain.StepResult to frecency.ChainStepResult
func convertToFrecencyStepResults(stepResults map[int]*chain.StepResult) []frecency.ChainStepResult {
	if len(stepResults) == 0 {
		return nil
	}

	// Find the max index to create properly sized slice
	maxIdx := -1
	for idx := range stepResults {
		if idx > maxIdx {
			maxIdx = idx
		}
	}

	results := make([]frecency.ChainStepResult, maxIdx+1)
	for idx, result := range stepResults {
		if result != nil {
			status := string(result.Status)
			results[idx] = frecency.ChainStepResult{
				Workflow:   result.Workflow,
				RunID:      result.RunID,
				Status:     status,
				Conclusion: result.Conclusion,
			}
		}
	}

	return results
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

func (m Model) fetchLogs(msg FetchLogsMsg) tea.Cmd {
	return func() tea.Msg {
		if m.logManager == nil {
			return LogsFetchedMsg{Error: fmt.Errorf("log manager not initialized")}
		}

		var runLogs *logs.RunLogs
		var err error
		var runID int64
		var workflow string

		if msg.ChainState != nil {
			runLogs, err = m.logManager.GetLogsForChain(*msg.ChainState, msg.Branch)
			// For chains, get runID from first step if available
			if runLogs != nil && len(runLogs.Steps) > 0 {
				runID = runLogs.Steps[0].RunID
				workflow = runLogs.Steps[0].Workflow
			}
		} else if msg.RunID != 0 {
			runLogs, err = m.logManager.GetLogsForRun(msg.RunID, msg.Workflow)
			runID = msg.RunID
			workflow = msg.Workflow
		} else {
			return LogsFetchedMsg{Error: fmt.Errorf("no chain state or run ID provided")}
		}

		return LogsFetchedMsg{
			Logs:       runLogs,
			ErrorsOnly: msg.ErrorsOnly,
			RunID:      runID,
			Workflow:   workflow,
			Error:      err,
		}
	}
}

func (m Model) showLogsViewer(runLogs *logs.RunLogs, errorsOnly bool, runID int64, workflow string) Model {
	var logsModal modal.Context
	if errorsOnly {
		logsModal = modal.NewLogsViewerModalWithError(runLogs, m.width, m.height)
	} else {
		logsModal = modal.NewLogsViewerModal(runLogs, m.width, m.height)
	}

	// Check if this is an active run and enable streaming
	if runID != 0 && m.ghClient != nil {
		run, err := m.ghClient.GetWorkflowRun(runID)
		if err == nil && (run.Status == "queued" || run.Status == "in_progress") {
			// Enable streaming on the modal
			if viewer, ok := logsModal.(*modal.LogsViewerModal); ok {
				viewer.EnableStreaming(runID, true)
			}
		}
	}

	m.modalStack.Push(logsModal)
	return m
}

func (m *Model) startLogStream(runID int64, workflow string) tea.Cmd {
	// Stop any existing streamer
	if m.logStreamer != nil {
		m.logStreamer.Stop()
	}

	// Create and start new streamer
	m.logStreamer = logs.NewLogStreamer(m.ghClient, runID, workflow)
	m.logStreamer.Start()

	return m.logStreamSubscription()
}

func (m Model) logStreamSubscription() tea.Cmd {
	if m.logStreamer == nil {
		return nil
	}
	return func() tea.Msg {
		update := <-m.logStreamer.Updates()
		return LogStreamUpdateMsg{Update: update}
	}
}

func (m *Model) stopLogStream() {
	if m.logStreamer != nil {
		m.logStreamer.Stop()
		m.logStreamer = nil
	}
}
