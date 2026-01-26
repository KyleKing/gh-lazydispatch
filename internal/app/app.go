// Package app implements the main Bubbletea application for the interactive workflow dispatcher TUI.
package app

import (
	"context"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/git"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
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
	branch           string
	inputs           map[string]string
	inputOrder       []string
	watchRun         bool

	modalStack *modal.Stack

	pendingInputName string

	selectedInput          int
	viewMode               ViewMode
	filterText             string
	filteredInputs         []string
	previewingHistoryEntry *frecency.HistoryEntry

	ghClient    *github.Client
	watcher     *watcher.RunWatcher
	logManager  *logs.Manager
	logStreamer *logs.LogStreamer

	wfdConfig     *config.WfdConfig
	chainExecutor *chain.ChainExecutor

	pendingChainName      string
	pendingChain          *config.Chain
	pendingChainVariables map[string]string
	pendingChainCommands  []string

	// Metadata for the currently executing chain
	executingChainName      string
	executingChainBranch    string
	executingChainVariables map[string]string

	rightPanel panes.TabbedRightModel

	width  int
	height int
	keys   KeyMap
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
		rightPanel:       panes.NewTabbedRight(),
	}

	if ghClient, err := github.NewClient(repo); err == nil {
		m.ghClient = ghClient
		m.watcher = watcher.NewWatcher(ghClient)

		// Initialize log manager
		cacheDir, _ := os.UserCacheDir()
		logCacheDir := filepath.Join(cacheDir, "lazydispatch", "logs")
		m.logManager = logs.NewManager(ghClient, logCacheDir)
		m.logManager.LoadCache()
	}

	if cfg, err := config.Load("."); err == nil && cfg != nil {
		m.wfdConfig = cfg
		m.rightPanel.SetChains(cfg.Chains)
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
		leftWidth := (m.width * 11) / 30
		rightWidth := m.width - leftWidth
		topHeight := (m.height - 1) / 2
		m.rightPanel.SetSize(rightWidth, topHeight)

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

	case modal.LiveViewClearMsg:
		if m.watcher != nil {
			m.watcher.Unwatch(msg.RunID)
		}

		return m, nil

	case modal.LiveViewClearAllMsg:
		if m.watcher != nil {
			m.watcher.ClearCompleted()
		}

		return m, nil

	case RunUpdateMsg:
		if m.watcher != nil {
			m.rightPanel.SetRuns(m.watcher.GetRuns())
		}

		return m, m.watcherSubscription()

	case ChainUpdateMsg:
		return m.handleChainUpdate(msg)

	case modal.ChainSelectResultMsg:
		return m.handleChainSelectResult(msg)

	case modal.ChainVariableResultMsg:
		return m.handleChainVariableResult(msg)

	case modal.ChainConfirmResultMsg:
		return m.handleChainConfirmResult(msg)

	case modal.ChainStatusStopMsg:
		return m.handleChainStatusStop()

	case modal.ChainStatusViewLogsMsg:
		return m, func() tea.Msg {
			return FetchLogsMsg{
				ChainState: &msg.State,
				Branch:     msg.Branch,
				ErrorsOnly: msg.ErrorsOnly,
			}
		}

	case modal.ValidationErrorResultMsg:
		return m.handleValidationErrorResult(msg)

	case FetchLogsMsg:
		return m, m.fetchLogs(msg)

	case LogsFetchedMsg:
		if msg.Error != nil {
			m.modalStack.Push(modal.NewErrorModal("Failed to Fetch Logs", msg.Error.Error()))
			return m, nil
		}

		return m, func() tea.Msg {
			return ShowLogsViewerMsg{
				Logs:       msg.Logs,
				ErrorsOnly: msg.ErrorsOnly,
				RunID:      msg.RunID,
				Workflow:   msg.Workflow,
			}
		}

	case ShowLogsViewerMsg:
		m = m.showLogsViewer(msg.Logs, msg.ErrorsOnly, msg.RunID, msg.Workflow)

		// Start streaming if the modal enabled it
		if topModal := m.modalStack.Current(); topModal != nil {
			if viewer, ok := topModal.(*modal.LogsViewerModal); ok && viewer.IsStreaming() {
				return m, m.startLogStream(msg.RunID, msg.Workflow)
			}
		}

		return m, nil

	case StartLogStreamMsg:
		return m, m.startLogStream(msg.RunID, msg.Workflow)

	case LogStreamUpdateMsg:
		// Update the logs viewer modal if it's on top
		if topModal := m.modalStack.Current(); topModal != nil {
			if viewer, ok := topModal.(*modal.LogsViewerModal); ok && viewer.IsStreaming() {
				viewer.AppendStreamUpdate(msg.Update)
			}
		}

		return m, m.logStreamSubscription()

	case StopLogStreamMsg:
		m.stopLogStream()
		return m, nil

	case panes.HistoryViewLogsMsg:
		return m, func() tea.Msg {
			// Reconstruct chain state from history entry
			chainState := reconstructChainStateFromHistory(msg.Entry)

			return FetchLogsMsg{
				ChainState: &chainState,
				Branch:     msg.Entry.Branch,
				ErrorsOnly: false,
			}
		}

	case tea.KeyMsg:
		return m.handleKeyMsg(msg)
	}

	return m, nil
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
