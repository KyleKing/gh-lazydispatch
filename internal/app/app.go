// Package app implements the main Bubbletea application for the interactive workflow dispatcher TUI.
package app

import (
	"context"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/git"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// FocusedPane represents which pane currently has focus.
type FocusedPane int

// Panes available in the TUI, in tab order: the left column top to bottom,
// then the right panel.
const (
	PaneWorkflows FocusedPane = iota
	PaneChains
	PaneConfig
	PaneRight
)

// ViewMode represents the current view mode.
type ViewMode int

// View modes for the config pane.
const (
	WorkflowListMode ViewMode = iota
	HistoryPreviewMode
	InputDetailMode
)

// leftPaneWidthNumerator/Denominator sizes the left column as a fraction of
// terminal width, between leftPaneMinWidth and leftPaneMaxWidth cells.
const (
	leftPaneWidthNumerator   = 3
	leftPaneWidthDenominator = 10
	leftPaneMinWidth         = 24
	leftPaneMaxWidth         = 40
)

// Model is the root bubbletea model for the application.
type Model struct {
	pendingChainVariables   map[string]string
	watcher                 *watcher.RunWatcher
	history                 *frecency.Store
	executingChainVariables map[string]string
	pendingChain            *config.Chain
	chainExecutor           *chain.ChainExecutor
	inputs                  map[string]string
	wfdConfig               *config.WfdConfig
	logStreamer             *logs.LogStreamer
	modalStack              *modal.Stack
	ghClient                *github.Client
	logManager              *logs.Manager
	previewingHistoryEntry  *frecency.HistoryEntry
	registry                Registry
	chains                  panes.ChainListModel
	detailRunName           string
	markedWorkflows         ui.MarkSet
	pendingRuns             []runner.RunConfig
	commandInput            textinput.Model
	completions             []Candidate
	pendingActions          []paneAction
	status                  string
	repo                    string
	executingChainName      string
	executingChainBranch    string
	branch                  string
	pendingChainName        string
	pendingInputName        string
	filterText              string
	keys                    KeyMap
	inputOrder              []string
	filteredInputs          []string
	pendingChainCommands    []string
	workflows               []workflow.File
	rightPanel              panes.TabbedRightModel
	detailRunID             int64
	height                  int
	lastLeftPane            FocusedPane
	viewMode                ViewMode
	focused                 FocusedPane
	selectedWorkflow        int
	width                   int
	selectedInput           int
	watchRun                bool
	commandMode             bool
}

// RunUpdateMsg is sent when a watched run is updated.
type RunUpdateMsg struct {
	Update watcher.RunUpdate
}

// ChainUpdateMsg is sent when a chain execution state changes.
type ChainUpdateMsg struct {
	Update chain.ChainUpdate
}

// New creates a new application model.
func New(workflows []workflow.File, history *frecency.Store, repo string) Model {
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
		rightPanel:       panes.NewTabbedRight(),
		chains:           panes.NewChainListModel(),
		registry:         DefaultRegistry(),
		commandInput:     newCommandInput(),
	}

	if ghClient, err := github.NewClient(repo); err == nil {
		m.ghClient = ghClient
		m.watcher = watcher.NewWatcher(ghClient)

		m.logManager = logs.NewManager(ghClient)
	}

	if cfg, err := config.Load("."); err == nil && cfg != nil {
		m.wfdConfig = cfg
		m.chains.SetChains(cfg.Chains)
	}

	if len(workflows) > 0 {
		m.selectedWorkflow = 0
		m.initializeInputs(workflows[0])
	} else {
		m.syncHistoryEntries()
	}

	return m
}

// Init implements tea.Model.
// Init loads the tab the panel opens on. The question a reader starts with is
// whether their branch is green, so that answer is fetched rather than waiting
// for them to find the tab holding it.
func (m Model) Init() tea.Cmd {
	return m.loadRunsIfNeeded()
}

