package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// Every rendering defect that reached a user was a pane that stopped being a
// rectangle, so the box is the contract: exactly the cells it was given, with
// all four corners, whatever it was asked to draw.
func TestPaneBox_IsExactlyItsRectangleForAnyContent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
	}{
		{name: "empty", content: ""},
		{name: "one short line", content: "hi"},
		{name: "a line wider than the pane", content: strings.Repeat("wide ", 40)},
		{name: "more lines than rows", content: strings.Repeat("row\n", 40)},
		{name: "styled content", content: ui.TitleStyle.Render("Workflows") + "\n" + ui.HelpStyle.Render("[?] help")},
		{name: "wide runes", content: strings.Repeat("\u65e5\u672c\u8a9e", 20)},
		{name: "both over at once", content: strings.Repeat(strings.Repeat("x", 90)+"\n", 30)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for _, size := range [][2]int{{20, 5}, {30, 3}, {80, 24}} {
				for _, focused := range []bool{false, true} {
					assertBox(t, ui.PaneBox(size[0], size[1], focused, tt.content), size[0], size[1])
				}
			}
		})
	}
}

func assertBox(t *testing.T, rendered string, width, height int) {
	t.Helper()

	lines := strings.Split(ansi.Strip(rendered), "\n")
	if len(lines) != height {
		t.Fatalf("a %dx%d pane drew %d rows", width, height, len(lines))
	}

	for i, line := range lines {
		if got := ansi.StringWidth(line); got != width {
			t.Errorf("row %d is %d cells wide, want %d: %q", i, got, width, line)
		}
	}

	for _, edge := range []string{lines[0], lines[height-1]} {
		runes := []rune(edge)
		if runes[0] == ' ' || runes[len(runes)-1] == ' ' {
			t.Errorf("a corner is blank, so the pane lost a border:\n%s", ansi.Strip(rendered))

			break
		}
	}
}

// The window is what keeps a selection on screen. A pane that drew every row
// and let MaxHeight cut the overflow hid the selection instead.
func TestScrollWindow_KeepsTheSelectionInsideAHalfOpenRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		selected, total, rows int
		first, last           int
	}{
		{name: "everything fits", selected: 3, total: 4, rows: 10, first: 0, last: 4},
		{name: "selection above the window", selected: -1, total: 40, rows: 5, first: 0, last: 5},
		{name: "selection inside the first window", selected: 2, total: 40, rows: 5, first: 0, last: 5},
		{name: "selection just past it", selected: 5, total: 40, rows: 5, first: 1, last: 6},
		{name: "selection at the end", selected: 39, total: 40, rows: 5, first: 35, last: 40},
		{name: "no room for a row", selected: 7, total: 40, rows: 0, first: 7, last: 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			first, last := ui.ScrollWindow(tt.selected, tt.total, tt.rows)
			if first != tt.first || last != tt.last {
				t.Errorf("window is [%d,%d), want [%d,%d)", first, last, tt.first, tt.last)
			}

			if tt.selected >= 0 && tt.selected < tt.total && (tt.selected < first || tt.selected >= last) {
				t.Errorf("row %d is outside the window [%d,%d) that is meant to show it", tt.selected, first, last)
			}
		})
	}
}

// A column set that does not fit hands the header an overflow marker, and that
// marker is what wraps the header onto a second line and costs the pane its
// bottom border. So the header the set produces has to fit the width it was
// chosen for.
func TestConfigColumnsFor_ProducesAHeaderThatFitsEveryWidth(t *testing.T) {
	t.Parallel()

	// Below the two-column floor no set fits at all, and the pane truncates.
	for width := ui.RowGutterWidth + ui.ColMinFlag + table.Gutter + ui.ColMinLabel; width <= 120; width++ {
		cols := ui.ConfigColumnsFor(width)
		if len(cols) == 0 {
			t.Fatalf("width %d chose no columns", width)
		}

		layout := ui.FitColumns(cols, width, ui.RowGutterWidth)

		header := ansi.Strip(ui.TableHeader(layout, ui.RowGutterWidth))
		if strings.Contains(header, "\n") {
			t.Fatalf("the header wraps at width %d: %q", width, header)
		}

		if got := ansi.StringWidth(header); got > width {
			t.Errorf("the header is %d cells at width %d: %q", got, width, header)
		}

		values := map[string]string{}
		for _, col := range cols {
			values[col.Key] = strings.Repeat("v", 50)
		}

		row := ansi.Strip(ui.TableRow(layout, values))
		if got := ansi.StringWidth(row) + ui.RowGutterWidth; got > width {
			t.Errorf("a full row is %d cells at width %d", got, width)
		}
	}
}

// A mark set outlives a reordering of the list it came from, so it holds row
// keys rather than indices and hands them back in a stable order.
func TestMarkSet_TogglesAndClears(t *testing.T) {
	t.Parallel()

	var marks ui.MarkSet

	if marks.Has("ci.yml") || marks.Len() != 0 {
		t.Fatal("a zero-value set reported a mark")
	}

	marks.Toggle("release.yml")
	marks.Toggle("ci.yml")

	if got := marks.Keys(); len(got) != 2 || got[0] != "ci.yml" {
		t.Errorf("keys are %v, want them sorted", got)
	}

	marks.Toggle("ci.yml")

	if marks.Has("ci.yml") || marks.Len() != 1 {
		t.Errorf("toggling twice left %d marks", marks.Len())
	}

	if ui.MarkGlyph(marks.Has("release.yml")) == ui.MarkGlyph(false) {
		t.Error("a marked row draws the same glyph as an unmarked one")
	}

	marks.Clear()

	if marks.Len() != 0 {
		t.Error("clear left marks behind")
	}
}

// The filter is what the input pane narrows by, and an empty query means the
// pane is not filtering rather than that nothing matches.
func TestApplyFuzzyFilter_NarrowsAndPassesAnEmptyQueryThrough(t *testing.T) {
	t.Parallel()

	items := []string{"deploy_target", "dry_run", "release_notes"}

	if got := ui.ApplyFuzzyFilter("", items); len(got) != len(items) {
		t.Errorf("an empty query kept %d of %d items", len(got), len(items))
	}

	got := ui.ApplyFuzzyFilter("dry", items)
	if len(got) != 1 || got[0] != "dry_run" {
		t.Errorf("filtering by dry gave %v", got)
	}

	if got := ui.ApplyFuzzyFilter("zzz", items); len(got) != 0 {
		t.Errorf("a query matching nothing gave %v", got)
	}
}

// An empty input value is a value, so it reads as ("") rather than as a blank
// cell indistinguishable from a column the table collapsed.
func TestFormatEmptyValue_NamesTheEmptyString(t *testing.T) {
	t.Parallel()

	if got := ui.FormatEmptyValue(""); got != `("")` {
		t.Errorf("an empty value formats as %q", got)
	}

	if got := ui.FormatEmptyValue("main"); got != "main" {
		t.Errorf("a set value formats as %q", got)
	}

	if ansi.Strip(ui.RenderEmptyValue("")) != `("")` {
		t.Error("the styled empty value does not read as the plain one")
	}
}
