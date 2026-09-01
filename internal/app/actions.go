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
		return actionMenu{title: paneWorkflows, target: m.selectedWorkflowName(), actions: workflowActions()}

	case PaneConfig:
		return actionMenu{title: "Configuration", target: m.selectedWorkflowName(), actions: configActions()}

	case PaneHistory:
		switch m.rightPanel.ActiveTab() {
		case panes.TabHistory:
			return actionMenu{title: "History", target: m.historyTarget(), actions: m.historyActions()}
		case panes.TabChains:
			return actionMenu{title: "Chains", target: m.chainsTarget(), actions: chainActions()}
		case panes.TabLive:
			return actionMenu{title: "Live", target: m.liveTarget(), actions: liveActions()}
		case panes.TabTimeline:
			return actionMenu{title: "Timeline", target: m.timelineTarget(), actions: timelineActions()}
		case panes.TabRuns:
			return actionMenu{title: "Runs", target: m.runsTarget(), actions: runsActions()}
		}
	}

	return actionMenu{title: paneWorkflows, actions: workflowActions()}
}

func workflowActions() []paneAction {
	return []paneAction{
		{key: "b", name: nameBranch, run: Model.openBranchModal},
		{key: "c", name: "run a chain", run: Model.openChainSelectModal},
		{key: keyEnter, name: "run the selected workflow", run: Model.executeWorkflow},
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
		{key: "v", name: "view logs", run: Model.viewSelectedLogs},
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

func liveActions() []paneAction {
	return []paneAction{
		{key: "L", name: "open the live overview", run: Model.openLiveViewModal},
		{key: "t", name: nameTimelineAction, run: Model.timelineForSelection},
		{key: "d", name: "stop watching the selected run", run: Model.clearSelectedRunAction},
		{key: "D", name: "stop watching every completed run", run: Model.clearCompletedRunsAction},
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
	name, chainDef, ok := m.rightPanel.SelectedChain()
	if !ok {
		return m, statusCmd("no chain selected")
	}

	return m.startChainFlow(name, chainDef)
}

func (m Model) clearSelectedRunAction() (tea.Model, tea.Cmd) {
	m.clearSelectedLiveRun()

	return m, nil
}

func (m Model) clearCompletedRunsAction() (tea.Model, tea.Cmd) {
	m.clearCompletedLiveRuns()

	return m, nil
}

// selectedWorkflowName names what a workflow or config verb would act on.
func (m Model) selectedWorkflowName() string {
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
	name, chainDef, ok := m.rightPanel.SelectedChain()
	if !ok {
		return "no chain selected"
	}

	return fmt.Sprintf("%s (%s steps)", name, strconv.Itoa(len(chainDef.Steps)))
}

func (m Model) liveTarget() string {
	run, ok := m.rightPanel.SelectedRun()
	if !ok {
		return "no run selected"
	}

	return run.Workflow + " " + run.Status
}

func timelineActions() []paneAction {
	return []paneAction{
		{key: keyEnter, name: "drill into the selected job's steps", run: Model.drillTimeline},
		{key: "esc", name: "back out to the jobs", run: Model.undrillTimeline},
	}
}

func (m Model) drillTimeline() (tea.Model, tea.Cmd) {
	m.rightPanel.Timeline().Drill()

	return m, nil
}

func (m Model) undrillTimeline() (tea.Model, tea.Cmd) {
	m.rightPanel.Timeline().Undrill()

	return m, nil
}

func (m Model) runsTarget() string {
	run, ok := m.rightPanel.SelectedGitHubRun()
	if !ok {
		return m.rightPanel.Runs().Scope().Label()
	}

	return run.Name
}

func (m Model) timelineTarget() string {
	return m.rightPanel.Timeline().Heading()
}
