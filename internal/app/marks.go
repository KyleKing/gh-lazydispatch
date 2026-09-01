package app

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// toggleMark marks the row under the cursor in whichever list holds one.
// Marking is only offered where a verb can act on a set: dispatching several
// workflows, and dropping several watched runs.
func (m *Model) toggleMark() {
	switch m.focused {
	case PaneWorkflows:
		if wf := m.SelectedWorkflow(); wf != nil {
			m.markedWorkflows.Toggle(wf.Filename)
		}
	case PaneRight:
		if m.rightPanel.ActiveTab() == panes.TabLive {
			m.rightPanel.Live().ToggleMark()
		}
	case PaneChains, PaneConfig:
	}
}

// markedWorkflowFiles are the marked workflows, or the selected one when
// nothing is marked, so a verb always has something to act on.
func (m Model) markedWorkflowFiles() []string {
	if m.markedWorkflows.Len() > 0 {
		return m.markedWorkflows.Keys()
	}

	if wf := m.SelectedWorkflow(); wf != nil {
		return []string{wf.Filename}
	}

	return nil
}

// runMarkedWorkflows confirms and dispatches every marked workflow.
//
// Only the selected workflow carries the values in the config pane; the rest go
// out with the defaults they declare, because there is one config pane and the
// set has many workflows. The confirmation lists the commands in full, which is
// where that shows.
func (m Model) runMarkedWorkflows() (tea.Model, tea.Cmd) {
	files := m.markedWorkflowFiles()
	if len(files) == 0 {
		return m, statusCmd("nothing marked to run")
	}

	configs := make([]runner.RunConfig, 0, len(files))
	preview := make([]string, 0, len(files))

	for _, file := range files {
		cfg := m.runConfigFor(file)
		if cfg == nil {
			continue
		}

		configs = append(configs, *cfg)
		preview = append(preview, "gh "+strings.Join(runner.BuildArgs(*cfg), " "))
	}

	if len(configs) == 0 {
		return m, statusCmd("nothing marked to run")
	}

	m.pendingRuns = configs
	m.modalStack.Push(modal.NewConfirmModal(
		"Run "+strconv.Itoa(len(configs))+" marked workflows",
		strings.Join(preview, "\n"),
		true, true,
	))

	return m, nil
}

// runConfigFor builds the dispatch for one workflow file, using the live input
// values only for the workflow the config pane is actually showing.
func (m Model) runConfigFor(file string) *runner.RunConfig {
	for i := range m.workflows {
		wf := &m.workflows[i]
		if wf.Filename != file {
			continue
		}

		inputs := make(map[string]string)

		if i == m.selectedWorkflow {
			for name, value := range m.inputs {
				inputs[name] = value
			}
		} else {
			for name, input := range wf.GetInputs() {
				inputs[name] = input.Default
			}
		}

		return &runner.RunConfig{Workflow: file, Branch: m.branch, Inputs: inputs, Watch: m.watchRun}
	}

	return nil
}

// dispatchPending runs the confirmed batch in order, each through the same
// path a single dispatch takes.
func (m Model) dispatchPending() (tea.Model, tea.Cmd) {
	configs := m.pendingRuns
	m.pendingRuns = nil
	m.markedWorkflows.Clear()

	cmds := make([]tea.Cmd, 0, len(configs))
	for _, cfg := range configs {
		cmds = append(cmds, m.dispatchCmd(cfg))
	}

	return m, tea.Sequence(cmds...)
}

// unwatchMarkedRuns stops watching every marked run.
func (m Model) unwatchMarkedRuns() (tea.Model, tea.Cmd) {
	live := m.rightPanel.Live()

	ids := live.MarkedRuns()
	if len(ids) == 0 || m.watcher == nil {
		return m, statusCmd("nothing to stop watching")
	}

	for _, id := range ids {
		m.watcher.Unwatch(id)
	}

	live.ClearMarks()
	m.rightPanel.SetRuns(m.watcher.GetRuns())

	return m, statusCmd("stopped watching " + strconv.Itoa(len(ids)) + " runs")
}

// markLabel names how many rows a verb would act on, so the action menu says
// whether it is about to act on a set or on the cursor.
func markLabel(n int) string {
	if n == 0 {
		return "the selected one"
	}

	return strconv.Itoa(n) + " marked"
}
