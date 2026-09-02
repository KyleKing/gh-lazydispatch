package app

import (
	"context"
	"errors"
	"os/exec"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

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

// paneCount is the number of focusable panes cycled through by Tab/Shift+Tab.
const paneCount = 4

// boolTrueValue is the string representation of a "true" boolean workflow input.
const boolTrueValue = "true"

const inputTypeChoice = "choice"

// ErrLogManagerNotInitialized indicates logs were requested before the log manager was set up.
var ErrLogManagerNotInitialized = errors.New("log manager not initialized")

// ErrNoChainStateOrRunID indicates a log fetch request lacked both chain state and a run ID.
var ErrNoChainStateOrRunID = errors.New("no chain state or run ID provided")

func (m Model) handleKeyMsg(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if model, cmd, handled := m.handleGlobalKey(msg); handled {
		return model, cmd
	}

	if model, handled := m.handleFocusKey(msg); handled {
		return model, nil
	}

	if model, cmd, handled := m.handlePaneKey(msg); handled {
		return model, cmd
	}

	return m.handleKeyMsgFallback(msg)
}

// handleGlobalKey handles keys with behavior independent of the focused pane.
func (m Model) handleGlobalKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Quit):
		return m, tea.Quit, true

	case key.Matches(msg, m.keys.Help):
		m.modalStack.Push(modal.NewHelpModal())
		return m, nil, true

	case key.Matches(msg, m.keys.Command):
		cmd := m.openCommandBar()

		return m, cmd, true

	case key.Matches(msg, m.keys.Actions):
		m.openActionMenu()
		return m, nil, true

	case key.Matches(msg, m.keys.Escape):
		m.handleEscapeKey()
		return m, nil, true

	case key.Matches(msg, m.keys.Up):
		m.handleUp()
		return m, nil, true

	case key.Matches(msg, m.keys.Down):
		m.handleDown()
		return m, nil, true

	case key.Matches(msg, m.keys.Enter):
		model, cmd := m.handleEnter()
		return model, cmd, true

	case key.Matches(msg, m.keys.Watch):
		m.watchRun = !m.watchRun
		return m, nil, true

	case key.Matches(msg, m.keys.Branch):
		model, cmd := m.openBranchModal()
		return model, cmd, true

	case key.Matches(msg, m.keys.LiveView):
		model, cmd := m.openLiveViewModal()
		return model, cmd, true

	case key.Matches(msg, m.keys.Chain):
		model, cmd := m.openChainSelectModal()
		return model, cmd, true
	}

	return m, nil, false
}

// handleFocusKey moves focus between panes. `tab` walks the whole cycle, and
// `h`/`l` cross between the left column and the right panel, which is the move
// a two-column layout is actually asked for.
func (m Model) handleFocusKey(msg tea.KeyPressMsg) (tea.Model, bool) {
	switch {
	case key.Matches(msg, m.keys.Tab):
		m.focusPane(m.stepPane(1))
	case key.Matches(msg, m.keys.ShiftTab):
		m.focusPane(m.stepPane(-1))
	case key.Matches(msg, m.keys.PaneRight):
		m.focusPane(PaneRight)
	case key.Matches(msg, m.keys.PaneLeft):
		if m.focused == PaneRight {
			m.focusPane(m.lastLeftPane)
		}
	default:
		return m, false
	}

	return m, true
}

// handlePaneKey handles keys whose behavior depends on the focused pane or active tab.
func (m Model) handlePaneKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.Space):
		m.toggleMark()
		return m, nil, true

	case key.Matches(msg, m.keys.Edit):
		model, cmd := m.handleEditKey()
		return model, cmd, true

	case key.Matches(msg, m.keys.Filter):
		model, cmd := m.handleConfigPaneAction(m.openFilterModal)
		return model, cmd, true

	case key.Matches(msg, m.keys.Copy):
		model, cmd := m.handleConfigPaneAction(m.copyCommandToClipboard)
		return model, cmd, true

	case key.Matches(msg, m.keys.Reset):
		model, cmd := m.handleConfigPaneAction(m.openResetModal)
		return model, cmd, true
	}

	return m.handleHistoryPaneKey(msg)
}

