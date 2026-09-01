package modal

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// SelectModal presents a list of options to choose from.
type SelectModal struct {
	title       string
	description string
	result      string
	keys        selectKeyMap
	options     []string
	selected    int
	defaultIdx  int
	done        bool
}

type selectKeyMap struct {
	Down           key.Binding
	Enter          key.Binding
	Escape         key.Binding
	RestoreDefault key.Binding
	Up             key.Binding
}

func defaultSelectKeyMap() selectKeyMap {
	return selectKeyMap{
		Down:           key.NewBinding(key.WithKeys("down", "j")),
		Enter:          key.NewBinding(key.WithKeys("enter")),
		Escape:         key.NewBinding(key.WithKeys("esc")),
		RestoreDefault: key.NewBinding(key.WithKeys("ctrl+r", "alt+d")),
		Up:             key.NewBinding(key.WithKeys("up", "k")),
	}
}

// NewSelectModal creates a new selection modal.
func NewSelectModal(title, description string, options []string, current, defaultVal string) *SelectModal {
	selected := 0
	defaultIdx := 0

	for i, opt := range options {
		if opt == current {
			selected = i
		}

		if opt == defaultVal {
			defaultIdx = i
		}
	}

	return &SelectModal{
		title:       title,
		description: description,
		options:     options,
		selected:    selected,
		defaultIdx:  defaultIdx,
		keys:        defaultSelectKeyMap(),
	}
}

// Update handles input for the select modal.
func (m *SelectModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	if msg, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(msg, m.keys.RestoreDefault):
			m.selected = m.defaultIdx
			return m, nil
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.options)-1 {
				m.selected++
			}
		case key.Matches(msg, m.keys.Enter):
			if m.selected < len(m.options) {
				m.result = m.options[m.selected]
			}

			m.done = true

			return m, func() tea.Msg {
				return SelectResultMsg{Value: m.result}
			}
		case key.Matches(msg, m.keys.Escape):
			m.done = true
			return m, nil
		}
	}

	return m, nil
}

// View renders the select modal.
func (m *SelectModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render(m.title) + "\n")

	if m.description != "" {
		s.WriteString(ui.SubtitleStyle.Render(m.description) + "\n")
	}

	s.WriteString("\n")

	for i, opt := range m.options {
		cursor := "  "
		style := ui.NormalStyle

		if i == m.selected {
			cursor = "> "
			style = ui.SelectedStyle
		}

		s.WriteString(style.Render(cursor + opt))

		if i == m.defaultIdx {
			s.WriteString(ui.HelpStyle.Render("  (default)"))
		}

		if i < len(m.options)-1 {
			s.WriteString("\n")
		}
	}

	s.WriteString("\n\n" + ui.HelpStyle.Render("[↑↓] navigate  [enter] select  [ctrl+r] default  [esc] cancel"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *SelectModal) IsDone() bool {
	return m.done
}

// Result returns the selected value.
func (m *SelectModal) Result() any {
	return m.result
}

// SelectResultMsg is sent when a selection is made.
type SelectResultMsg struct {
	Value string
}