// Update implements tea.Model.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if sized, ok := msg.(tea.WindowSizeMsg); ok {
		return m.handleWindowSize(sized), nil
	}

	// Everything a background operation or a closing modal sends reaches its
	// own handler before the modal stack sees it. A modal reporting on that
	// work is usually the active one, so routing by "is a modal open" is what
	// stopped a chain's status ever updating and dropped a modal's result when
	// a keystroke raced it.
	if model, cmd, handled := m.handleModalResultMsg(msg); handled {
		return model, cmd
	}

	if model, cmd, handled := m.handleChainMsg(msg); handled {
		return model, cmd
	}

	if model, cmd, handled := m.handleLogMsg(msg); handled {
		return model, cmd
	}

	if model, cmd, handled := m.handleTimelineMsg(msg); handled {
		return model, cmd
	}

	if model, cmd, handled := m.handleRunsMsg(msg); handled {
		return model, cmd
	}

	if model, cmd, handled := m.handleFlakyMsg(msg); handled {
		return model, cmd
	}

	if done, ok := msg.(executionDoneMsg); ok {
		return m.handleExecutionDone(done)
	}

	if status, ok := msg.(StatusMsg); ok {
		m.status = status.Text

		return m, nil
	}

	// The command bar owns every key while it is open, so a command's own
	// letters cannot also fire the actions they are bound to.
	if m.commandMode {
		if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
			return m.handleCommandKey(keyMsg)
		}

		return m, nil
	}

	if m.modalStack.HasActive() {
		return m.updateModal(msg)
	}

	if keyMsg, ok := msg.(tea.KeyPressMsg); ok {
		m.status = ""

		return m.handleKeyMsg(keyMsg)
	}

	return m, nil
}

// handleWindowSize applies a terminal resize to the model's layout.
func (m Model) handleWindowSize(msg tea.WindowSizeMsg) Model {
	m.width = msg.Width
	m.height = msg.Height
	m.modalStack.SetSize(msg.Width, msg.Height)

	box := m.layout()
	m.rightPanel.SetSize(box.rightWidth, box.rightHeight)

	return m
}

// handleModalResultMsg dispatches messages carrying the result of a closed modal.
func (m Model) handleModalResultMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case modal.SelectResultMsg:
		model, cmd := m.handleSelectResult(msg)
		return model, cmd, true

	case modal.BranchResultMsg:
		model, cmd := m.handleBranchResult(msg)
		return model, cmd, true

	case modal.InputResultMsg:
		model, cmd := m.handleInputResult(msg)
		return model, cmd, true

	case modal.ConfirmResultMsg:
		model, cmd := m.handleConfirmResult(msg)
		return model, cmd, true

	case modal.FilterResultMsg:
		model, cmd := m.handleFilterResult(msg)
		return model, cmd, true

	case modal.ResetResultMsg:
		model, cmd := m.handleResetResult(msg)
		return model, cmd, true

	case modal.RunConfirmResultMsg:
		model, cmd := m.handleRunConfirmResult(msg)
		return model, cmd, true

	case modal.RemapResultMsg:
		model, cmd := m.handleRemapResult(msg)
		return model, cmd, true

	case modal.ValidationErrorResultMsg:
		model, cmd := m.handleValidationErrorResult(msg)
		return model, cmd, true

	case modal.ActionResultMsg:
		model, cmd := m.handleActionResult(msg)
		return model, cmd, true
	}

	return m, nil, false
}

// handleChainMsg dispatches messages related to chain execution and its modals.
func (m Model) handleChainMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case modal.LiveViewClearMsg:
		if m.watcher != nil {
			m.watcher.Unwatch(msg.RunID)
		}

		return m, nil, true

	case modal.LiveViewClearAllMsg:
		if m.watcher != nil {
			m.watcher.ClearCompleted()
		}

		return m, nil, true

	case RunUpdateMsg:
		m.refreshWatchedRuns()
		return m, m.watcherSubscription(), true

	case ChainUpdateMsg:
		model, cmd := m.handleChainUpdate(msg)
		return model, cmd, true

	case modal.ChainSelectResultMsg:
		model, cmd := m.handleChainSelectResult(msg)
		return model, cmd, true

	case modal.ChainVariableResultMsg:
		model, cmd := m.handleChainVariableResult(msg)
		return model, cmd, true

	case modal.ChainConfirmResultMsg:
		model, cmd := m.handleChainConfirmResult(msg)
		return model, cmd, true

	case modal.ChainStatusStopMsg:
		model, cmd := m.handleChainStatusStop()
		return model, cmd, true

	case modal.ChainStatusViewLogsMsg:
		return m, func() tea.Msg {
			return FetchLogsMsg{
				ChainState: &msg.State,
				Branch:     msg.Branch,
				ErrorsOnly: msg.ErrorsOnly,
			}
		}, true
	}

	return m, nil, false
}

