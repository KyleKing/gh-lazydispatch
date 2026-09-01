package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

const (
	testChainName = "deploy-pipeline"
	testChainVar  = "target"
)

// testChainConfig is a chain whose second step reads both a variable and the
// step above it, so a preview that interpolates either one wrong shows up in
// the rendered command.
func testChainConfig() *config.WfdConfig {
	return &config.WfdConfig{
		Version: 1,
		Chains: map[string]config.Chain{
			testChainName: {
				Description: "build then deploy",
				Variables: []config.ChainVariable{
					{
						Name:     testChainVar,
						Type:     "choice",
						Default:  testValueStaging,
						Options:  []string{testValueStaging, "production"},
						Required: true,
					},
				},
				Steps: []config.ChainStep{
					{
						Workflow: "ci.yml",
						WaitFor:  config.WaitSuccess,
						Inputs:   map[string]string{"env": "{{ var.target }}"},
					},
					{
						Workflow:  "deploy.yml",
						WaitFor:   config.WaitCompletion,
						OnFailure: config.FailureAbort,
						Inputs:    map[string]string{"env": "{{ previous.inputs.env }}"},
					},
				},
			},
		},
	}
}

func newChainModel(t *testing.T) Model {
	t.Helper()

	m := resize(t, newRenderModel(), 120, 40)
	m.wfdConfig = testChainConfig()

	return m
}

// openChainVariables walks the chain-select modal to the variable prompt.
func openChainVariables(t *testing.T, m Model) Model {
	t.Helper()

	m = pressRune(t, m, 'C')
	if !m.modalStack.HasActive() {
		t.Fatal("C did not open the chain select modal")
	}

	m = pressSpecial(t, m, tea.KeyEnter)
	if !m.modalStack.HasActive() {
		t.Fatal("selecting a chain closed the stack instead of prompting for variables")
	}

	return m
}

// TestJourney_ChainPreviewInterpolatesTheChosenVariable walks the whole
// pre-dispatch chain flow, which is the only place a user sees what a chain
// will actually run before committing to it. Nothing dispatches: the model has
// no gh client, so handleChainConfirmResult stops at the preview.
func TestJourney_ChainPreviewInterpolatesTheChosenVariable(t *testing.T) {
	t.Parallel()

	m := openChainVariables(t, newChainModel(t))

	// Step the choice off its default, so the preview cannot pass by echoing it.
	m = pressSpecial(t, m, tea.KeyRight)
	m = pressSpecial(t, m, tea.KeyEnter)

	if !m.modalStack.HasActive() {
		t.Fatal("confirming variables closed the stack instead of showing the confirmation")
	}

	view := m.View().Content
	for _, want := range []string{"Confirm Chain Execution", testChainName, "ci.yml", "deploy.yml", "production"} {
		if !strings.Contains(view, want) {
			t.Errorf("chain confirmation does not mention %q", want)
		}
	}

	if strings.Contains(view, "{{") {
		t.Errorf("chain confirmation shows an uninterpolated template:\n%s", view)
	}

	if strings.Contains(view, testValueStaging) {
		t.Errorf("chain confirmation still shows the default the user stepped away from:\n%s", view)
	}
}

func TestJourney_CancelingAChainClearsItsPendingState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		abort func(t *testing.T, m Model) Model
	}{
		{"at the variable prompt", func(t *testing.T, m Model) Model {
			t.Helper()

			return pressSpecial(t, m, tea.KeyEscape)
		}},
		{"at the confirmation", func(t *testing.T, m Model) Model {
			t.Helper()

			m = pressSpecial(t, m, tea.KeyEnter)

			return pressRune(t, m, 'n')
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := tt.abort(t, openChainVariables(t, newChainModel(t)))

			if m.pendingChain != nil || m.pendingChainName != "" {
				t.Errorf("chain %q still pending after cancel", m.pendingChainName)
			}

			if m.chainExecutor != nil {
				t.Error("canceling started an executor")
			}
		})
	}
}

// TestJourney_ResetRestoresEveryEditedInput covers the edit-then-undo loop:
// the reset modal must list what changed and put it back.
func TestJourney_ResetRestoresEveryEditedInput(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneConfig
	m.inputs[testInputEnvironment] = "production"

	m = pressRune(t, m, 'r')
	if !m.modalStack.HasActive() {
		t.Fatal("r did not open the reset modal")
	}

	if view := m.View().Content; !strings.Contains(view, testInputEnvironment) {
		t.Errorf("reset modal does not name the input it would change:\n%s", view)
	}

	m = pressRune(t, m, 'y')

	if got := m.inputs[testInputEnvironment]; got != testValueStaging {
		t.Errorf("environment is %q after reset, want the workflow default %q", got, testValueStaging)
	}
}

