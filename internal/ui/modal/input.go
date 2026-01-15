package modal

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-wfd/internal/ui"
)

// InputModal presents a text input field.
type InputModal struct {
	title         string
	description   string
	defaultVal    string
	inputType     string
	options       []string
	input         textinput.Model
	done          bool
	result        string
	keys          inputKeyMap
	validationErr string
	hasError      bool
}

type inputKeyMap struct {
	Enter          key.Binding
	Escape         key.Binding
	RestoreDefault key.Binding
}

func defaultInputKeyMap() inputKeyMap {
	return inputKeyMap{
		Enter:          key.NewBinding(key.WithKeys("enter")),
		Escape:         key.NewBinding(key.WithKeys("esc")),
		RestoreDefault: key.NewBinding(key.WithKeys("alt+d")),
	}
}

// NewInputModal creates a new text input modal.
func NewInputModal(title, description, defaultVal, inputType, current string, options []string) *InputModal {
	ti := textinput.New()
	ti.SetValue(current)
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40

	return &InputModal{
		title:       title,
		description: description,
		defaultVal:  defaultVal,
		inputType:   inputType,
		options:     options,
		input:       ti,
		keys:        defaultInputKeyMap(),
	}
}

func (m *InputModal) validate() string {
	value := m.input.Value()

	if m.inputType == "choice" && len(m.options) > 0 && value != "" {
		for _, opt := range m.options {
			if opt == value {
				return ""
			}
		}
		return "\"" + value + "\" is not a valid option"
	}

	return ""
}

// Update handles input for the input modal.
func (m *InputModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.RestoreDefault):
			m.input.SetValue(m.defaultVal)
			m.validationErr = ""
			m.hasError = false
			return m, nil
		case key.Matches(msg, m.keys.Enter):
			if err := m.validate(); err != "" && !m.hasError {
				m.validationErr = err
				m.hasError = true
				return m, nil
			}
			m.result = m.input.Value()
			m.done = true
			return m, func() tea.Msg {
				return InputResultMsg{Value: m.result}
			}
		case key.Matches(msg, m.keys.Escape):
			if m.hasError {
				m.validationErr = ""
				m.hasError = false
				return m, nil
			}
			m.done = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	prevValue := m.input.Value()
	m.input, cmd = m.input.Update(msg)
	if m.input.Value() != prevValue {
		m.validationErr = ""
		m.hasError = false
	}
	return m, cmd
}

// View renders the input modal.
func (m *InputModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render(m.title))
	s.WriteString("\n")

	if m.description != "" {
		s.WriteString(ui.SubtitleStyle.Render(m.description))
		s.WriteString("\n")
	}

	if m.inputType == "choice" && len(m.options) > 0 {
		s.WriteString("\n")
		s.WriteString(ui.SubtitleStyle.Render("Options: " + strings.Join(m.options, " / ")))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(m.input.View())
	s.WriteString("\n\n")

	defaultDisplay := m.defaultVal
	if defaultDisplay == "" {
		defaultDisplay = `("")`
	}
	s.WriteString(ui.SubtitleStyle.Render("Default: " + defaultDisplay))
	s.WriteString("\n")

	if m.validationErr != "" {
		s.WriteString("\n")
		s.WriteString(ui.SelectedStyle.Render("! " + m.validationErr))
		s.WriteString("\n\n")
		s.WriteString(ui.HelpStyle.Render("[enter] apply anyway  [esc] keep editing  [alt+d] restore default"))
	} else {
		s.WriteString("\n")
		s.WriteString(ui.HelpStyle.Render("[enter] confirm  [esc] cancel  [alt+d] restore default"))
	}

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *InputModal) IsDone() bool {
	return m.done
}

// Result returns the entered value.
func (m *InputModal) Result() any {
	return m.result
}

// InputResultMsg is sent when input is confirmed.
type InputResultMsg struct {
	Value string
}
