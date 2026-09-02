package modal

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/rule"
	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/validation"
	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// pressKey builds the keypress the runtime would deliver for one key name,
// spelling out the codes bubbletea gives its own names.
func pressKey(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "ctrl+r":
		return tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl}
	}

	return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
}

// typeKeys drives ctx through a key script, checking after each key that the
// modal still draws. A modal that stops rendering mid-edit is invisible in a
// test that only reads its final state.
func typeKeys(t *testing.T, ctx Context, keys ...string) Context {
	t.Helper()

	for _, name := range keys {
		next, _ := ctx.Update(pressKey(name))
		if strings.TrimSpace(next.View()) == "" {
			t.Fatalf("the modal rendered nothing after %q", name)
		}

		ctx = next
	}

	return ctx
}

// Escape is the only key every modal must honor; the rest it must at least
// survive. Editing keys reach code that navigation alone never does, which is
// where a modal that edits its own state keeps its arithmetic.
func TestModalsSurviveTheEditingKeys(t *testing.T) {
	t.Parallel()

	// c copies to the clipboard and o opens a browser, so neither belongs in a
	// sweep that presses everything.
	script := []string{"tab", "space", "e", "v", "backspace", "right", "left", "enter", "j", "k", "enter"}

	for _, tt := range contractModals() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctx := tt.make()
			if sizer, ok := ctx.(Sizer); ok {
				sizer.SetSize(80, 24)
			}

			typeKeys(t, ctx, script...)
		})
	}
}

// An input rejects a value once and then takes it, because the workflow file's
// rules are advisory: the tool cannot know what the workflow will accept, so
// the second enter applies anyway rather than trapping the value.
func TestInputModal_WarnsOnceThenAppliesAnyway(t *testing.T) {
	t.Parallel()

	newModal := func() *InputModal {
		return NewInputModal("tag", "release tag", "v1.0.0", "string", "nope", nil,
			[]rule.ValidationRule{{Type: rule.RuleRegex, Pattern: `^v\d`}})
	}

	m := newModal()

	ctx, cmd := m.Update(pressKey("enter"))
	if ctx.IsDone() || cmd != nil {
		t.Fatal("a value that breaks a rule was applied without a word")
	}

	if !strings.Contains(ansi.Strip(ctx.View()), "!") {
		t.Errorf("the rejection is not shown:\n%s", ansi.Strip(ctx.View()))
	}

	ctx, cmd = ctx.Update(pressKey("enter"))
	if !ctx.IsDone() || cmd == nil {
		t.Fatal("the second enter did not apply the value anyway")
	}

	if got, ok := cmd().(InputResultMsg); !ok || got.Value != "nope" {
		t.Errorf("the input answered %#v, want the typed value", cmd())
	}

	// Escape out of the warning keeps the value being edited rather than
	// closing the modal, so a typo is correctable where it was made.
	warned, _ := newModal().Update(pressKey("enter"))

	kept, _ := warned.Update(pressKey("esc"))
	if kept.IsDone() {
		t.Error("escape out of the warning closed the modal")
	}

	// Restoring the default is what makes the warning recoverable without
	// retyping, so it has to clear the warning too.
	restored, _ := kept.Update(pressKey("ctrl+r"))
	if !strings.Contains(ansi.Strip(restored.View()), "v1.0.0") {
		t.Errorf("ctrl+r did not restore the default:\n%s", ansi.Strip(restored.View()))
	}
}

// A choice input warns about a value outside its options for the same reason,
// since the options come from the same workflow file.
func TestInputModal_WarnsAboutAValueOutsideTheChoices(t *testing.T) {
	t.Parallel()

	m := NewInputModal("environment", "", "staging", inputTypeChoice, "nowhere",
		[]string{"staging", "production"}, nil)

	ctx, _ := m.Update(pressKey("enter"))
	if ctx.IsDone() {
		t.Fatal("a value outside the options was applied without a word")
	}

	if !strings.Contains(ansi.Strip(ctx.View()), "not a valid option") {
		t.Errorf("the warning does not say why:\n%s", ansi.Strip(ctx.View()))
	}
}

