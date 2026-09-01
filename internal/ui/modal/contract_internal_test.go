package modal

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/validation"
	"github.com/kyleking/gh-lazydispatch/internal/watcher"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// contractChain is a chain with a variable of every kind the variable modal
// renders, so stepping through it exercises each branch.
func contractChain() *config.Chain {
	return &config.Chain{
		Description: "build then deploy",
		Variables: []config.ChainVariable{
			{
				Name: "target", Type: "choice", Default: "staging",
				Options: []string{"staging", "production"}, Required: true,
			},
			{Name: "tag", Type: "string", Description: "release tag"},
			{Name: "dry_run", Type: "boolean", Default: "true"},
		},
		Steps: []config.ChainStep{
			{Workflow: "ci.yml", WaitFor: config.WaitSuccess, Inputs: map[string]string{"env": "{{ var.target }}"}},
			{Workflow: "deploy.yml", WaitFor: config.WaitCompletion, OnFailure: config.FailureAbort},
		},
	}
}

func contractChainState() chain.ChainState {
	return chain.ChainState{
		ChainName:    "deploy-pipeline",
		Status:       chain.ChainRunning,
		CurrentStep:  1,
		StepStatuses: []chain.StepStatus{chain.StepCompleted, chain.StepRunning},
		StepResults: map[int]*chain.StepResult{
			0: {Workflow: "ci.yml", RunID: 1, Status: chain.StepCompleted, Conclusion: "success"},
		},
	}
}

func contractRuns() []watcher.WatchedRun {
	return []watcher.WatchedRun{
		{
			RunID: 1, Workflow: "ci.yml", Status: github.StatusInProgress,
			UpdatedAt: time.Now(), Jobs: []watcher.JobStatus{{Name: "build", Status: github.StatusInProgress}},
		},
		{
			RunID: 2, Workflow: "deploy.yml", Status: github.StatusCompleted,
			Conclusion: github.ConclusionFailure, UpdatedAt: time.Now(),
		},
	}
}

func contractHistoryEntry() *frecency.HistoryEntry {
	return &frecency.HistoryEntry{
		Type:      frecency.EntryTypeChain,
		ChainName: "deploy-pipeline",
		Branch:    "main",
		Inputs:    map[string]string{"target": "staging"},
		StepResults: []frecency.ChainStepResult{
			{Workflow: "ci.yml", Status: "completed", Conclusion: "success", RunID: 1},
			{Workflow: "deploy.yml", Status: "failed", Conclusion: "failure", RunID: 2},
		},
	}
}

// contractModals is every modal the stack can hold, built in a state worth
// rendering. A modal missing here is one nothing renders at an odd size.
func contractModals() []struct {
	name string
	make func() Context
} {
	return []struct {
		name string
		make func() Context
	}{
		{"branch", func() Context {
			return NewSimpleBranchModal("Select Branch", []string{"main", "develop", "feature/a"}, "main", "main")
		}},
		{"chain_confirm", func() Context {
			return NewChainConfirmModal(
				"deploy-pipeline", contractChain(), map[string]string{"target": "production"}, "main", true,
			)
		}},
		{"chain_rerun", func() Context { return NewChainRerunModal(contractHistoryEntry()) }},
		{"chain_select", func() Context {
			return NewChainSelectModal(&config.WfdConfig{
				Version: 1,
				Chains:  map[string]config.Chain{"deploy-pipeline": *contractChain()},
			})
		}},
		{"chain_status", func() Context {
			return NewChainStatusModalWithCommands(contractChainState(),
				[]string{"gh workflow run ci.yml", "gh workflow run deploy.yml"}, "main")
		}},
		{"chain_variable", func() Context { return NewChainVariableModal("deploy-pipeline", contractChain()) }},
		{"confirm", func() Context { return NewConfirmModal("dry_run", "Skip the deploy", true, false) }},
		{"error", func() Context { return NewErrorModal("Dispatch failed", "gh: not authenticated") }},
		{"filter", func() Context { return NewFilterModal("Filter Inputs", []string{"environment", "tag"}, "") }},
		{"help", func() Context { return NewHelpModal() }},
		{"live_view", func() Context { return NewLiveViewModal(contractRuns()) }},
		{"logs_viewer", func() Context { return NewLogsViewerModal(createTestRunLogs(), 0, 0) }},
		{"logs_viewer_error", func() Context { return NewLogsViewerModalWithError(createTestRunLogs(), 0, 0) }},
		{"live_view_empty", func() Context { return NewLiveViewModal(nil) }},
		{"remap", func() Context {
			return NewRemapModal(
				[]validation.ConfigValidationError{{
					HistoricalName: "env", HistoricalValue: "prod",
					Status: validation.StatusMissing, Suggestion: "environment",
				}},
				map[string]workflow.Input{"environment": {Type: "string"}},
			)
		}},
		{"reset", func() Context {
			return NewResetModal([]ResetDiff{{Name: "environment", Current: "production", Default: "staging"}})
		}},
		{"select", func() Context {
			return NewSelectModal(
				"environment", "Where to deploy", []string{"staging", "production"}, "staging", "staging",
			)
		}},
		{"validation_error", func() Context {
			return NewValidationErrorModal(map[string][]string{"tag": {"must match v*"}})
		}},
	}
}

