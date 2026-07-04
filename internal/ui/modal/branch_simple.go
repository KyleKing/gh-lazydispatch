package modal

import (
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// SimpleBranchModal is a branch selector without bubbles/list complexity.
type SimpleBranchModal struct {
	result           string
	currentBranch    string
	defaultBranch    string
	title            string
	keys             simpleBranchKeyMap
	allBranches      []string
	pinnedBranches   []string
	filteredBranches []string
	filterInput      textinput.Model
	selected         int
	maxHeight        int
	scrollOffset     int
	done             bool
	filtering        bool
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
func NewSimpleBranchModal(title string, branches []string, current, defaultBranch string) *SimpleBranchModal {
	pinnedBranches := _pinBranches(branches, current, defaultBranch)

	ti := textinput.New()
	ti.Placeholder = "Type to filter..."
	ti.Prompt = "/ "

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
		maxHeight:        branchModalInitialHeight,
	}
}

// simpleBranchModalChrome is the vertical space reserved for the title, filter input, and help text.
const simpleBranchModalChrome = 6

// SetSize updates the modal dimensions.
func (m *SimpleBranchModal) SetSize(_, height int) {
	maxHeight := int(float64(height) * branchModalHeightRatio)
	if maxHeight > branchModalMaxHeight {
		maxHeight = branchModalMaxHeight
	}

	if maxHeight < branchModalMinHeight {
		maxHeight = branchModalMinHeight
	}

	m.maxHeight = maxHeight - simpleBranchModalChrome
}

// Update handles input for the simple branch modal.
func (m *SimpleBranchModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	if m.filtering {
		return m.updateFiltering(keyMsg)
	}

	return m.updateNavigating(keyMsg)
}

// updateFiltering handles key input while the branch filter is active.
func (m *SimpleBranchModal) updateFiltering(msg tea.KeyPressMsg) (Context, tea.Cmd) {
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

// updateNavigating handles key input while browsing the branch list.
func (m *SimpleBranchModal) updateNavigating(msg tea.KeyPressMsg) (Context, tea.Cmd) {
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
		if len(msg.String()) == 1 && msg.String() >= " " && msg.String() <= "~" {
			m.filtering = true
			m.filterInput.Focus()
			m.filterInput.SetValue(msg.String())
			m.applyFilter()
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

	m.renderBranchList(&s, endIdx, visibleLines)

	// Help
	s.WriteString("\n\n")

	helpText := "[↑↓] navigate  [enter] select  [esc] cancel"
	if m.filtering {
		helpText = "[enter] done filtering  [esc] clear/cancel"
	}

	s.WriteString(ui.HelpStyle.Render(helpText))

	return s.String()
}

// renderBranchList writes the visible slice of filtered branches (with cursor,
// selection style, and current/default indicators) plus a scroll indicator.
func (m *SimpleBranchModal) renderBranchList(s *strings.Builder, endIdx, visibleLines int) {
	if len(m.filteredBranches) == 0 {
		s.WriteString(ui.SubtitleStyle.Render("No branches found"))
		return
	}

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

	if len(m.filteredBranches) <= visibleLines {
		return
	}

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

// IsDone returns true if the modal is finished.
func (m *SimpleBranchModal) IsDone() bool {
	return m.done
}

// Result returns the selected branch.
func (m *SimpleBranchModal) Result() any {
	return m.result
}
