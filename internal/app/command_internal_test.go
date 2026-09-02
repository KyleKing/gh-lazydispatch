package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

func testRegistry() Registry {
	return NewRegistry(
		Command{Name: "reset", Description: "reset"},
		Command{Name: "chain", Description: "chain"},
		Command{Name: "check", Description: "check"},
		Command{Name: "watch", Description: "watch"},
	)
}

// TestRegistryLookup covers the rule that keeps `:c` from running whichever
// command happens to sort first: a prefix resolves only when one command
// answers to it.
func TestRegistryLookup(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"exact", "reset", "reset"},
		{"unique prefix", "r", "reset"},
		{"ambiguous prefix", "ch", ""},
		{"exact wins over prefix", "chain", "chain"},
		{"unknown", "zzz", ""},
		{"empty", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			command, found := testRegistry().Lookup(tt.query)

			if tt.want == "" {
				if found {
					t.Errorf("Lookup(%q) resolved to %q, want nothing", tt.query, command.Name)
				}

				return
			}

			if !found || command.Name != tt.want {
				t.Errorf("Lookup(%q) = %q (%v), want %q", tt.query, command.Name, found, tt.want)
			}
		})
	}
}

func TestCommonPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		want  string
		names []string
	}{
		{"none", "", nil},
		{"one", "chain", []string{"chain"}},
		{"shared", "ch", []string{"chain", "check"}},
		{"nothing shared", "", []string{"chain", "reset"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			candidates := make([]Candidate, len(tt.names))
			for i, name := range tt.names {
				candidates[i] = Candidate{Name: name}
			}

			if got := commonPrefix(candidates); got != tt.want {
				t.Errorf("commonPrefix(%v) = %q, want %q", tt.names, got, tt.want)
			}
		})
	}
}