// contractSizes are terminal sizes, starting at the smallest the app renders
// at all (app.MinTerminalWidth x app.MinTerminalHeight); below that it draws
// "Terminal too small" and no modal is reachable.
var contractSizes = []struct {
	name          string
	width, height int
}{
	{"minimum_80x20", 80, 20},
	{"small_80x24", 80, 24},
	{"standard_120x40", 120, 40},
}

// TestModalsRenderInsideTheRoomTheyAreGiven holds every modal to the contract
// the stack assumes. Stack.Render clips a modal to the room inside its border
// rather than growing the frame, so a modal wider than that loses content with
// nothing to say it did: a dispatch command truncated mid-flag still looks
// like a command.
func TestModalsRenderInsideTheRoomTheyAreGiven(t *testing.T) {
	t.Parallel()

	for _, tt := range contractModals() {
		for _, size := range contractSizes {
			t.Run(tt.name+"/"+size.name, func(t *testing.T) {
				t.Parallel()

				// The room the stack hands out, which is what a modal must fit.
				width := size.width - modalChromeHorizontal

				m := tt.make()
				if sizer, ok := m.(Sizer); ok {
					sizer.SetSize(width, size.height-modalChromeVertical)
				}

				view := m.View()
				if strings.TrimSpace(view) == "" {
					t.Fatal("rendered nothing")
				}

				for i, line := range strings.Split(view, "\n") {
					if got := ansi.StringWidth(line); got > width {
						t.Errorf("line %d is %d cells wide and will be clipped to %d: %q",
							i+1, got, width, ansi.Strip(line))
					}
				}
			})
		}
	}
}

// TestModalsCloseOnEscape pins the one key every modal must honor. A modal
// that ignores escape traps the whole program, since the stack only pops on
// IsDone.
func TestModalsCloseOnEscape(t *testing.T) {
	t.Parallel()

	for _, tt := range contractModals() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := tt.make()
			if sizer, ok := m.(Sizer); ok {
				sizer.SetSize(80, 24)
			}

			if m.IsDone() {
				t.Fatal("a freshly built modal reports itself done")
			}

			ctx, _ := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			if !ctx.IsDone() {
				t.Error("escape did not close the modal")
			}
		})
	}
}

// TestModalsSurviveNavigationPastTheirBounds walks each modal off both ends of
// whatever it lists. Clamping is per-modal arithmetic, so an off-by-one only
// shows up at a boundary.
func TestModalsSurviveNavigationPastTheirBounds(t *testing.T) {
	t.Parallel()

	keys := []tea.KeyPressMsg{
		{Code: tea.KeyUp},
		{Code: tea.KeyUp},
		{Code: tea.KeyUp},
		{Code: tea.KeyDown},
		{Code: tea.KeyDown},
		{Code: tea.KeyDown},
		{Code: tea.KeyDown},
		{Code: tea.KeyDown},
		{Code: tea.KeyDown},
		{Code: tea.KeyUp},
	}

	for _, tt := range contractModals() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := tt.make()
			if sizer, ok := ctx.(Sizer); ok {
				sizer.SetSize(80, 24)
			}

			for _, key := range keys {
				ctx, _ = ctx.Update(key)

				if strings.TrimSpace(ctx.View()) == "" {
					t.Fatalf("rendered nothing after %v", key)
				}
			}
		})
	}
}

// TestStackResizesEveryModalItHolds is the regression test for a modal that
// was not a Sizer. Update routes tea.WindowSizeMsg before the stack sees it,
// so a modal that only resizes on that message never resizes at all: the log
// viewer kept the viewport it was built with for the life of the session.
func TestStackResizesEveryModalItHolds(t *testing.T) {
	t.Parallel()

	for _, tt := range contractModals() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			stack := NewStack()
			stack.SetSize(120, 40)
			stack.Push(tt.make())
			stack.SetSize(80, 20)

			for i, line := range strings.Split(stack.Current().View(), "\n") {
				if got := ansi.StringWidth(line); got > 80-modalChromeHorizontal {
					t.Errorf("line %d is %d cells wide after shrinking: %q",
						i+1, got, ansi.Strip(line))
				}
			}
		})
	}
}

// TestLogsViewerViewportFollowsTheStack is the concrete case behind the resize
// contract: the log viewer sizes a bubbles viewport, and a viewport that keeps
// its old width re-wraps nothing, so every line past the new edge is lost.
func TestLogsViewerViewportFollowsTheStack(t *testing.T) {
	t.Parallel()

	viewer := NewLogsViewerModal(createTestRunLogs(), 0, 0)

	stack := NewStack()
	stack.Push(viewer)
	stack.SetSize(120, 40)

	if got := viewer.viewport.Width(); got != 120-modalChromeHorizontal-logsViewportWidthMargin {
		t.Errorf("viewport is %d wide in a 120 column terminal", got)
	}

	stack.SetSize(80, 20)

	if got := viewer.viewport.Width(); got != 80-modalChromeHorizontal-logsViewportWidthMargin {
		t.Errorf("viewport is still %d wide after shrinking to 80 columns", got)
	}
}
