package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/kyleking/gh-lazydispatch/internal/ui/panes"
)

// A frame is a stack of boxes at known coordinates, and every rendering defect
// so far has been a box that stopped being one. A clipped frame still fits the
// terminal, so a width check catches none of them.
const (
	boxTopLeft     = '╭'
	boxTopRight    = '╮'
	boxBottomLeft  = '╰'
	boxBottomRight = '╯'
	boxSide        = '│'

	focusTopLeft     = '┏'
	focusTopRight    = '┓'
	focusBottomLeft  = '┗'
	focusBottomRight = '┛'
	focusSide        = '┃'
)

func isAny(r rune, want ...rune) bool {
	for _, w := range want {
		if r == w {
			return true
		}
	}

	return false
}

// frameRows is everything between the status bar and the footer, stripped of
// styling and indexed by visible cell.
func frameRows(t *testing.T, m Model) [][]rune {
	t.Helper()

	lines := strings.Split(m.View().Content, "\n")
	if len(lines) != m.height {
		t.Fatalf("the frame is %d lines, the terminal is %d", len(lines), m.height)
	}

	rows := make([][]rune, 0, len(lines)-2)

	for i, line := range lines {
		if w := ansi.StringWidth(line); w > m.width {
			t.Fatalf("line %d is %d cells wide, the terminal is %d", i+1, w, m.width)
		}

		if i == 0 || i == len(lines)-1 {
			continue
		}

		row := []rune(ansi.Strip(line))
		for len(row) < m.width {
			row = append(row, ' ')
		}

		rows = append(rows, row)
	}

	return rows
}

// assertBox holds one pane to the rectangle the layout gave it.
func assertBox(t *testing.T, name string, rows [][]rune, top, height, left, width int) {
	t.Helper()

	if height <= 0 {
		return
	}

	if top+height > len(rows) {
		t.Errorf("%s claims rows %d-%d of %d", name, top, top+height, len(rows))
		return
	}

	right := left + width - 1

	for i := range height {
		row := rows[top+i]

		gotLeft, gotRight := row[left], row[right]

		var wantLeft, wantRight []rune

		switch i {
		case 0:
			wantLeft, wantRight = []rune{boxTopLeft, focusTopLeft}, []rune{boxTopRight, focusTopRight}
		case height - 1:
			wantLeft, wantRight = []rune{boxBottomLeft, focusBottomLeft}, []rune{boxBottomRight, focusBottomRight}
		default:
			wantLeft, wantRight = []rune{boxSide, focusSide}, []rune{boxSide, focusSide}
		}

		if !isAny(gotLeft, wantLeft...) || !isAny(gotRight, wantRight...) {
			t.Errorf("%s row %d of %d is %q…%q, want %q…%q\n%s",
				name, i+1, height, string(gotLeft), string(gotRight),
				string(wantLeft[0]), string(wantRight[0]), string(row))

			return
		}
	}
}

// assertFrameWellFormed checks the frame against the layout that drew it. It
// reads no text, so it holds in every state, including the ones nobody thought
// to write a golden for.
func assertFrameWellFormed(t *testing.T, m Model) {
	t.Helper()

	if m.width < MinTerminalWidth || m.height < MinTerminalHeight {
		return
	}

	if m.modalStack.HasActive() {
		t.Fatal("a modal overlays the panes, so the frame cannot be read as boxes")
	}

	box := m.layout()

	// A detail too tall for its pane overlays the frame rather than being cut,
	// and an overlay covers the boxes this reads.
	if m.promotedDetail(box) != "" {
		return
	}

	rows := frameRows(t, m)

	assertBox(t, "workflow pane", rows, 0, box.workflowHeight, 0, box.leftWidth)
	assertBox(t, "chains pane", rows, box.workflowHeight, box.chainsHeight, 0, box.leftWidth)

	configTop := box.workflowHeight + box.chainsHeight + box.chainsHintRows
	assertBox(t, "config pane", rows, configTop, box.configHeight, 0, box.leftWidth)
	assertBox(t, "right panel", rows, 0, box.rightHeight, box.leftWidth, box.rightWidth)

	if got := configTop + box.configHeight; got != len(rows) {
		t.Errorf("the left column fills %d of %d body rows", got, len(rows))
	}
}

