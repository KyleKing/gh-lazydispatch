package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/exec"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
)

// newDispatchingModel wires the model to a GitHub client backed by a mock
// executor, so the chain flow runs its real dispatch path without reaching
// GitHub. Nothing here shells out: exec.MockExecutor is the seam, and
// runner.SetExecutor is package-global, which rules out t.Parallel for every
// test that calls this.
func newDispatchingModel(t *testing.T) Model {
	t.Helper()
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	mockExec := exec.NewMockExecutor()
	runner.SetExecutor(mockExec)
	t.Cleanup(func() { runner.SetExecutor(nil) })

	client, err := github.NewClientWithExecutor("owner/repo", mockExec)
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}

	m := newChainModel(t)
	m.ghClient = client
	m.watcher = watcher.NewWatcher(client)

	t.Cleanup(m.watcher.Stop)

	return m
}

// confirmChain answers the confirmation modal without draining the command it
// returns. That command is the chain subscription, which yields an update and
// resubscribes forever, so following it is not something a test can finish.
func confirmChain(t *testing.T, m Model) Model {
	t.Helper()

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
	if cmd == nil {
		t.Fatal("confirming produced no result message")
	}

	next, _ := asModel(t, updated).Update(cmd())

	return asModel(t, next)
}

// TestJourney_ConfirmingAChainRecordsWhatItWillRun covers the handoff from
// preview to execution: the commands the status modal shows have to be the
// ones the executor was handed, or the preview is describing a different run.
//
//nolint:paralleltest // newDispatchingModel mutates runner's package-global executor
func TestJourney_ConfirmingAChainRecordsWhatItWillRun(t *testing.T) {
	m := newDispatchingModel(t)

	m = openChainVariables(t, m)
	m = pressSpecial(t, m, tea.KeyRight)
	m = pressSpecial(t, m, tea.KeyEnter)
	m = confirmChain(t, m)

	if m.chainExecutor == nil {
		t.Fatal("confirming did not start an executor")
	}

	t.Cleanup(m.chainExecutor.Stop)

	if len(m.pendingChainCommands) != len(testChainConfig().Chains[testChainName].Steps) {
		t.Fatalf("recorded %d commands for a %d step chain",
			len(m.pendingChainCommands), len(testChainConfig().Chains[testChainName].Steps))
	}

	for _, command := range m.pendingChainCommands {
		if !strings.HasPrefix(command, "gh workflow run") {
			t.Errorf("recorded command %q is not a dispatch", command)
		}

		if strings.Contains(command, "{{") {
			t.Errorf("recorded command %q was never interpolated", command)
		}
	}

	if m.executingChainName != testChainName {
		t.Errorf("executing chain is %q, want %q", m.executingChainName, testChainName)
	}

	if m.pendingChain != nil || m.pendingChainName != "" {
		t.Error("pending state survived the handoff to the executor")
	}

	if view := m.View().Content; !strings.Contains(view, testChainName) {
		t.Errorf("the status modal does not name the chain:\n%s", view)
	}
}

// TestJourney_StoppingAChainReleasesItsExecutor keeps the stop key from
// leaving a goroutine polling GitHub after the user has walked away.
//
//nolint:paralleltest // newDispatchingModel mutates runner's package-global executor
func TestJourney_StoppingAChainReleasesItsExecutor(t *testing.T) {
	m := newDispatchingModel(t)

	m = openChainVariables(t, m)
	m = pressSpecial(t, m, tea.KeyEnter)
	m = confirmChain(t, m)

	if m.chainExecutor == nil {
		t.Fatal("confirming did not start an executor")
	}

	updated, _ := m.handleChainStatusStop()

	if asModel(t, updated).chainExecutor != nil {
		t.Error("stopping left the executor attached")
	}
}

// TestJourney_ClearingLiveRuns covers the two Live-tab keys that drop runs
// from the watcher, both of which are no-ops unless that tab has focus.
//
//nolint:paralleltest // newDispatchingModel mutates runner's package-global executor
func TestJourney_ClearingLiveRuns(t *testing.T) {
	m := newDispatchingModel(t)
	m.watcher.Watch(1, "ci.yml")
	m.watcher.Watch(2, "deploy.yml")
	m.rightPanel.SetRuns(m.watcher.GetRuns())

	if got := m.watcher.TotalCount(); got != 2 {
		t.Fatalf("watching %d runs, want 2", got)
	}

	// The workflows pane has focus, so neither key may touch the watcher.
	m = pressRune(t, m, 'd')
	m = pressRune(t, m, 'D')

	if got := m.watcher.TotalCount(); got != 2 {
		t.Errorf("clearing from the wrong pane dropped runs: %d left", got)
	}

	m.focused = PaneHistory
	for m.rightPanel.ActiveTab() != panes.TabLive {
		m = pressRune(t, m, 'l')
	}

	m = pressRune(t, m, 'd')

	if got := m.watcher.TotalCount(); got != 1 {
		t.Errorf("d left %d runs, want 1", got)
	}
}

// TestJourney_EscapeReturnsToTheWorkflowList pins the one key that has to work
// from every view, since it is how a user gets unstuck.
func TestJourney_EscapeReturnsToTheWorkflowList(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneHistory
	m = selectHistoryEntryWithInput(t, m, testInputEnvironment, "prod")
	m = pressSpecial(t, m, tea.KeyEnter)

	if m.viewMode != HistoryPreviewMode {
		t.Fatal("did not enter preview mode")
	}

	m = pressSpecial(t, m, tea.KeyEscape)

	if m.viewMode != WorkflowListMode {
		t.Errorf("view mode is %v after escape, want the workflow list", m.viewMode)
	}

	if m.previewingHistoryEntry != nil {
		t.Error("escape left a history entry in preview")
	}

	if m.selectedInput != -1 {
		t.Errorf("escape left input %d selected", m.selectedInput)
	}
}
