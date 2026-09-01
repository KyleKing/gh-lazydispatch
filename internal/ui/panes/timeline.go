package panes

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
	"github.com/kyleking/gh-lazydispatch/internal/ui/timeline"
)

// TimelineModel draws a run's jobs on one time axis, and the steps of a job
// once you drill into it. Two renderings of the same layout: the question
// changes from "what ran together" to "where inside this did the time go".
type TimelineModel struct {
	title    string
	jobs     []github.Job
	drilled  int
	selected int
	width    int
	height   int
	focused  bool
}

// noDrill is drilled's value while the lanes rather than a job's steps are on
// screen.
const noDrill = -1

// NewTimelineModel creates an empty timeline.
func NewTimelineModel() TimelineModel {
	return TimelineModel{drilled: noDrill}
}

// SetRun replaces what the timeline draws, keeping the drill-down only while
// the same job is still there.
func (m *TimelineModel) SetRun(title string, jobs []github.Job) {
	m.title = title
	m.jobs = jobs

	if m.drilled >= len(jobs) {
		m.drilled = noDrill
	}

	if m.selected >= len(jobs) {
		m.selected = 0
	}
}

// SetSize updates the pane dimensions.
func (m *TimelineModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused updates the focus state.
func (m *TimelineModel) SetFocused(focused bool) { m.focused = focused }

// Drilled reports whether a job's steps are on screen.
func (m TimelineModel) Drilled() bool { return m.drilled != noDrill }

// MoveUp moves the selection toward the first row.
func (m *TimelineModel) MoveUp() {
	if m.selected > 0 {
		m.selected--
	}
}

// MoveDown moves the selection toward the last row.
func (m *TimelineModel) MoveDown() {
	if m.selected < m.rowCount()-1 {
		m.selected++
	}
}

// Drill opens the selected job's steps.
func (m *TimelineModel) Drill() {
	if m.drilled == noDrill && m.selected < len(m.jobs) {
		m.drilled = m.selected
		m.selected = 0
	}
}

// Undrill returns to the jobs, reselecting the one that was open.
func (m *TimelineModel) Undrill() {
	if m.drilled != noDrill {
		m.selected = m.drilled
		m.drilled = noDrill
	}
}

// Update handles navigation and drilling.
//
//nolint:unparam // uniform (TabbedRightModel, tea.Cmd) signature, required by the tab dispatch
func (m TimelineModel) Update(msg tea.Msg) (TimelineModel, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "up", "k":
		m.MoveUp()
	case "down", "j":
		m.MoveDown()
	case "enter":
		m.Drill()
	case "esc":
		m.Undrill()
	}

	return m, nil
}

func (m TimelineModel) rowCount() int {
	if m.drilled == noDrill {
		return len(m.jobs)
	}

	return len(m.jobs[m.drilled].Steps)
}

// spans builds what is currently on screen: every job, or one job's steps.
func (m TimelineModel) spans() []timeline.Span {
	if m.drilled != noDrill {
		job := m.jobs[m.drilled]

		spans := make([]timeline.Span, 0, len(job.Steps))
		for _, step := range job.Steps {
			if step.StartedAt.IsZero() {
				continue
			}

			spans = append(spans, timeline.Span{
				Label: stepLabel(step.Name), Start: step.StartedAt,
				End: step.CompletedAt, Conclusion: step.Conclusion,
			})
		}

		return spans
	}

	spans := make([]timeline.Span, 0, len(m.jobs))

	for i := range m.jobs {
		job := &m.jobs[i]
		spans = append(spans, timeline.Span{
			Label: job.Name, Start: job.StartedAt,
			End: job.CompletedAt, Conclusion: job.Conclusion,
		})
	}

	return spans
}

// stepLabel strips the action-reference noise GitHub puts in a step name, so
// "Run actions/checkout@3d3c42e..." reads as "actions/checkout".
func stepLabel(name string) string {
	trimmed := strings.TrimPrefix(name, "Run ")
	if ref, _, found := strings.Cut(trimmed, "@"); found {
		return ref
	}

	return trimmed
}