// frameStates are the states the panes reach, exercised at every size. A new
// state is one entry here rather than a new test.
func frameStates() []struct {
	name string
	set  func(t *testing.T, m Model) Model
} {
	return []struct {
		name string
		set  func(t *testing.T, m Model) Model
	}{
		{"workflow_list", func(_ *testing.T, m Model) Model { return m }},
		{"no_chains", func(_ *testing.T, m Model) Model {
			m.chains.SetChains(nil)
			return m
		}},
		{"no_workflow_selected", func(_ *testing.T, m Model) Model {
			m.selectedWorkflow = -1
			return m
		}},
		{"long_branch", func(_ *testing.T, m Model) Model {
			m.branch = "renovate/all-the-dependencies-in-one-very-long-branch-name"
			return m
		}},
		{"filtering", func(_ *testing.T, m Model) Model {
			m.filterText = "env"
			m.applyFilter()

			return m
		}},
		{"input_detail", func(_ *testing.T, m Model) Model {
			m.viewMode = InputDetailMode
			m.selectedInput = 0

			return m
		}},
		{"history_preview", func(_ *testing.T, m Model) Model {
			m.viewMode = HistoryPreviewMode
			entries := m.history.TopForRepo("owner/repo", "", 1)

			if len(entries) > 0 {
				m.previewingHistoryEntry = &entries[0]
			}

			return m
		}},
		{"status_message", func(_ *testing.T, m Model) Model {
			m.status = "dispatch failed: gh exited 1"
			return m
		}},
		{"command_bar", func(t *testing.T, m Model) Model {
			t.Helper()
			return pressRune(t, m, ':')
		}},
		{"detail_open", func(t *testing.T, m Model) Model {
			t.Helper()
			m.rightPanel.ShowDetail(nameRunsTab, "CI", detailJobs(true))

			return m
		}},
		{"detail_drilled", func(t *testing.T, m Model) Model {
			t.Helper()
			m.rightPanel.ShowDetail(nameRunsTab, "CI", detailJobs(false))
			m.rightPanel.Detail().Drill()

			return m
		}},
	}
}

func TestFrame_StaysBoxesInEveryStateAndSize(t *testing.T) {
	t.Parallel()

	for _, size := range renderSizes {
		for _, state := range frameStates() {
			for _, focus := range []FocusedPane{PaneWorkflows, PaneChains, PaneConfig, PaneRight} {
				name := fmt.Sprintf("%s/%s/focus_%d", size.name, state.name, focus)

				t.Run(name, func(t *testing.T) {
					t.Parallel()

					m := state.set(t, newRenderModel())
					m.focusPane(focus)
					m = resize(t, m, size.width, size.height)

					assertFrameWellFormed(t, m)
				})
			}
		}
	}
}

// Every tab draws into the same panel, so each has to fill it exactly.
func TestFrame_EveryTabFillsTheRightPanel(t *testing.T) {
	t.Parallel()

	for _, size := range renderSizes {
		for tab := range panes.RightTab(panes.TabCount) {
			t.Run(fmt.Sprintf("%s/tab_%d", size.name, tab), func(t *testing.T) {
				t.Parallel()

				m := resize(t, newRenderModel(), size.width, size.height)
				m.focusPane(PaneRight)
				m.rightPanel.SetTab(tab)

				assertFrameWellFormed(t, m)
			})
		}
	}
}

// The frame check proves a state stays a box. A golden proves it reads right.
func TestFrame_GoldenStates(t *testing.T) {
	t.Parallel()

	want := map[string]bool{"no_chains": true, "input_detail": true, "history_preview": true, "detail_open": true}

	for _, state := range frameStates() {
		if !want[state.name] {
			continue
		}

		t.Run(state.name, func(t *testing.T) {
			t.Parallel()

			m := resize(t, state.set(t, newRenderModel()), 80, 24)

			assertFrameWellFormed(t, m)
			golden.RequireEqual(t, []byte(m.View().Content))
		})
	}
}

// An input's details are the value, the default, and the keys that change
// them, so a pane too short to hold them cuts off exactly what the view is
// for. It overlays the frame at that size instead, and goes back in the pane
// as soon as there is room.
func TestInputDetail_OverlaysTheFrameOnlyWhereThePaneIsTooShort(t *testing.T) {
	t.Parallel()

	build := frameStates()[5]
	if build.name != "input_detail" {
		t.Fatalf("the input detail state moved to %q", build.name)
	}

	small := build.set(t, resize(t, newRenderModel(), MinTerminalWidth, MinTerminalHeight))

	detail := small.promotedDetail(small.layout())
	if detail == "" {
		t.Fatal("the detail fits an 80x24 pane, so nothing needed promoting")
	}

	view := ansi.Strip(small.View().Content)
	for _, want := range []string{"Type:", "Current:"} {
		if !strings.Contains(view, want) {
			t.Errorf("the overlay does not carry %q:\n%s", want, view)
		}
	}

	// The list is still drawn underneath, so escaping the overlay reveals it
	// rather than a pane that was never rendered.
	if !strings.Contains(ansi.Strip(small.viewLeftColumn(small.layout(), true)), "Workflows") {
		t.Error("the promoted detail left no workflow list under it")
	}

	large := build.set(t, resize(t, newRenderModel(), 160, 50))
	if got := large.promotedDetail(large.layout()); got != "" {
		t.Error("a terminal with room to spare still promoted the detail")
	}
}
