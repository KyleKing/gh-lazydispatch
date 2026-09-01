package panes

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/forge"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

const (
	testValueStaging = "staging"
	testChainAlpha   = "alpha"
)

func testWorkflows() []workflow.File {
	return []workflow.File{
		{
			Name:     "Deploy",
			Filename: "deploy.yml",
			On: workflow.OnTrigger{
				Dispatch: &workflow.Dispatch{
					Inputs: map[string]workflow.Input{
						"environment": {
							Type:    "choice",
							Default: testValueStaging,
							Options: []string{"production", testValueStaging},
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
				Dispatch: &workflow.Dispatch{},
			},
		},
	}
}

func TestWorkflowModel_SelectedWorkflow(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	wf := workflow.File{Name: "Deploy", Filename: "deploy.yml"}
	item := WorkflowItem{workflow: wf}

	fv := item.FilterValue()
	if fv != "Deploy deploy.yml" {
		t.Errorf("expected 'Deploy deploy.yml', got %q", fv)
	}
}

func TestHistoryModel_SetEntries(t *testing.T) {
	t.Parallel()

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

func TestHistoryModel_SetEntries_Empty(t *testing.T) {
	t.Parallel()

	m := NewHistoryModel()
	m.SetSize(60, 20)

	m.SetEntries([]frecency.HistoryEntry{}, "workflow.yml")

	if m.SelectedEntry() != nil {
		t.Error("expected nil SelectedEntry for empty list")
	}
}

func TestWorkflowItem_Title_NoName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		wf          workflow.File
		expectTitle string
		expectDesc  string
	}{
		{
			name: "name and filename",
			wf: workflow.File{
				Name:     "Deploy",
				Filename: "deploy.yml",
			},
			expectTitle: "Deploy",
			expectDesc:  "deploy.yml",
		},
		{
			name: "no name fallback to filename",
			wf: workflow.File{
				Name:     "",
				Filename: "test.yml",
			},
			expectTitle: "test.yml",
			expectDesc:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

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
	t.Parallel()

	m := NewWorkflowModel([]workflow.File{})
	m.SetSize(40, 20)

	wf := m.SelectedWorkflow()
	if wf != nil {
		t.Error("expected nil workflow for empty list")
	}
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
	t.Parallel()

	m := NewTabbedRight()

	if m.ActiveTab() != TabHistory {
		t.Errorf("expected initial tab to be TabHistory, got %v", m.ActiveTab())
	}
}

func TestTabbedRightModel_TabSwitching(t *testing.T) {
	t.Parallel()

	m := NewTabbedRight()
	m.SetSize(80, 24)
	m.SetFocused(true)

	// Every tab is reachable by walking forward, and the walk lands back where
	// it started, so adding a tab does not need this test rewritten.
	for i := range tabCount {
		if want := RightTab(i); m.ActiveTab() != want {
			t.Fatalf("after %d NextTab calls the active tab is %v, want %v", i, m.ActiveTab(), want)
		}

		m.NextTab()
	}

	if m.ActiveTab() != TabHistory {
		t.Errorf("a full walk forward ended on %v, want the first tab", m.ActiveTab())
	}

	m.PrevTab()

	if want := RightTab(tabCount - 1); m.ActiveTab() != want {
		t.Errorf("PrevTab from the first tab landed on %v, want %v", m.ActiveTab(), want)
	}
}

func TestTabbedRightModel_SetSize(t *testing.T) {
	t.Parallel()

	m := NewTabbedRight()
	m.SetSize(100, 30)

	view := m.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}

func TestTabbedRightModel_SetHistoryEntries(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

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
	t.Parallel()

	m := NewLiveRunsModel()
	m.SetSize(80, 24)

	view := m.ViewContent()
	if !findSubstring(view, "No active runs") {
		t.Error("empty view should indicate no active runs")
	}
}

func TestLiveRunsModel_ViewWithRuns(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

	tests := []struct {
		status     string
		conclusion string
		expected   string
	}{
		{"queued", "", "o"},
		{"in_progress", "", "*"},
		{"completed", "success", "+"},
		{"completed", "failure", "x"},
		{"completed", "cancelled", "-"}, //nolint:misspell // matches GitHub Actions API's actual conclusion value
		{"completed", "unknown", "?"},
		{"unknown", "", "?"},
	}

	for _, tt := range tests {
		t.Run(tt.status+"_"+tt.conclusion, func(t *testing.T) {
			t.Parallel()

			got := runStatusIcon(tt.status, tt.conclusion)
			if got != tt.expected {
				t.Errorf("runStatusIcon(%q, %q) = %q, want %q", tt.status, tt.conclusion, got, tt.expected)
			}
		})
	}
}

// --- ChainListModel Tests ---

func TestChainListModel_Creation(t *testing.T) {
	t.Parallel()

	m := NewChainListModel()

	_, _, ok := m.SelectedChain()
	if ok {
		t.Error("expected no chain selected initially")
	}
}

func TestChainListModel_SetChains(t *testing.T) {
	t.Parallel()

	m := NewChainListModel()
	m.SetSize(80, 24)

	chains := map[string]config.Chain{
		testChainAlpha: {Description: "Alpha chain"},
		"beta":         {Description: "Beta chain"},
		"gamma":        {Description: "Gamma chain"},
	}

	m.SetChains(chains)

	name, _, ok := m.SelectedChain()
	if !ok {
		t.Fatal("expected chain to be selected")
	}

	if name != testChainAlpha {
		t.Errorf("expected first chain alphabetically 'alpha', got %q", name)
	}
}

func TestChainListModel_Navigation(t *testing.T) {
	t.Parallel()

	m := NewChainListModel()
	m.SetSize(80, 24)

	chains := map[string]config.Chain{
		testChainAlpha: {Description: "Alpha"},
		"beta":         {Description: "Beta"},
		"gamma":        {Description: "Gamma"},
	}
	m.SetChains(chains)

	name, _, _ := m.SelectedChain()
	if name != testChainAlpha {
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
	if name != testChainAlpha {
		t.Error("expected to stay at 'alpha' at upper boundary")
	}
}

func TestChainListModel_ViewEmpty(t *testing.T) {
	t.Parallel()

	m := NewChainListModel()
	m.SetSize(80, 24)

	view := m.ViewContent()
	if !findSubstring(view, "No chains configured") {
		t.Error("empty view should indicate no chains")
	}
}

func TestChainListModel_ViewWithChains(t *testing.T) {
	t.Parallel()

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
	t.Parallel()

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

// A pane's rows are built from the width its model was handed, so a row wider
// than the pane's content wraps and corrupts the table. Lipgloss counts the
// border inside Width, which is what made the two disagree.
func TestTabbedPanelFitsTheWidthItIsGiven(t *testing.T) {
	t.Parallel()

	panel := NewTabbedRight()
	panel.SetChains(map[string]config.Chain{
		"a-long-chain-name-that-overflows": {
			Description: strings.Repeat("long description ", 8),
			Steps:       []config.ChainStep{{Workflow: "one.yml"}, {Workflow: "two.yml"}},
		},
	})
	panel.SetHistoryEntries([]frecency.HistoryEntry{
		{Workflow: "a-workflow-with-a-very-long-filename.yml", Branch: "a-long-branch-name", LastRunAt: time.Now()},
	}, "")
	panel.SetRuns([]watcher.WatchedRun{
		{Workflow: "another-very-long-workflow-filename.yml", Status: "in_progress"},
	})

	const panelHeight = 20

	for _, width := range []int{40, 60, 80, 120, 200} {
		panel.SetSize(width, panelHeight)

		for _, tab := range []RightTab{TabHistory, TabChains, TabLive} {
			panel.activeTab = tab

			lines := strings.Split(panel.View(), "\n")
			if len(lines) != panelHeight {
				t.Errorf("tab %v at width %d rendered %d lines, want %d: a row wrapped",
					tab, width, len(lines), panelHeight)
			}

			for i, line := range lines {
				if got := ansi.StringWidth(line); got != width {
					t.Errorf("tab %v at width %d: line %d is %d cells wide", tab, width, i+1, got)
				}
			}
		}
	}
}

func testRuns(n int) []github.WorkflowRun {
	runs := make([]github.WorkflowRun, 0, n)
	for i := range n {
		runs = append(runs, github.WorkflowRun{
			ID: int64(i + 1), Name: fmt.Sprintf("workflow-%02d", i), HeadBranch: "topic",
			Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess,
			CreatedAt: time.Now(),
		})
	}

	return runs
}

// A list longer than the pane must scroll rather than draw past its border,
// which MaxHeight would silently truncate.
func TestRunsModel_ScrollsToKeepTheSelectionVisible(t *testing.T) {
	t.Parallel()

	m := NewRunsModel()
	m.SetSize(60, 10)
	m.SetRuns(ScopeBranch, "topic", testRuns(30))

	for range 20 {
		m.MoveDown()
	}

	content := m.ViewContent()
	if !strings.Contains(content, "workflow-20") {
		t.Errorf("the selected row is off screen:\n%s", content)
	}

	if lines := strings.Count(content, "\n") + 1; lines > 10 {
		t.Errorf("the list drew %d lines into 10 rows", lines)
	}
}

// A pull request scope draws rollups rather than runs, and the row has to name
// the pull request and how its checks stand.
func TestRunsModel_PullRequestRowsCarryTheirRollup(t *testing.T) {
	t.Parallel()

	m := NewRunsModel()
	m.SetSize(60, 10)
	m.SetPRs(ScopeMine, []github.PullRequest{
		{
			Number: 7, Title: "Add the runs tab", HeadRef: "topic",
			Checks: forge.ChecksStatus{Total: 3, Passing: 2, Failing: 1},
		},
		{Number: 8, Title: "Bump the linter", HeadRef: "lint", Checks: forge.ChecksStatus{Total: 0}},
	})

	content := ansi.Strip(m.ViewContent())
	for _, want := range []string{"PR", "Checks", "#7", "Add the runs tab", "2+ 1x", "#8", "none"} {
		if !strings.Contains(content, want) {
			t.Errorf("the rollup rows do not carry %q:\n%s", want, content)
		}
	}

	if strings.Contains(content, "Workflow") {
		t.Errorf("a pull request scope drew the run columns:\n%s", content)
	}
}

// The tab bar is the only place a reader sees what the other tabs hold.
func TestTabbedRight_TabBarCarriesTheCounts(t *testing.T) {
	t.Parallel()

	m := NewTabbedRight()
	m.SetSize(80, 20)
	m.Runs().SetRuns(ScopeBranch, "topic", testRuns(2))

	bar := ansi.Strip(m.renderTabHeader())
	if !strings.Contains(bar, "Runs 2+") {
		t.Errorf("the tab bar does not report the runs verdict: %q", bar)
	}

	// A bar too wide for the pane abbreviates the names and keeps the counts.
	m.SetSize(30, 20)

	narrow := ansi.Strip(m.renderTabHeader())
	if strings.Contains(narrow, "Timeline") || !strings.Contains(narrow, "R 2+") {
		t.Errorf("the narrow tab bar is %q, want abbreviated names with counts", narrow)
	}
}