// The chain variable modal is the only place a chain's variables are set, so
// each kind has to be settable: a choice cycles, a boolean flips, and a string
// opens an editor. Confirming carries all three back.
func TestChainVariableModal_SetsEveryKindOfVariable(t *testing.T) {
	t.Parallel()

	m := NewChainVariableModal("deploy-pipeline", contractChain())

	// target is a choice: space cycles it off its default.
	ctx := typeKeys(t, m, "space")
	if got := m.variables["target"]; got != "production" {
		t.Errorf("space left the choice on %q", got)
	}

	// tag is a string: enter opens the editor, typing fills it, enter saves.
	ctx = typeKeys(t, ctx, "down", "enter", "v", "2", "enter")
	if got := m.variables["tag"]; got != "v2" {
		t.Errorf("the edited string is %q, want v2", got)
	}

	// dry_run is a boolean: space flips it off its "true" default.
	ctx = typeKeys(t, ctx, "space")
	if got := m.variables["dry_run"]; got != "false" {
		t.Errorf("space left the boolean on %q", got)
	}

	// Enter on the last variable confirms the whole set.
	ctx, cmd := ctx.Update(pressKey("enter"))
	if !ctx.IsDone() || cmd == nil {
		t.Fatal("enter on the last variable did not confirm")
	}

	result, ok := cmd().(ChainVariableResultMsg)
	if !ok || result.Canceled || result.ChainName != "deploy-pipeline" {
		t.Fatalf("confirming answered %#v", cmd())
	}

	if result.Variables["target"] != "production" || result.Variables["tag"] != "v2" {
		t.Errorf("the confirmed variables are %v", result.Variables)
	}
}

// A required variable left empty holds the confirmation rather than dispatching
// a chain that would fail on its first step.
func TestChainVariableModal_HoldsOnAMissingRequiredVariable(t *testing.T) {
	t.Parallel()

	def := contractChain()
	def.Variables[0].Default = ""

	m := NewChainVariableModal("deploy-pipeline", def)
	m.selectedIndex = len(def.Variables) - 1

	ctx, cmd := m.Update(pressKey("enter"))
	if ctx.IsDone() || cmd != nil {
		t.Fatal("the chain was confirmed with a required variable empty")
	}

	if !strings.Contains(ansi.Strip(ctx.View()), "target") {
		t.Errorf("the modal does not name what is missing:\n%s", ansi.Strip(ctx.View()))
	}

	// Escaping is still available, and says the chain was abandoned rather than
	// leaving the caller to guess from an empty variable set.
	canceled, cmd := ctx.Update(pressKey("esc"))
	if !canceled.IsDone() || cmd == nil {
		t.Fatal("escape did not abandon the chain")
	}

	if result, ok := cmd().(ChainVariableResultMsg); !ok || !result.Canceled {
		t.Errorf("escape answered %#v, want a canceled result", cmd())
	}
}

// The live overview is where a run stops being watched, so both clears have to
// name what they are clearing.
func TestLiveViewModal_ClearsOneRunAndEveryCompletedOne(t *testing.T) {
	t.Parallel()

	m := NewLiveViewModal(contractRuns())

	moved := typeKeys(t, m, "down")

	cleared, cmd := moved.Update(pressKey("d"))
	if !cleared.IsDone() || cmd == nil {
		t.Fatal("d cleared nothing")
	}

	if got, ok := cmd().(LiveViewClearMsg); !ok || got.RunID != 2 {
		t.Errorf("d cleared %#v, want the selected run", cmd())
	}

	all, cmd := NewLiveViewModal(contractRuns()).Update(pressKey("D"))
	if !all.IsDone() || cmd == nil {
		t.Fatal("D cleared nothing")
	}

	if _, ok := cmd().(LiveViewClearAllMsg); !ok {
		t.Errorf("D answered %#v", cmd())
	}
}

// A run list that shrinks under the cursor has to pull the cursor back with it,
// since the overview is updated by the watcher rather than by a keypress.
func TestLiveViewModal_KeepsTheCursorOnARunThatStillExists(t *testing.T) {
	t.Parallel()

	m := NewLiveViewModal(contractRuns())
	m.selected = 1

	m.UpdateRuns(contractRuns()[:1])

	if m.selected != 0 {
		t.Errorf("the cursor is on row %d of a one-row list", m.selected)
	}

	// An empty list leaves nothing to point at, and must not point past it.
	m.UpdateRuns(nil)

	if strings.TrimSpace(m.View()) == "" {
		t.Error("an empty overview rendered nothing")
	}
}