// handleHistoryPaneKey handles keys specific to the history pane's tabs (History/Chains/Live).
func (m Model) handleHistoryPaneKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, m.keys.TabNext):
		cmd := m.cycleRightTab(m.rightPanel.NextTab)

		return m, cmd, true

	case key.Matches(msg, m.keys.TabPrev):
		cmd := m.cycleRightTab(m.rightPanel.PrevTab)

		return m, cmd, true
	}

	return m.handleRunListKey(msg)
}

// handleRunListKey handles the keys that act on whatever the right panel is
// listing. Each is inert outside the tab it means something in, rather than
// bound globally and silently doing nothing.
func (m Model) handleRunListKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	tab := m.rightPanel.ActiveTab()

	switch {
	case key.Matches(msg, m.keys.Scope):
		if m.focused != PaneRight || tab != panes.TabRuns {
			return m, nil, true
		}

		model, cmd := m.cycleRunsScope()

		return model, cmd, true

	case key.Matches(msg, m.keys.Reload):
		return m.reloadFocusedTab(tab)

	case key.Matches(msg, m.keys.Clear):
		if m.focused != PaneRight || tab != panes.TabLive {
			return m, nil, true
		}

		model, cmd := m.unwatchMarkedRuns()

		return model, cmd, true

	case key.Matches(msg, m.keys.ClearAll):
		m.clearCompletedLiveRuns()

		return m, nil, true

	case key.Matches(msg, m.keys.ViewLogs):
		if m.focused != PaneRight || tab != panes.TabHistory {
			return m, nil, true
		}

		return m, m.rightPanel.History().HandleViewLogs(), true
	}

	return m, nil, false
}

func (m Model) reloadFocusedTab(tab panes.RightTab) (tea.Model, tea.Cmd, bool) {
	if m.focused != PaneRight {
		return m, nil, true
	}

	if tab == panes.TabFlaky {
		model, cmd := m.reloadFlaky()

		return model, cmd, true
	}

	model, cmd := m.reloadRuns()

	return model, cmd, true
}

// stepPane is the next focusable pane in one direction, skipping the chains
// pane in a repository that configures none: the layout does not draw it, so
// stopping on it would be a stop with nothing on screen.
func (m Model) stepPane(step int) FocusedPane {
	next := m.focused

	for range paneCount {
		next = FocusedPane((int(next) + step + paneCount) % paneCount)
		if next != PaneChains || m.chains.Count() > 0 {
			return next
		}
	}

	return m.focused
}

// focusPane moves focus and remembers the left-column pane to come back to, so
// `l` then `h` returns where it left rather than to the top of the column.
func (m *Model) focusPane(pane FocusedPane) {
	if pane != PaneRight {
		m.lastLeftPane = pane
	}

	m.focused = pane
}

// handleEditKey opens the input modal when a config input is selected for editing.
func (m Model) handleEditKey() (tea.Model, tea.Cmd) {
	if m.viewMode == InputDetailMode && m.selectedInput >= 0 {
		return m.openInputModalFiltered(m.selectedInput)
	}

	return m, nil
}

// handleConfigPaneAction runs action only when the config pane is focused.
func (m Model) handleConfigPaneAction(action func() (tea.Model, tea.Cmd)) (tea.Model, tea.Cmd) {
	if m.focused != PaneConfig {
		return m, nil
	}

	return action()
}

// cycleRightTab moves the right panel's tabs and loads whatever the tab it
// lands on needs. It is not guarded on focus: the right panel is the only
// tabbed thing on screen, so `[` and `]` mean the same wherever the cursor is.
func (m *Model) cycleRightTab(cycle func()) tea.Cmd {
	cycle()

	return tea.Batch(m.loadRunsIfNeeded(), m.loadFlakyIfNeeded())
}

// handleEscapeKey backs out one level, innermost first: a job's steps, then the
// run they belong to, then a preview or input detail, then nothing. Peeling one
// layer per press is what keeps escape from throwing away two states at once.
func (m *Model) handleEscapeKey() {
	if detail := m.rightPanel.Detail(); detail != nil {
		if detail.Drilled() {
			detail.Undrill()
		} else {
			m.rightPanel.CloseDetail()
		}

		return
	}

	if m.viewMode == WorkflowListMode {
		return
	}

	m.viewMode = WorkflowListMode
	m.selectedInput = -1
	m.previewingHistoryEntry = nil
}

