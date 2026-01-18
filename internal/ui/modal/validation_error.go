package modal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ValidationErrorModal displays validation errors and allows override or fixing.
type ValidationErrorModal struct {
	errors   map[string][]string
	done     bool
	override bool
	selected int
	keys     validationErrorKeyMap
}

type validationErrorKeyMap struct {
	Close    key.Binding
	Fix      key.Binding
	Continue key.Binding
	Up       key.Binding
	Down     key.Binding
}

func defaultValidationErrorKeyMap() validationErrorKeyMap {
	return validationErrorKeyMap{
		Close:    key.NewBinding(key.WithKeys("esc", "q")),
		Fix:      key.NewBinding(key.WithKeys("f", "enter")),
		Continue: key.NewBinding(key.WithKeys("c")),
		Up:       key.NewBinding(key.WithKeys("up", "k")),
		Down:     key.NewBinding(key.WithKeys("down", "j")),
	}
}

// ValidationErrorResultMsg is returned when the modal closes.
type ValidationErrorResultMsg struct {
	Override bool
}

// NewValidationErrorModal creates a new validation error modal.
func NewValidationErrorModal(errors map[string][]string) *ValidationErrorModal {
	return &ValidationErrorModal{
		errors: errors,
		keys:   defaultValidationErrorKeyMap(),
	}
}

// Update handles input for the validation error modal.
func (m *ValidationErrorModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Close):
			m.done = true
		case key.Matches(msg, m.keys.Fix):
			m.done = true
			m.override = false
		case key.Matches(msg, m.keys.Continue):
			m.done = true
			m.override = true
			return m, func() tea.Msg {
				return ValidationErrorResultMsg{Override: true}
			}
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, m.keys.Down):
			maxIdx := len(m.errors) - 1
			if m.selected < maxIdx {
				m.selected++
			}
		}
	}
	return m, nil
}

// View renders the validation error modal.
func (m *ValidationErrorModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Validation Errors"))
	s.WriteString("\n\n")

	s.WriteString(ui.SubtitleStyle.Render("The following inputs have validation errors:"))
	s.WriteString("\n\n")

	idx := 0
	for inputName, errs := range m.errors {
		prefix := "  "
		if idx == m.selected {
			prefix = "> "
		}

		line := fmt.Sprintf("%s%s:", prefix, inputName)
		if idx == m.selected {
			s.WriteString(ui.SelectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}
		s.WriteString("\n")

		for _, errMsg := range errs {
			errLine := fmt.Sprintf("    - %s", errMsg)
			if idx == m.selected {
				s.WriteString(ui.SubtitleStyle.Render(errLine))
			} else {
				s.WriteString(ui.HelpStyle.Render(errLine))
			}
			s.WriteString("\n")
		}
		idx++
	}

	s.WriteString("\n")
	s.WriteString(ui.HelpStyle.Render("[f/Enter] Fix Inputs  [c] Continue Anyway  [Esc] Cancel"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *ValidationErrorModal) IsDone() bool {
	return m.done
}

// Result returns nil for validation error modal.
func (m *ValidationErrorModal) Result() any {
	if m.override {
		return ValidationErrorResultMsg{Override: true}
	}
	return nil
}
