package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/modal"
)

// ChainStepAdoptedMsg carries what a source: existing step would adopt, or
// the reason it found nothing.
type ChainStepAdoptedMsg struct {
	Err       error
	Run       *github.WorkflowRun
	ChainName string
	StepIndex int
}

// resolveChainAdoptionCmd resolves the run each source: existing step in
// chainDef would adopt, one message per such step. Nil for a chain with none.
func (m Model) resolveChainAdoptionCmd(chainName string, chainDef *config.Chain, branch string) tea.Cmd {
	if m.ghClient == nil {
		return nil
	}

	client := m.ghClient

	var cmds []tea.Cmd

	for i, step := range chainDef.Steps {
		if step.Source != config.SourceExisting {
			continue
		}

		cmds = append(cmds, func() tea.Msg {
			run, err := chain.ResolveExistingRun(client, step.Workflow, branch)
			return ChainStepAdoptedMsg{ChainName: chainName, StepIndex: i, Run: run, Err: err}
		})
	}

	if len(cmds) == 0 {
		return nil
	}

	return tea.Batch(cmds...)
}

// handleChainStepAdoptedMsg pushes a resolved adoption into the confirm modal
// still open for that chain, so a stale result never lands on a different one.
func (m Model) handleChainStepAdoptedMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	adopted, ok := msg.(ChainStepAdoptedMsg)
	if !ok {
		return m, nil, false
	}

	found := m.modalStack.Find(func(ctx modal.Context) bool {
		confirmModal, ok := ctx.(*modal.ChainConfirmModal)
		return ok && confirmModal.ChainName() == adopted.ChainName
	})

	if confirmModal, ok := found.(*modal.ChainConfirmModal); ok {
		confirmModal.SetAdoptedRun(adopted.StepIndex, adopted.Run, adopted.Err)
	}

	return m, nil, true
}

// pinnedAdoptionResolver adopts the run already shown in the confirm modal,
// falling back to a live resolution for any workflow it didn't resolve.
func pinnedAdoptionResolver(pinned map[string]*github.WorkflowRun) chain.ExistingRunResolver {
	return func(client chain.GitHubClient, workflow, branch string) (*github.WorkflowRun, error) {
		if run, ok := pinned[workflow]; ok {
			return run, nil
		}

		return chain.ResolveExistingRun(client, workflow, branch)
	}
}
