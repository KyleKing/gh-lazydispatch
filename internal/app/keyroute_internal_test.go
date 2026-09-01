package app

import (
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// Each case here stands for a binding that was previously shadowed: digits by
// the input branch, `l` by the live-view modal, and Enter by an empty case.
func TestKeyRouting_DigitsFollowFocusedPane(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneWorkflows

	m = pressRune(t, m, '2')
	if m.selectedWorkflow != 1 {
		t.Fatalf("digit in the workflows pane selected %d, want 1", m.selectedWorkflow)
	}

	m = pressRune(t, m, '0')
	if m.selectedWorkflow != -1 {
		t.Fatalf("0 in the workflows pane selected %d, want -1", m.selectedWorkflow)
	}

	m.selectedWorkflow = 0
	m.initializeInputs(m.workflows[0])
	m.syncFilteredInputs()
	m.focused = PaneConfig

	m = pressRune(t, m, '1')
	if !m.modalStack.HasActive() {
		t.Fatal("digit in the config pane opened no input modal")
	}
}

func TestKeyRouting_BracketMovesTabsAndLOpensLiveView(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneRight

	m = pressRune(t, m, ']')
	if got := m.rightPanel.ActiveTab(); got != panes.TabLive {
		t.Fatalf("] moved to tab %v, want %v", got, panes.TabLive)
	}

	if m.modalStack.HasActive() {
		t.Fatal("] opened a modal instead of switching tabs")
	}

	m.watcher = nil
	m = pressRune(t, m, 'L')

	if m.modalStack.HasActive() {
		t.Fatal("L opened the live view with no watcher attached")
	}
}

func TestKeyRouting_EnterInWorkflowsPaneConfirmsTheRun(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneWorkflows
	m = pressRune(t, m, '2')

	m = pressSpecial(t, m, '\r')
	if !m.modalStack.HasActive() {
		t.Fatal("Enter in the workflows pane opened no run confirmation")
	}
}
