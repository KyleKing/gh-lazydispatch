package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

const (
	testInputEnvironment = "environment"
	testValueStaging     = "staging"
	testValueCustom      = "custom"
)

func testWorkflows() []workflow.WorkflowFile {
	return []workflow.WorkflowFile{
		{
			Name:     "Deploy",
			Filename: "deploy.yml",
			On: workflow.OnTrigger{
				WorkflowDispatch: &workflow.WorkflowDispatch{
					Inputs: map[string]workflow.WorkflowInput{
						testInputEnvironment: {
							Type:    "choice",
							Default: testValueStaging,
							Options: []string{"production", testValueStaging},
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
	store.Record("owner/repo", "deploy.yml", "main", map[string]string{testInputEnvironment: "prod"})
	store.Record("owner/repo", "ci.yml", "main", map[string]string{})
	store.Record("owner/repo", "deploy.yml", "develop", map[string]string{testInputEnvironment: testValueStaging})

	return store
}

func TestNew(t *testing.T) {
	t.Parallel()

	workflows := testWorkflows()
	history := testHistory()

	m := New(workflows, history, "owner/repo")

	if m.focused != PaneWorkflows {
		t.Errorf("expected initial focus on PaneWorkflows, got %d", m.focused)
	}

	if m.selectedWorkflow != 0 {
		t.Errorf("expected selectedWorkflow 0, got %d", m.selectedWorkflow)
	}

	if m.inputs[testInputEnvironment] != testValueStaging {
		t.Errorf("expected environment default 'staging', got %q", m.inputs[testInputEnvironment])
	}
}

func TestUpdate_Tab(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")

	msg := tea.KeyPressMsg{Code: tea.KeyTab}
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
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")

	msg := tea.KeyPressMsg{Code: tea.KeyTab, Mod: tea.ModShift}
	result, _ := m.Update(msg)
	m = result.(Model)

	if m.focused != PaneConfig {
		t.Errorf("expected focus on PaneConfig after shift-tab, got %d", m.focused)
	}
}

func TestUpdate_UpDown_Workflows(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")

	down := tea.KeyPressMsg{Code: tea.KeyDown}
	result, _ := m.Update(down)
	m = result.(Model)

	if m.selectedWorkflow != 1 {
		t.Errorf("expected selectedWorkflow 1 after down, got %d", m.selectedWorkflow)
	}

	up := tea.KeyPressMsg{Code: tea.KeyUp}
	result, _ = m.Update(up)
	m = result.(Model)

	if m.selectedWorkflow != 0 {
		t.Errorf("expected selectedWorkflow 0 after up, got %d", m.selectedWorkflow)
	}
}

func TestUpdate_Watch(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")

	msg := tea.KeyPressMsg{Code: 'w', Text: "w"}
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
	t.Parallel()

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
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.width = 120
	m.height = 40

	view := m.View()
	if view.Content == "" {
		t.Error("expected non-empty view")
	}

	if len(view.Content) < 100 {
		t.Errorf("expected view to be substantial, got length %d", len(view.Content))
	}
}

func TestSelectedWorkflow(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")

	wf := m.SelectedWorkflow()
	if wf == nil {
		t.Fatal("expected non-nil workflow")
	}

	if wf.Filename != "deploy.yml" {
		t.Errorf("expected 'deploy.yml', got %q", wf.Filename)
	}
}

func TestUpdate_UpDown_History(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.focused = PaneHistory

	entries := m.currentHistoryEntries()
	if len(entries) < 2 {
		t.Skip("need at least 2 history entries for this test")
	}

	down := tea.KeyPressMsg{Code: tea.KeyDown}
	result, _ := m.Update(down)
	m = result.(Model)

	entry := m.rightPanel.SelectedHistoryEntry()
	if entry == nil {
		t.Error("expected selected history entry after down")
	}

	up := tea.KeyPressMsg{Code: tea.KeyUp}
	result, _ = m.Update(up)
	m = result.(Model)

	entry = m.rightPanel.SelectedHistoryEntry()
	if entry == nil {
		t.Error("expected selected history entry after up")
	}
}

func TestUpdate_UpDown_Config(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.focused = PaneConfig

	down := tea.KeyPressMsg{Code: tea.KeyDown}
	result, _ := m.Update(down)
	m = result.(Model)

	if m.selectedInput != 0 {
		t.Errorf("expected selectedInput 0 after down, got %d", m.selectedInput)
	}

	if m.viewMode != InputDetailMode {
		t.Errorf("expected InputDetailMode, got %d", m.viewMode)
	}

	up := tea.KeyPressMsg{Code: tea.KeyUp}
	result, _ = m.Update(up)
	m = result.(Model)

	if m.selectedInput != 0 {
		t.Errorf("expected selectedInput 0 (already at top), got %d", m.selectedInput)
	}
}

func TestUpdate_Space(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.focused = PaneWorkflows

	msg := tea.KeyPressMsg{Code: ' ', Text: " "}
	result, _ := m.Update(msg)
	m = result.(Model)

	if m.focused != PaneConfig {
		t.Errorf("expected focus on PaneConfig after space, got %d", m.focused)
	}
}

func TestHandleSelectResult(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.pendingInputName = testInputEnvironment

	result, _ := m.handleSelectResult(modal.SelectResultMsg{Value: "production"})
	m = result.(Model)

	if m.inputs[testInputEnvironment] != "production" {
		t.Errorf("expected environment=production, got %q", m.inputs[testInputEnvironment])
	}

	if m.pendingInputName != "" {
		t.Error("expected pendingInputName to be cleared")
	}
}

func TestHandleBranchResult(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")

	result, _ := m.handleBranchResult(modal.BranchResultMsg{Value: "feature/test"})
	m = result.(Model)

	if m.branch != "feature/test" {
		t.Errorf("expected branch=feature/test, got %q", m.branch)
	}
}

func TestHandleInputResult(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.pendingInputName = testInputEnvironment

	result, _ := m.handleInputResult(modal.InputResultMsg{Value: testValueStaging})
	m = result.(Model)

	if m.inputs[testInputEnvironment] != testValueStaging {
		t.Errorf("expected environment=staging, got %q", m.inputs[testInputEnvironment])
	}

	if m.pendingInputName != "" {
		t.Error("expected pendingInputName to be cleared")
	}
}

func TestHandleConfirmResult(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value bool
		want  string
	}{
		{"true value", true, "true"},
		{"false value", false, "false"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := New(testWorkflows(), testHistory(), "owner/repo")
			m.pendingInputName = "debug"

			result, _ := m.handleConfirmResult(modal.ConfirmResultMsg{Value: tt.value})
			m = result.(Model)

			if m.inputs["debug"] != tt.want {
				t.Errorf("expected debug=%s, got %q", tt.want, m.inputs["debug"])
			}

			if m.pendingInputName != "" {
				t.Error("expected pendingInputName to be cleared")
			}
		})
	}
}

func TestHandleFilterResult(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")

	result, _ := m.handleFilterResult(modal.FilterResultMsg{Value: "env", Canceled: false})
	m = result.(Model)

	if m.filterText != "env" {
		t.Errorf("expected filterText=env, got %q", m.filterText)
	}

	if m.selectedInput != -1 {
		t.Errorf("expected selectedInput=-1 after filter, got %d", m.selectedInput)
	}
}

func TestHandleFilterResult_Canceled(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.filterText = "existing"

	result, _ := m.handleFilterResult(modal.FilterResultMsg{Value: "new", Canceled: true})
	m = result.(Model)

	if m.filterText != "existing" {
		t.Errorf("expected filterText unchanged, got %q", m.filterText)
	}
}

func TestHandleResetResult(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.inputs[testInputEnvironment] = testValueCustom

	result, _ := m.handleResetResult(modal.ResetResultMsg{Confirmed: true})
	m = result.(Model)

	if m.inputs[testInputEnvironment] != testValueStaging {
		t.Errorf("expected environment reset to staging, got %q", m.inputs[testInputEnvironment])
	}
}

func TestHandleResetResult_Canceled(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")
	m.inputs[testInputEnvironment] = testValueCustom

	result, _ := m.handleResetResult(modal.ResetResultMsg{Confirmed: false})
	m = result.(Model)

	if m.inputs[testInputEnvironment] != testValueCustom {
		t.Errorf("expected environment unchanged, got %q", m.inputs[testInputEnvironment])
	}
}

func TestBuildCLIString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		workflow     int
		branch       string
		inputs       map[string]string
		wantContains []string
	}{
		{
			name:         "basic workflow",
			workflow:     0,
			branch:       "",
			inputs:       map[string]string{},
			wantContains: []string{"gh workflow run deploy.yml"},
		},
		{
			name:         "with branch",
			workflow:     0,
			branch:       "main",
			inputs:       map[string]string{},
			wantContains: []string{"gh workflow run deploy.yml", "--ref main"},
		},
		{
			name:     "with inputs",
			workflow: 0,
			branch:   "main",
			inputs:   map[string]string{testInputEnvironment: "production"},
			wantContains: []string{
				"gh workflow run deploy.yml",
				"--ref main",
				"-f environment=production",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := New(testWorkflows(), testHistory(), "owner/repo")
			m.selectedWorkflow = tt.workflow
			m.branch = tt.branch
			m.inputs = tt.inputs

			cmd := m.buildCLIString()

			for _, want := range tt.wantContains {
				if !contains(cmd, want) {
					t.Errorf("buildCLIString() missing %q in: %s", want, cmd)
				}
			}
		})
	}
}

func TestHandleWorkflowKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		keyNum          int
		wantWorkflow    int
		wantInputsSet   bool
		wantEnvironment string
	}{
		{
			name:            "key 0 clears selection",
			keyNum:          0,
			wantWorkflow:    -1,
			wantInputsSet:   false,
			wantEnvironment: "",
		},
		{
			name:            "key 1 selects first workflow",
			keyNum:          1,
			wantWorkflow:    0,
			wantInputsSet:   true,
			wantEnvironment: testValueStaging,
		},
		{
			name:            "key 2 selects second workflow",
			keyNum:          2,
			wantWorkflow:    1,
			wantInputsSet:   true,
			wantEnvironment: "",
		},
		{
			name:            "key 3 out of range",
			keyNum:          3,
			wantWorkflow:    0,
			wantInputsSet:   true,
			wantEnvironment: testValueStaging,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := New(testWorkflows(), testHistory(), "owner/repo")
			m.focused = PaneWorkflows

			result, _ := m.handleWorkflowKey(tt.keyNum)
			m = result.(Model)

			if m.selectedWorkflow != tt.wantWorkflow {
				t.Errorf("expected selectedWorkflow=%d, got %d", tt.wantWorkflow, m.selectedWorkflow)
			}

			if tt.wantInputsSet {
				if env, ok := m.inputs[testInputEnvironment]; ok {
					if env != tt.wantEnvironment {
						t.Errorf("expected environment=%q, got %q", tt.wantEnvironment, env)
					}
				} else if tt.wantEnvironment != "" {
					t.Error("expected inputs to be set")
				}
			}
		})
	}
}

func TestCurrentHistoryEntries(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")

	entries := m.currentHistoryEntries()
	if len(entries) == 0 {
		t.Error("expected history entries")
	}

	m.selectedWorkflow = 0
	filteredEntries := m.currentHistoryEntries()

	if len(filteredEntries) > len(entries) {
		t.Errorf("filtered entries should be <= all entries")
	}
}

func TestGetSelectedInputName(t *testing.T) {
	t.Parallel()

	m := New(testWorkflows(), testHistory(), "owner/repo")

	m.selectedInput = -1
	if name := m.getSelectedInputName(); name != "" {
		t.Errorf("expected empty name for -1 index, got %q", name)
	}

	m.selectedInput = 0

	name := m.getSelectedInputName()
	if name != testInputEnvironment {
		t.Errorf("expected 'environment', got %q", name)
	}

	m.selectedInput = 999
	if name := m.getSelectedInputName(); name != "" {
		t.Errorf("expected empty name for out-of-range index, got %q", name)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}
