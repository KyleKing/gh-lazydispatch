package modal

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ErrorModal displays an error message with a dismiss button.
type ErrorModal struct {
	title   string
	message string
	done    bool
	keys    errorKeyMap
}

type errorKeyMap struct {
	Close key.Binding
}

func defaultErrorKeyMap() errorKeyMap {
	return errorKeyMap{
		Close: key.NewBinding(key.WithKeys("esc", "q", "enter")),
	}
}

// NewErrorModal creates a new error modal with a title and message.
func NewErrorModal(title, message string) *ErrorModal {
	return &ErrorModal{
		title:   title,
		message: message,
		keys:    defaultErrorKeyMap(),
	}
}

// Update handles input for the error modal.
func (m *ErrorModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if key.Matches(keyMsg, m.keys.Close) {
			m.done = true
		}
	}
	return m, nil
}

// View renders the error modal.
func (m *ErrorModal) View() string {
	var s strings.Builder

	s.WriteString(ui.ErrorTitleStyle.Render(m.title))
	s.WriteString("\n\n")

	for _, line := range strings.Split(m.message, "\n") {
		s.WriteString(ui.ErrorStyle.Render(line))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(ui.HelpStyle.Render("[Enter/Esc] Dismiss"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *ErrorModal) IsDone() bool {
	return m.done
}

// Result returns nil for error modal.
func (m *ErrorModal) Result() any {
	return nil
}
