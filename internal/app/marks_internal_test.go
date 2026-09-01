package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// Marking turns the workflow list into an operator-object grammar: `space`
// names the set, and enter runs all of it behind one confirmation.
func TestMarkedWorkflows_RunTheWholeSetBehindOneConfirmation(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneWorkflows

	for range len(m.workflows) {
		m.toggleMark()
		m.handleDown()
	}

	if got := m.markedWorkflows.Len(); got != len(m.workflows) {
		t.Fatalf("marked %d workflows, want %d", got, len(m.workflows))
	}

	model, _ := m.handleEnter()

	marked, ok := model.(Model)
	if !ok {
		t.Fatalf("enter returned %T, want a Model", model)
	}

	if len(marked.pendingRuns) != len(m.workflows) {
		t.Fatalf("confirming %d dispatches, want %d", len(marked.pendingRuns), len(m.workflows))
	}

	if !marked.modalStack.HasActive() {
		t.Fatal("a marked set dispatched without a confirmation")
	}

	// The confirmation spells every command, because only the selected
	// workflow carries the config pane's values and the rest go out with the
	// defaults they declare.
	view := marked.View().Content
	for _, wf := range m.workflows {
		if !strings.Contains(view, wf.Filename) {
			t.Errorf("the confirmation does not name %s:\n%s", wf.Filename, view)
		}
	}

	// Declining leaves nothing pending and nothing dispatched.
	model, _ = marked.handleConfirmResult(modal.ConfirmResultMsg{Value: false})

	declined, ok := model.(Model)
	if !ok {
		t.Fatalf("declining returned %T, want a Model", model)
	}

	if len(declined.pendingRuns) != 0 {
		t.Error("declining left the batch pending")
	}
}

// A verb with nothing marked acts on the cursor, so marking is an option
// rather than a step.
func TestMarkedWorkflows_FallBackToTheSelection(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneWorkflows

	wf := m.SelectedWorkflow()
	if wf == nil {
		t.Fatal("no workflow is selected")
	}

	files := m.markedWorkflowFiles()
	if len(files) != 1 || files[0] != wf.Filename {
		t.Errorf("with nothing marked the verb acts on %v, want the selection", files)
	}

	if got := markLabel(0); !strings.Contains(got, "selected") {
		t.Errorf("an empty mark set is labeled %q", got)
	}
}

// Space on a live row marks a run, and `d` then drops every marked run rather
// than only the one under the cursor.
//
//nolint:paralleltest // newDispatchingModel mutates runner's package-global executor
func TestMarkedRuns_UnwatchTheWholeSet(t *testing.T) {
	m := newDispatchingModel(t)
	m.watcher.Watch(1, "ci.yml")
	m.watcher.Watch(2, "deploy.yml")
	m.watcher.Watch(3, "release.yml")
	m.rightPanel.SetRuns(m.watcher.GetRuns())

	m.focused = PaneRight
	m.rightPanel.SetTab(panes.TabLive)

	m = pressRune(t, m, ' ')
	m = pressSpecial(t, m, tea.KeyDown)
	m = pressRune(t, m, ' ')

	if got := m.rightPanel.Live().MarkCount(); got != 2 {
		t.Fatalf("marked %d runs, want 2", got)
	}

	m = pressRune(t, m, 'd')

	if got := m.watcher.TotalCount(); got != 1 {
		t.Errorf("unwatching the marked set left %d runs, want 1", got)
	}

	if got := m.rightPanel.Live().MarkCount(); got != 0 {
		t.Errorf("%d marks survived the verb that consumed them", got)
	}
}
