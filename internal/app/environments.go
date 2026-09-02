package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/workflow"
)

// inputTypeEnvironment is the workflow input type whose values are the
// repository's deployment environments rather than anything the workflow
// declares, so they can only come from the API.
const inputTypeEnvironment = "environment"

// EnvironmentsFetchedMsg carries the repository's deployment environments, or
// the reason they could not be read.
type EnvironmentsFetchedMsg struct {
	Error error
	Names []string
}

// loadEnvironmentsCmd reads the deployment environments, and nothing at all
// where no workflow declares an input that needs them. An environment input is
// rare, so a repository without one spends no call on it.
func (m Model) loadEnvironmentsCmd() tea.Cmd {
	if m.ghClient == nil || m.environmentsLoaded || !declaresEnvironmentInput(m.workflows) {
		return nil
	}

	client := m.ghClient

	return func() tea.Msg {
		names, err := client.ListEnvironments()

		return EnvironmentsFetchedMsg{Names: names, Error: err}
	}
}

func declaresEnvironmentInput(workflows []workflow.File) bool {
	for _, wf := range workflows {
		for _, input := range wf.GetInputs() {
			if input.InputType() == inputTypeEnvironment {
				return true
			}
		}
	}

	return false
}

// handleEnvironmentsMsg records the environments an environment input picks
// from. A failure is not reported: the input falls back to free text, which
// says so itself, and a modal over the workflow list at startup would be
// reporting on something nobody asked for yet.
func (m Model) handleEnvironmentsMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	fetched, ok := msg.(EnvironmentsFetchedMsg)
	if !ok {
		return m, nil, false
	}

	m.environmentsLoaded = true
	if fetched.Error == nil {
		m.environments = fetched.Names
	}

	return m, nil, true
}