// The status bar is the one-line summary of what is being watched, so it has to
// count each outcome rather than only the total.
func TestFormatStatusBar_CountsEachOutcome(t *testing.T) {
	t.Parallel()

	if got := FormatStatusBar(nil); got != "" {
		t.Errorf("nothing watched reported %q", got)
	}

	got := FormatStatusBar(contractRuns())
	for _, want := range []string{"Watching: 2 runs", "1 active", "1 failed"} {
		if !strings.Contains(got, want) {
			t.Errorf("the status bar %q is missing %q", got, want)
		}
	}
}

// The remap wizard is what makes a stale history entry runnable again, so each
// option has to produce a decision the caller can act on.
func TestRemapModal_DecidesEachStaleInput(t *testing.T) {
	t.Parallel()

	m := NewRemapModal(
		[]validation.ConfigValidationError{
			{
				HistoricalName: "env", HistoricalValue: "prod",
				Status: validation.StatusMissing, Suggestion: "environment",
			},
			{HistoricalName: "gone", HistoricalValue: "1", Status: validation.StatusMissing},
		},
		map[string]workflow.Input{"environment": {Type: "string"}},
	)

	// The first error takes the suggestion, the second is dropped.
	ctx := typeKeys(t, m, "down", "enter", "enter")

	if !ctx.IsDone() {
		t.Fatalf("the wizard did not finish after deciding both inputs")
	}

	wizard, ok := ctx.(*RemapModal)
	if !ok {
		t.Fatalf("the wizard is a %T", ctx)
	}

	if decisions, ok := wizard.Result().([]RemapDecision); !ok || len(decisions) != 2 {
		t.Fatalf("the wizard decided %#v", wizard.Result())
	}

	// Escaping partway abandons every decision rather than applying half of them.
	abandoned := typeKeys(t, NewRemapModal(
		[]validation.ConfigValidationError{{HistoricalName: "env", Status: validation.StatusMissing}},
		nil,
	), "esc")

	wizard, ok = abandoned.(*RemapModal)
	if !ok {
		t.Fatalf("the abandoned wizard is a %T", abandoned)
	}

	if got := wizard.Result(); got != nil {
		t.Errorf("an abandoned wizard decided %#v", got)
	}
}

// The chain status modal is the only view of a chain while it runs, so it has
// to answer both ways out: stopping it, and reading what it did.
func TestChainStatusModal_StopsAChainAndReadsAFinishedOne(t *testing.T) {
	t.Parallel()

	running := NewChainStatusModalWithCommands(contractChainState(),
		[]string{"gh workflow run ci.yml", "gh workflow run deploy.yml"}, "main")

	stopped, cmd := running.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !stopped.IsDone() || cmd == nil {
		t.Fatal("ctrl+c did not stop the chain")
	}

	if _, ok := cmd().(ChainStatusStopMsg); !ok {
		t.Errorf("stopping answered %#v", cmd())
	}

	if !running.WasStopped() {
		t.Error("a stopped chain does not report itself stopped")
	}

	// A chain still running has no log to open yet, and one that failed does.
	if _, cmd := running.Update(pressKey("v")); cmd != nil {
		t.Error("a running chain offered a log to read")
	}

	state := contractChainState()
	state.Status = chain.ChainFailed

	failed := NewChainStatusModal(state)
	failed.SetCommands([]string{"gh workflow run ci.yml"}, "main")

	_, cmd = failed.Update(pressKey("v"))
	if cmd == nil {
		t.Fatal("a failed chain offered no log to read")
	}

	msg, ok := cmd().(ChainStatusViewLogsMsg)
	if !ok || !msg.ErrorsOnly || msg.Branch != "main" {
		t.Errorf("reading a failed chain's log asked for %#v", cmd())
	}
}

// The exported script is what carries a chain out of the tool, so it has to
// name every step and say what it leaves out.
func TestChainStatusModal_ExportsEveryStepAndItsLimits(t *testing.T) {
	t.Parallel()

	commands := []string{"gh workflow run ci.yml", "gh workflow run deploy.yml"}
	script := NewChainStatusModalWithCommands(contractChainState(), commands, "main").buildBashScript()

	for _, want := range append([]string{"#!/bin/bash", "set -e", "deploy-pipeline", "WARNING"}, commands...) {
		if !strings.Contains(script, want) {
			t.Errorf("the exported script is missing %q:\n%s", want, script)
		}
	}
}

