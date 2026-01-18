package modal

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/kyleking/lazydispatch/internal/github"
	"github.com/kyleking/lazydispatch/internal/ui"
	"github.com/kyleking/lazydispatch/internal/watcher"
)

// LiveViewModal displays the status of watched workflow runs.
type LiveViewModal struct {
	runs     []watcher.WatchedRun
	selected int
	done     bool
	keys     liveViewKeyMap
}

type liveViewKeyMap struct {
	Close      key.Binding
	Up         key.Binding
	Down       key.Binding
	Clear      key.Binding
	ClearAll   key.Binding
}

func defaultLiveViewKeyMap() liveViewKeyMap {
	return liveViewKeyMap{
		Close:    key.NewBinding(key.WithKeys("esc", "l", "q")),
		Up:       key.NewBinding(key.WithKeys("up", "k")),
		Down:     key.NewBinding(key.WithKeys("down", "j")),
		Clear:    key.NewBinding(key.WithKeys("d")),
		ClearAll: key.NewBinding(key.WithKeys("D")),
	}
}

// NewLiveViewModal creates a new live view modal.
func NewLiveViewModal(runs []watcher.WatchedRun) *LiveViewModal {
	return &LiveViewModal{
		runs: runs,
		keys: defaultLiveViewKeyMap(),
	}
}

// Update handles input for the live view modal.
func (m *LiveViewModal) Update(msg tea.Msg) (Context, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, m.keys.Close):
			m.done = true
		case key.Matches(msg, m.keys.Up):
			if m.selected > 0 {
				m.selected--
			}
		case key.Matches(msg, m.keys.Down):
			if m.selected < len(m.runs)-1 {
				m.selected++
			}
		case key.Matches(msg, m.keys.Clear):
			if len(m.runs) > 0 && m.selected < len(m.runs) {
				m.done = true
				return m, func() tea.Msg {
					return LiveViewClearMsg{RunID: m.runs[m.selected].RunID}
				}
			}
		case key.Matches(msg, m.keys.ClearAll):
			m.done = true
			return m, func() tea.Msg {
				return LiveViewClearAllMsg{}
			}
		}
	}
	return m, nil
}

// UpdateRuns updates the list of watched runs.
func (m *LiveViewModal) UpdateRuns(runs []watcher.WatchedRun) {
	m.runs = runs
	if m.selected >= len(runs) && len(runs) > 0 {
		m.selected = len(runs) - 1
	}
}

// View renders the live view modal.
func (m *LiveViewModal) View() string {
	var s strings.Builder

	s.WriteString(ui.TitleStyle.Render("Watched Runs"))
	s.WriteString("\n\n")

	if len(m.runs) == 0 {
		s.WriteString(ui.SubtitleStyle.Render("No runs being watched"))
		s.WriteString("\n\n")
		s.WriteString(ui.HelpStyle.Render("Press l or Esc to close"))
		return s.String()
	}

	for i, run := range m.runs {
		prefix := "  "
		if i == m.selected {
			prefix = "> "
		}

		statusIcon := runStatusIcon(run.Status, run.Conclusion)
		line := fmt.Sprintf("%s%s %s (%s)", prefix, statusIcon, run.Workflow, run.Status)

		if i == m.selected {
			s.WriteString(ui.SelectedStyle.Render(line))
		} else {
			s.WriteString(line)
		}
		s.WriteString("\n")

		if i == m.selected && len(run.Jobs) > 0 {
			for _, job := range run.Jobs {
				jobIcon := runStatusIcon(job.Status, job.Conclusion)
				s.WriteString(fmt.Sprintf("    %s %s\n", jobIcon, job.Name))

				for _, step := range job.Steps {
					stepIcon := runStatusIcon(step.Status, step.Conclusion)
					s.WriteString(fmt.Sprintf("      %s %s\n", stepIcon, step.Name))
				}
			}
		}
	}

	s.WriteString("\n")
	s.WriteString(ui.HelpStyle.Render("j/k navigate  d clear  D clear all  l/Esc close"))

	return s.String()
}

// IsDone returns true if the modal is finished.
func (m *LiveViewModal) IsDone() bool {
	return m.done
}

// Result returns nil for live view modal.
func (m *LiveViewModal) Result() any {
	return nil
}

// LiveViewClearMsg is sent when user wants to clear a specific run.
type LiveViewClearMsg struct {
	RunID int64
}

// LiveViewClearAllMsg is sent when user wants to clear all completed runs.
type LiveViewClearAllMsg struct{}

func runStatusIcon(status, conclusion string) string {
	switch status {
	case github.StatusQueued:
		return "o"
	case github.StatusInProgress:
		return "*"
	case github.StatusCompleted:
		switch conclusion {
		case github.ConclusionSuccess:
			return "+"
		case github.ConclusionFailure:
			return "x"
		case github.ConclusionCancelled:
			return "-"
		default:
			return "?"
		}
	default:
		return "?"
	}
}

// FormatStatusBar returns a formatted status bar string for watched runs.
func FormatStatusBar(runs []watcher.WatchedRun) string {
	if len(runs) == 0 {
		return ""
	}

	active := 0
	success := 0
	failed := 0
	for _, run := range runs {
		switch {
		case run.IsActive():
			active++
		case run.IsSuccess():
			success++
		default:
			failed++
		}
	}

	parts := []string{fmt.Sprintf("Watching: %d runs", len(runs))}
	if active > 0 {
		parts = append(parts, fmt.Sprintf("%d active", active))
	}
	if success > 0 {
		parts = append(parts, fmt.Sprintf("%d done", success))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}

	return strings.Join(parts, ", ")
}
