package panes

import (
	"fmt"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
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
						"dry_run": {
							Type:    "boolean",
							Default: "false",
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

func testWorkflowWithInputs(name, filename string, inputs map[string]workflow.WorkflowInput) workflow.WorkflowFile {
	return workflow.WorkflowFile{
		Name:     name,
		Filename: filename,
		On: workflow.OnTrigger{
			WorkflowDispatch: &workflow.WorkflowDispatch{
				Inputs: inputs,
			},
		},
	}
}

func testManyInputsWorkflow() workflow.WorkflowFile {
	inputs := make(map[string]workflow.WorkflowInput)

	for i := range 15 {
		name := fmt.Sprintf("input%02d", i)
		inputs[name] = workflow.WorkflowInput{
			Type:    "string",
			Default: "",
		}
	}

	return testWorkflowWithInputs("Many Inputs", "many.yml", inputs)
}

func testRequiredInputsWorkflow() workflow.WorkflowFile {
	return testWorkflowWithInputs("Required", "required.yml", map[string]workflow.WorkflowInput{
		"api_key": {
			Type:     "string",
			Required: true,
			Default:  "",
		},
		"optional": {
			Type:    "string",
			Default: "default-val",
		},
	})
}

func TestWorkflowModel_SelectedWorkflow(t *testing.T) {
	m := NewWorkflowModel(testWorkflows())
	m.SetSize(40, 20)

	wf := m.SelectedWorkflow()
	if wf == nil {
		t.Fatal("expected non-nil workflow")
	}

	if wf.Filename != "deploy.yml" {
		t.Errorf("expected 'deploy.yml', got %q", wf.Filename)
	}
}

func TestWorkflowItem_FilterValue(t *testing.T) {
	wf := workflow.WorkflowFile{Name: "Deploy", Filename: "deploy.yml"}
	item := WorkflowItem{workflow: wf}

	fv := item.FilterValue()
	if fv != "Deploy deploy.yml" {
		t.Errorf("expected 'Deploy deploy.yml', got %q", fv)
	}
}

func TestHistoryModel_SetEntries(t *testing.T) {
	m := NewHistoryModel()
	m.SetSize(60, 20)

	entries := []frecency.HistoryEntry{
		{Workflow: "deploy.yml", Branch: "main", LastRunAt: time.Now()},
		{Workflow: "deploy.yml", Branch: "feature", LastRunAt: time.Now().Add(-1 * time.Hour)},
	}

	m.SetEntries(entries, "deploy.yml")

	entry := m.SelectedEntry()
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	if entry.Branch != "main" {
		t.Errorf("expected branch 'main', got %q", entry.Branch)
	}
}

func TestConfigModel_SetWorkflow(t *testing.T) {
	m := NewConfigModel()
	m.SetSize(80, 20)

	wfs := testWorkflows()
	m.SetWorkflow(&wfs[0])

	names := m.GetInputNames()
	if len(names) != 2 {
		t.Errorf("expected 2 inputs, got %d", len(names))
	}

	envVal := m.GetInputValue("environment")
	if envVal != "staging" {
		t.Errorf("expected default 'staging', got %q", envVal)
	}
}

func TestConfigModel_BuildCommand(t *testing.T) {
	m := NewConfigModel()
	wfs := testWorkflows()
	m.SetWorkflow(&wfs[0])
	m.SetBranch("main")
	m.SetInput("environment", "production")

	cmd := m.BuildCommand()
	if len(cmd) < 3 {
		t.Fatalf("expected at least 3 args, got %d", len(cmd))
	}

	if cmd[0] != "workflow" || cmd[1] != "run" || cmd[2] != "deploy.yml" {
		t.Errorf("unexpected command prefix: %v", cmd[:3])
	}

	hasRef := false
	hasEnv := false

	for i, arg := range cmd {
		if arg == "--ref" && i+1 < len(cmd) && cmd[i+1] == "main" {
			hasRef = true
		}

		if arg == "-f" && i+1 < len(cmd) && cmd[i+1] == "environment=production" {
			hasEnv = true
		}
	}

	if !hasRef {
		t.Error("expected --ref main in command")
	}

	if !hasEnv {
		t.Error("expected -f environment=production in command")
	}
}

func TestConfigModel_ToggleWatchRun(t *testing.T) {
	m := NewConfigModel()

	if m.WatchRun() {
		t.Error("expected watchRun to be false initially")
	}

	m.ToggleWatchRun()

	if !m.WatchRun() {
		t.Error("expected watchRun to be true after toggle")
	}

	m.ToggleWatchRun()

	if m.WatchRun() {
		t.Error("expected watchRun to be false after second toggle")
	}
}

func TestConfigModel_SelectUpDown_Boundaries(t *testing.T) {
	m := NewConfigModel()
	m.SetSize(80, 30)

	wf := testManyInputsWorkflow()
	m.SetWorkflow(&wf)

	if m.selectedRow != -1 {
		t.Errorf("expected initial selectedRow = -1, got %d", m.selectedRow)
	}

	m.SelectDown()

	if m.selectedRow != 0 {
		t.Errorf("expected selectedRow = 0 after first SelectDown, got %d", m.selectedRow)
	}

	m.SelectUp()

	if m.selectedRow != 0 {
		t.Errorf("expected selectedRow = 0 at top boundary, got %d", m.selectedRow)
	}

	for range 20 {
		m.SelectDown()
	}

	maxIdx := len(m.filteredOrder) - 1
	if m.selectedRow != maxIdx {
		t.Errorf("expected selectedRow = %d at bottom boundary, got %d", maxIdx, m.selectedRow)
	}

	m.SelectDown()

	if m.selectedRow != maxIdx {
		t.Errorf("expected selectedRow = %d to stay at bottom, got %d", maxIdx, m.selectedRow)
	}
}

func TestConfigModel_SetFilter_FuzzyMatching(t *testing.T) {
	tests := []struct {
		name           string
		filter         string
		expectMinCount int
		expectSelected int
	}{
		{
			name:           "no filter",
			filter:         "",
			expectMinCount: 15,
			expectSelected: 5,
		},
		{
			name:           "matches some",
			filter:         "01",
			expectMinCount: 1,
			expectSelected: 0,
		},
		{
			name:           "matches none",
			filter:         "xyz",
			expectMinCount: 0,
			expectSelected: -1,
		},
		{
			name:           "partial match",
			filter:         "input",
			expectMinCount: 15,
			expectSelected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewConfigModel()
			m.SetSize(80, 30)

			wf := testManyInputsWorkflow()
			m.SetWorkflow(&wf)
			m.selectedRow = 5

			m.SetFilter(tt.filter)

			if len(m.filteredOrder) < tt.expectMinCount {
				t.Errorf("expected at least %d filtered items, got %d", tt.expectMinCount, len(m.filteredOrder))
			}

			if tt.expectMinCount > 0 && tt.expectSelected >= 0 {
				if m.selectedRow >= len(m.filteredOrder) {
					t.Errorf("selectedRow %d out of bounds for filtered list length %d", m.selectedRow, len(m.filteredOrder))
				}
			}

			if m.scrollOffset != 0 {
				t.Errorf("expected scrollOffset = 0 after filter, got %d", m.scrollOffset)
			}
		})
	}
}

func TestConfigModel_GetModifiedInputs(t *testing.T) {
	m := NewConfigModel()
	wfs := testWorkflows()
	m.SetWorkflow(&wfs[0])

	modified := m.GetModifiedInputs()
	if len(modified) != 0 {
		t.Errorf("expected no modifications initially, got %d", len(modified))
	}

	m.SetInput("environment", "production")

	modified = m.GetModifiedInputs()
	if len(modified) != 1 {
		t.Errorf("expected 1 modification, got %d", len(modified))
	}

	if mod, ok := modified["environment"]; !ok || mod.Current != "production" || mod.Default != "staging" {
		t.Errorf("unexpected modification: %+v", modified["environment"])
	}

	m.SetInput("dry_run", "false")

	modified = m.GetModifiedInputs()
	if len(modified) != 1 {
		t.Errorf("expected 1 modification (dry_run unchanged), got %d", len(modified))
	}
}

func TestConfigModel_BuildCommand_EdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		setupFunc    func(*ConfigModel)
		expectBranch bool
		expectInput  string
	}{
		{
			name: "empty branch omits ref",
			setupFunc: func(m *ConfigModel) {
				m.SetBranch("")
			},
			expectBranch: false,
		},
		{
			name: "with branch includes ref",
			setupFunc: func(m *ConfigModel) {
				m.SetBranch("main")
			},
			expectBranch: true,
		},
		{
			name: "empty input values omitted",
			setupFunc: func(m *ConfigModel) {
				m.SetInput("environment", "")
			},
			expectInput: "",
		},
		{
			name: "non-empty input included",
			setupFunc: func(m *ConfigModel) {
				m.SetInput("environment", "production")
			},
			expectInput: "environment=production",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewConfigModel()
			wfs := testWorkflows()
			m.SetWorkflow(&wfs[0])
			tt.setupFunc(&m)

			cmd := m.BuildCommand()
			cmdStr := fmt.Sprintf("%v", cmd)

			hasRef := false

			for i, arg := range cmd {
				if arg == "--ref" && i+1 < len(cmd) {
					hasRef = true
					break
				}
			}

			if hasRef != tt.expectBranch {
				t.Errorf("expected branch in command = %v, got %v", tt.expectBranch, hasRef)
			}

			if tt.expectInput != "" {
				found := false

				for _, arg := range cmd {
					if arg == tt.expectInput {
						found = true
						break
					}
				}

				if !found {
					t.Errorf("expected input %q in command: %s", tt.expectInput, cmdStr)
				}
			}
		})
	}
}

