package app

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

func testWorkflows() []workflow.WorkflowFile {
	return []workflow.WorkflowFile{
		{
			Name:     "Deploy",
			Filename: "deploy.yml",
			On: workflow.OnTrigger{
				WorkflowDispatch: &workflow.WorkflowDispatch{
					Inputs: map[string]workflow.WorkflowInput{
						"environment": {
							Type:    "choice",
							Default: "staging",
							Options: []string{"production", "staging"},
						},
					},
				},
			},
		},
		{
			Name:     "CI",
			Filename: "ci.yml",
			On: workflow.OnTrigger{
				WorkflowDispatch: &workflow.WorkflowDispatch{},
			},
		},
	}
}

func testHistory() *frecency.Store {
	store := frecency.NewStore()
	store.Record("owner/repo", "deploy.yml", "main", map[string]string{"environment": "prod"})
	return store
}

func TestNew(t *testing.T) {
	workflows := testWorkflows()
	history := testHistory()

	m := New(workflows, history, "owner/repo")

	if m.focused != PaneWorkflows {
		t.Errorf("expected initial focus on PaneWorkflows, got %d", m.focused)
	}

	if m.selectedWorkflow != 0 {
		t.Errorf("expected selectedWorkflow 0, got %d", m.selectedWorkflow)
	}

	if m.inputs["environment"] != "staging" {
		t.Errorf("expected environment default 'staging', got %q", m.inputs["environment"])
	}
}

func TestUpdate_Tab(t *testing.T) {
	m := New(testWorkflows(), testHistory(), "owner/repo")

	msg := tea.KeyMsg{Type: tea.KeyTab}
	result, _ := m.Update(msg)
	m = result.(Model)

	if m.focused != PaneHistory {
		t.Errorf("expected focus on PaneHistory after tab, got %d", m.focused)
	}

	result, _ = m.Update(msg)
	m = result.(Model)

	if m.focused != PaneConfig {
		t.Errorf("expected focus on PaneConfig after second tab, got %d", m.focused)
	}

	result, _ = m.Update(msg)
	m = result.(Model)

	if m.focused != PaneWorkflows {
		t.Errorf("expected focus back on PaneWorkflows after third tab, got %d", m.focused)
	}
}

func TestUpdate_ShiftTab(t *testing.T) {
	m := New(testWorkflows(), testHistory(), "owner/repo")

	msg := tea.KeyMsg{Type: tea.KeyShiftTab}
	result, _ := m.Update(msg)
	m = result.(Model)

	if m.focused != PaneConfig {
		t.Errorf("expected focus on PaneConfig after shift-tab, got %d", m.focused)
	}
}

func TestUpdate_UpDown_Workflows(t *testing.T) {
	m := New(testWorkflows(), testHistory(), "owner/repo")

	down := tea.KeyMsg{Type: tea.KeyDown}
	result, _ := m.Update(down)
	m = result.(Model)

	if m.selectedWorkflow != 1 {
		t.Errorf("expected selectedWorkflow 1 after down, got %d", m.selectedWorkflow)
	}

	up := tea.KeyMsg{Type: tea.KeyUp}
	result, _ = m.Update(up)
	m = result.(Model)

	if m.selectedWorkflow != 0 {
		t.Errorf("expected selectedWorkflow 0 after up, got %d", m.selectedWorkflow)
	}
}

func TestUpdate_Watch(t *testing.T) {
	m := New(testWorkflows(), testHistory(), "owner/repo")

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}}
	result, _ := m.Update(msg)
	m = result.(Model)

	if !m.watchRun {
		t.Error("expected watchRun to be true after 'w'")
	}

	result, _ = m.Update(msg)
	m = result.(Model)

	if m.watchRun {
		t.Error("expected watchRun to be false after second 'w'")
	}
}

func TestUpdate_WindowSize(t *testing.T) {
	m := New(testWorkflows(), testHistory(), "owner/repo")

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	result, _ := m.Update(msg)
	m = result.(Model)

	if m.width != 120 {
		t.Errorf("expected width 120, got %d", m.width)
	}
	if m.height != 40 {
		t.Errorf("expected height 40, got %d", m.height)
	}
}

func TestView_NotEmpty(t *testing.T) {
	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.width = 120
	m.height = 40

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
	if len(view) < 100 {
		t.Errorf("expected view to be substantial, got length %d", len(view))
	}
}

func TestSelectedWorkflow(t *testing.T) {
	m := New(testWorkflows(), testHistory(), "owner/repo")

	wf := m.SelectedWorkflow()
	if wf == nil {
		t.Fatal("expected non-nil workflow")
	}
	if wf.Filename != "deploy.yml" {
		t.Errorf("expected 'deploy.yml', got %q", wf.Filename)
	}
}
