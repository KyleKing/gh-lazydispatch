package app

// paneLayout is the row and column budget the panes draw into.
//
// View and the resize handler both take it from layoutFor. Computing the split
// twice is what left the right panel scrolling against a height one row taller
// than the one it rendered in.
type paneLayout struct {
	leftWidth    int
	rightWidth   int
	topHeight    int
	configHeight int
}

// configPaneChrome is what the config pane spends on its border, title, branch
// line, the input table's header, and the command preview. Only the input rows
// grow past it.
const configPaneChrome = 10

// configPaneEmptyHeight is the pane with no workflow selected: a border, a
// title, and one line of prompt.
const configPaneEmptyHeight = 5

// minTopPaneHeight keeps a border, a title, a column header, and three rows
// above the config pane however many inputs the config pane has.
const minTopPaneHeight = 7

// layoutFor divides the terminal between the top panes and the config pane.
//
// The config pane takes what its content needs rather than half the screen, so
// a workflow with no inputs spends nothing on an empty table and one with twenty
// does not scroll them through five rows.
func layoutFor(width, height, wantConfig int) paneLayout {
	left := (width * leftPaneWidthNumerator) / leftPaneWidthDenominator
	available := height - viewsFixedChromeHeight

	config := wantConfig
	if ceiling := available - minTopPaneHeight; config > ceiling {
		config = ceiling
	}

	if config < configPaneEmptyHeight {
		config = configPaneEmptyHeight
	}

	return paneLayout{
		leftWidth:    left,
		rightWidth:   width - left,
		topHeight:    available - config,
		configHeight: config,
	}
}

// configPaneHeight is how many rows the config pane wants.
func (m Model) configPaneHeight() int {
	if m.selectedWorkflow < 0 || m.selectedWorkflow >= len(m.workflows) {
		return configPaneEmptyHeight
	}

	// An empty table still costs the row its absence is drawn in, so a workflow
	// with no inputs asks for the same height as one with a single input.
	rows := max(len(m.filteredInputs), 1)
	if m.filterText != "" {
		rows++
	}

	return configPaneChrome + rows
}

// layout resolves the current terminal size against what the config pane needs.
func (m Model) layout() paneLayout {
	return layoutFor(m.width, m.height, m.configPaneHeight())
}
