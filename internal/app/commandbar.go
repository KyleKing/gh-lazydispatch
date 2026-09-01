package app

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// commandLeader opens the command bar. It is a separate grammar from the
// action leader: the bar is a name you type, the menu is a list you read.
const commandLeader = ":"

// StatusMsg carries a one-line result to the footer. A command that answers
// with text rather than a new view says so here.
type StatusMsg struct {
	Text string
}

func statusCmd(text string) tea.Cmd {
	return func() tea.Msg { return StatusMsg{Text: text} }
}

// newCommandInput builds the bar's text input with the modal backgrounds
// stripped, so it sits on the footer rather than painting a block over it.
func newCommandInput() textinput.Model {
	ti := textinput.New()
	ti.Prompt = commandLeader
	ti.Placeholder = "command"

	s := ti.Styles()
	s.Focused.Prompt = s.Focused.Prompt.UnsetBackground()
	s.Focused.Text = s.Focused.Text.UnsetBackground()
	s.Focused.Placeholder = s.Focused.Placeholder.UnsetBackground()
	s.Blurred.Prompt = s.Blurred.Prompt.UnsetBackground()
	s.Blurred.Text = s.Blurred.Text.UnsetBackground()
	ti.SetStyles(s)

	return ti
}

// openCommandBar focuses the bar with an empty line.
func (m *Model) openCommandBar() tea.Cmd {
	m.commandMode = true
	m.status = ""
	m.completions = nil
	m.commandInput.SetValue("")

	return m.commandInput.Focus()
}

func (m *Model) closeCommandBar() {
	m.commandMode = false
	m.completions = nil
	m.commandInput.Blur()
}

// handleCommandKey drives the bar while it has focus. Every key belongs to it
// until it closes, which is what keeps a command name from triggering the
// actions its letters are bound to.
func (m Model) handleCommandKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.closeCommandBar()

		return m, nil

	case keyEnter:
		line := m.commandInput.Value()
		m.closeCommandBar()

		return m.runCommandLine(line)

	case "ctrl+c":
		return m, tea.Quit

	case "tab":
		m.completeCommandLine()

		return m, nil
	}

	m.completions = nil

	var cmd tea.Cmd
	m.commandInput, cmd = m.commandInput.Update(msg)

	return m, cmd
}

// runCommandLine resolves and runs one typed line.
func (m Model) runCommandLine(line string) (tea.Model, tea.Cmd) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return m, nil
	}

	command, found := m.registry.Lookup(fields[0])
	if !found {
		return m, statusCmd("no command matches " + commandLeader + fields[0])
	}

	return command.Run(m, fields[1:])
}

// completeCommandLine fills in as far as every candidate agrees, and lists
// them when they disagree. Filling only the shared prefix is what keeps tab
// from choosing between two commands on the user's behalf.
func (m *Model) completeCommandLine() {
	candidates, completable := m.registry.completionsFor(*m, m.commandInput.Value())
	if !completable || len(candidates) == 0 {
		m.completions = nil

		return
	}

	shared := commonPrefix(candidates)
	line := m.commandInput.Value()

	if replaced := replaceLastWord(line, shared); shared != "" && replaced != line {
		m.commandInput.SetValue(replaced)
		m.commandInput.SetCursor(len(replaced))
	}

	if len(candidates) == 1 {
		m.completions = nil

		return
	}

	m.completions = candidates
}

// replaceLastWord swaps the word being typed for word, keeping everything
// before it. A line ending in a space is opening a new word rather than
// extending the last one.
func replaceLastWord(line, word string) string {
	if strings.HasSuffix(line, " ") || line == "" {
		return line + word
	}

	idx := strings.LastIndex(line, " ")
	if idx < 0 {
		return word
	}

	return line[:idx+1] + word
}

// viewCommandBar renders the bar and, beside it, whatever tab could not choose
// between. It is exactly one line: the layout gives the footer one row, and a
// second would push the frame past the bottom of the terminal.
func (m Model) viewCommandBar(width int) string {
	if !m.commandMode {
		return ""
	}

	line := " " + m.commandInput.View()

	if len(m.completions) > 0 {
		room := width - ansi.StringWidth(line) - len(completionGap)
		if hints := renderCompletions(m.completions, room); hints != "" {
			line += completionGap + ui.TableDimmedStyle.Render(hints)
		}
	}

	return ansi.Truncate(line, width, "…")
}

// completionGap separates the typed line from the candidates beside it.
const completionGap = "   "

// renderCompletions lists the candidate names, or the one candidate with its
// description, in the room left beside the input. Candidates past the room
// available are replaced by a count, since a name cut in half is worse than
// knowing how many were not shown.
func renderCompletions(candidates []Candidate, width int) string {
	if width < 1 {
		return ""
	}

	if len(candidates) == 1 {
		return ansi.Truncate(candidates[0].Name+"  "+candidates[0].Description, width, "…")
	}

	line := ""

	for i, candidate := range candidates {
		next := candidate.Name
		if line != "" {
			next = line + "  " + candidate.Name
		}

		more := fmt.Sprintf(" +%d", len(candidates)-i)
		if ansi.StringWidth(next)+ansi.StringWidth(more) > width {
			return line + more
		}

		line = next
	}

	return line
}
