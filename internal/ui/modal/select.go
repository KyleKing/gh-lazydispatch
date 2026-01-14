package modal

import (
	"fmt"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-wfr/internal/ui"
)

// SelectModal presents a list of options to choose from.
type SelectModal struct {
	title    string
	options  []string
	selected int
	done     bool
	result   string
	keys     selectKeyMap
}

type selectKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
}

func defaultSelectKeyMap() selectKeyMap {
	return selectKeyMap{
		Up:     key.NewBinding(key.WithKeys("up", "k")),
		Down:   key.NewBinding(key.WithKeys("down", "j")),
		Enter:  key.NewBinding(key.WithKeys("enter")),
		Escape: key.NewBinding(key.WithKeys("esc")),
	}
}

// NewSelectModal creates a new selection modal.
func NewSelectModal(title string, options []string, current string) *SelectModal {
	selected := 0
	for i, opt := range options {
		if opt == current {
			selected = i
			break
		}
	}

	return &SelectModal{
		title:    title,
		options:  options,
		selected: selected,
		keys:     defaultSelectKeyMap(),
	}
}

// Update handles input for the select modal.
func (m *SelectModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
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
	s := ui.TitleStyle.Render(m.title) + "\n\n"

	for i, opt := range m.options {
		cursor := "  "
		style := ui.NormalStyle
		if i == m.selected {
			cursor = "> "
			style = ui.SelectedStyle
		}
		s += style.Render(fmt.Sprintf("%s%s", cursor, opt))
		if i < len(m.options)-1 {
			s += "\n"
		}
	}

	s += "\n\n" + ui.HelpStyle.Render("[↑↓] navigate  [enter] select  [esc] cancel")
	return s
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
