package ui

import (
	"strings"

	"github.com/kyleking/aragonite/tui/table"
)

// Column keys and titles shared by more than one table.
const (
	ColKeyName     = "name"
	ColKeyWorkflow = "workflow"
	ColTitleName   = "Name"
)

// Column sizing vocabulary shared by every table, in display cells. Naming the
// floors keeps a table readable as a set of tradeoffs rather than a wall of
// digits. Every column holding a short value is capped, because surplus width
// spread across them pushes a row's cells so far apart that the eye has to
// travel between them on a wide terminal.
const (
	ColWidthReq = 3
	ColMinFlag  = 1
	ColMinCount = 4
	ColMinShort = 6
	ColMinLabel = 8
	ColMinName  = 10

	ColMaxFlag   = 1
	ColMaxCount  = 4
	ColMaxSteps  = 5
	ColMaxTime   = 12
	ColMaxStatus = 14
	ColMaxValue  = 24
	ColMaxBranch = 28
	ColMaxName   = 36

	WeightLow  = 1
	WeightMid  = 2
	WeightHigh = 3

	PrioFirstToGo  = 1
	PrioSecondToGo = 2
	PrioThirdToGo  = 3
)

// ConfigColumns describes the workflow-input table. Name is the column a
// reader scans, so it keeps the most weight and is the last to give ground.
func ConfigColumns() []table.Column {
	return []table.Column{
		{Key: "num", Title: "#", Min: ColMinFlag, Max: ColMaxFlag},
		{Key: "req", Title: "Req", Min: ColWidthReq, Max: ColWidthReq, Priority: PrioFirstToGo},
		{Key: ColKeyName, Title: ColTitleName, Min: ColMinLabel, Max: ColMaxName, Weight: WeightHigh},
		{Key: "value", Title: "Value", Min: ColMinShort, Max: ColMaxValue, Weight: WeightMid},
		{
			Key: "default", Title: "Default", Min: ColMinShort, Max: ColMaxValue,
			Weight: WeightMid, Priority: PrioSecondToGo,
		},
	}
}

// RowGutterWidth is the "> " selection gutter and any status glyph that sit
// left of the first column, which the table itself never sees.
const RowGutterWidth = 2

// FitColumns resolves cols against the room a pane leaves for its rows, past
// the gutter its selection indicator and status glyphs occupy.
func FitColumns(cols []table.Column, width, gutter int) table.Layout {
	return table.Fit(cols, width-gutter)
}

// TableHeader renders a column header indented past the row gutter.
func TableHeader(layout table.Layout, gutter int) string {
	return TableHeaderStyle.Render(strings.Repeat(" ", gutter) + table.Header(layout))
}

// TableRow pads each value to its resolved column width, skipping the columns
// collapse hid.
func TableRow(layout table.Layout, values map[string]string) string {
	cells := make([]string, 0, len(layout.Columns))
	for _, col := range layout.Columns {
		cells = append(cells, table.Pad(values[col.Key], layout.Width(col.Key), col.Align))
	}

	return table.Join(cells)
}
