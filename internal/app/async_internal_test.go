package app

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
)

// Update routed every message to the modal stack whenever a modal was open, so
// a chain's status modal could not see its own updates and the log fetch behind
// the viewer never landed. A modal is open for most of the time these messages
// arrive, so that is the case worth pinning.
func TestUpdate_AsyncMessagesReachTheirHandlersBehindAModal(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		msg    tea.Msg
		assert func(t *testing.T, m Model)
	}{
		{
			name: "a chain update refreshes the status modal",
			msg: ChainUpdateMsg{Update: chain.ChainUpdate{State: chain.ChainState{
				ChainName:    "demo",
				Status:       chain.ChainCompleted,
				StepStatuses: []chain.StepStatus{chain.StepCompleted},
			}}},
			assert: func(t *testing.T, m Model) {
				t.Helper()

				status, ok := m.modalStack.Current().(*modal.ChainStatusModal)
				if !ok {
					t.Fatalf("top of the stack is %T", m.modalStack.Current())
				}

				if view := status.View(); !strings.Contains(view, string(chain.ChainCompleted)) {
					t.Errorf("the status modal still reports the old state:\n%s", view)
				}
			},
		},
		{
			name: "fetched logs open the viewer",
			msg:  LogsFetchedMsg{Logs: logsWithOneEntry(), RunID: 1},
			assert: func(t *testing.T, m Model) {
				t.Helper()

				if _, ok := m.modalStack.Current().(*modal.LogsViewerModal); !ok {
					t.Fatalf("top of the stack is %T, want the logs viewer", m.modalStack.Current())
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			m := resize(t, newRenderModel(), 120, 40)
			m.chainExecutor = &chain.ChainExecutor{}
			m.modalStack.Push(modal.NewChainStatusModal(chain.ChainState{
				ChainName:    "demo",
				Status:       chain.ChainRunning,
				StepStatuses: []chain.StepStatus{chain.StepRunning},
			}))

			if !m.modalStack.HasActive() {
				t.Fatal("the case needs a modal open to be meaningful")
			}

			tt.assert(t, applyMsg(t, m, tt.msg))
		})
	}
}

func logsWithOneEntry() *logs.RunLogs {
	runLogs := logs.NewRunLogs("demo", "main")
	runLogs.AddStep(&logs.StepLogs{
		Workflow: "demo-test.yml",
		Entries:  []logs.LogEntry{{Content: "hello", Level: logs.LogLevelInfo}},
	})

	return runLogs
}