// Layout widths, where timelineGutter is every separator a row spends outside
// the track: the two-cell cursor, the outcome mark, and the three single
// spaces between the four columns.
const (
	timelineLabelWidth = 18
	timelineDurWidth   = 7
	timelineGutter     = 5
)

// ViewContent renders the timeline without the surrounding pane border.
func (m TimelineModel) ViewContent() string {
	if len(m.jobs) == 0 {
		return ui.TableDimmedStyle.Render(
			"No run selected.\n\nOpen a run from History or Live with [a] then [t]\nto see where its time went.",
		)
	}

	track := m.width - timelineLabelWidth - timelineDurWidth - timelineGutter

	layout := timeline.Lay(m.spans(), track, time.Now())
	if len(layout.Rows) == 0 {
		return ui.TableDimmedStyle.Render("Not enough room to draw a timeline.")
	}

	var s strings.Builder

	s.WriteString(ui.SubtitleStyle.Render(ansi.Truncate(m.Heading(), m.width, "…")))
	s.WriteString("\n")

	for i, row := range layout.Rows {
		s.WriteString(m.renderRow(i, row, track))
		s.WriteString("\n")
	}

	s.WriteString(ui.TableDimmedStyle.Render(TimelineModel{}.renderAxis(layout, track)))

	return s.String()
}

// Heading names what is on screen: the run, or the run and the job drilled into.
func (m TimelineModel) Heading() string {
	if m.drilled == noDrill {
		return m.title
	}

	return m.title + " › " + m.jobs[m.drilled].Name
}

func (m TimelineModel) renderRow(i int, row timeline.Row, track int) string {
	cursor := "  "
	style := ui.NormalStyle

	if i == m.selected {
		cursor = "> "
		style = ui.SelectedStyle
	}

	fill := "█"
	if row.Running {
		fill = "▓"
	}

	bar := strings.Repeat(" ", row.Offset) + strings.Repeat(fill, row.Width)

	return style.Render(fmt.Sprintf("%s%s %s %s %s",
		cursor,
		conclusionMark(row.Conclusion, row.Running),
		ansi.Truncate(row.Label, timelineLabelWidth, "…")+strings.Repeat(" ",
			max(timelineLabelWidth-ansi.StringWidth(ansi.Truncate(row.Label, timelineLabelWidth, "…")), 0)),
		bar+strings.Repeat(" ", max(track-ansi.StringWidth(bar), 0)),
		fmt.Sprintf("%*s", timelineDurWidth-1, shortDuration(row.Duration))))
}

// axisEnds is the two corner glyphs the rule spends on itself.
const axisEnds = 2

// renderAxis draws the rule under the bars and labels its far end with the
// wall clock the whole picture is measured against.
func (TimelineModel) renderAxis(layout timeline.Layout, track int) string {
	if track < axisEnds {
		return ""
	}

	rule := "└" + strings.Repeat("─", track-axisEnds) + "┘"
	label := shortDuration(layout.Span)
	scale := "0" + fmt.Sprintf("%*s", max(track-1, 0), label)

	indent := strings.Repeat(" ", timelineLabelWidth+timelineGutter)

	return indent + rule + "\n" + indent + ansi.Truncate(scale, track, "")
}

func conclusionMark(conclusion string, running bool) string {
	if running {
		return "·"
	}

	switch conclusion {
	case github.ConclusionSuccess:
		return "✓"
	case github.ConclusionFailure:
		return "✗"
	case github.ConclusionCancelled, github.ConclusionSkipped:
		return "-"
	}

	return "·"
}

const secondsPerMinute = 60

// shortDuration reads as a duration a reader scans rather than parses: seconds
// under a minute, then minutes and seconds.
func shortDuration(d time.Duration) string {
	seconds := int(d.Round(time.Second).Seconds())
	if seconds < secondsPerMinute {
		return fmt.Sprintf("%ds", seconds)
	}

	return fmt.Sprintf("%dm%02ds", seconds/secondsPerMinute, seconds%secondsPerMinute)
}
