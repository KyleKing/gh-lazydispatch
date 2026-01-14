package modal

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-wfr/internal/ui"
)

// InputModal presents a text input field.
type InputModal struct {
	title       string
	description string
	defaultVal  string
	inputType   string
	input       textinput.Model
	done        bool
	result      string
	keys        inputKeyMap
}

type inputKeyMap struct {
	Enter  key.Binding
	Escape key.Binding
}

func defaultInputKeyMap() inputKeyMap {
	return inputKeyMap{
		Enter:  key.NewBinding(key.WithKeys("enter")),
		Escape: key.NewBinding(key.WithKeys("esc")),
	}
}

// NewInputModal creates a new text input modal.
func NewInputModal(title, description, defaultVal, inputType, current string) *InputModal {
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
		input:       ti,
		keys:        defaultInputKeyMap(),
	}
}

// Update handles input for the input modal.
func (m *InputModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Enter):
			m.result = m.input.Value()
			m.done = true
			return m, func() tea.Msg {
				return InputResultMsg{Value: m.result}
			}
		case key.Matches(msg, m.keys.Escape):
			m.done = true
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

// View renders the input modal.
func (m *InputModal) View() string {
	s := ui.TitleStyle.Render(m.title) + "\n"
	if m.description != "" {
		s += ui.SubtitleStyle.Render(m.description) + "\n"
	}
	if m.inputType != "" {
		s += ui.SubtitleStyle.Render("Type: "+m.inputType) + "\n"
	}
	if m.defaultVal != "" {
		s += ui.SubtitleStyle.Render("Default: "+m.defaultVal) + "\n"
	}
	s += "\n"
	s += m.input.View()
	s += "\n\n" + ui.HelpStyle.Render("[enter] confirm  [esc] cancel")
	return s
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
