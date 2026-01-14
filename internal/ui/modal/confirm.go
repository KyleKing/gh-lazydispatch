package modal

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-wfd/internal/ui"
)

// ConfirmModal presents a yes/no choice.
type ConfirmModal struct {
	title       string
	description string
	selected    bool
	done        bool
	result      bool
	keys        confirmKeyMap
}

type confirmKeyMap struct {
	Left   key.Binding
	Right  key.Binding
	Yes    key.Binding
	No     key.Binding
	Enter  key.Binding
	Escape key.Binding
}

func defaultConfirmKeyMap() confirmKeyMap {
	return confirmKeyMap{
		Left:   key.NewBinding(key.WithKeys("left", "h")),
		Right:  key.NewBinding(key.WithKeys("right", "l")),
		Yes:    key.NewBinding(key.WithKeys("y")),
		No:     key.NewBinding(key.WithKeys("n")),
		Enter:  key.NewBinding(key.WithKeys("enter")),
		Escape: key.NewBinding(key.WithKeys("esc")),
	}
}

// NewConfirmModal creates a new confirmation modal.
func NewConfirmModal(title, description string, current bool) *ConfirmModal {
	return &ConfirmModal{
		title:       title,
		description: description,
		selected:    current,
		keys:        defaultConfirmKeyMap(),
	}
}

// Update handles input for the confirm modal.
func (m *ConfirmModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Left):
			m.selected = true
		case key.Matches(msg, m.keys.Right):
			m.selected = false
		case key.Matches(msg, m.keys.Yes):
			m.result = true
			m.done = true
			return m, func() tea.Msg {
				return ConfirmResultMsg{Value: true}
			}
		case key.Matches(msg, m.keys.No):
			m.result = false
			m.done = true
			return m, func() tea.Msg {
				return ConfirmResultMsg{Value: false}
			}
		case key.Matches(msg, m.keys.Enter):
			m.result = m.selected
			m.done = true
			return m, func() tea.Msg {
				return ConfirmResultMsg{Value: m.result}
			}
		case key.Matches(msg, m.keys.Escape):
			m.done = true
			return m, nil
		}
	}
	return m, nil
}

// View renders the confirm modal.
func (m *ConfirmModal) View() string {
	s := ui.TitleStyle.Render(m.title) + "\n"
	if m.description != "" {
		s += ui.SubtitleStyle.Render(m.description) + "\n"
	}
	s += "\n"

	yesStyle := ui.NormalStyle
	noStyle := ui.NormalStyle
	if m.selected {
		yesStyle = ui.SelectedStyle
	} else {
		noStyle = ui.SelectedStyle
	}

	s += "  " + yesStyle.Render("[ Yes ]") + "  " + noStyle.Render("[ No ]")
	s += "\n\n" + ui.HelpStyle.Render("[←→] select  [y/n] quick  [enter] confirm  [esc] cancel")
	return s
}

// IsDone returns true if the modal is finished.
func (m *ConfirmModal) IsDone() bool {
	return m.done
}

// Result returns the confirmed value.
func (m *ConfirmModal) Result() any {
	return m.result
}

// ConfirmResultMsg is sent when confirmation is made.
type ConfirmResultMsg struct {
	Value bool
}
