package app

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

func modelOnARun(t *testing.T) Model {
	t.Helper()

	m := resize(t, newRenderModel(), 120, 40)
	m.focused = PaneRight
	m.rightPanel.SetTab(panes.TabRuns)
	m.rightPanel.Runs().SetRuns(panes.ScopeBranch, "main", []github.WorkflowRun{
		run(11, "CI", time.Minute, github.StatusCompleted, github.ConclusionFailure),
	})

	client, err := github.NewClientWithExecutor("owner/repo", nil)
	if err != nil {
		t.Fatal(err)
	}

	m.ghClient = client

	return m
}

// Re-running or canceling somebody's run reaches GitHub, so it is confirmed
// the way a dispatch is, and the confirmation shows the gh command rather than
// describing it.
func TestRunActions_ConfirmBeforeTheyReachGitHub(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		open func(Model) (tea.Model, tea.Cmd)
		kind modal.RunActionKind
		want string
	}{
		{name: "rerun", open: Model.rerunSelectedRun, kind: modal.RunActionRerun, want: "gh run rerun 11 --failed"},
		{name: "cancel", open: Model.cancelSelectedRun, kind: modal.RunActionCancel, want: "gh run cancel 11"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			model, _ := tt.open(modelOnARun(t))

			opened, ok := model.(Model)
			if !ok {
				t.Fatalf("opening the confirmation returned %T", model)
			}

			view := ansi.Strip(opened.modalStack.Current().View())
			if !strings.Contains(view, tt.want) {
				t.Errorf("the confirmation does not show %q:\n%s", tt.want, view)
			}

			// Declining reaches nothing.
			declined, cmd := opened.handleRunActionResult(
				modal.RunActionResultMsg{Kind: tt.kind, RunID: 11, Confirmed: false},
			)
			if cmd != nil {
				t.Error("declining still issued the mutation")
			}

			if _, ok := declined.(Model); !ok {
				t.Fatalf("declining returned %T", declined)
			}
		})
	}
}

// A failed mutation says so. The dispatch path already learned once that a
// message with no handler makes a failure look exactly like a success.
func TestRunMutation_ReportsBothOutcomes(t *testing.T) {
	t.Parallel()

	m := modelOnARun(t)

	_, cmd, handled := m.handleRunMutationMsg(RunMutationDoneMsg{Kind: modal.RunActionCancel, RunID: 11})
	if !handled || cmd == nil {
		t.Fatal("a successful cancel reported nothing")
	}

	if got := statusText(t, cmd); !strings.Contains(got, "canceling run 11") {
		t.Errorf("a successful cancel said %q", got)
	}

	_, cmd, _ = m.handleRunMutationMsg(RunMutationDoneMsg{
		Kind: modal.RunActionRerun, RunID: 11, Error: errRefused,
	})

	if got := statusText(t, cmd); !strings.Contains(got, "failed") || !strings.Contains(got, "re-running run 11") {
		t.Errorf("a failed re-run said %q", got)
	}
}

func statusText(t *testing.T, cmd tea.Cmd) string {
	t.Helper()

	if cmd == nil {
		t.Fatal("no status was sent")
	}

	status, ok := cmd().(StatusMsg)
	if !ok {
		t.Fatalf("the command sent %T, want a StatusMsg", cmd())
	}

	return status.Text
}

// A run has to be selected, and a client has to exist, before either verb
// offers to reach GitHub.
func TestRunActions_NeedARunAndAClient(t *testing.T) {
	t.Parallel()

	empty := resize(t, newRenderModel(), 120, 40)
	empty.focused = PaneRight

	if _, cmd := empty.cancelSelectedRun(); cmd == nil {
		t.Error("canceling with nothing selected opened a confirmation")
	}

	noClient := modelOnARun(t)
	noClient.ghClient = nil

	if _, cmd := noClient.rerunSelectedRun(); cmd == nil {
		t.Error("re-running with no client opened a confirmation")
	}

	if _, cmd := noClient.handleRunActionResult(
		modal.RunActionResultMsg{Kind: modal.RunActionRerun, RunID: 11, Confirmed: true},
	); cmd != nil {
		t.Error("confirming with no client still issued the mutation")
	}
}
