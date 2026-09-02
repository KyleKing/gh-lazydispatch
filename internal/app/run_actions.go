package app

import (
	"strconv"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
)

// RunMutationDoneMsg reports what a re-run or a cancel did.
type RunMutationDoneMsg struct {
	Error error
	Kind  modal.RunActionKind
	RunID int64
}

func (m Model) rerunSelectedRun() (tea.Model, tea.Cmd) {
	return m.confirmRunAction(modal.RunActionRerun)
}

func (m Model) cancelSelectedRun() (tea.Model, tea.Cmd) {
	return m.confirmRunAction(modal.RunActionCancel)
}

// confirmRunAction asks before mutating somebody else's run, the same way a
// dispatch is asked about before it goes out.
func (m Model) confirmRunAction(kind modal.RunActionKind) (tea.Model, tea.Cmd) {
	runID, name, ok := m.selectedRun()
	if !ok {
		return m, statusCmd("no run selected")
	}

	if m.ghClient == nil {
		return m, statusCmd("no GitHub client")
	}

	m.modalStack.Push(modal.NewRunActionModal(kind, runID, name))

	return m, nil
}

// handleRunActionResult runs the mutation the modal confirmed.
func (m Model) handleRunActionResult(msg modal.RunActionResultMsg) (tea.Model, tea.Cmd) {
	if !msg.Confirmed || m.ghClient == nil {
		return m, nil
	}

	return m, runMutationCmd(m.ghClient, msg.Kind, msg.RunID)
}

func runMutationCmd(client *github.Client, kind modal.RunActionKind, runID int64) tea.Cmd {
	return func() tea.Msg {
		var err error
		if kind == modal.RunActionRerun {
			err = client.RerunFailedJobs(runID)
		} else {
			err = client.CancelRun(runID)
		}

		return RunMutationDoneMsg{Kind: kind, RunID: runID, Error: err}
	}
}

// handleRunMutationMsg reports the outcome and drops the cached listing, since
// what the mutation changed is exactly what that listing was showing.
func (m Model) handleRunMutationMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	done, ok := msg.(RunMutationDoneMsg)
	if !ok {
		return m, nil, false
	}

	verb := "re-running"
	if done.Kind == modal.RunActionCancel {
		verb = "canceling"
	}

	id := strconv.FormatInt(done.RunID, 10)

	if done.Error != nil {
		return m, statusCmd(verb + " run " + id + " failed: " + done.Error.Error()), true
	}

	m.rightPanel.Runs().Invalidate()

	return m, statusCmd(verb + " run " + id), true
}