func TestConfigModel_SetInputs_Nil(t *testing.T) {
	m := NewConfigModel()
	wfs := testWorkflows()
	m.SetWorkflow(&wfs[0])

	m.SetInputs(nil)

	val := m.GetInputValue("environment")
	if val != "staging" {
		t.Errorf("expected default value after nil SetInputs, got %q", val)
	}
}

func TestConfigModel_SelectedInput_EdgeCases(t *testing.T) {
	m := NewConfigModel()
	m.SetSize(80, 30)

	wf := testManyInputsWorkflow()
	m.SetWorkflow(&wf)

	m.selectedRow = -1
	if selected := m.SelectedInput(); selected != "" {
		t.Errorf("expected empty string for negative row, got %q", selected)
	}

	m.selectedRow = 100
	if selected := m.SelectedInput(); selected != "" {
		t.Errorf("expected empty string for row beyond max, got %q", selected)
	}

	m.SetFilter("xyz")

	m.selectedRow = 0
	if selected := m.SelectedInput(); selected != "" {
		t.Errorf("expected empty string for empty filtered list, got %q", selected)
	}
}

func TestConfigModel_ResetAllInputs_NoWorkflow(t *testing.T) {
	m := NewConfigModel()
	m.ResetAllInputs()
}

func TestConfigModel_View_NoWorkflow(t *testing.T) {
	m := NewConfigModel()
	m.SetSize(80, 30)
	m.SetFocused(true)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestFormatTimeAgo(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		timeAgo  time.Duration
		expected string
	}{
		{
			name:     "just now",
			timeAgo:  30 * time.Second,
			expected: "just now",
		},
		{
			name:     "5 minutes ago",
			timeAgo:  5 * time.Minute,
			expected: "5m ago",
		},
		{
			name:     "3 hours ago",
			timeAgo:  3 * time.Hour,
			expected: "3h ago",
		},
		{
			name:     "2 days ago",
			timeAgo:  48 * time.Hour,
			expected: "2d ago",
		},
		{
			name:     "59 seconds",
			timeAgo:  59 * time.Second,
			expected: "just now",
		},
		{
			name:     "59 minutes",
			timeAgo:  59 * time.Minute,
			expected: "59m ago",
		},
		{
			name:     "23 hours",
			timeAgo:  23 * time.Hour,
			expected: "23h ago",
		},
		{
			name:     "6 days",
			timeAgo:  6 * 24 * time.Hour,
			expected: "6d ago",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testTime := now.Add(-tt.timeAgo)

			result := formatTimeAgo(testTime)
			if result != tt.expected {
				t.Errorf("formatTimeAgo() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHistoryModel_SetEntries_Empty(t *testing.T) {
	m := NewHistoryModel()
	m.SetSize(60, 20)

	m.SetEntries([]frecency.HistoryEntry{}, "workflow.yml")

	if m.SelectedEntry() != nil {
		t.Error("expected nil SelectedEntry for empty list")
	}
}

func TestWorkflowItem_Title_NoName(t *testing.T) {
	tests := []struct {
		name        string
		wf          workflow.WorkflowFile
		expectTitle string
		expectDesc  string
	}{
		{
			name: "name and filename",
			wf: workflow.WorkflowFile{
				Name:     "Deploy",
				Filename: "deploy.yml",
			},
			expectTitle: "Deploy",
			expectDesc:  "deploy.yml",
		},
		{
			name: "no name fallback to filename",
			wf: workflow.WorkflowFile{
				Name:     "",
				Filename: "test.yml",
			},
			expectTitle: "test.yml",
			expectDesc:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := WorkflowItem{workflow: tt.wf}

			title := item.Title()
			if title != tt.expectTitle {
				t.Errorf("Title() = %q, want %q", title, tt.expectTitle)
			}

			desc := item.Description()
			if tt.expectDesc != "" && desc != tt.expectDesc {
				t.Errorf("Description() = %q, want %q", desc, tt.expectDesc)
			}
		})
	}
}

func TestWorkflowModel_SelectedWorkflow_EmptyList(t *testing.T) {
	m := NewWorkflowModel([]workflow.WorkflowFile{})
	m.SetSize(40, 20)

	wf := m.SelectedWorkflow()
	if wf != nil {
		t.Error("expected nil workflow for empty list")
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}

	return false
}

// --- TabbedRightModel Tests ---

func TestTabbedRightModel_Creation(t *testing.T) {
	m := NewTabbedRight()

	if m.ActiveTab() != TabHistory {
		t.Errorf("expected initial tab to be TabHistory, got %v", m.ActiveTab())
	}
}

func TestTabbedRightModel_TabSwitching(t *testing.T) {
	m := NewTabbedRight()
	m.SetSize(80, 24)
	m.SetFocused(true)

	if m.ActiveTab() != TabHistory {
		t.Error("expected TabHistory initially")
	}

	m.NextTab()

	if m.ActiveTab() != TabChains {
		t.Error("expected TabChains after NextTab")
	}

	m.NextTab()

	if m.ActiveTab() != TabLive {
		t.Error("expected TabLive after second NextTab")
	}

	m.NextTab()

	if m.ActiveTab() != TabHistory {
		t.Error("expected TabHistory after third NextTab (wrap around)")
	}

	m.PrevTab()

	if m.ActiveTab() != TabLive {
		t.Error("expected TabLive after PrevTab")
	}
}

func TestTabbedRightModel_SetSize(t *testing.T) {
	m := NewTabbedRight()
	m.SetSize(100, 30)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestTabbedRightModel_SetHistoryEntries(t *testing.T) {
	m := NewTabbedRight()
	m.SetSize(80, 24)

	entries := []frecency.HistoryEntry{
		{Workflow: "test.yml", Branch: "main", LastRunAt: time.Now()},
	}

	m.SetHistoryEntries(entries, "test.yml")

	entry := m.SelectedHistoryEntry()
	if entry == nil {
		t.Fatal("expected non-nil entry")
	}

	if entry.Workflow != "test.yml" {
		t.Errorf("expected workflow 'test.yml', got %q", entry.Workflow)
	}
}

func TestTabbedRightModel_SetChains(t *testing.T) {
	m := NewTabbedRight()
	m.SetSize(80, 24)

	chains := map[string]config.Chain{
		"deploy": {
			Description: "Deploy to prod",
			Steps:       []config.ChainStep{{Workflow: "build.yml"}},
		},
	}

	m.SetChains(chains)
	m.NextTab()

	name, chain, ok := m.SelectedChain()
	if !ok {
		t.Fatal("expected chain to be selected")
	}

	if name != "deploy" {
		t.Errorf("expected chain name 'deploy', got %q", name)
	}

	if chain.Description != "Deploy to prod" {
		t.Errorf("expected description 'Deploy to prod', got %q", chain.Description)
	}
}

func TestTabbedRightModel_ViewRendering(t *testing.T) {
	m := NewTabbedRight()
	m.SetSize(80, 24)
	m.SetFocused(true)

	view := m.View()
	if !findSubstring(view, "History") {
		t.Error("view should contain History tab")
	}

	if !findSubstring(view, "Chains") {
		t.Error("view should contain Chains tab")
	}

	if !findSubstring(view, "Live") {
		t.Error("view should contain Live tab")
	}
}

// --- LiveRunsModel Tests ---

func TestLiveRunsModel_Creation(t *testing.T) {
	m := NewLiveRunsModel()

	if m.RunCount() != 0 {
		t.Errorf("expected 0 runs initially, got %d", m.RunCount())
	}

	_, ok := m.SelectedRun()
	if ok {
		t.Error("expected no selected run initially")
	}
}

func TestLiveRunsModel_SetRuns(t *testing.T) {
	m := NewLiveRunsModel()
	m.SetSize(80, 24)

	runs := []watcher.WatchedRun{
		{RunID: 1, Workflow: "test.yml", Status: "in_progress"},
		{RunID: 2, Workflow: "build.yml", Status: "completed", Conclusion: "success"},
	}

	m.SetRuns(runs)

	if m.RunCount() != 2 {
		t.Errorf("expected 2 runs, got %d", m.RunCount())
	}

	run, ok := m.SelectedRun()
	if !ok {
		t.Fatal("expected selected run")
	}

	if run.RunID != 1 {
		t.Errorf("expected first run selected, got ID %d", run.RunID)
	}
}

func TestLiveRunsModel_Navigation(t *testing.T) {
	m := NewLiveRunsModel()
	m.SetSize(80, 24)

	runs := []watcher.WatchedRun{
		{RunID: 1, Workflow: "first.yml"},
		{RunID: 2, Workflow: "second.yml"},
		{RunID: 3, Workflow: "third.yml"},
	}
	m.SetRuns(runs)

	if m.SelectedIndex() != 0 {
		t.Errorf("expected initial index 0, got %d", m.SelectedIndex())
	}

	m.MoveDown()

	if m.SelectedIndex() != 1 {
		t.Errorf("expected index 1 after MoveDown, got %d", m.SelectedIndex())
	}

	m.MoveDown()

	if m.SelectedIndex() != 2 {
		t.Errorf("expected index 2 after second MoveDown, got %d", m.SelectedIndex())
	}

	m.MoveDown()

	if m.SelectedIndex() != 2 {
		t.Error("expected index to stay at 2 at boundary")
	}

	m.MoveUp()

	if m.SelectedIndex() != 1 {
		t.Errorf("expected index 1 after MoveUp, got %d", m.SelectedIndex())
	}

	m.MoveUp()
	m.MoveUp()

	if m.SelectedIndex() != 0 {
		t.Error("expected index to stay at 0 at upper boundary")
	}
}

func TestLiveRunsModel_SetRunsAdjustsSelection(t *testing.T) {
	m := NewLiveRunsModel()

	runs := []watcher.WatchedRun{
		{RunID: 1}, {RunID: 2}, {RunID: 3},
	}
	m.SetRuns(runs)
	m.MoveDown()
	m.MoveDown()

	m.SetRuns([]watcher.WatchedRun{{RunID: 1}})

	if m.SelectedIndex() != 0 {
		t.Errorf("expected selection to adjust to 0, got %d", m.SelectedIndex())
	}
}

func TestLiveRunsModel_ActiveCount(t *testing.T) {
	m := NewLiveRunsModel()

	runs := []watcher.WatchedRun{
		{RunID: 1, Status: "in_progress"},
		{RunID: 2, Status: "completed", Conclusion: "success"},
		{RunID: 3, Status: "queued"},
	}
	m.SetRuns(runs)

	if m.ActiveCount() != 2 {
		t.Errorf("expected 2 active runs, got %d", m.ActiveCount())
	}
}

func TestLiveRunsModel_ViewEmpty(t *testing.T) {
	m := NewLiveRunsModel()
	m.SetSize(80, 24)

	view := m.ViewContent()
	if !findSubstring(view, "No active runs") {
		t.Error("empty view should indicate no active runs")
	}
}

func TestLiveRunsModel_ViewWithRuns(t *testing.T) {
	m := NewLiveRunsModel()
	m.SetSize(80, 24)

	runs := []watcher.WatchedRun{
		{RunID: 1, Workflow: "test.yml", Status: "in_progress"},
	}
	m.SetRuns(runs)

	view := m.ViewContent()
	if !findSubstring(view, "test.yml") {
		t.Error("view should contain workflow name")
	}
}

func TestRunStatusIcon(t *testing.T) {
	tests := []struct {
		status     string
		conclusion string
		expected   string
	}{
		{"queued", "", "o"},
		{"in_progress", "", "*"},
		{"completed", "success", "+"},
		{"completed", "failure", "x"},
		{"completed", "cancelled", "-"},
		{"completed", "unknown", "?"},
		{"unknown", "", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.status+"_"+tt.conclusion, func(t *testing.T) {
			got := runStatusIcon(tt.status, tt.conclusion)
			if got != tt.expected {
				t.Errorf("runStatusIcon(%q, %q) = %q, want %q", tt.status, tt.conclusion, got, tt.expected)
			}
		})
	}
}

// --- ChainListModel Tests ---

func TestChainListModel_Creation(t *testing.T) {
	m := NewChainListModel()

	_, _, ok := m.SelectedChain()
	if ok {
		t.Error("expected no chain selected initially")
	}
}

func TestChainListModel_SetChains(t *testing.T) {
	m := NewChainListModel()
	m.SetSize(80, 24)

	chains := map[string]config.Chain{
		"alpha": {Description: "Alpha chain"},
		"beta":  {Description: "Beta chain"},
		"gamma": {Description: "Gamma chain"},
	}

	m.SetChains(chains)

	name, _, ok := m.SelectedChain()
	if !ok {
		t.Fatal("expected chain to be selected")
	}

	if name != "alpha" {
		t.Errorf("expected first chain alphabetically 'alpha', got %q", name)
	}
}

func TestChainListModel_Navigation(t *testing.T) {
	m := NewChainListModel()
	m.SetSize(80, 24)

	chains := map[string]config.Chain{
		"alpha": {Description: "Alpha"},
		"beta":  {Description: "Beta"},
		"gamma": {Description: "Gamma"},
	}
	m.SetChains(chains)

	name, _, _ := m.SelectedChain()
	if name != "alpha" {
		t.Errorf("expected 'alpha', got %q", name)
	}

	m.MoveDown()

	name, _, _ = m.SelectedChain()
	if name != "beta" {
		t.Errorf("expected 'beta', got %q", name)
	}

	m.MoveDown()

	name, _, _ = m.SelectedChain()
	if name != "gamma" {
		t.Errorf("expected 'gamma', got %q", name)
	}

	m.MoveDown()

	name, _, _ = m.SelectedChain()
	if name != "gamma" {
		t.Error("expected to stay at 'gamma' at boundary")
	}

	m.MoveUp()

	name, _, _ = m.SelectedChain()
	if name != "beta" {
		t.Errorf("expected 'beta', got %q", name)
	}

	m.MoveUp()
	m.MoveUp()

	name, _, _ = m.SelectedChain()
	if name != "alpha" {
		t.Error("expected to stay at 'alpha' at upper boundary")
	}
}

func TestChainListModel_ViewEmpty(t *testing.T) {
	m := NewChainListModel()
	m.SetSize(80, 24)

	view := m.ViewContent()
	if !findSubstring(view, "No chains configured") {
		t.Error("empty view should indicate no chains")
	}
}

func TestChainListModel_ViewWithChains(t *testing.T) {
	m := NewChainListModel()
	m.SetSize(80, 24)

	chains := map[string]config.Chain{
		"deploy": {
			Description: "Deploy to prod",
			Steps:       []config.ChainStep{{Workflow: "build.yml"}, {Workflow: "deploy.yml"}},
			Variables:   []config.ChainVariable{{Name: "env"}},
		},
	}
	m.SetChains(chains)

	view := m.ViewContent()
	if !findSubstring(view, "deploy") {
		t.Error("view should contain chain name")
	}

	if !findSubstring(view, "Deploy to prod") {
		t.Error("view should contain description")
	}
}

func TestChainListModel_FocusState(t *testing.T) {
	m := NewChainListModel()
	m.SetFocused(true)

	if !m.focused {
		t.Error("expected focused to be true")
	}

	m.SetFocused(false)

	if m.focused {
		t.Error("expected focused to be false")
	}
}
