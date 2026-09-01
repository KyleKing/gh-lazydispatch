package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
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
