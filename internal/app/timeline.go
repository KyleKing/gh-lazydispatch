package app

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// FetchTimelineMsg asks for a run's jobs so the panel can draw them.
type FetchTimelineMsg struct {
	Title string
	RunID int64
}

// TimelineFetchedMsg carries the jobs, or why they could not be read.
type TimelineFetchedMsg struct {
	Error error
	Title string
	Jobs  []github.Job
	RunID int64
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

		m.focused = PaneRight
		m.detailRunID = msg.RunID
		m.detailRunName = msg.Title
		m.rightPanel.ShowDetail(m.detailCrumb(), msg.Title, msg.Jobs)

		return m, m.timelineTick(), true

	case TimelineTickMsg:
		// Arriving is the whole tick: the layout closes an open span at now, so
		// a running bar grows only when a frame is asked for.
		return m, m.timelineTick(), true
	}

	return m, nil, false
}

// TimelineTickMsg asks for a frame so an open bar is drawn against a clock
// that has moved.
type TimelineTickMsg struct{}

// timelineRedrawInterval is how often a run still going is redrawn. Finer than
// the axis can resolve on any run worth drawing, and coarse enough to cost
// nothing.
const timelineRedrawInterval = time.Second

// timelineTick schedules the next redraw, and stops once every bar on screen
// has an end: a closed timeline draws the same picture forever.
func (m Model) timelineTick() tea.Cmd {
	detail := m.rightPanel.Detail()
	if detail == nil || !detail.Running() {
		return nil
	}

	return tea.Tick(timelineRedrawInterval, func(time.Time) tea.Msg { return TimelineTickMsg{} })
}

// detailCrumb names the list a run detail was opened from.
func (m Model) detailCrumb() string {
	switch m.rightPanel.ActiveTab() {
	case panes.TabLive:
		return "Live"
	case panes.TabHistory:
		return "History"
	case panes.TabFlaky:
		return "Flaky"
	case panes.TabRuns:
		return nameRunsTab
	}

	return nameRunsTab
}

func (m Model) fetchTimeline(msg FetchTimelineMsg) tea.Cmd {
	client := m.ghClient

	return func() tea.Msg {
		if client == nil {
			return TimelineFetchedMsg{Error: ErrLogManagerNotInitialized}
		}

		jobs, err := client.GetWorkflowRunJobs(msg.RunID)

		return TimelineFetchedMsg{RunID: msg.RunID, Title: msg.Title, Jobs: jobs, Error: err}
	}
}

// showTimeline is the verb behind opening a run and behind the :timeline
// command: it names the run and asks for its jobs.
func (m Model) showTimeline(runID int64, title string) (tea.Model, tea.Cmd) {
	if runID == 0 {
		return m, statusCmd("no run to draw a timeline for")
	}

	return m, func() tea.Msg { return FetchTimelineMsg{RunID: runID, Title: title} }
}

// timelineForSelection opens the run whatever has focus is pointing at.
func (m Model) timelineForSelection() (tea.Model, tea.Cmd) {
	runID, title, ok := m.selectedRun()
	if !ok {
		return m, statusCmd("no run to draw a timeline for")
	}

	return m.showTimeline(runID, title)
}

// selectedRun is the run every per-run verb acts on, wherever the cursor is:
// a row in one of the run lists, or the run already drilled into. Resolving it
// in one place is what lets diagnose, logs, and the timeline be offered from
// each of them without four copies of the lookup.
func (m Model) selectedRun() (int64, string, bool) {
	if m.rightPanel.Detail() != nil && m.detailRunID != 0 {
		return m.detailRunID, m.detailRunName, true
	}

	switch m.rightPanel.ActiveTab() {
	case panes.TabRuns:
		if run, ok := m.rightPanel.SelectedGitHubRun(); ok {
			return run.ID, run.Name, true
		}
	case panes.TabFlaky:
		if run, ok := m.rightPanel.Flaky().SelectedRun(); ok {
			return run.ID, run.Name, true
		}
	case panes.TabLive:
		if run, ok := m.rightPanel.SelectedRun(); ok {
			return run.RunID, run.Workflow, true
		}
	case panes.TabHistory:
		if entry := m.rightPanel.SelectedHistoryEntry(); entry != nil {
			for _, step := range entry.StepResults {
				if step.RunID != 0 {
					return step.RunID, entry.ChainName, true
				}
			}
		}
	}

	return 0, "", false
}
