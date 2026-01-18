package modal

import (
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// HelpModal displays keyboard shortcuts and help.
type HelpModal struct {
	done bool
	keys helpKeyMap
}

type helpKeyMap struct {
	Close key.Binding
}

func defaultHelpKeyMap() helpKeyMap {
	return helpKeyMap{
		Close: key.NewBinding(key.WithKeys("esc", "?", "q")),
	}
}

// NewHelpModal creates a new help modal.
func NewHelpModal() *HelpModal {
	return &HelpModal{
		keys: defaultHelpKeyMap(),
	}
}

// Update handles input for the help modal.
func (m *HelpModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if key.Matches(msg, m.keys.Close) {
			m.done = true
		}
	}
	return m, nil
}

// View renders the help modal.
func (m *HelpModal) View() string {
	return ui.TitleStyle.Render("Keyboard Shortcuts") + `

` + ui.SubtitleStyle.Render("Navigation") + `
  Tab / Shift+Tab    Switch between panes
  ↑/k, ↓/j           Navigate lists and select input
  Enter              Select / Execute / Edit selected
  Esc                Deselect / Close modal

` + ui.SubtitleStyle.Render("Config Panel") + `
  1-9, 0             Edit input by number (1-10)
  b                  Select branch
  w                  Toggle watch mode
  /                  Start filtering inputs
  c                  Command - copy to clipboard
  r                  Reset all inputs to defaults

` + ui.SubtitleStyle.Render("Input Editing") + `
  Alt+D              Restore default value
  Enter              Confirm (or apply anyway)
  Esc                Cancel / Keep editing

` + ui.SubtitleStyle.Render("Application") + `
  ?                  Show this help
  q, Ctrl+C          Quit

` + ui.HelpStyle.Render("Press ? or Esc to close")
}

// IsDone returns true if the modal is finished.
func (m *HelpModal) IsDone() bool {
	return m.done
}

// Result returns nil for help modal.
func (m *HelpModal) Result() any {
	return nil
}
