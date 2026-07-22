package app

import "testing"

// The "all workflows" row sets selectedWorkflow to -1 while viewMode and
// filteredInputs can still hold a prior workflow's input selection, so View
// must not index m.workflows before checking the sentinel.
func TestViewWithNoWorkflowSelected(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		viewMode      ViewMode
		selectedInput int
	}{
		{"stale input detail", InputDetailMode, 0},
		{"stale history preview", HistoryPreviewMode, 0},
		{"workflow list", WorkflowListMode, -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := newRenderModel()
			m.width, m.height = 120, 40
			m.initializeInputs(m.workflows[0])
			m.selectedWorkflow = -1
			m.selectedInput = tt.selectedInput
			m.viewMode = tt.viewMode

			_ = m.View()
		})
	}
}
