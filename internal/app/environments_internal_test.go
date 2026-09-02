package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// errRefused stands in for a token that cannot read the environments.
var errRefused = errors.New("HTTP 403")

func environmentWorkflow() []workflow.File {
	return []workflow.File{{
		Name:     "Deploy",
		Filename: "deploy.yml",
		On: workflow.OnTrigger{Dispatch: &workflow.Dispatch{
			Inputs: map[string]workflow.Input{"target": {Type: inputTypeEnvironment}},
		}},
	}}
}

// An environment input names one of the repository's deployment environments,
// which nothing in the workflow file lists. Once they are read it picks from
// them; until then it is free text that says the value goes out as typed.
func TestEnvironmentInput_PicksFromTheEnvironmentsTheRepositoryHas(t *testing.T) {
	t.Parallel()

	m := resize(t, New(environmentWorkflow(), testHistory(), "owner/repo"), 120, 40)
	m.ghClient = nil

	model, _ := m.openInputModalForName("target")

	unresolved, ok := model.(Model)
	if !ok {
		t.Fatalf("opening the input returned %T", model)
	}

	if !strings.Contains(ansi.Strip(unresolved.modalStack.Current().View()), "could not be read") {
		t.Error("an unresolved environment input does not say the value is sent as typed")
	}

	loaded, _, handled := m.handleEnvironmentsMsg(EnvironmentsFetchedMsg{Names: []string{"production", "staging"}})
	if !handled {
		t.Fatal("the fetched environments were not routed")
	}

	resolved, ok := loaded.(Model)
	if !ok {
		t.Fatalf("routing returned %T, want a Model", loaded)
	}

	model, _ = resolved.openInputModalForName("target")

	picker, ok := model.(Model)
	if !ok {
		t.Fatalf("opening the input returned %T", model)
	}

	view := ansi.Strip(picker.modalStack.Current().View())
	for _, want := range []string{"production", "staging"} {
		if !strings.Contains(view, want) {
			t.Errorf("the environment picker does not offer %q:\n%s", want, view)
		}
	}
}

// A failed read leaves the input on free text rather than on an empty picker,
// and is not reported: nothing asked for it, and the modal says what it is
// doing when it is opened.
func TestEnvironments_AFailedReadFallsBackWithoutReportingIt(t *testing.T) {
	t.Parallel()

	m := resize(t, New(environmentWorkflow(), testHistory(), "owner/repo"), 120, 40)

	model, cmd, handled := m.handleEnvironmentsMsg(EnvironmentsFetchedMsg{Error: errRefused})
	if !handled || cmd != nil {
		t.Fatalf("a failed read was handled=%v with cmd=%v", handled, cmd != nil)
	}

	failed, ok := model.(Model)
	if !ok {
		t.Fatalf("routing returned %T, want a Model", model)
	}

	if len(failed.environments) != 0 || failed.status != "" {
		t.Errorf("a failed read left %d environments and status %q", len(failed.environments), failed.status)
	}

	// Loaded is set either way, so a repository that cannot answer is asked once.
	if !failed.environmentsLoaded || failed.loadEnvironmentsCmd() != nil {
		t.Error("a failed read left the environments queued for another attempt")
	}
}

// A repository whose workflows declare no environment input spends no call on
// the endpoint at all.
func TestLoadEnvironments_AsksOnlyWhereAWorkflowNeedsThem(t *testing.T) {
	t.Parallel()

	if got := New(testWorkflows(), testHistory(), "owner/repo").loadEnvironmentsCmd(); got != nil {
		t.Error("a repository with no environment input still read the endpoint")
	}
}
