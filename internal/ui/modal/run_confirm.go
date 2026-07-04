package modal

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/runner"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// RunConfirmResultMsg is sent when workflow execution is confirmed or canceled.
type RunConfirmResultMsg struct {
	Config    runner.RunConfig
	Confirmed bool
}

type runConfirmKeyMap struct {
	Confirm key.Binding
	Cancel  key.Binding
}

// RunConfirmModal shows the command that will be executed and confirms.
type RunConfirmModal struct {
	keys   runConfirmKeyMap
	config runner.RunConfig
	result RunConfirmResultMsg
	done   bool
}

// NewRunConfirmModal creates a run confirmation modal.
func NewRunConfirmModal(cfg runner.RunConfig) *RunConfirmModal {
	return &RunConfirmModal{
		config: cfg,
		keys: runConfirmKeyMap{
			Confirm: key.NewBinding(key.WithKeys("enter", "y")),
			Cancel:  key.NewBinding(key.WithKeys("esc", "n")),
		},
	}
}

// Update handles input for the run confirm modal.
func (m *RunConfirmModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, m.keys.Confirm):
			m.done = true
			m.result = RunConfirmResultMsg{Confirmed: true, Config: m.config}

			return m, func() tea.Msg {
				return m.result
			}
		case key.Matches(msg, m.keys.Cancel):
			m.done = true
			m.result = RunConfirmResultMsg{Confirmed: false, Config: m.config}

			return m, func() tea.Msg {
				return m.result
			}
		}
	}

	return m, nil
}

// View renders the run confirm modal.
func (m *RunConfirmModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Confirm Workflow Execution"))
	s.WriteString("\n\n")

	s.WriteString(ui.SubtitleStyle.Render("Workflow Details:"))
	s.WriteString("\n\n")

	s.WriteString(ui.NormalStyle.Render("  Workflow: "))
	s.WriteString(ui.TableDimmedStyle.Render(m.config.Workflow))
	s.WriteString("\n")

	s.WriteString(ui.NormalStyle.Render("  Branch:   "))

	branch := m.config.Branch
	if branch == "" {
		branch = "(default)"
	}

	s.WriteString(ui.TableDimmedStyle.Render(branch))
	s.WriteString("\n")

	inputCount := 0

	for _, v := range m.config.Inputs {
		if v != "" {
			inputCount++
		}
	}

	s.WriteString(ui.NormalStyle.Render("  Inputs:   "))
	s.WriteString(ui.TableDimmedStyle.Render(fmt.Sprintf("%d value(s) provided", inputCount)))
	s.WriteString("\n\n")

	s.WriteString(ui.SubtitleStyle.Render("Command:"))
	s.WriteString("\n")

	cmd := m.buildCommand()
	s.WriteString(ui.TableDimmedStyle.Render("  " + cmd))
	s.WriteString("\n\n")

	s.WriteString(ui.HelpStyle.Render("[enter/y] confirm  [esc/n] cancel"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *RunConfirmModal) IsDone() bool {
	return m.done
}

// Result returns the confirmation result.
func (m *RunConfirmModal) Result() any {
	return m.result
}

func (m *RunConfirmModal) buildCommand() string {
	args := runner.BuildArgs(m.config)
	return runner.FormatCommand(args)
}
