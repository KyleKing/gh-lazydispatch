package modal

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/atotto/clipboard"

	"github.com/kyleking/gh-lazydispatch/internal/browser"
	"github.com/kyleking/gh-lazydispatch/internal/chain"
	chainerr "github.com/kyleking/gh-lazydispatch/internal/errors"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ChainStatusStopMsg is sent when the user requests to stop the chain.
type ChainStatusStopMsg struct{}

// ChainStatusViewLogsMsg is sent when the user requests to view logs.
type ChainStatusViewLogsMsg struct {
	Branch     string
	State      chain.ChainState
	ErrorsOnly bool
}

// ChainStatusModal displays the current status of a chain execution.
type ChainStatusModal struct {
	branch   string
	keys     chainStatusKeyMap
	commands []string
	state    chain.ChainState
	done     bool
	stopped  bool
	copied   bool
}

type chainStatusKeyMap struct {
	Close       key.Binding
	Stop        key.Binding
	Copy        key.Binding
	ViewLogs    key.Binding
	OpenBrowser key.Binding
}

func defaultChainStatusKeyMap() chainStatusKeyMap {
	return chainStatusKeyMap{
		Close:       key.NewBinding(key.WithKeys("esc", "q")),
		Stop:        key.NewBinding(key.WithKeys("ctrl+c")),
		Copy:        key.NewBinding(key.WithKeys("c")),
		ViewLogs:    key.NewBinding(key.WithKeys("v")),
		OpenBrowser: key.NewBinding(key.WithKeys("o")),
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
	if msg, ok := msg.(tea.KeyPressMsg); ok {
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
			//nolint:errcheck,gosec // best-effort clipboard write; no error-surfacing UI hook exists for this action
			clipboard.WriteAll(script)

			m.copied = true

			return m, nil
		case key.Matches(msg, m.keys.ViewLogs):
			if m.state.Status == chain.ChainCompleted || m.state.Status == chain.ChainFailed {
				errorsOnly := m.state.Status == chain.ChainFailed

				return m, func() tea.Msg {
					return ChainStatusViewLogsMsg{
						State:      m.state,
						Branch:     m.branch,
						ErrorsOnly: errorsOnly,
					}
				}
			}
		case key.Matches(msg, m.keys.OpenBrowser):
			if url := m.GetFailedStepRunURL(); url != "" {
				//nolint:errcheck,gosec // best-effort browser launch; no error-surfacing UI hook exists for this action
				browser.Open(url)
			}
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
		fmt.Fprintf(&sb, "# Step %d\n", i+1)
		sb.WriteString(cmd)
		sb.WriteString("\n\n")
	}

	return sb.String()
}

// View renders the chain status modal.
func (m *ChainStatusModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Chain: " + m.state.ChainName))
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
		m.renderStepLine(&s, i, status)
	}

	m.renderError(&s)

	s.WriteString("\n")

	if m.copied {
		s.WriteString(ui.SubtitleStyle.Render("Script copied to clipboard!"))
		s.WriteString("\n\n")
	}

	hasFailedURL := m.GetFailedStepRunURL() != ""

	switch {
	case m.state.Status == chain.ChainRunning:
		s.WriteString(ui.HelpStyle.Render("[esc/q] close (continues)  [C-c] stop  [c] copy script"))
	case m.state.Status == chain.ChainFailed && hasFailedURL:
		s.WriteString(ui.HelpStyle.Render("[esc/q] close  [o] open in browser  [v] view logs  [c] copy script"))
	case m.state.Status == chain.ChainCompleted || m.state.Status == chain.ChainFailed:
		s.WriteString(ui.HelpStyle.Render("[esc/q] close  [v] view logs  [c] copy script"))
	default:
		s.WriteString(ui.HelpStyle.Render("[esc/q] close  [c] copy script"))
	}

	return s.String()
}

// renderStepLine writes a single chain step's status line, plus its preview
// command if one was recorded, to s.
func (m *ChainStatusModal) renderStepLine(s *strings.Builder, i int, status chain.StepStatus) {
	icon := stepStatusIcon(status)

	isCurrent := i == m.state.CurrentStep && m.state.Status == chain.ChainRunning

	prefix := "  "
	if isCurrent {
		prefix = "> "
	}

	var stepName string
	if result, ok := m.state.StepResults[i]; ok {
		stepName = result.Workflow
	} else {
		stepName = fmt.Sprintf("Step %d", i+1)
	}

	line := fmt.Sprintf("%s%s %s (%s)", prefix, icon, stepName, status)

	if isCurrent {
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

// renderError writes the chain's error, run URL, and suggestion (if any) to s.
func (m *ChainStatusModal) renderError(s *strings.Builder) {
	if m.state.Error == nil {
		return
	}

	s.WriteString("\n")
	s.WriteString(ui.ErrorTitleStyle.Render("Error:"))
	s.WriteString("\n")
	s.WriteString(ui.ErrorStyle.Render("  " + m.state.Error.Error()))
	s.WriteString("\n")

	if url := chainerr.GetRunURL(m.state.Error); url != "" {
		s.WriteString(ui.SubtitleStyle.Render("  Run: "))
		s.WriteString(ui.LinkStyle.Render(url))
		s.WriteString("\n")
	}

	if suggestion := chainerr.GetSuggestion(m.state.Error); suggestion != "" {
		s.WriteString(ui.SubtitleStyle.Render("  Hint: "))
		s.WriteString(ui.NormalStyle.Render(suggestion))
		s.WriteString("\n")
	}
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
func (*ChainStatusModal) Result() any {
	return nil
}

// GetFailedStepRunURL returns the URL of the failed step's run, if available.
func (m *ChainStatusModal) GetFailedStepRunURL() string {
	if m.state.Error != nil {
		if url := chainerr.GetRunURL(m.state.Error); url != "" {
			return url
		}
	}

	for _, result := range m.state.StepResults {
		if result != nil && result.Status == chain.StepFailed && result.RunURL != "" {
			return result.RunURL
		}
	}

	return ""
}

// GetDetailedError returns a detailed error message with context.
func (m *ChainStatusModal) GetDetailedError() string {
	if m.state.Error == nil {
		return ""
	}

	var sb strings.Builder

	sb.WriteString(m.state.Error.Error())

	if url := chainerr.GetRunURL(m.state.Error); url != "" {
		sb.WriteString("\nRun URL: ")
		sb.WriteString(url)
	}

	if suggestion := chainerr.GetSuggestion(m.state.Error); suggestion != "" {
		sb.WriteString("\nSuggestion: ")
		sb.WriteString(suggestion)
	}

	return sb.String()
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
