// Package modal provides modal dialogs for user interactions in the TUI application.
package modal

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// BranchItem represents a branch in the list.
type BranchItem struct {
	name string
}

func (i BranchItem) Title() string {
	return i.name
}

func (i BranchItem) Description() string {
	return ""
}

func (i BranchItem) FilterValue() string {
	return i.name
}

// BranchModal presents a filterable list of branches.
type BranchModal struct {
	list           list.Model
	done           bool
	result         string
	allBranches    []string
	currentBranch  string
	defaultBranch  string
	wasFiltering   bool
	originalItems  []list.Item
	terminalWidth  int
	terminalHeight int
}

// NewBranchModal creates a new branch selection modal.
func NewBranchModal(title string, branches []string, current string) *BranchModal {
	return NewBranchModalWithDefault(title, branches, current, "")
}

// NewBranchModalWithDefault creates a new branch selection modal with default branch pinning.
func NewBranchModalWithDefault(title string, branches []string, current string, defaultBranch string) *BranchModal {
	pinnedBranches := _pinBranches(branches, current, defaultBranch)

	items := make([]list.Item, len(pinnedBranches))
	selectedIdx := 0

	for i, branch := range pinnedBranches {
		items[i] = BranchItem{name: branch}

		if branch == current {
			selectedIdx = i
		}
	}

	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(1)
	delegate.SetSpacing(0)
	delegate.Styles.SelectedTitle = ui.SelectedStyle.UnsetBackground()
	delegate.Styles.SelectedDesc = ui.SubtitleStyle.UnsetBackground()
	delegate.Styles.NormalTitle = ui.NormalStyle.UnsetBackground()
	delegate.Styles.NormalDesc = ui.SubtitleStyle.UnsetBackground()
	delegate.Styles.DimmedTitle = delegate.Styles.DimmedTitle.UnsetBackground()
	delegate.Styles.DimmedDesc = delegate.Styles.DimmedDesc.UnsetBackground()
	delegate.Styles.FilterMatch = delegate.Styles.FilterMatch.UnsetBackground()

	l := list.New(items, delegate, 0, 0)
	l.Title = title
	l.SetShowStatusBar(false)
	l.SetShowHelp(true)
	l.SetFilteringEnabled(true)
	l = ui.RemoveListBackgrounds(l)

	if selectedIdx < len(items) {
		l.Select(selectedIdx)
	}

	// Initial size - will be set properly by SetSize() call from app
	l.SetSize(60, 20)

	return &BranchModal{
		list:          l,
		allBranches:   branches,
		currentBranch: current,
		defaultBranch: defaultBranch,
		originalItems: items,
	}
}

// SetSize updates the modal dimensions based on terminal size.
func (m *BranchModal) SetSize(width, height int) {
	m.terminalWidth = width
	m.terminalHeight = height

	maxHeight := int(float64(height) * 0.8)

	listHeight := maxHeight
	if listHeight > 30 {
		listHeight = 30
	}

	if listHeight < 10 {
		listHeight = 10
	}

	listWidth := 60
	if width < 70 {
		listWidth = width - 10
	}

	m.list.SetSize(listWidth, listHeight)
}

func _pinBranches(branches []string, current string, defaultBranch string) []string {
	if current == "" && defaultBranch == "" {
		return branches
	}

	result := make([]string, 0, len(branches))
	remaining := make([]string, 0, len(branches))

	for _, branch := range branches {
		if branch != current && branch != defaultBranch {
			remaining = append(remaining, branch)
		}
	}

	if current != "" {
		result = append(result, current)
	}

	if defaultBranch != "" && defaultBranch != current {
		result = append(result, defaultBranch)
	}

	result = append(result, remaining...)

	return result
}

// Update handles input for the branch modal.
func (m *BranchModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if item, ok := m.list.SelectedItem().(BranchItem); ok {
				m.result = item.name
				m.done = true

				return m, func() tea.Msg {
					return BranchResultMsg{Value: m.result}
				}
			}
		case "esc":
			if m.list.FilterState() == list.Filtering {
				m.list.ResetFilter()
				return m, nil
			}

			m.done = true

			return m, nil
		}
	}

	isFiltering := m.list.FilterState() == list.Filtering || m.list.FilterState() == list.FilterApplied

	if !m.wasFiltering && isFiltering {
		m.wasFiltering = true
	} else if m.wasFiltering && !isFiltering {
		m.wasFiltering = false
		m.list.SetItems(m.originalItems)

		if m.currentBranch != "" {
			for i, item := range m.originalItems {
				if branchItem, ok := item.(BranchItem); ok && branchItem.name == m.currentBranch {
					m.list.Select(i)
					break
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)

	return m, cmd
}

// View renders the branch modal.
func (m *BranchModal) View() string {
	return m.list.View()
}

// IsDone returns true if the modal is finished.
func (m *BranchModal) IsDone() bool {
	return m.done
}

// Result returns the selected branch.
func (m *BranchModal) Result() any {
	return m.result
}

// BranchResultMsg is sent when a branch is selected.
type BranchResultMsg struct {
	Value string
}
