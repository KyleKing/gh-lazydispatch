package modal

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// press feeds a key to the viewer and keeps it typed.
func press(t *testing.T, m *LogsViewerModal, key tea.KeyPressMsg) *LogsViewerModal {
	t.Helper()

	ctx, _ := m.Update(key)

	next, ok := ctx.(*LogsViewerModal)
	if !ok {
		t.Fatalf("Update returned %T, want *LogsViewerModal", ctx)
	}

	return next
}

func typeSearch(t *testing.T, m *LogsViewerModal, query string) *LogsViewerModal {
	t.Helper()

	m = press(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, r := range query {
		m = press(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}

	return press(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
}

func newSizedViewer(t *testing.T) *LogsViewerModal {
	t.Helper()

	m := NewLogsViewerModal(createTestRunLogs(), 0, 0)
	m.SetSize(100, 30)

	return m
}

// TestLogsViewer_SearchWalksItsMatches covers the loop a reader actually uses:
// search, step through the hits, and wrap around both ends without moving off
// the list or panicking.
func TestLogsViewer_SearchWalksItsMatches(t *testing.T) {
	t.Parallel()

	m := typeSearch(t, newSizedViewer(t), "Build")

	if len(m.matches) == 0 {
		t.Fatal("searching for a term the fixture contains found nothing")
	}

	first := m.currentMatch
	m = press(t, m, tea.KeyPressMsg{Code: 'n', Text: "n"})

	if len(m.matches) > 1 && m.currentMatch == first {
		t.Error("n did not advance to the next match")
	}

	for range len(m.matches) + 2 {
		m = press(t, m, tea.KeyPressMsg{Code: 'n', Text: "n"})
	}

	if m.currentMatch < 0 || m.currentMatch >= len(m.matches) {
		t.Errorf("walking past the end left match %d of %d", m.currentMatch, len(m.matches))
	}

	for range len(m.matches) + 2 {
		m = press(t, m, tea.KeyPressMsg{Code: 'N', Text: "N"})
	}

	if m.currentMatch < 0 || m.currentMatch >= len(m.matches) {
		t.Errorf("walking past the start left match %d of %d", m.currentMatch, len(m.matches))
	}

	if view := m.View(); !strings.Contains(ansi.Strip(view), "Build") {
		t.Error("the matched line is not on screen")
	}
}

func TestLogsViewer_SearchCanBeAbandoned(t *testing.T) {
	t.Parallel()

	m := newSizedViewer(t)
	m = press(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})

	if !m.searchMode {
		t.Fatal("/ did not enter search mode")
	}

	m = press(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})

	if m.searchMode {
		t.Error("escape did not leave search mode")
	}

	if m.done {
		t.Error("escape in search mode closed the whole viewer")
	}
}

// TestLogsViewer_EmptyFilterExplainsItself keeps a filter that matches nothing
// from looking like a viewer that failed to load.
func TestLogsViewer_EmptyFilterExplainsItself(t *testing.T) {
	t.Parallel()

	m := typeSearch(t, newSizedViewer(t), "nothing-matches-this")

	view := ansi.Strip(m.View())
	if strings.TrimSpace(view) == "" {
		t.Fatal("rendered nothing")
	}

	if !strings.Contains(strings.ToLower(view), "no ") {
		t.Errorf("an empty result renders no explanation:\n%s", view)
	}
}

// TestLogsViewer_FoldingSectionsIsReversible covers the keys that make a long
// run readable, including the per-section toggle under the cursor.
func TestLogsViewer_FoldingSectionsIsReversible(t *testing.T) {
	t.Parallel()

	m := newSizedViewer(t)
	full := m.View()

	m = press(t, m, tea.KeyPressMsg{Code: 'C', Text: "C"})

	collapsed := m.View()
	if collapsed == full {
		t.Error("collapse all changed nothing")
	}

	m = press(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	m = press(t, m, tea.KeyPressMsg{Code: 'E', Text: "E"})

	if m.View() != full {
		t.Error("expand all did not restore the original view")
	}
}

// TestLogsViewer_StreamingAppendsWithoutLosingWhatIsThere covers the live path:
// a run still in progress gains steps while the viewer is open.
func TestLogsViewer_StreamingAppendsWithoutLosingWhatIsThere(t *testing.T) {
	t.Parallel()

	m := newSizedViewer(t)
	before := len(m.runLogs.AllSteps())

	m.EnableStreaming(12345, true)

	if !m.IsStreaming() || m.StreamRunID() != 12345 {
		t.Fatalf("streaming is %v for run %d", m.IsStreaming(), m.StreamRunID())
	}

	if !strings.Contains(ansi.Strip(m.View()), "LIVE") {
		t.Error("a streaming viewer does not say so")
	}

	m.AppendStreamUpdate(logs.StreamUpdate{
		RunID:  12345,
		Status: github.StatusInProgress,
		NewSteps: []*logs.StepLogs{{
			StepIndex: before,
			StepName:  "Deploy",
			Workflow:  "test.yml",
			Status:    github.StatusInProgress,
			Entries:   []logs.LogEntry{{Timestamp: time.Now(), Content: "Deploying", Level: logs.LogLevelInfo}},
		}},
	})

	if got := len(m.runLogs.AllSteps()); got != before+1 {
		t.Errorf("the run has %d steps after an update, want %d", got, before+1)
	}

	view := ansi.Strip(m.View())
	for _, want := range []string{"Setup", "Deploy"} {
		if !strings.Contains(view, want) {
			t.Errorf("%q is missing after the stream update", want)
		}
	}

	m.AppendStreamUpdate(logs.StreamUpdate{RunID: 12345, Error: errStream})

	if got := len(m.runLogs.AllSteps()); got != before+1 {
		t.Errorf("a failed update changed the run to %d steps", got)
	}

	m = press(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.autoScroll {
		t.Error("s did not turn auto-scroll off")
	}

	m.DisableStreaming()

	if m.IsStreaming() {
		t.Error("DisableStreaming left streaming on")
	}
}

// errStream stands in for a poll that failed, which must leave the viewer
// holding what it already had.
var errStream = errors.New("stream interrupted")