// clearCompletedLiveRuns stops watching all completed runs in the Live tab.
func (m *Model) clearCompletedLiveRuns() {
	if m.focused != PaneRight || m.rightPanel.ActiveTab() != panes.TabLive {
		return
	}

	if m.watcher == nil {
		return
	}

	m.watcher.ClearCompleted()
	m.rightPanel.SetRuns(m.watcher.GetRuns())
}

// handleKeyMsgFallback handles keys not covered by a named binding: dynamic
// input-select and workflow-select shortcuts.
func (m Model) handleKeyMsgFallback(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// InputKeys and WorkflowKeys bind the same digits, so the focused pane is
	// the only thing that tells them apart.
	if m.focused == PaneConfig {
		for i, k := range m.keys.InputKeys() {
			if key.Matches(msg, k) {
				return m.handleInputKey(i)
			}
		}

		return m, nil
	}

	for i, k := range m.keys.WorkflowKeys() {
		if key.Matches(msg, k) {
			return m.handleWorkflowKey(i)
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
		m.syncHistoryEntries()
		m.syncSelectedWorkflow()

		return m, nil
	}

	workflowIdx := num - 1
	if workflowIdx < len(m.workflows) {
		m.selectedWorkflow = workflowIdx
		m.initializeInputs(m.workflows[workflowIdx])
		m.syncSelectedWorkflow()
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
			} else {
				// Back on the "all workflows" row, where history is unfiltered.
				m.syncHistoryEntries()
			}

			m.syncSelectedWorkflow()
		}
	case PaneChains:
		m.chains.MoveUp()
	case PaneRight:
		m.rightPanel.MoveUp()
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

			m.syncSelectedWorkflow()
		}
	case PaneChains:
		m.chains.MoveDown()
	case PaneRight:
		m.rightPanel.MoveDown()
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
	case PaneWorkflows:
		if m.markedWorkflows.Len() > 0 {
			return m.runMarkedWorkflows()
		}

		return m.executeWorkflow()
	case PaneChains:
		if name, chainDef, ok := m.chains.SelectedChain(); ok {
			return m.startChainFlow(name, chainDef)
		}
	case PaneRight:
		return m.enterInRightPanel()
	case PaneConfig:
		return m.executeWorkflow()
	}

	return m, nil
}

// enterInRightPanel opens whatever the right panel has under the cursor.
func (m Model) enterInRightPanel() (tea.Model, tea.Cmd) {
	if detail := m.rightPanel.Detail(); detail != nil {
		detail.Drill()

		return m, nil
	}

	switch m.rightPanel.ActiveTab() {
	case panes.TabLive, panes.TabFlaky:
		return m.timelineForSelection()
	case panes.TabRuns:
		return m.openSelectedRunsRow()
	case panes.TabHistory:
		return m.enterHistoryEntry()
	}

	return m, nil
}

