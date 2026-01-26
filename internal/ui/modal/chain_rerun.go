package modal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/frecency"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ChainRerunResultMsg is sent when chain re-run options are selected.
type ChainRerunResultMsg struct {
	Action         string // "full", "resume", "cancel"
	ResumeFromStep int
	HistoryEntry   *frecency.HistoryEntry
}

type chainRerunKeyMap struct {
	Up      key.Binding
	Down    key.Binding
	Confirm key.Binding
	Cancel  key.Binding
}

// ChainRerunModal presents options for re-running a chain from history.
type ChainRerunModal struct {
	entry         *frecency.HistoryEntry
	options       []rerunOption
	selectedIndex int
	done          bool
	result        ChainRerunResultMsg
	keys          chainRerunKeyMap
}

type rerunOption struct {
	label  string
	action string
	step   int
}

// NewChainRerunModal creates a chain re-run modal.
func NewChainRerunModal(entry *frecency.HistoryEntry) *ChainRerunModal {
	options := []rerunOption{
		{label: "Full re-run (all steps)", action: "full", step: 0},
	}

	for i, stepResult := range entry.StepResults {
		if stepResult.Status == "failed" || stepResult.Conclusion == "failure" {
			options = append(options, rerunOption{
				label:  fmt.Sprintf("Resume from step %d (%s)", i+1, stepResult.Workflow),
				action: "resume",
				step:   i,
			})

			break
		}
	}

	return &ChainRerunModal{
		entry:   entry,
		options: options,
		keys: chainRerunKeyMap{
			Up:      key.NewBinding(key.WithKeys("up", "k")),
			Down:    key.NewBinding(key.WithKeys("down", "j")),
			Confirm: key.NewBinding(key.WithKeys("enter")),
			Cancel:  key.NewBinding(key.WithKeys("esc", "q")),
		},
	}
}

// Update handles input for the chain re-run modal.
func (m *ChainRerunModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selectedIndex > 0 {
				m.selectedIndex--
			}

			return m, nil
		case key.Matches(msg, m.keys.Down):
			if m.selectedIndex < len(m.options)-1 {
				m.selectedIndex++
			}

			return m, nil
		case key.Matches(msg, m.keys.Confirm):
			opt := m.options[m.selectedIndex]
			m.done = true
			m.result = ChainRerunResultMsg{
				Action:         opt.action,
				ResumeFromStep: opt.step,
				HistoryEntry:   m.entry,
			}

			return m, func() tea.Msg { return m.result }
		case key.Matches(msg, m.keys.Cancel):
			m.done = true
			m.result = ChainRerunResultMsg{Action: "cancel"}

			return m, func() tea.Msg { return m.result }
		}
	}

	return m, nil
}

// View renders the chain re-run modal.
func (m *ChainRerunModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Re-run Chain: " + m.entry.ChainName))
	s.WriteString("\n\n")

	s.WriteString(ui.SubtitleStyle.Render("Previous Run:"))
	s.WriteString("\n")
	s.WriteString(ui.NormalStyle.Render("  Branch: " + m.entry.Branch))
	s.WriteString("\n")
	s.WriteString(ui.NormalStyle.Render(fmt.Sprintf("  Steps: %d", len(m.entry.StepResults))))
	s.WriteString("\n\n")

	if len(m.entry.StepResults) > 0 {
		s.WriteString(ui.SubtitleStyle.Render("Step Results:"))
		s.WriteString("\n")

		for i, step := range m.entry.StepResults {
			status := step.Status
			if step.Conclusion != "" {
				status = step.Conclusion
			}

			icon := "+"
			switch status {
			case "failed", "failure":
				icon = "x"
			case "skipped":
				icon = "-"
			}

			s.WriteString(ui.NormalStyle.Render(fmt.Sprintf("  %s %d. %s", icon, i+1, step.Workflow)))
			s.WriteString("\n")
		}

		s.WriteString("\n")
	}

	s.WriteString(ui.SubtitleStyle.Render("Options:"))
	s.WriteString("\n")

	for i, opt := range m.options {
		indicator := "  "
		style := ui.TableRowStyle

		if i == m.selectedIndex {
			indicator = "> "
			style = ui.TableSelectedStyle
		}

		s.WriteString(style.Render(indicator + opt.label))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(ui.HelpStyle.Render("[enter] select  [esc] cancel"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *ChainRerunModal) IsDone() bool {
	return m.done
}

// Result returns the re-run selection result.
func (m *ChainRerunModal) Result() any {
	return m.result
}
