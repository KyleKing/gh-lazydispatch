package modal

import (
	"strconv"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-wfd/internal/ui"
	"github.com/sahilm/fuzzy"
)

// FilterResultMsg is sent when filter is applied or cancelled.
type FilterResultMsg struct {
	Value     string
	Cancelled bool
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
	cancelled bool
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
	ti.Width = 40

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
	if query == "" {
		m.matches = m.items
		return
	}

	results := fuzzy.Find(query, m.items)
	m.matches = make([]string, len(results))
	for i, r := range results {
		m.matches[i] = r.Str
	}
}

// Update handles input for the filter modal.
func (m *FilterModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Enter):
			m.done = true
			return m, func() tea.Msg {
				return FilterResultMsg{Value: m.input.Value()}
			}
		case key.Matches(msg, m.keys.Escape):
			m.done = true
			m.cancelled = true
			return m, func() tea.Msg {
				return FilterResultMsg{Cancelled: true}
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
	s := ui.TitleStyle.Render(m.title) + "\n\n"
	s += m.input.View() + "\n\n"

	matchText := "Matches: " + strconv.Itoa(len(m.matches)) + "/" + strconv.Itoa(len(m.items))
	s += ui.SubtitleStyle.Render(matchText) + "\n\n"

	maxPreview := 5
	if len(m.matches) < maxPreview {
		maxPreview = len(m.matches)
	}
	for i := 0; i < maxPreview; i++ {
		s += ui.NormalStyle.Render("  "+m.matches[i]) + "\n"
	}
	if len(m.matches) > 5 {
		s += ui.SubtitleStyle.Render("  ...and " + strconv.Itoa(len(m.matches)-5) + " more")
	}

	s += "\n" + ui.HelpStyle.Render("[enter] apply  [esc] cancel")
	return s
}

// IsDone returns true if the modal is finished.
func (m *FilterModal) IsDone() bool {
	return m.done
}

// Result returns the filter value.
func (m *FilterModal) Result() any {
	return m.input.Value()
}
