package modal

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ResetResultMsg is sent when reset is confirmed or cancelled.
type ResetResultMsg struct {
	Confirmed bool
}

// ResetDiff represents a single value change.
type ResetDiff struct {
	Name    string
	Current string
	Default string
}

type resetKeyMap struct {
	Confirm key.Binding
	Cancel  key.Binding
}

// ResetModal shows values that will be reset and confirms.
type ResetModal struct {
	diffs  []ResetDiff
	done   bool
	result bool
	keys   resetKeyMap
}

// NewResetModal creates a reset confirmation modal.
func NewResetModal(diffs []ResetDiff) *ResetModal {
	return &ResetModal{
		diffs: diffs,
		keys: resetKeyMap{
			Confirm: key.NewBinding(key.WithKeys("enter", "y")),
			Cancel:  key.NewBinding(key.WithKeys("esc", "n")),
		},
	}
}

// Update handles input for the reset modal.
func (m *ResetModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Confirm):
			if len(m.diffs) == 0 {
				m.done = true
				return m, nil
			}
			m.done = true
			m.result = true
			return m, func() tea.Msg {
				return ResetResultMsg{Confirmed: true}
			}
		case key.Matches(msg, m.keys.Cancel):
			m.done = true
			return m, func() tea.Msg {
				return ResetResultMsg{Confirmed: false}
			}
		}
	}
	return m, nil
}

// View renders the reset modal.
func (m *ResetModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Reset All Inputs?"))
	s.WriteString("\n\n")

	if len(m.diffs) == 0 {
		s.WriteString(ui.SubtitleStyle.Render("No modified values to reset."))
		s.WriteString("\n\n")
		s.WriteString(ui.HelpStyle.Render("[enter/esc] close"))
		return s.String()
	}

	s.WriteString(ui.SubtitleStyle.Render("The following values will be reset:"))
	s.WriteString("\n\n")

	for _, d := range m.diffs {
		currentDisplay := ui.FormatEmptyValue(d.Current)
		defaultDisplay := ui.FormatEmptyValue(d.Default)

		s.WriteString(ui.NormalStyle.Render("  " + d.Name + ":"))
		s.WriteString("\n")
		s.WriteString(ui.TableDimmedStyle.Render("    " + currentDisplay + " -> " + defaultDisplay))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(ui.HelpStyle.Render("[enter/y] confirm  [esc/n] cancel"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *ResetModal) IsDone() bool {
	return m.done
}

// Result returns the confirmation result.
func (m *ResetModal) Result() any {
	return m.result
}
