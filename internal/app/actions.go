package app

import (
	"fmt"
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// Nouns shared across command names, action names, key help, and pane titles,
// so the same thing reads the same everywhere it is named.
const (
	keyEnter           = "enter"
	nameBranch         = "branch"
	nameRunsTab        = "Runs"
	nameCancelRun      = "cancel this run"
	nameRerunFailed    = "re-run the failed jobs"
	nameViewLogs       = "view logs"
	nameTimelineAction = "timeline for this run"
	nameWorkflow       = "workflow"
	paneWorkflows      = "Workflows"
)

// paneAction is one verb the action leader offers, scoped to a pane. Keys are
// scoped rather than global, so the same letter can mean different things
// where it means different things, and each stays mnemonic.
type paneAction struct {
	run  func(Model) (tea.Model, tea.Cmd)
	key  string
	name string
}

// actionMenu is what the leader is acting on and what it offers.
type actionMenu struct {
	title   string
	target  string
	actions []paneAction
}

// actionsFor returns the verbs that apply where focus currently is. This is
// the single place that decides scope: a handler that used to guard itself
// with "am I the focused pane" now simply is not offered.
func (m Model) actionsFor() actionMenu {
	switch m.focused {
	case PaneWorkflows:
		return actionMenu{title: paneWorkflows, target: m.selectedWorkflowName(), actions: m.workflowActions()}

	case PaneChains:
		return actionMenu{title: "Chains", target: m.chainsTarget(), actions: chainActions()}

	case PaneConfig:
		return actionMenu{title: "Configuration", target: m.selectedWorkflowName(), actions: configActions()}

	case PaneRight:
		if m.rightPanel.Detail() != nil {
			return actionMenu{title: "Run", target: m.timelineTarget(), actions: timelineActions()}
		}

		switch m.rightPanel.ActiveTab() {
		case panes.TabHistory:
			return actionMenu{title: "History", target: m.historyTarget(), actions: m.historyActions()}
		case panes.TabLive:
			return actionMenu{title: "Live", target: m.liveTarget(), actions: m.liveActions()}
		case panes.TabFlaky:
			return actionMenu{title: "Flaky", target: m.flakyTarget(), actions: flakyActions()}
		case panes.TabRuns:
			return actionMenu{title: nameRunsTab, target: m.runsTarget(), actions: runsActions()}
		}
	}

	return actionMenu{title: paneWorkflows, actions: m.workflowActions()}
}

func (m Model) workflowActions() []paneAction {
	run := paneAction{key: keyEnter, name: "run the selected workflow", run: Model.executeWorkflow}
	if n := m.markedWorkflows.Len(); n > 0 {
		run = paneAction{key: keyEnter, name: "run the " + markLabel(n) + " workflows", run: Model.runMarkedWorkflows}
	}

	return []paneAction{
		{key: "b", name: nameBranch, run: Model.openBranchModal},
		{key: "c", name: "run a chain", run: Model.openChainSelectModal},
		run,
		{key: "w", name: "toggle watch after dispatch", run: Model.toggleWatch},
	}
}

func configActions() []paneAction {
	return []paneAction{
		{key: "b", name: nameBranch, run: Model.openBranchModal},
		{key: "f", name: "filter inputs", run: Model.openFilterModal},
		{key: "r", name: "reset inputs to their defaults", run: Model.openResetModal},
		{key: "w", name: "toggle watch after dispatch", run: Model.toggleWatch},
		{key: "y", name: "copy the dispatch command", run: Model.copyCommandToClipboard},
	}
}

func (m Model) historyActions() []paneAction {
	actions := []paneAction{
		{key: "t", name: nameTimelineAction, run: Model.timelineForSelection},
		{key: "v", name: nameViewLogs, run: Model.viewSelectedLogs},
	}

	// Remapping only means something for an entry whose inputs no longer fit
	// the workflow, which is why it was unreachable while it lived on a bare
	// key with no way to see whether it applied.
	if m.viewMode == HistoryPreviewMode && m.previewingHistoryEntry != nil {
		actions = append(actions, paneAction{key: "m", name: "remap the stale inputs", run: Model.openRemapModal})
	}

	return actions
}

func chainActions() []paneAction {
	return []paneAction{
		{key: "c", name: "run the selected chain", run: Model.runSelectedChain},
	}
}

func (m Model) liveActions() []paneAction {
	marked := m.rightPanel.Live().MarkCount()

	return []paneAction{
		{key: "L", name: "open the live overview", run: Model.openLiveViewModal},
		{key: "t", name: nameTimelineAction, run: Model.timelineForSelection},
		{key: "d", name: "stop watching " + markLabel(marked), run: Model.unwatchMarkedRuns},
		{key: "D", name: "stop watching every completed run", run: Model.clearCompletedRunsAction},
		{key: "x", name: nameCancelRun, run: Model.cancelSelectedRun},
		{key: "z", name: nameRerunFailed, run: Model.rerunSelectedRun},
	}
}

// openActionMenu pushes the menu for whatever has focus and remembers its
// verbs, so the chosen key resolves against the same set that was shown.
func (m *Model) openActionMenu() {
	menu := m.actionsFor()
	m.pendingActions = menu.actions

	items := make([]modal.ActionItem, 0, len(menu.actions))
	for _, action := range menu.actions {
		items = append(items, modal.ActionItem{Key: action.key, Name: action.name})
	}

	m.modalStack.Push(modal.NewActionMenuModal(menu.title, menu.target, items))
}

// handleActionResult runs the verb the menu returned.
func (m Model) handleActionResult(msg modal.ActionResultMsg) (tea.Model, tea.Cmd) {
	actions := m.pendingActions
	m.pendingActions = nil

	if msg.Key == "" {
		return m, nil
	}

	for _, action := range actions {
		if action.key == msg.Key {
			return action.run(m)
		}
	}

	return m, nil
}

// The verbs below adapt existing handlers to paneAction's signature, so the
// menu and the direct keys run exactly the same code.

func (m Model) toggleWatch() (tea.Model, tea.Cmd) {
	m.watchRun = !m.watchRun

	return m, nil
}

func (m Model) viewSelectedLogs() (tea.Model, tea.Cmd) {
	return m, m.rightPanel.History().HandleViewLogs()
}

func (m Model) runSelectedChain() (tea.Model, tea.Cmd) {
	name, chainDef, ok := m.chains.SelectedChain()
	if !ok {
		return m, statusCmd("no chain selected")
	}

	return m.startChainFlow(name, chainDef)
}

func (m Model) clearCompletedRunsAction() (tea.Model, tea.Cmd) {
	m.clearCompletedLiveRuns()

	return m, nil
}

// selectedWorkflowName names what a workflow or config verb would act on.
func (m Model) selectedWorkflowName() string {
	if n := m.markedWorkflows.Len(); n > 0 {
		return markLabel(n) + " workflows on " + m.branchLabel()
	}

	if m.selectedWorkflow < 0 || m.selectedWorkflow >= len(m.workflows) {
		return "all workflows"
	}

	wf := m.workflows[m.selectedWorkflow]

	return wf.Name + " (" + wf.Filename + ") on " + m.branchLabel()
}

func (m Model) branchLabel() string {
	if m.branch == "" {
		return "the default branch"
	}

	return m.branch
}

func (m Model) historyTarget() string {
	entry := m.rightPanel.SelectedHistoryEntry()
	if entry == nil {
		return "no entry selected"
	}

	if entry.ChainName != "" {
		return entry.ChainName + " on " + entry.Branch
	}

	return entry.Workflow + " on " + entry.Branch
}

func (m Model) chainsTarget() string {
	name, chainDef, ok := m.chains.SelectedChain()
	if !ok {
		return "no chain selected"
	}

	return fmt.Sprintf("%s (%s steps)", name, strconv.Itoa(len(chainDef.Steps)))
}

func (m Model) liveTarget() string {
	if n := m.rightPanel.Live().MarkCount(); n > 0 {
		return markLabel(n) + " runs"
	}

	run, ok := m.rightPanel.SelectedRun()
	if !ok {
		return "no run selected"
	}

	return run.Workflow + " " + run.Status
}

func timelineActions() []paneAction {
	return []paneAction{
		{key: "d", name: "diagnose the failure", run: Model.diagnoseSelectedRun},
		{key: keyEnter, name: "drill into the selected job's steps", run: Model.drillTimeline},
		{key: "esc", name: "back out of the run", run: Model.undrillTimeline},
		{key: "v", name: nameViewLogs, run: Model.logsForSelectedRun},
		{key: "x", name: nameCancelRun, run: Model.cancelSelectedRun},
		{key: "z", name: nameRerunFailed, run: Model.rerunSelectedRun},
	}
}

func flakyActions() []paneAction {
	return []paneAction{
		{key: "R", name: "reload the run history", run: Model.reloadFlaky},
		{key: "t", name: nameTimelineAction, run: Model.timelineForSelection},
		{key: "v", name: nameViewLogs, run: Model.logsForSelectedRun},
		{key: "z", name: nameRerunFailed, run: Model.rerunSelectedRun},
	}
}

func (m Model) drillTimeline() (tea.Model, tea.Cmd) {
	if detail := m.rightPanel.Detail(); detail != nil {
		detail.Drill()
	}

	return m, nil
}

// undrillTimeline peels one layer: a job's steps first, then the run itself.
func (m Model) undrillTimeline() (tea.Model, tea.Cmd) {
	detail := m.rightPanel.Detail()
	if detail == nil {
		return m, nil
	}

	if detail.Drilled() {
		detail.Undrill()

		return m, nil
	}

	m.rightPanel.CloseDetail()

	return m, nil
}

// flakyTarget names the row a pass-rate verb would act on.
func (m Model) flakyTarget() string {
	if run, ok := m.rightPanel.Flaky().SelectedRun(); ok {
		return run.Name + " on " + run.HeadBranch
	}

	return m.rightPanel.Flaky().Title()
}

func (m Model) runsTarget() string {
	if pr, ok := m.rightPanel.Runs().SelectedPR(); ok {
		return "#" + strconv.Itoa(pr.Number)
	}

	if run, ok := m.rightPanel.SelectedGitHubRun(); ok {
		return run.Name
	}

	return m.rightPanel.Runs().Scope().Label()
}

func (m Model) timelineTarget() string {
	if detail := m.rightPanel.Detail(); detail != nil {
		return detail.Heading()
	}

	return "no run open"
}