// enterHistoryEntry previews a past dispatch's inputs, and runs them on the
// second press: one key to see what would be sent, one to send it.
func (m Model) enterHistoryEntry() (tea.Model, tea.Cmd) {
	entry := m.rightPanel.SelectedHistoryEntry()
	if entry == nil {
		return m, nil
	}

	if m.viewMode != HistoryPreviewMode {
		m.viewMode = HistoryPreviewMode
		m.previewingHistoryEntry = entry

		return m, nil
	}

	m.branch = entry.Branch
	m.inputs = make(map[string]string, len(entry.Inputs))

	for k, v := range entry.Inputs {
		m.inputs[k] = v
	}

	m.viewMode = WorkflowListMode
	m.previewingHistoryEntry = nil

	return m.executeWorkflow()
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

//nolint:unparam // uniform (tea.Model, tea.Cmd) signature, required by handleModalResultMsg's dispatch
func (m Model) handleChainVariableResult(msg modal.ChainVariableResultMsg) (tea.Model, tea.Cmd) {
	if msg.Canceled || m.pendingChain == nil {
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
	//nolint:errcheck,gosec // best-effort persistence; a failed history write does not block dispatching the chain
	m.history.Save()

	statusModal := modal.NewChainStatusModalWithCommands(executor.State(), commands, branch)
	m.modalStack.Push(statusModal)

	m.pendingChainName = ""
	m.pendingChain = nil
	m.pendingChainVariables = nil

	return m, m.chainSubscription()
}

func (Model) buildChainCommands(chainDef *config.Chain, variables map[string]string, branch string) []string {
	resolved := chain.ResolveSteps(chainDef, variables, branch)

	commands := make([]string, len(resolved))
	for i, step := range resolved {
		commands[i] = step.Command
	}

	return commands
}

//nolint:unparam // uniform (tea.Model, tea.Cmd) signature, required by handleModalResultMsg's dispatch
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

// validateAllInputs collects everything that would make the dispatch fail: the
// repository's own rules, what GitHub rejects about a value's type, and an
// environment the repository does not have.
func (m Model) validateAllInputs(wf workflow.File) map[string][]string {
	errs := make(map[string][]string)

	inputs := wf.GetInputs()
	for name, input := range inputs {
		if rules := input.ValidationRules; len(rules) > 0 {
			if validationErrs := rule.ValidateValue(m.inputs[name], rules); len(validationErrs) > 0 {
				errs[name] = validationErrs
			}
		}

		if input.InputType() == inputTypeEnvironment {
			if message := validation.ValidateEnvironment(m.inputs[name], m.environments); message != "" {
				errs[name] = append(errs[name], message)
			}
		}
	}

	for name, messages := range validation.ValidateDispatch(inputs, m.inputs) {
		errs[name] = append(errs[name], messages...)
	}

	return errs
}

type executionDoneMsg struct {
	err error
}

// handleExecutionDone reports a failed dispatch and moves the right panel to
// the list that now has something to say. The message had no handler at all
// before, so a dispatch that failed looked exactly like one that worked.
func (m Model) handleExecutionDone(msg executionDoneMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		return m, statusCmd("dispatch failed: " + msg.err.Error())
	}

	m.syncHistoryEntries()

	tab := panes.TabHistory
	if m.watchRun {
		tab = panes.TabLive
	}

	m.rightPanel.SetTab(tab)
	m.focusPane(PaneRight)
	m.rightPanel.Runs().Invalidate()

	return m, nil
}

func (m Model) openBranchModal() (tea.Model, tea.Cmd) {
	ctx := context.Background()

	branches, err := git.FetchBranches(ctx)
	if err != nil {
		branches = git.DefaultBranches()
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

	// An environment input picks from the repository's environments, which only
	// the API names. Where they could not be read it falls back to free text,
	// which is what the text modal says it is doing.
	switch inputType := input.InputType(); {
	case inputType == "boolean":
		current := currentVal == boolTrueValue
		defaultVal := input.Default == boolTrueValue
		m.modalStack.Push(modal.NewConfirmModal(name, input.Description, current, defaultVal))
	case inputType == inputTypeChoice:
		m.modalStack.Push(modal.NewSelectModal(name, input.Description, input.Options, currentVal, input.Default))
	case inputType == inputTypeEnvironment && len(m.environments) > 0:
		m.modalStack.Push(modal.NewSelectModal(name, input.Description, m.environments, currentVal, input.Default))
	default:
		m.modalStack.Push(modal.NewInputModal(
			name, input.Description, input.Default, input.InputType(), currentVal, input.Options, input.ValidationRules,
		))
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

//nolint:unparam // uniform (tea.Model, tea.Cmd) signature, required by handleModalResultMsg's dispatch
func (m Model) handleSelectResult(msg modal.SelectResultMsg) (tea.Model, tea.Cmd) {
	if m.pendingInputName != "" {
		m.inputs[m.pendingInputName] = msg.Value
		m.pendingInputName = ""
	}

	return m, nil
}

//nolint:unparam // uniform (tea.Model, tea.Cmd) signature, required by handleModalResultMsg's dispatch
func (m Model) handleBranchResult(msg modal.BranchResultMsg) (tea.Model, tea.Cmd) {
	m.branch = msg.Value
	return m, nil
}

//nolint:unparam // uniform (tea.Model, tea.Cmd) signature, required by handleModalResultMsg's dispatch
func (m Model) handleInputResult(msg modal.InputResultMsg) (tea.Model, tea.Cmd) {
	if m.pendingInputName != "" {
		m.inputs[m.pendingInputName] = msg.Value
		m.pendingInputName = ""
	}

	return m, nil
}

func (m Model) handleConfirmResult(msg modal.ConfirmResultMsg) (tea.Model, tea.Cmd) {
	if len(m.pendingRuns) > 0 {
		if !msg.Value {
			m.pendingRuns = nil

			return m, nil
		}

		return m.dispatchPending()
	}

	if m.pendingInputName != "" {
		if msg.Value {
			m.inputs[m.pendingInputName] = boolTrueValue
		} else {
			m.inputs[m.pendingInputName] = "false"
		}

		m.pendingInputName = ""
	}

	return m, nil
}

//nolint:unparam // uniform (tea.Model, tea.Cmd) signature, required by handleModalResultMsg's dispatch
func (m Model) handleFilterResult(msg modal.FilterResultMsg) (tea.Model, tea.Cmd) {
	if !msg.Canceled {
		m.filterText = msg.Value
		m.applyFilter()
	}

	return m, nil
}

//nolint:unparam // uniform (tea.Model, tea.Cmd) signature, required by handleModalResultMsg's dispatch
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

//nolint:unparam // uniform (tea.Model, tea.Cmd) signature, required by handleModalResultMsg's dispatch
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
	m.refreshChainStatusModal(state)
	if state.Status == chain.ChainCompleted || state.Status == chain.ChainFailed {
		// Convert chain step results to frecency step results for history
		stepResults := convertToFrecencyStepResults(state.StepResults)

		// Update history with step results
		m.history.RecordChain(
			m.repo, m.executingChainName, m.executingChainBranch, m.executingChainVariables, stepResults,
		)
		//nolint:errcheck,gosec // best-effort persistence; failed history write doesn't affect the completed run
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

// refreshChainStatusModal pushes the executor's latest state into the status
// modal. The modal holds a snapshot rather than a live pointer, so without this
// it reports the chain's first step for the whole run.
func (m Model) refreshChainStatusModal(state chain.ChainState) {
	found := m.modalStack.Find(func(ctx modal.Context) bool {
		_, ok := ctx.(*modal.ChainStatusModal)
		return ok
	})

	if statusModal, ok := found.(*modal.ChainStatusModal); ok {
		statusModal.UpdateState(state)
	}
}

// convertToFrecencyStepResults converts chain.StepResult to frecency.ChainStepResult.
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

//nolint:unparam // uniform (tea.Model, tea.Cmd) signature, required by handleModalResultMsg's dispatch
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
	return m, m.dispatchCmd(cfg)
}

// dispatchCmd records the dispatch and hands back the command that runs it, so
// one workflow and a marked set travel exactly the same path.
func (m Model) dispatchCmd(cfg runner.RunConfig) tea.Cmd {
	m.history.Record(m.repo, cfg.Workflow, cfg.Branch, cfg.Inputs)
	//nolint:errcheck,gosec // best-effort persistence; failed history write doesn't block dispatching the workflow
	m.history.Save()

	//nolint:gosec,noctx // shells out to trusted gh CLI by design; tea.ExecProcess has no context to thread through
	cmd := exec.Command("gh", runner.BuildArgs(cfg)...)

	return tea.ExecProcess(cmd, func(err error) tea.Msg {
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
	//nolint:errcheck,gosec // best-effort clipboard write; no error-surfacing UI hook exists for this action
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
			return LogsFetchedMsg{Error: ErrLogManagerNotInitialized}
		}

		var runLogs *logs.RunLogs

		var err error

		var runID int64

		var workflowName string

		switch {
		case msg.ChainState != nil:
			runLogs, err = m.logManager.GetLogsForChain(*msg.ChainState, msg.Branch)
			// For chains, get runID from first step if available
			if runLogs != nil && len(runLogs.Steps) > 0 {
				runID = runLogs.Steps[0].RunID
				workflowName = runLogs.Steps[0].Workflow
			}
		case msg.RunID != 0:
			runLogs, err = m.logManager.GetLogsForRun(msg.RunID, msg.Workflow)
			runID = msg.RunID
			workflowName = msg.Workflow
		default:
			return LogsFetchedMsg{Error: ErrNoChainStateOrRunID}
		}

		return LogsFetchedMsg{
			Logs:       runLogs,
			ErrorsOnly: msg.ErrorsOnly,
			RunID:      runID,
			Workflow:   workflowName,
			Error:      err,
		}
	}
}

func (m Model) showLogsViewer(runLogs *logs.RunLogs, errorsOnly bool, runID int64, _ string) Model {
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

func (m *Model) startLogStream(runID int64, workflowName string) tea.Cmd {
	// Stop any existing streamer
	if m.logStreamer != nil {
		m.logStreamer.Stop()
	}

	// Create and start new streamer
	m.logStreamer = logs.NewLogStreamer(m.ghClient, runID, workflowName)
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
