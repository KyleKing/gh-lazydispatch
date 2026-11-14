package modal

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// ActionItem is one verb the menu offers, identified by the key that runs it.
type ActionItem struct {
	Key  string
	Name string
}

// ActionResultMsg names the verb chosen, or nothing when the menu was
// dismissed.
type ActionResultMsg struct {
	Key string
}

// ActionMenuModal lists the verbs that apply to whatever has focus. It holds
// only their names and keys: the caller owns what they do, which is what lets
// the same menu serve every pane. Keys are spelled `[b]` here as they are in
// the footer, so the same key reads the same in both places.
type ActionMenuModal struct {
	keys     actionMenuKeyMap
	title    string
	target   string
	result   ActionResultMsg
	items    []ActionItem
	width    int
	selected int
	done     bool
}

type actionMenuKeyMap struct {
	Close  key.Binding
	Up     key.Binding
	Down   key.Binding
	Choose key.Binding
}

// NewActionMenuModal builds a menu of items acting on target.
func NewActionMenuModal(title, target string, items []ActionItem) *ActionMenuModal {
	return &ActionMenuModal{title: title, target: target, items: items, keys: actionMenuKeyMap{
		Close:  key.NewBinding(key.WithKeys("esc", "q", "a")),
		Up:     key.NewBinding(key.WithKeys("up", "k")),
		Down:   key.NewBinding(key.WithKeys("down", "j")),
		Choose: key.NewBinding(key.WithKeys("enter")),
	}}
}

// SetSize records the room the menu has, so a long target line truncates
// rather than widening the modal past the terminal.
func (m *ActionMenuModal) SetSize(width, _ int) {
	m.width = width
}

// Update handles input for the action menu. A verb's own key runs it directly,
// which is what makes the menu a way to learn the keys rather than a detour
// around them.
func (m *ActionMenuModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Close):
		m.done = true

		return m, func() tea.Msg { return ActionResultMsg{} }

	case key.Matches(keyMsg, m.keys.Up):
		if m.selected > 0 {
			m.selected--
		}

		return m, nil

	case key.Matches(keyMsg, m.keys.Down):
		if m.selected < len(m.items)-1 {
			m.selected++
		}

		return m, nil

	case key.Matches(keyMsg, m.keys.Choose):
		if m.selected < len(m.items) {
			return m.choose(m.items[m.selected].Key)
		}

		return m, nil
	}

	for _, item := range m.items {
		if item.Key == keyMsg.String() {
			return m.choose(item.Key)
		}
	}

	return m, nil
}

// Records is false for a verb's own key, which the caller logs as the verb.
func (m *ActionMenuModal) Records(k string) bool { return bound(m.keys, k) }

func (m *ActionMenuModal) choose(verb string) (Context, tea.Cmd) {
	m.done = true
	m.result = ActionResultMsg{Key: verb}

	return m, func() tea.Msg { return m.result }
}

// View renders the menu.
func (m *ActionMenuModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Actions: " + m.title))
	s.WriteString("\n")

	if m.target != "" {
		s.WriteString(ui.TableDimmedStyle.Render(ansi.Truncate(m.target, max(m.width, 1), "…")))
		s.WriteString("\n")
	}

	s.WriteString("\n")

	if len(m.items) == 0 {
		s.WriteString(ui.SubtitleStyle.Render("Nothing to do here."))
		s.WriteString("\n")
	}

	for i, item := range m.items {
		cursor := "  "
		style := ui.NormalStyle

		if i == m.selected {
			cursor = "> "
			style = ui.SelectedStyle
		}

		s.WriteString(style.Render(cursor + "[" + item.Key + "] " + item.Name))
		s.WriteString("\n")
	}

	s.WriteString("\n")
	s.WriteString(renderHints(m.width, "[a key] run it", "[↑↓] move", "[enter] run", "[esc] close"))

	return s.String()
}

// IsDone returns true if the menu is finished.
func (m *ActionMenuModal) IsDone() bool { return m.done }

// Result returns the chosen verb.
func (m *ActionMenuModal) Result() any { return m.result }
