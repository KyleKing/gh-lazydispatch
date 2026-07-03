package modal

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/rule"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// defaultTextInputWidth is the standard character width for single-line text inputs across modals.
const defaultTextInputWidth = 40

// Workflow input type names, shared across the modal package.
const (
	inputTypeChoice  = "choice"
	inputTypeBoolean = "boolean"
	boolTrueValue    = "true"
)

// InputModal presents a text input field.
type InputModal struct {
	title           string
	description     string
	defaultVal      string
	inputType       string
	options         []string
	validationRules []rule.ValidationRule
	input           textinput.Model
	done            bool
	result          string
	keys            inputKeyMap
	validationErr   string
	hasError        bool
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
		RestoreDefault: key.NewBinding(key.WithKeys("ctrl+r", "alt+d")),
	}
}

// NewInputModal creates a new text input modal.
func NewInputModal(title, description, defaultVal, inputType, current string, options []string, rules []rule.ValidationRule) *InputModal {
	ti := textinput.New()
	ti.SetValue(current)
	ti.Focus()
	ti.CharLimit = 256
	ti.SetWidth(defaultTextInputWidth)

	// Remove backgrounds from textinput styles to prevent visual artifacts in modal
	s := ti.Styles()
	s.Focused.Prompt = s.Focused.Prompt.UnsetBackground()
	s.Focused.Text = s.Focused.Text.UnsetBackground()
	s.Focused.Placeholder = s.Focused.Placeholder.UnsetBackground()
	s.Focused.Suggestion = s.Focused.Suggestion.UnsetBackground()
	s.Blurred.Prompt = s.Blurred.Prompt.UnsetBackground()
	s.Blurred.Text = s.Blurred.Text.UnsetBackground()
	s.Blurred.Placeholder = s.Blurred.Placeholder.UnsetBackground()
	s.Blurred.Suggestion = s.Blurred.Suggestion.UnsetBackground()
	ti.SetStyles(s)

	return &InputModal{
		title:           title,
		description:     description,
		defaultVal:      defaultVal,
		inputType:       inputType,
		options:         options,
		validationRules: rules,
		input:           ti,
		keys:            defaultInputKeyMap(),
	}
}

func (m *InputModal) validate() string {
	value := m.input.Value()

	if m.inputType == inputTypeChoice && len(m.options) > 0 && value != "" {
		validOption := false

		for _, opt := range m.options {
			if opt == value {
				validOption = true
				break
			}
		}

		if !validOption {
			return "\"" + value + "\" is not a valid option"
		}
	}

	if len(m.validationRules) > 0 {
		errors := rule.ValidateValue(value, m.validationRules)
		if len(errors) > 0 {
			return strings.Join(errors, "; ")
		}
	}

	return ""
}

// Update handles input for the input modal.
func (m *InputModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
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

	if m.inputType == inputTypeChoice && len(m.options) > 0 {
		s.WriteString("\n")
		s.WriteString(ui.SubtitleStyle.Render("Options: " + strings.Join(m.options, " / ")))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(m.input.View())
	s.WriteString("\n\n")

	defaultDisplay := ui.FormatEmptyValue(m.defaultVal)
	s.WriteString(ui.SubtitleStyle.Render("Default: " + defaultDisplay))
	s.WriteString("\n")

	if m.validationErr != "" {
		s.WriteString("\n")
		s.WriteString(ui.SelectedStyle.Render("! " + m.validationErr))
		s.WriteString("\n\n")
		s.WriteString(ui.HelpStyle.Render("[enter] apply anyway  [esc] keep editing  [ctrl+r] restore default"))
	} else {
		s.WriteString("\n")
		s.WriteString(ui.HelpStyle.Render("[enter] confirm  [esc] cancel  [ctrl+r] restore default"))
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
