package modal

import (
	"strconv"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// RunActionKind names a mutation offered on a run that already exists.
type RunActionKind string

// The mutations a run row offers.
const (
	RunActionRerun  RunActionKind = "rerun"
	RunActionCancel RunActionKind = "cancel"
)

// RunActionResultMsg reports whether a run mutation was confirmed.
type RunActionResultMsg struct {
	Name      string
	Kind      RunActionKind
	RunID     int64
	Confirmed bool
}

// RunActionModal confirms a mutation on an existing run. Every dispatch is
// confirmed before it goes out, and re-running or canceling somebody's run is
// no less outward-facing than starting one.
type RunActionModal struct {
	name   string
	kind   RunActionKind
	keys   runConfirmKeyMap
	result RunActionResultMsg
	runID  int64
	width  int
	done   bool
}

// NewRunActionModal confirms kind against the named run.
func NewRunActionModal(kind RunActionKind, runID int64, name string) *RunActionModal {
	return &RunActionModal{
		kind: kind, runID: runID, name: name,
		keys: runConfirmKeyMap{
			Confirm: key.NewBinding(key.WithKeys("enter", "y")),
			Cancel:  key.NewBinding(key.WithKeys("esc", "n")),
		},
	}
}

// SetSize records the room the modal has.
func (m *RunActionModal) SetSize(width, _ int) { m.width = width }

// Update handles input for the run action modal.
func (m *RunActionModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, m.keys.Confirm):
		return m.finish(true)
	case key.Matches(keyMsg, m.keys.Cancel):
		return m.finish(false)
	}

	return m, nil
}

func (m *RunActionModal) finish(confirmed bool) (Context, tea.Cmd) {
	m.done = true
	m.result = RunActionResultMsg{Kind: m.kind, RunID: m.runID, Name: m.name, Confirmed: confirmed}

	return m, func() tea.Msg { return m.result }
}

// Command is the gh invocation the modal is confirming, shown so what runs is
// what was read rather than something the modal describes in its own words.
func (m *RunActionModal) Command() string {
	id := strconv.FormatInt(m.runID, 10)
	if m.kind == RunActionRerun {
		return "gh run rerun " + id + " --failed"
	}

	return "gh run cancel " + id
}

func (m *RunActionModal) title() string {
	if m.kind == RunActionRerun {
		return "Re-run the failed jobs"
	}

	return "Cancel this run"
}

// View renders the confirmation.
func (m *RunActionModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render(m.title()))
	s.WriteString("\n\n")
	s.WriteString(ui.SubtitleStyle.Render(m.name + " (run " + strconv.FormatInt(m.runID, 10) + ")"))
	s.WriteString("\n\n")
	s.WriteString(ui.CLIPreviewStyle.Render(m.Command()))
	s.WriteString("\n\n")
	s.WriteString(renderHints(m.width, "[enter] confirm", "[esc] cancel"))

	return s.String()
}

// IsDone returns true when the modal is finished.
func (m *RunActionModal) IsDone() bool { return m.done }

// Result returns the confirmation.
func (m *RunActionModal) Result() any { return m.result }
