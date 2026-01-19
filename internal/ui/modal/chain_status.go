package modal

import (
	"fmt"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ChainStatusStopMsg is sent when the user requests to stop the chain.
type ChainStatusStopMsg struct{}

// ChainStatusModal displays the current status of a chain execution.
type ChainStatusModal struct {
	state    chain.ChainState
	commands []string
	branch   string
	done     bool
	stopped  bool
	copied   bool
	keys     chainStatusKeyMap
}

type chainStatusKeyMap struct {
	Close key.Binding
	Stop  key.Binding
	Copy  key.Binding
}

func defaultChainStatusKeyMap() chainStatusKeyMap {
	return chainStatusKeyMap{
		Close: key.NewBinding(key.WithKeys("esc", "q")),
		Stop:  key.NewBinding(key.WithKeys("ctrl+c")),
		Copy:  key.NewBinding(key.WithKeys("c")),
	}
}

// NewChainStatusModal creates a new chain status modal.
func NewChainStatusModal(state chain.ChainState) *ChainStatusModal {
	return &ChainStatusModal{
		state: state,
		keys:  defaultChainStatusKeyMap(),
	}
}

// NewChainStatusModalWithCommands creates a chain status modal with command strings.
func NewChainStatusModalWithCommands(state chain.ChainState, commands []string, branch string) *ChainStatusModal {
	return &ChainStatusModal{
		state:    state,
		commands: commands,
		branch:   branch,
		keys:     defaultChainStatusKeyMap(),
	}
}

// UpdateState updates the chain state displayed in the modal.
func (m *ChainStatusModal) UpdateState(state chain.ChainState) {
	m.state = state
}

// SetCommands sets the command strings for each step.
func (m *ChainStatusModal) SetCommands(commands []string, branch string) {
	m.commands = commands
	m.branch = branch
}

// Update handles input for the chain status modal.
func (m *ChainStatusModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Close):
			m.done = true
			return m, nil
		case key.Matches(msg, m.keys.Stop):
			m.stopped = true
			m.done = true
			return m, func() tea.Msg {
				return ChainStatusStopMsg{}
			}
		case key.Matches(msg, m.keys.Copy):
			script := m.buildBashScript()
			clipboard.WriteAll(script)
			m.copied = true
			return m, nil
		}
	}
	return m, nil
}

func (m *ChainStatusModal) buildBashScript() string {
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("# Chain: ")
	sb.WriteString(m.state.ChainName)
	sb.WriteString("\n")
	sb.WriteString("# WARNING: This is a simplified export. Wait conditions and failure handling are not included.\n\n")
	sb.WriteString("set -e\n\n")

	for i, cmd := range m.commands {
		sb.WriteString(fmt.Sprintf("# Step %d\n", i+1))
		sb.WriteString(cmd)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// View renders the chain status modal.
func (m *ChainStatusModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render(fmt.Sprintf("Chain: %s", m.state.ChainName)))
	s.WriteString("\n\n")

	s.WriteString(ui.SubtitleStyle.Render(fmt.Sprintf("Status: %s", m.state.Status)))
	if m.branch != "" {
		s.WriteString("  ")
		s.WriteString(ui.TableDimmedStyle.Render(fmt.Sprintf("(branch: %s)", m.branch)))
	}
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

		if i < len(m.commands) && m.commands[i] != "" {
			s.WriteString(ui.CLIPreviewStyle.Render("     " + m.commands[i]))
			s.WriteString("\n")
		}
	}

	if m.state.Error != nil {
		s.WriteString("\n")
		s.WriteString(ui.SelectedStyle.Render(fmt.Sprintf("Error: %s", m.state.Error.Error())))
		s.WriteString("\n")
	}

	s.WriteString("\n")

	if m.copied {
		s.WriteString(ui.SubtitleStyle.Render("Script copied to clipboard!"))
		s.WriteString("\n\n")
	}

	if m.state.Status == chain.ChainRunning {
		s.WriteString(ui.HelpStyle.Render("[esc/q] close (continues)  [C-c] stop  [c] copy script"))
	} else {
		s.WriteString(ui.HelpStyle.Render("[esc/q] close  [c] copy script"))
	}

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *ChainStatusModal) IsDone() bool {
	return m.done
}

// WasStopped returns true if the user requested to stop the chain.
func (m *ChainStatusModal) WasStopped() bool {
	return m.stopped
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