// TestJourney_RemappingAStaleHistoryEntry covers replaying a run recorded
// against an older version of the workflow, where a value no longer validates.
// Remap is offered by the action leader rather than by a key of its own, so
// the journey is how a user who does not know the key still reaches it.
func TestJourney_RemappingAStaleHistoryEntry(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneHistory

	m = selectHistoryEntryWithInput(t, m, testInputEnvironment, "prod")
	m = pressSpecial(t, m, tea.KeyEnter)

	if m.viewMode != HistoryPreviewMode || m.previewingHistoryEntry == nil {
		t.Fatal("enter on a history entry did not enter preview mode")
	}

	m = pressRune(t, m, 'a')
	if !m.modalStack.HasActive() {
		t.Fatal("a did not open the action menu")
	}

	if view := m.View().Content; !strings.Contains(view, "remap") {
		t.Fatalf("the history action menu does not offer remap:\n%s", view)
	}

	m = pressRune(t, m, 'm')
	if !m.modalStack.HasActive() {
		t.Fatal("choosing remap closed the stack instead of opening the wizard")
	}

	if view := m.View().Content; !strings.Contains(view, "prod") {
		t.Errorf("remap modal does not show the stale value:\n%s", view)
	}
}

// TestJourney_ActionMenuOnlyOffersWhatApplies is the rule the leader replaced
// the scattered focus guards with: a verb that would do nothing here is not
// listed, rather than listed and silently inert.
func TestJourney_ActionMenuOnlyOffersWhatApplies(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneHistory

	m = pressRune(t, m, 'a')

	if view := m.View().Content; strings.Contains(view, "remap") {
		t.Errorf("remap is offered with no entry in preview:\n%s", view)
	}

	m = pressSpecial(t, m, tea.KeyEscape)

	m.focused = PaneConfig
	m = pressRune(t, m, 'a')

	view := m.View().Content
	for _, want := range []string{"reset", "filter", "copy"} {
		if !strings.Contains(view, want) {
			t.Errorf("the config action menu does not offer %q:\n%s", want, view)
		}
	}

	if strings.Contains(view, "remap") {
		t.Error("the config pane offers a history verb")
	}
}

// selectHistoryEntryWithInput moves the history selection down until the entry
// carrying name=value is selected, so the test does not depend on frecency
// ranking.
func selectHistoryEntryWithInput(t *testing.T, m Model, name, value string) Model {
	t.Helper()

	for range len(m.currentHistoryEntries()) {
		if entry := m.rightPanel.SelectedHistoryEntry(); entry != nil && entry.Inputs[name] == value {
			return m
		}

		m = pressSpecial(t, m, tea.KeyDown)
	}

	t.Fatalf("no history entry has %s=%s", name, value)

	return m
}

// TestJourney_EscapePeelsOneLayerAtATime covers the rule that keeps escape
// predictable: a timeline drilled into a job backs out to the jobs before it
// leaves anything else, rather than discarding two states at once.
func TestJourney_EscapePeelsOneLayerAtATime(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneHistory
	m = selectHistoryEntryWithInput(t, m, testInputEnvironment, "prod")
	m = pressSpecial(t, m, tea.KeyEnter)

	if m.viewMode != HistoryPreviewMode {
		t.Fatal("did not enter preview mode")
	}

	m.rightPanel.SetTimelineRun("run 1", timelineTestJobs())
	m.rightPanel.SetTab(panes.TabTimeline)
	m.rightPanel.Timeline().Drill()

	m = pressSpecial(t, m, tea.KeyEscape)

	if m.rightPanel.Timeline().Drilled() {
		t.Fatal("escape did not back out of the drilled job")
	}

	if m.viewMode != HistoryPreviewMode {
		t.Error("escape left preview mode in the same press that closed the drill-down")
	}

	m = pressSpecial(t, m, tea.KeyEscape)

	if m.viewMode != WorkflowListMode {
		t.Error("the second escape did not leave preview mode")
	}
}

func timelineTestJobs() []github.Job {
	start := time.Now().Add(-2 * time.Minute)

	return []github.Job{{
		Name: "ci", StartedAt: start, CompletedAt: start.Add(time.Minute), Conclusion: "failure",
		Steps: []github.Step{
			{Name: "Set up job", Number: 1, StartedAt: start, CompletedAt: start.Add(time.Second)},
		},
	}}
}