// The status bar and the overview both read a watched run's state, so the icon
// has to distinguish every one rather than falling through to a question mark.
func TestRunStatusIcon_DistinguishesEveryOutcome(t *testing.T) {
	t.Parallel()

	seen := make(map[string]string)

	for _, tt := range []struct{ status, conclusion string }{
		{status: github.StatusQueued},
		{status: github.StatusInProgress},
		{status: github.StatusCompleted, conclusion: github.ConclusionSuccess},
		{status: github.StatusCompleted, conclusion: github.ConclusionFailure},
		{status: github.StatusCompleted, conclusion: github.ConclusionCancelled},
	} {
		icon := runStatusIcon(tt.status, tt.conclusion)
		if prior, ok := seen[icon]; ok {
			t.Errorf("%s/%s draws the same icon as %s", tt.status, tt.conclusion, prior)
		}

		seen[icon] = tt.status + "/" + tt.conclusion
	}

	if got := runStatusIcon(github.StatusCompleted, "timed_out"); got != "?" {
		t.Errorf("an unrecognized conclusion draws %q, want a question mark", got)
	}
}

// The confirmation names the exact command it will run, since re-running or
// canceling somebody else's run is not undoable.
func TestRunActionModal_NamesWhatItWillRunAndAnswersBothWays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		kind RunActionKind
		want string
	}{
		{kind: RunActionRerun, want: "gh run rerun 42 --failed"},
		{kind: RunActionCancel, want: "gh run cancel 42"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			t.Parallel()

			m := NewRunActionModal(tt.kind, 42, "CI")
			if !strings.Contains(ansi.Strip(m.View()), tt.want) {
				t.Errorf("the confirmation does not name %q:\n%s", tt.want, ansi.Strip(m.View()))
			}

			confirmed, cmd := m.Update(pressKey("y"))
			if !confirmed.IsDone() || cmd == nil {
				t.Fatal("y confirmed nothing")
			}

			result, ok := cmd().(RunActionResultMsg)
			if !ok || !result.Confirmed || result.RunID != 42 || result.Kind != tt.kind {
				t.Fatalf("confirming answered %#v", cmd())
			}

			refused, cmd := NewRunActionModal(tt.kind, 42, "CI").Update(pressKey("n"))
			if !refused.IsDone() || cmd == nil {
				t.Fatal("n answered nothing")
			}

			if result, ok := cmd().(RunActionResultMsg); !ok || result.Confirmed {
				t.Errorf("refusing answered %#v", cmd())
			}
		})
	}
}

// The action menu hands back the key beside the verb the reader picked, which
// is the only thing connecting the list they read to the verb that runs.
func TestActionMenuModal_AnswersWithTheKeyBesideThePickedVerb(t *testing.T) {
	t.Parallel()

	items := []ActionItem{{Key: "b", Name: "branch"}, {Key: "w", Name: "toggle watch"}}

	picked, cmd := typeKeys(t, NewActionMenuModal("Workflows", "CI on main", items), "down").
		Update(pressKey("enter"))

	if !picked.IsDone() || cmd == nil {
		t.Fatal("picking a verb answered nothing")
	}

	if got, ok := cmd().(ActionResultMsg); !ok || got.Key != "w" {
		t.Errorf("the menu answered %#v, want the second verb's key", cmd())
	}

	// The key itself works too, which is what makes the menu a reference
	// rather than a step.
	direct, cmd := NewActionMenuModal("Workflows", "CI on main", items).Update(pressKey("b"))
	if !direct.IsDone() || cmd == nil {
		t.Fatal("pressing a verb's own key answered nothing")
	}

	if got, ok := cmd().(ActionResultMsg); !ok || got.Key != "b" {
		t.Errorf("the key answered %#v", cmd())
	}
}

// Watching is the reason a dispatch is confirmed rather than fired, so the
// confirmation has to carry the whole config back.
func TestRunConfirmModal_CarriesTheConfigItShowed(t *testing.T) {
	t.Parallel()

	cfg := runnerConfigForTest()

	confirmed, cmd := NewRunConfirmModal(cfg).Update(pressKey("y"))
	if !confirmed.IsDone() || cmd == nil {
		t.Fatal("confirming answered nothing")
	}

	result, ok := cmd().(RunConfirmResultMsg)
	if !ok || !result.Confirmed || result.Config.Workflow != cfg.Workflow {
		t.Fatalf("confirming answered %#v", cmd())
	}

	if !result.Config.Watch {
		t.Error("the confirmed config dropped the watch flag")
	}
}

func runnerConfigForTest() runner.RunConfig {
	return runner.RunConfig{
		Workflow: "deploy.yml", Branch: "main",
		Inputs: map[string]string{"target": "staging"}, Watch: true,
	}
}
