package modal

import (
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// SimpleBranchModal is a branch selector without bubbles/list complexity.
type SimpleBranchModal struct {
	title            string
	allBranches      []string
	pinnedBranches   []string
	filteredBranches []string
	currentBranch    string
	defaultBranch    string
	selected         int
	done             bool
	result           string
	filterInput      textinput.Model
	filtering        bool
	keys             simpleBranchKeyMap
	maxHeight        int
	scrollOffset     int
}

type simpleBranchKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Enter  key.Binding
	Escape key.Binding
	Filter key.Binding
}

func defaultSimpleBranchKeyMap() simpleBranchKeyMap {
	return simpleBranchKeyMap{
		Up:     key.NewBinding(key.WithKeys("up", "k")),
		Down:   key.NewBinding(key.WithKeys("down", "j")),
		Enter:  key.NewBinding(key.WithKeys("enter")),
		Escape: key.NewBinding(key.WithKeys("esc")),
		Filter: key.NewBinding(key.WithKeys("/")),
	}
}

// NewSimpleBranchModal creates a simple branch modal with filtering.
func NewSimpleBranchModal(title string, branches []string, current string, defaultBranch string) *SimpleBranchModal {
	pinnedBranches := _pinBranches(branches, current, defaultBranch)

	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Prompt = "/ "

	// Remove backgrounds from textinput styles to prevent visual artifacts in modal
	ti.PromptStyle = ti.PromptStyle.UnsetBackground()
	ti.TextStyle = ti.TextStyle.UnsetBackground()
	ti.PlaceholderStyle = ti.PlaceholderStyle.UnsetBackground()
	ti.CompletionStyle = ti.CompletionStyle.UnsetBackground()
	ti.Cursor.Style = ti.Cursor.Style.UnsetBackground()

	selected := 0

	for i, branch := range pinnedBranches {
		if branch == current {
			selected = i
			break
		}
	}

	return &SimpleBranchModal{
		title:            title,
		allBranches:      branches,
		pinnedBranches:   pinnedBranches,
		filteredBranches: pinnedBranches,
		currentBranch:    current,
		defaultBranch:    defaultBranch,
		selected:         selected,
		filterInput:      ti,
		keys:             defaultSimpleBranchKeyMap(),
		maxHeight:        20,
	}
}

// SetSize updates the modal dimensions.
func (m *SimpleBranchModal) SetSize(width, height int) {
	maxHeight := int(float64(height) * 0.8)
	if maxHeight > 30 {
		maxHeight = 30
	}

	if maxHeight < 10 {
		maxHeight = 10
	}

	m.maxHeight = maxHeight - 6 // Account for title, filter, help text
}

// Update handles input for the simple branch modal.
func (m *SimpleBranchModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.filtering {
			switch msg.String() {
			case "enter":
				m.filtering = false
				m.filterInput.Blur()

				return m, nil
			case "esc":
				if m.filterInput.Value() == "" {
					m.filtering = false
					m.filterInput.Blur()
				} else {
					m.filterInput.SetValue("")
					m.applyFilter()
				}

				return m, nil
			default:
				var cmd tea.Cmd
				m.filterInput, cmd = m.filterInput.Update(msg)
				m.applyFilter()

				return m, cmd
			}
		}

		switch {
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
				m.adjustScroll()
			}
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.filteredBranches)-1 {
				m.selected++
				m.adjustScroll()
			}
		case key.Matches(msg, m.keys.Enter):
			if m.selected < len(m.filteredBranches) {
				m.result = m.filteredBranches[m.selected]
			}

			m.done = true

			return m, func() tea.Msg {
				return BranchResultMsg{Value: m.result}
			}
		case key.Matches(msg, m.keys.Escape):
			m.done = true
			return m, nil
		default:
			// Auto-start filtering on any printable character
			if !m.filtering && len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
				m.filtering = true
				m.filterInput.Focus()
				m.filterInput.SetValue(msg.String())
				m.applyFilter()
			}
		}
	}

	return m, nil
}

func (m *SimpleBranchModal) applyFilter() {
	query := m.filterInput.Value()
	if query == "" {
		m.filteredBranches = m.pinnedBranches
		m.selected = 0
		m.scrollOffset = 0

		return
	}

	// Use unpinned branches for filtering
	m.filteredBranches = ui.ApplyFuzzyFilter(query, m.allBranches)
	if len(m.filteredBranches) == 0 {
		m.filteredBranches = []string{}
	}

	if m.selected >= len(m.filteredBranches) {
		m.selected = 0
	}

	m.scrollOffset = 0
}

func (m *SimpleBranchModal) adjustScroll() {
	visibleLines := m.maxHeight

	if m.selected < m.scrollOffset {
		m.scrollOffset = m.selected
	}

	if m.selected >= m.scrollOffset+visibleLines {
		m.scrollOffset = m.selected - visibleLines + 1
	}
}

// View renders the simple branch modal.
func (m *SimpleBranchModal) View() string {
	var s strings.Builder

	// Title
	s.WriteString(ui.TitleStyle.Render(m.title))
	s.WriteString("\n\n")

	// Filter input
	if m.filtering {
		s.WriteString(m.filterInput.View())
		s.WriteString("\n\n")
	} else {
		s.WriteString(ui.SubtitleStyle.Render("Press any key to filter, / to focus filter"))
		s.WriteString("\n\n")
	}

	// Branch list
	visibleLines := m.maxHeight

	endIdx := m.scrollOffset + visibleLines
	if endIdx > len(m.filteredBranches) {
		endIdx = len(m.filteredBranches)
	}

	if len(m.filteredBranches) == 0 {
		s.WriteString(ui.SubtitleStyle.Render("No branches found"))
	} else {
		for i := m.scrollOffset; i < endIdx; i++ {
			branch := m.filteredBranches[i]
			cursor := "  "
			style := ui.NormalStyle

			if i == m.selected {
				cursor = "> "
				style = ui.SelectedStyle
			}

			// Add indicators for current/default
			indicator := ""
			switch branch {
			case m.currentBranch:
				indicator = " *"
			case m.defaultBranch:
				indicator = " ·"
			}

			s.WriteString(style.Render(cursor + branch + indicator))

			if i < endIdx-1 {
				s.WriteString("\n")
			}
		}

		// Show scroll indicator
		if len(m.filteredBranches) > visibleLines {
			s.WriteString("\n")
			s.WriteString(ui.SubtitleStyle.Render("  "))

			scrollInfo := ""
			if m.scrollOffset > 0 {
				scrollInfo += "↑ "
			}

			scrollInfo += "  "
			if endIdx < len(m.filteredBranches) {
				scrollInfo += "↓"
			}

			s.WriteString(ui.SubtitleStyle.Render(scrollInfo))
		}
	}

	// Help
	s.WriteString("\n\n")

	helpText := "[↑↓] navigate  [enter] select  [esc] cancel"
	if m.filtering {
		helpText = "[enter] done filtering  [esc] clear/cancel"
	}

	s.WriteString(ui.HelpStyle.Render(helpText))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *SimpleBranchModal) IsDone() bool {
	return m.done
}

// Result returns the selected branch.
func (m *SimpleBranchModal) Result() any {
	return m.result
}
