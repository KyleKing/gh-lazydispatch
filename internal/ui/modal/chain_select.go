package modal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/lazydispatch/internal/config"
	"github.com/kyleking/lazydispatch/internal/ui"
)

// ChainSelectModal displays available chains for selection.
type ChainSelectModal struct {
	chains   map[string]config.Chain
	names    []string
	selected int
	done     bool
	keys     chainSelectKeyMap
}

type chainSelectKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
}

func defaultChainSelectKeyMap() chainSelectKeyMap {
	return chainSelectKeyMap{
		Up:     key.NewBinding(key.WithKeys("up", "k")),
		Down:   key.NewBinding(key.WithKeys("down", "j")),
		Enter:  key.NewBinding(key.WithKeys("enter")),
		Escape: key.NewBinding(key.WithKeys("esc")),
	}
}

// NewChainSelectModal creates a new chain selection modal.
func NewChainSelectModal(cfg *config.WfdConfig) *ChainSelectModal {
	names := cfg.ChainNames()
	return &ChainSelectModal{
		chains: cfg.Chains,
		names:  names,
		keys:   defaultChainSelectKeyMap(),
	}
}

// Update handles input for the chain select modal.
func (m *ChainSelectModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.names)-1 {
				m.selected++
			}
		case key.Matches(msg, m.keys.Enter):
			m.done = true
			if m.selected >= 0 && m.selected < len(m.names) {
				name := m.names[m.selected]
				chain := m.chains[name]
				return m, func() tea.Msg {
					return ChainSelectResultMsg{
						ChainName: name,
						Chain:     chain,
					}
				}
			}
		case key.Matches(msg, m.keys.Escape):
			m.done = true
		}
	}
	return m, nil
}

// View renders the chain select modal.
func (m *ChainSelectModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Select Chain"))
	s.WriteString("\n\n")

	if len(m.names) == 0 {
		s.WriteString(ui.SubtitleStyle.Render("No chains defined"))
		s.WriteString("\n\n")
		s.WriteString(ui.HelpStyle.Render("Press Esc to close"))
		return s.String()
	}

	for i, name := range m.names {
		chain := m.chains[name]
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}

		line := fmt.Sprintf("%s%s", prefix, name)
		if chain.Description != "" {
			line += fmt.Sprintf(" - %s", chain.Description)
		}
		line += fmt.Sprintf(" (%d steps)", len(chain.Steps))

		if i == m.selected {
			s.WriteString(ui.SelectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}
		s.WriteString("\n")

		if i == m.selected {
			for j, step := range chain.Steps {
				stepLine := fmt.Sprintf("      %d. %s", j+1, step.Workflow)
				if step.WaitFor != "" && step.WaitFor != config.WaitSuccess {
					stepLine += fmt.Sprintf(" (wait: %s)", step.WaitFor)
				}
				s.WriteString(ui.SubtitleStyle.Render(stepLine))
				s.WriteString("\n")
			}
		}
	}

	s.WriteString("\n")
	s.WriteString(ui.HelpStyle.Render("j/k navigate  enter select  esc cancel"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *ChainSelectModal) IsDone() bool {
	return m.done
}

// Result returns nil for chain select modal.
func (m *ChainSelectModal) Result() any {
	return nil
}

// ChainSelectResultMsg is sent when a chain is selected.
type ChainSelectResultMsg struct {
	ChainName string
	Chain     config.Chain
}
