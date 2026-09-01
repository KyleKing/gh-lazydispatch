package app

// paneLayout is the row and column budget the panes draw into.
//
// View and the resize handler both take it from layoutFor. Computing the split
// twice is what left the right panel scrolling against a height one row taller
// than the one it rendered in.
type paneLayout struct {
	leftWidth      int
	rightWidth     int
	rightHeight    int
	workflowHeight int
	chainsHeight   int
	chainsHintRows int
	configHeight   int
}

// configPaneChrome is what the config pane spends on its border, title, branch
// line, the input table's header, and the command preview. Only the input rows
// grow past it.
const configPaneChrome = 10

// configPaneEmptyHeight is the pane with no workflow selected: a border, a
// title, and one line of prompt.
const configPaneEmptyHeight = 5

// chainsPaneChrome is the chains pane's border, title, and table header.
const chainsPaneChrome = 4

// chainsPaneMaxRows caps how much of the left column the chains list may take
// from the workflow list, which is the one a reader drives with.
const chainsPaneMaxRows = 6

// chainsHintHeight is the line standing in for the chains pane in a repository
// that configures none, so the feature is named somewhere.
const chainsHintHeight = 1

// minWorkflowPaneHeight keeps a border, a title, the "all workflows" row, and
// two workflows above whatever the panes below it need.
const minWorkflowPaneHeight = 6

// layoutFor divides the terminal into a left column of stacked panes and a
// full-height right panel.
//
// The left column is sized from the bottom up: the config pane takes what its
// content needs rather than half the screen, the chains pane takes its rows or
// gives way to a one-line hint when there are none, and the workflow list keeps
// the rest. When
// they do not all fit, the chains pane gives ground before the config pane,
// because the config pane holds the command about to be dispatched.
func layoutFor(width, height, wantConfig, wantChains int, wantHint bool) paneLayout {
	left := leftColumnWidth(width)
	available := height - viewsFixedChromeHeight

	config := max(wantConfig, configPaneEmptyHeight)
	chains := wantChains

	hint := 0
	if wantHint {
		hint = chainsHintHeight
	}

	available -= hint

	if over := config + chains + minWorkflowPaneHeight - available; over > 0 {
		chains = max(chains-over, 0)
	}

	// A chains pane too short to hold a single row is chrome with nothing in
	// it, so it goes entirely rather than down to its border.
	if chains < chainsPaneChrome+1 {
		chains = 0
	}

	if over := config + chains + minWorkflowPaneHeight - available; over > 0 {
		config = max(config-over, configPaneEmptyHeight)
	}

	return paneLayout{
		leftWidth:      left,
		rightWidth:     width - left,
		rightHeight:    available + hint,
		workflowHeight: available - config - chains,
		chainsHeight:   chains,
		chainsHintRows: hint,
		configHeight:   config,
	}
}

// leftColumnWidth sizes the left column as a fraction of the terminal, floored
// so a workflow name stays readable and capped so a wide terminal spends its
// extra cells on the runs rather than on whitespace beside short names.
func leftColumnWidth(width int) int {
	left := (width * leftPaneWidthNumerator) / leftPaneWidthDenominator

	return min(max(left, leftPaneMinWidth), leftPaneMaxWidth)
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

// chainsPaneHeight is how many rows the chains pane wants: none without chains,
// and none while a detail holds the top-left pane, which needs the column more.
func (m Model) chainsPaneHeight() int {
	n := m.chains.Count()
	if n == 0 || m.viewMode != WorkflowListMode {
		return 0
	}

	return chainsPaneChrome + min(n, chainsPaneMaxRows)
}

// layout resolves the current terminal size against what the stacked panes need.
func (m Model) layout() paneLayout {
	return layoutFor(m.width, m.height, m.configPaneHeight(), m.chainsPaneHeight(), m.chains.Count() == 0)
}
