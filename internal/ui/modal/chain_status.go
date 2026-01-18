package modal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/lazydispatch/internal/chain"
	"github.com/kyleking/lazydispatch/internal/ui"
)

// ChainStatusModal displays the current status of a chain execution.
type ChainStatusModal struct {
	state chain.ChainState
	done  bool
	keys  chainStatusKeyMap
}

type chainStatusKeyMap struct {
	Close key.Binding
}

func defaultChainStatusKeyMap() chainStatusKeyMap {
	return chainStatusKeyMap{
		Close: key.NewBinding(key.WithKeys("esc", "q")),
	}
}

// NewChainStatusModal creates a new chain status modal.
func NewChainStatusModal(state chain.ChainState) *ChainStatusModal {
	return &ChainStatusModal{
		state: state,
		keys:  defaultChainStatusKeyMap(),
	}
}

// UpdateState updates the chain state displayed in the modal.
func (m *ChainStatusModal) UpdateState(state chain.ChainState) {
	m.state = state
}

// Update handles input for the chain status modal.
func (m *ChainStatusModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Close) {
			m.done = true
		}
	}
	return m, nil
}

// View renders the chain status modal.
func (m *ChainStatusModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render(fmt.Sprintf("Chain: %s", m.state.ChainName)))
	s.WriteString("\n\n")

	s.WriteString(ui.SubtitleStyle.Render(fmt.Sprintf("Status: %s", m.state.Status)))
	s.WriteString("\n\n")

	s.WriteString(ui.SubtitleStyle.Render("Steps:"))
	s.WriteString("\n")

	for i, status := range m.state.StepStatuses {
		icon := stepStatusIcon(status)
		prefix := "  "
		if i == m.state.CurrentStep && m.state.Status == chain.ChainRunning {
			prefix = "> "
		}

		var stepName string
		if result, ok := m.state.StepResults[i]; ok {
			stepName = result.Workflow
		} else {
			stepName = fmt.Sprintf("Step %d", i+1)
		}

		line := fmt.Sprintf("%s%s %s (%s)", prefix, icon, stepName, status)

		if i == m.state.CurrentStep && m.state.Status == chain.ChainRunning {
			s.WriteString(ui.SelectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}
		s.WriteString("\n")
	}

	if m.state.Error != nil {
		s.WriteString("\n")
		s.WriteString(ui.SelectedStyle.Render(fmt.Sprintf("Error: %s", m.state.Error.Error())))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(ui.HelpStyle.Render("Press Esc or q to close"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *ChainStatusModal) IsDone() bool {
	return m.done
}

// Result returns nil for chain status modal.
func (m *ChainStatusModal) Result() any {
	return nil
}

func stepStatusIcon(status chain.StepStatus) string {
	switch status {
	case chain.StepPending:
		return "o"
	case chain.StepRunning:
		return "*"
	case chain.StepWaiting:
		return "~"
	case chain.StepCompleted:
		return "+"
	case chain.StepFailed:
		return "x"
	case chain.StepSkipped:
		return "-"
	default:
		return "?"
	}
}