// typeCommand opens the bar and types line into it.
func typeCommand(t *testing.T, m Model, line string) Model {
	t.Helper()

	m = pressRune(t, m, ':')
	if !m.commandMode {
		t.Fatal(": did not open the command bar")
	}

	for _, r := range line {
		m = applyMsg(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	return m
}

// TestJourney_CommandBarCompletesAndRuns is the whole point of the bar: a name
// you half-remember, completed and run without knowing its key.
func TestJourney_CommandBarCompletesAndRuns(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	m.inputs[testInputEnvironment] = "production"

	m = typeCommand(t, m, "res")
	m = pressSpecial(t, m, tea.KeyTab)

	if got := m.commandInput.Value(); got != "reset" {
		t.Fatalf("tab completed to %q, want reset", got)
	}

	m = pressSpecial(t, m, tea.KeyEnter)

	if m.commandMode {
		t.Error("enter left the bar open")
	}

	if got := m.inputs[testInputEnvironment]; got != testValueStaging {
		t.Errorf("environment is %q after :reset, want the default %q", got, testValueStaging)
	}
}

// TestJourney_CommandBarListsAnAmbiguousPrefix keeps tab from choosing between
// two commands: it fills in what they share and shows what it could not decide.
func TestJourney_CommandBarListsAnAmbiguousPrefix(t *testing.T) {
	t.Parallel()

	m := typeCommand(t, resize(t, newRenderModel(), 120, 40), "w")
	m = pressSpecial(t, m, tea.KeyTab)

	if len(m.completions) == 0 && m.commandInput.Value() == "w" {
		t.Fatal("tab on an ambiguous prefix neither completed nor listed")
	}

	view := m.View().Content
	for _, want := range []string{"watch", "workflow"} {
		if !strings.Contains(view, want) {
			t.Errorf("the completion list is missing %q:\n%s", want, view)
		}
	}

	// The footer has one row, so listing candidates must not claim a second.
	assertFits(t, view, 120, 40)
}

// TestJourney_CommandBarOwnsItsKeys guards the reason the bar is routed before
// everything else: a command's own letters must not also fire the actions they
// are bound to.
func TestJourney_CommandBarOwnsItsKeys(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)
	before := m.watchRun

	// "watch" contains q, b, and w, each of which is bound to something.
	m = typeCommand(t, m, "watch")

	if m.watchRun != before {
		t.Error("typing the command name toggled watch while the bar was open")
	}

	if m.modalStack.HasActive() {
		t.Error("typing the command name opened a modal")
	}

	if got := m.commandInput.Value(); got != "watch" {
		t.Errorf("the bar holds %q, want watch", got)
	}

	m = pressSpecial(t, m, tea.KeyEnter)

	if m.watchRun == before {
		t.Error("running :watch did not toggle it")
	}
}

func TestJourney_CommandBarReportsAnUnknownCommand(t *testing.T) {
	t.Parallel()

	m := typeCommand(t, resize(t, newRenderModel(), 120, 40), "nope")
	m = pressSpecial(t, m, tea.KeyEnter)

	if m.status == "" {
		t.Fatal("an unknown command said nothing")
	}

	if view := m.View().Content; !strings.Contains(view, "nope") {
		t.Errorf("the footer does not name the command it rejected:\n%s", view)
	}
}

// TestJourney_EscapeAbandonsTheCommandBar keeps the bar from trapping input.
func TestJourney_EscapeAbandonsTheCommandBar(t *testing.T) {
	t.Parallel()

	m := typeCommand(t, resize(t, newRenderModel(), 120, 40), "quit")
	m = pressSpecial(t, m, tea.KeyEscape)

	if m.commandMode {
		t.Error("escape left the bar open")
	}

	m = pressRune(t, m, 'a')
	if !m.modalStack.HasActive() {
		t.Error("keys did not reach the app after the bar closed")
	}
}

// TestDefaultRegistryIsComplete keeps a command from shipping with no way to
// find it: every one needs a name and the one-line description the bar shows.
func TestDefaultRegistryIsComplete(t *testing.T) {
	t.Parallel()

	commands := DefaultRegistry().Commands()
	if len(commands) == 0 {
		t.Fatal("no commands registered")
	}

	seen := make(map[string]bool, len(commands))

	for _, command := range commands {
		if command.Name == "" || command.Description == "" {
			t.Errorf("command %+v is missing a name or description", command)
		}

		if command.Run == nil {
			t.Errorf("command %q has nothing to run", command.Name)
		}

		if seen[command.Name] {
			t.Errorf("command %q is registered twice", command.Name)
		}

		seen[command.Name] = true
	}
}

// runLine runs one typed line and hands back what it asked for, without feeding
// the result to Update: a command's answer is the message it returns, and
// routing it further is what the handler tests already cover.
func runLine(t *testing.T, m Model, line string) (Model, tea.Cmd) {
	t.Helper()

	model, cmd := m.runCommandLine(line)

	return asModel(t, model), cmd
}

func statusOf(t *testing.T, cmd tea.Cmd) string {
	t.Helper()

	if cmd == nil {
		t.Fatal("the command answered with nothing, want a status line")
	}

	status, ok := cmd().(StatusMsg)
	if !ok {
		t.Fatalf("the command answered with %T, want a status line", cmd())
	}

	return status.Text
}

// A name you half-remember is the reason the bar exists, so guessing its
// argument wrong has to say what it wanted rather than looking like it ran.
func TestCommands_SayWhatTheyWantedWhenTheArgumentIsWrong(t *testing.T) {
	t.Parallel()

	tests := []struct {
		line string
		want string
	}{
		{line: ":branch", want: "usage: :branch <name>"},
		{line: ":chain", want: "usage: :chain <name>"},
		{line: ":chain nope", want: `no chain named "nope"`},
		{line: ":diagnose", want: "usage: :diagnose <run-id>"},
		{line: ":diagnose abc", want: `"abc" is not a run ID`},
		{line: ":logs", want: "usage: :logs <run-id>"},
		{line: ":logs abc", want: `"abc" is not a run ID`},
		{line: ":runs nowhere", want: "scopes are branch, mine, and reviewing"},
		{line: ":timeline", want: "no run to draw a timeline for"},
		{line: ":timeline abc", want: `"abc" is not a run ID`},
		{line: ":workflow", want: "usage: :workflow <file>"},
		{line: ":workflow nope.yml", want: `no workflow named "nope.yml"`},
		{line: ":nope", want: "no command matches :nope"},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			t.Parallel()

			m := resize(t, newRenderModel(), 120, 40)
			m.wfdConfig = testChainConfig()

			_, cmd := runLine(t, m, strings.TrimPrefix(tt.line, ":"))
			if got := statusOf(t, cmd); got != tt.want {
				t.Errorf("%s answered %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

// What each command changes when its argument is right. These are the routes
// no key reaches, since a workflow named by hand is the whole point of
// :workflow.
func TestCommands_ChangeWhatTheirNameSays(t *testing.T) {
	t.Parallel()

	tests := []struct {
		check func(*testing.T, Model)
		name  string
		line  string
	}{
		{name: "branch", line: "branch topic", check: func(t *testing.T, m Model) {
			t.Helper()

			if m.branch != "topic" {
				t.Errorf("the dispatch ref is %q, want topic", m.branch)
			}
		}},
		{name: "chain", line: "chain " + testChainName, check: func(t *testing.T, m Model) {
			t.Helper()

			if m.pendingChainName != testChainName {
				t.Errorf("the chain flow started on %q", m.pendingChainName)
			}
		}},
		{name: "filter", line: "filter env", check: func(t *testing.T, m Model) {
			t.Helper()

			if m.filterText != "env" {
				t.Errorf("the config filter is %q, want env", m.filterText)
			}
		}},
		{name: "help", line: "help", check: func(t *testing.T, m Model) {
			t.Helper()

			if !m.modalStack.HasActive() {
				t.Error("the keyboard reference did not open")
			}
		}},
		{name: "runs", line: "runs mine", check: func(t *testing.T, m Model) {
			t.Helper()

			if m.focused != PaneRight || m.rightPanel.ActiveTab() != panes.TabRuns {
				t.Error(":runs did not move to the Runs tab")
			}

			if got := m.rightPanel.Runs().Scope(); got != panes.ScopeMine {
				t.Errorf("the scope is %v, want mine", got)
			}
		}},
		{name: "reset", line: "reset", check: func(t *testing.T, m Model) {
			t.Helper()

			if got := m.inputs[testInputEnvironment]; got != testValueStaging {
				t.Errorf("environment is %q, want the default %q", got, testValueStaging)
			}
		}},
		{name: "watch", line: "watch", check: func(t *testing.T, m Model) {
			t.Helper()

			if !m.watchRun {
				t.Error(":watch did not turn watching on")
			}
		}},
		{name: "workflow", line: "workflow ci.yml", check: func(t *testing.T, m Model) {
			t.Helper()

			if m.selectedWorkflow != 1 {
				t.Errorf(":workflow selected %d, want the CI workflow", m.selectedWorkflow)
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := resize(t, newRenderModel(), 120, 40)
			m.wfdConfig = testChainConfig()
			m.inputs[testInputEnvironment] = "production"

			after, _ := runLine(t, m, tt.line)
			tt.check(t, after)
		})
	}
}

// A run ID typed by hand is what :logs, :diagnose, and :timeline exist for, so
// each has to ask for exactly the run named and nothing else.
func TestCommands_AskForTheRunTheyName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		want tea.Msg
		name string
		line string
	}{
		{name: "diagnose", line: "diagnose 42", want: FetchLogsMsg{RunID: 42, ErrorsOnly: true}},
		{name: "logs", line: "logs 42", want: FetchLogsMsg{RunID: 42}},
		{name: "timeline", line: "timeline 42", want: FetchTimelineMsg{RunID: 42, Title: "run 42"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, cmd := runLine(t, resize(t, newRenderModel(), 120, 40), tt.line)
			if cmd == nil {
				t.Fatalf(":%s asked for nothing", tt.name)
			}

			if got := cmd(); got != tt.want {
				t.Errorf(":%s asked for %#v, want %#v", tt.name, got, tt.want)
			}
		})
	}

	_, cmd := runLine(t, resize(t, newRenderModel(), 120, 40), "quit")
	if cmd == nil {
		t.Fatal(":quit answered nothing")
	}

	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Errorf(":quit answered %T", cmd())
	}
}

// Completing an argument is what makes a command guessable past its name, so
// each command that takes one has to offer what it will accept.
func TestCommands_CompleteTheirOwnArguments(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want string
	}{
		{name: "a command name", line: "wor", want: nameWorkflow},
		{name: "a workflow file", line: "workflow ", want: "ci.yml"},
		{name: "a workflow prefix", line: "workflow dep", want: "deploy.yml"},
		{name: "a chain name", line: "chain ", want: testChainName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := resize(t, newRenderModel(), 120, 40)
			m.wfdConfig = testChainConfig()

			candidates, completable := m.registry.completionsFor(m, tt.line)
			if !completable {
				t.Fatalf("%q completes nothing", tt.line)
			}

			found := false
			for _, candidate := range candidates {
				if candidate.Name == tt.want {
					found = true
				}
			}

			if !found {
				t.Errorf("%q offers %v, missing %q", tt.line, candidates, tt.want)
			}
		})
	}

	// A command with nothing to complete says so rather than offering the
	// command names again, which would complete a word that is not one.
	m := resize(t, newRenderModel(), 120, 40)
	if _, completable := m.registry.completionsFor(m, "watch "); completable {
		t.Error("a command taking no argument still offered completions")
	}
}

// The listing is the reference for a caller with no help modal, so every
// registered command has to appear in it beside its description.
func TestRegistryListing_NamesEveryCommand(t *testing.T) {
	t.Parallel()

	registry := DefaultRegistry()
	listing := registry.Listing()

	for _, command := range registry.Commands() {
		if !strings.Contains(listing, command.Name+"\t"+command.Description) {
			t.Errorf("the listing is missing %q:\n%s", command.Name, listing)
		}
	}
}
