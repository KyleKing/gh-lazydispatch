package modal

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// FilterResultMsg is sent when filter is applied or canceled.
type FilterResultMsg struct {
	Value     string
	Canceled bool
}

type filterKeyMap struct {
	Enter  key.Binding
	Escape key.Binding
}

// FilterModal presents a fuzzy filter input.
type FilterModal struct {
	title     string
	input     textinput.Model
	items     []string
	matches   []string
	done      bool
	canceled bool
	keys      filterKeyMap
}

// NewFilterModal creates a new filter modal.
func NewFilterModal(title string, items []string, currentFilter string) *FilterModal {
	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Prompt = "/ "
	ti.SetValue(currentFilter)
	ti.Focus()
	ti.CharLimit = 64
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

	m := &FilterModal{
		title: title,
		input: ti,
		items: items,
		keys: filterKeyMap{
			Enter:  key.NewBinding(key.WithKeys("enter")),
			Escape: key.NewBinding(key.WithKeys("esc")),
		},
	}
	m.updateMatches()

	return m
}

func (m *FilterModal) updateMatches() {
	query := m.input.Value()
	m.matches = ui.ApplyFuzzyFilter(query, m.items)
}

// Update handles input for the filter modal.
func (m *FilterModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keys.Enter):
			m.done = true

			return m, func() tea.Msg {
				return FilterResultMsg{Value: m.input.Value()}
			}
		case key.Matches(msg, m.keys.Escape):
			m.done = true
			m.canceled = true

			return m, func() tea.Msg {
				return FilterResultMsg{Canceled: true}
			}
		}
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	m.updateMatches()

	return m, cmd
}

// View renders the filter modal.
func (m *FilterModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render(m.title) + "\n\n")
	s.WriteString(m.input.View() + "\n\n")

	matchText := "Matches: " + strconv.Itoa(len(m.matches)) + "/" + strconv.Itoa(len(m.items))
	s.WriteString(ui.SubtitleStyle.Render(matchText) + "\n\n")

	const previewLimit = 5

	maxPreview := previewLimit
	if len(m.matches) < maxPreview {
		maxPreview = len(m.matches)
	}

	for i := range maxPreview {
		s.WriteString(ui.NormalStyle.Render("  "+m.matches[i]) + "\n")
	}

	if len(m.matches) > previewLimit {
		s.WriteString(ui.SubtitleStyle.Render("  ...and " + strconv.Itoa(len(m.matches)-previewLimit) + " more"))
	}

	s.WriteString("\n" + ui.HelpStyle.Render("[enter] apply  [esc] cancel"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *FilterModal) IsDone() bool {
	return m.done
}

// Result returns the filter value.
func (m *FilterModal) Result() any {
	return m.input.Value()
}