// handleLogMsg dispatches messages related to fetching, viewing, and streaming workflow logs.
func (m Model) handleLogMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case FetchLogsMsg:
		return m, m.fetchLogs(msg), true

	case LogsFetchedMsg:
		if msg.Error != nil {
			m.modalStack.Push(modal.NewErrorModal("Failed to Fetch Logs", msg.Error.Error()))
			return m, nil, true
		}

		return m, func() tea.Msg {
			return ShowLogsViewerMsg{
				Logs:       msg.Logs,
				ErrorsOnly: msg.ErrorsOnly,
				RunID:      msg.RunID,
				Workflow:   msg.Workflow,
			}
		}, true

	case ShowLogsViewerMsg:
		m = m.showLogsViewer(msg.Logs, msg.ErrorsOnly, msg.RunID, msg.Workflow)

		if !m.topLogsViewerIsStreaming() {
			return m, nil, true
		}

		cmd := m.startLogStream(msg.RunID, msg.Workflow)

		return m, cmd, true

	case StartLogStreamMsg:
		cmd := m.startLogStream(msg.RunID, msg.Workflow)

		return m, cmd, true

	case LogStreamUpdateMsg:
		m.appendStreamUpdateToTopViewer(msg.Update)
		return m, m.logStreamSubscription(), true

	case StopLogStreamMsg:
		m.stopLogStream()
		return m, nil, true

	case panes.HistoryViewLogsMsg:
		return m, func() tea.Msg {
			// Reconstruct chain state from history entry
			chainState := reconstructChainStateFromHistory(msg.Entry)

			return FetchLogsMsg{
				ChainState: &chainState,
				Branch:     msg.Entry.Branch,
				ErrorsOnly: false,
			}
		}, true
	}

	return m, nil, false
}

// refreshWatchedRuns syncs the Live tab's run list from the watcher, if any.
func (m *Model) refreshWatchedRuns() {
	if m.watcher != nil {
		m.rightPanel.SetRuns(m.watcher.GetRuns())
	}
}

// topLogsViewerIsStreaming reports whether the topmost modal is a streaming logs viewer.
func (m Model) topLogsViewerIsStreaming() bool {
	viewer, ok := m.topLogsViewer()
	return ok && viewer.IsStreaming()
}

// appendStreamUpdateToTopViewer forwards a log stream update to the topmost
// modal, if it is a streaming logs viewer.
func (m Model) appendStreamUpdateToTopViewer(update logs.StreamUpdate) {
	if viewer, ok := m.topLogsViewer(); ok && viewer.IsStreaming() {
		viewer.AppendStreamUpdate(update)
	}
}

// topLogsViewer returns the topmost modal as a *modal.LogsViewerModal, if that's what it is.
func (m Model) topLogsViewer() (*modal.LogsViewerModal, bool) {
	topModal := m.modalStack.Current()
	if topModal == nil {
		return nil, false
	}

	viewer, ok := topModal.(*modal.LogsViewerModal)

	return viewer, ok
}

func (m Model) updateModal(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Check if the current modal is a streaming logs viewer
	var wasStreaming bool

	if current := m.modalStack.Current(); current != nil {
		if viewer, ok := current.(*modal.LogsViewerModal); ok {
			wasStreaming = viewer.IsStreaming() && viewer.IsDone()
		}
	}

	cmd := m.modalStack.Update(msg)

	// If a streaming modal was closed, stop the stream
	if wasStreaming {
		m.stopLogStream()
	}

	return m, cmd
}

// reconstructChainStateFromHistory converts a history entry to a chain state for log viewing.
func reconstructChainStateFromHistory(entry frecency.HistoryEntry) chain.ChainState {
	stepResults := make(map[int]*chain.StepResult)
	stepStatuses := make([]chain.StepStatus, len(entry.StepResults))

	for i, result := range entry.StepResults {
		status := chain.StepCompleted

		switch result.Status {
		case "completed":
			status = chain.StepCompleted
		case "failed":
			status = chain.StepFailed
		case "skipped":
			status = chain.StepSkipped
		case "pending":
			status = chain.StepPending
		case "running":
			status = chain.StepRunning
		case "waiting":
			status = chain.StepWaiting
		}

		stepStatuses[i] = status
		stepResults[i] = &chain.StepResult{
			Workflow:   result.Workflow,
			RunID:      result.RunID,
			Status:     status,
			Conclusion: result.Conclusion,
		}
	}

	return chain.ChainState{
		ChainName:    entry.ChainName,
		CurrentStep:  len(entry.StepResults) - 1,
		StepResults:  stepResults,
		StepStatuses: stepStatuses,
		Status:       chain.ChainCompleted,
	}
}
