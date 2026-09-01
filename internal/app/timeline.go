package app

import (
	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// FetchTimelineMsg asks for a run's jobs so the Timeline tab can draw them.
type FetchTimelineMsg struct {
	Title string
	RunID int64
}

// TimelineFetchedMsg carries the jobs, or why they could not be read.
type TimelineFetchedMsg struct {
	Error error
	Title string
	Jobs  []github.Job
}

// handleTimelineMsg routes the timeline's own background work.
func (m Model) handleTimelineMsg(msg tea.Msg) (tea.Model, tea.Cmd, bool) {
	switch msg := msg.(type) {
	case FetchTimelineMsg:
		return m, m.fetchTimeline(msg), true

	case TimelineFetchedMsg:
		if msg.Error != nil {
			return m, statusCmd("could not read the run's jobs: " + msg.Error.Error()), true
		}

		m.rightPanel.SetTimelineRun(msg.Title, msg.Jobs)
		m.focused = PaneHistory
		m.rightPanel.SetTab(panes.TabTimeline)

		return m, nil, true
	}

	return m, nil, false
}

func (m Model) fetchTimeline(msg FetchTimelineMsg) tea.Cmd {
	client := m.ghClient

	return func() tea.Msg {
		if client == nil {
			return TimelineFetchedMsg{Error: ErrLogManagerNotInitialized}
		}

		jobs, err := client.GetWorkflowRunJobs(msg.RunID)

		return TimelineFetchedMsg{Title: msg.Title, Jobs: jobs, Error: err}
	}
}

// showTimeline is the verb behind the Timeline action and the :timeline
// command: it names the run and asks for its jobs.
func (m Model) showTimeline(runID int64, title string) (tea.Model, tea.Cmd) {
	if runID == 0 {
		return m, statusCmd("no run to draw a timeline for")
	}

	return m, func() tea.Msg { return FetchTimelineMsg{RunID: runID, Title: title} }
}

// timelineForSelection picks the run whatever has focus is pointing at.
func (m Model) timelineForSelection() (tea.Model, tea.Cmd) {
	if run, ok := m.rightPanel.SelectedRun(); ok && m.rightPanel.ActiveTab() == panes.TabLive {
		return m.showTimeline(run.RunID, run.Workflow)
	}

	if entry := m.rightPanel.SelectedHistoryEntry(); entry != nil {
		for _, step := range entry.StepResults {
			if step.RunID != 0 {
				return m.showTimeline(step.RunID, entry.ChainName)
			}
		}
	}

	return m, statusCmd("no run to draw a timeline for")
}
