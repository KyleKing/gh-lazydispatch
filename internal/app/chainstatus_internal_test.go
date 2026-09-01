package app

import (
	"strings"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
)

// The status modal holds a snapshot of the chain state, so every executor
// update has to be pushed into it; otherwise the screen reports the chain's
// first step for the whole run.
func TestChainUpdate_RefreshesTheStatusModal(t *testing.T) {
	t.Parallel()

	m := resize(t, newRenderModel(), 120, 40)

	statusModal := modal.NewChainStatusModal(chain.ChainState{
		ChainName:    "demo",
		Status:       chain.ChainRunning,
		CurrentStep:  0,
		StepStatuses: []chain.StepStatus{chain.StepRunning, chain.StepPending},
	})
	m.modalStack.Push(statusModal)

	before := statusModal.View()

	m.refreshChainStatusModal(chain.ChainState{
		ChainName:    "demo",
		Status:       chain.ChainCompleted,
		CurrentStep:  1,
		StepStatuses: []chain.StepStatus{chain.StepCompleted, chain.StepFailed},
	})

	after := statusModal.View()
	if after == before {
		t.Fatalf("status modal did not re-render after the update:\n%s", after)
	}

	if !strings.Contains(after, string(chain.ChainCompleted)) {
		t.Errorf("status modal still reports the old chain status:\n%s", after)
	}
}
