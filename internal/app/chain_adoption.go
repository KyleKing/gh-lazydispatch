package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
)

// ChainStepAdoptedMsg carries what a source: existing step would adopt, or
// the reason it found nothing, for the confirm modal to show before the chain
// runs.
type ChainStepAdoptedMsg struct {
	Err       error
	Run       *github.WorkflowRun
	StepIndex int
}

// resolveChainAdoptionCmd resolves the run each source: existing step in
// chainDef would adopt, one message per such step, so the confirm modal can
// name it before the user commits. Nothing fires for a chain with no such
// step.
func (m Model) resolveChainAdoptionCmd(chainDef *config.Chain, branch string) tea.Cmd {
	if m.ghClient == nil {
		return nil
	}

	client := m.ghClient

	var cmds []tea.Cmd

	for i, step := range chainDef.Steps {
		if step.Source != config.SourceExisting {
			continue
		}

		idx, workflowFile := i, step.Workflow
		cmds = append(cmds, func() tea.Msg {
			run, err := chain.ResolveExistingRun(client, workflowFile, branch)
			return ChainStepAdoptedMsg{StepIndex: idx, Run: run, Err: err}
		})
	}

	if len(cmds) == 0 {
		return nil
	}

	return tea.Batch(cmds...)
}

// handleChainStepAdoptedMsg pushes a resolved adoption into the open chain
// confirm modal, if any.
func (m Model) handleChainStepAdoptedMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	adopted, ok := msg.(ChainStepAdoptedMsg)
	if !ok {
		return m, nil, false
	}

	found := m.modalStack.Find(func(ctx modal.Context) bool {
		_, ok := ctx.(*modal.ChainConfirmModal)
		return ok
	})

	if confirmModal, ok := found.(*modal.ChainConfirmModal); ok {
		confirmModal.SetAdoptedRun(adopted.StepIndex, adopted.Run, adopted.Err)
	}

	return m, nil, true
}
