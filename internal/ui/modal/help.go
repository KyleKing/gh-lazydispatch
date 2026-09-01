package modal

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// HelpModal displays keyboard shortcuts and help.
type HelpModal struct {
	keys   helpKeyMap
	done   bool
	offset int
	height int
}

type helpKeyMap struct {
	Close key.Binding
	Down  key.Binding
	Up    key.Binding
}

func defaultHelpKeyMap() helpKeyMap {
	return helpKeyMap{
		Close: key.NewBinding(key.WithKeys("esc", "?", "q")),
		Down:  key.NewBinding(key.WithKeys("down", "j")),
		Up:    key.NewBinding(key.WithKeys("up", "k")),
	}
}

// NewHelpModal creates a new help modal.
func NewHelpModal() *HelpModal {
	return &HelpModal{
		keys: defaultHelpKeyMap(),
	}
}

// SetSize records how many content lines the modal has room for.
func (m *HelpModal) SetSize(_, height int) {
	m.height = height
	m.clampOffset()
}

// Update handles input for the help modal.
func (m *HelpModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Close):
		m.done = true
	case key.Matches(keyMsg, m.keys.Down):
		m.offset++
		m.clampOffset()
	case key.Matches(keyMsg, m.keys.Up):
		m.offset--
		m.clampOffset()
	}

	return m, nil
}

// helpFooterHeight is the blank line plus the footer hint that sit under the
// scrolling body.
const helpFooterHeight = 2

func (m *HelpModal) clampOffset() {
	maxOffset := len(helpLines()) - m.bodyHeight()
	if m.offset > maxOffset {
		m.offset = maxOffset
	}

	if m.offset < 0 {
		m.offset = 0
	}
}

// bodyHeight is how many shortcut lines fit, or all of them when the modal has
// not been sized yet.
func (m *HelpModal) bodyHeight() int {
	if m.height <= helpFooterHeight {
		return len(helpLines())
	}

	return m.height - helpFooterHeight
}

// View renders the help modal.
func (m *HelpModal) View() string {
	lines := helpLines()
	body := m.bodyHeight()

	footer := "Press ? or Esc to close"

	if body < len(lines) {
		end := m.offset + body
		if end > len(lines) {
			end = len(lines)
		}

		lines = lines[m.offset:end]
		footer = "j/k scroll  ·  ? or Esc to close"
	}

	return strings.Join(lines, "\n") + "\n\n" + ui.HelpStyle.Render(footer)
}

func helpLines() []string {
	sections := []struct {
		title string
		rows  []string
	}{
		{"Navigation", []string{
			"Tab / Shift+Tab    Switch between panes",
			"↑/k, ↓/j           Navigate lists and select input",
			"h/←, l/→           Previous / next tab (right pane)",
			"Enter              Select / Execute / Edit selected",
			"Esc                Deselect / Close modal",
		}},
		{"Workflows Pane", []string{
			"1-9                Select workflow by number",
			"0                  Clear the workflow selection",
			"Space              Jump to the config pane",
		}},
		{"Config Pane", []string{
			"1-9, 0             Edit input by number (1-10)",
			"e                  Edit the selected input",
			"b                  Select branch",
			"w                  Toggle watch mode",
			"/                  Start filtering inputs",
			"c                  Copy the gh command to the clipboard",
			"r                  Reset all inputs to defaults",
		}},
		{"Runs and Chains", []string{
			"C                  Run a chain",
			"L                  Live run overview",
			"v                  View logs for the selected entry",
			"a                  Remap inputs of a previewed entry",
			"d / D              Stop watching one / all finished runs",
		}},
		{"Input Editing", []string{
			"Ctrl+R             Restore default value",
			"Enter              Confirm (or apply anyway)",
			"Esc                Cancel / Keep editing",
		}},
		{"Application", []string{
			"?                  Show this help",
			"q, Ctrl+C          Quit",
		}},
	}

	lines := []string{ui.TitleStyle.Render("Keyboard Shortcuts")}

	for _, section := range sections {
		lines = append(lines, "", ui.SubtitleStyle.Render(section.title))
		for _, row := range section.rows {
			lines = append(lines, "  "+row)
		}
	}

	return lines
}

// IsDone returns true if the modal is finished.
func (m *HelpModal) IsDone() bool {
	return m.done
}

// Result returns nil for help modal.
func (*HelpModal) Result() any {
	return nil
}
