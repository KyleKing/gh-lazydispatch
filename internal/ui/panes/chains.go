// Package panes provides the main UI panes for workflow, history, and configuration views.
package panes

import (
	"sort"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/tui/table"

	"github.com/kyleking/gh-lazydispatch/internal/config"
	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// Chain table widths: the narrowest each column set fits in, past the row
// gutter. A set is chosen rather than dropped by priority because a dropped
// column leaves an overflow marker in the header, and that marker is what wraps
// a header one cell too wide onto a second line, costing the pane the row its
// bottom border sits on.
const (
	chainColumnsFull = ui.ColMinName + ui.ColMaxSteps + ui.ColMinCount + 2*ui.RowGutterWidth
	chainColumnsTwo  = ui.ColMinName + ui.ColMaxSteps + ui.RowGutterWidth
)

// chainColumnsFor is the chain table narrowed to what width holds.
func chainColumnsFor(width int) []table.Column {
	name := table.Column{
		Key: ui.ColKeyName, Title: ui.ColTitleName,
		Min: ui.ColMinName, Max: ui.ColMaxName, Weight: ui.WeightHigh,
	}
	steps := table.Column{
		Key: "steps", Title: "Steps", Min: ui.ColMaxSteps, Max: ui.ColMaxSteps, Align: table.AlignRight,
	}
	vars := table.Column{Key: "vars", Title: "Vars", Min: ui.ColMinCount, Max: ui.ColMaxCount, Align: table.AlignRight}

	switch {
	case width >= chainColumnsFull:
		return []table.Column{name, steps, vars}
	case width >= chainColumnsTwo:
		return []table.Column{name, steps}
	}

	return []table.Column{name}
}

// ChainListModel manages the chain list display.
type ChainListModel struct {
	chains        map[string]config.Chain
	chainNames    []string
	selectedIndex int
	width         int
	height        int
	focused       bool
}

// NewChainListModel creates a new chain list model.
func NewChainListModel() ChainListModel {
	return ChainListModel{selectedIndex: 0}
}

// SetChains updates the chain definitions.
func (m *ChainListModel) SetChains(chains map[string]config.Chain) {
	m.chains = chains
	m.chainNames = make([]string, 0, len(chains))

	for name := range chains {
		m.chainNames = append(m.chainNames, name)
	}

	sort.Strings(m.chainNames)
}

// Count reports how many chains the repository configures.
func (m ChainListModel) Count() int {
	return len(m.chainNames)
}

// SetSize updates the pane dimensions.
func (m *ChainListModel) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// SetFocused updates the focus state.
func (m *ChainListModel) SetFocused(focused bool) {
	m.focused = focused
}

// MoveUp moves selection up.
func (m *ChainListModel) MoveUp() {
	if m.selectedIndex > 0 {
		m.selectedIndex--
	}
}

// MoveDown moves selection down.
func (m *ChainListModel) MoveDown() {
	if m.selectedIndex < len(m.chainNames)-1 {
		m.selectedIndex++
	}
}

// SelectedChain returns the currently selected chain.
//
//nolint:unparam // the definition is read from the app package, outside unparam's view
func (m ChainListModel) SelectedChain() (string, config.Chain, bool) {
	if len(m.chainNames) == 0 {
		return "", config.Chain{}, false
	}

	name := m.chainNames[m.selectedIndex]

	return name, m.chains[name], true
}

// Update handles messages for the chain list.
func (m ChainListModel) Update(_ tea.Msg) (ChainListModel, tea.Cmd) {
	return m, nil
}

// chainsPaneChrome is the border, title, and table header the pane spends
// before its first row, which is what the scroll window is measured against.
const chainsPaneChrome = 4

// window is the range of chains the pane has room to draw.
func (m ChainListModel) window() (int, int) {
	return ui.ScrollWindow(m.selectedIndex, len(m.chainNames), m.height-chainsPaneChrome)
}

// ViewContent renders the chain list content without the pane border.
func (m ChainListModel) ViewContent() string {
	var content strings.Builder

	// SetSize is given the pane's outer width, unlike the panes inside the
	// tabbed panel, which are handed a width the border is already out of.
	room := m.width - ui.PaneBorderSize
	layout := ui.FitColumns(chainColumnsFor(room-ui.RowGutterWidth), room, ui.RowGutterWidth)

	first, last := m.window()

	content.WriteString(ui.TableHeader(layout, ui.RowGutterWidth))

	for i := first; i < last; i++ {
		name := m.chainNames[i]
		chain := m.chains[name]

		indicator := "  "
		if i == m.selectedIndex {
			indicator = "> "
		}

		cells := ui.TableRow(layout, map[string]string{
			ui.ColKeyName: name,
			"steps":       strconv.Itoa(len(chain.Steps)),
			"vars":        strconv.Itoa(len(chain.Variables)),
		})

		rowStyle := ui.TableRowStyle
		if i == m.selectedIndex {
			rowStyle = ui.TableSelectedStyle
		}

		content.WriteString("\n")
		content.WriteString(rowStyle.Render(indicator + cells))
	}

	return content.String()
}

// View renders the chain list pane with border.
func (m ChainListModel) View() string {
	style := ui.PaneStyle(m.width, m.height, m.focused)
	first, last := m.window()
	title := ui.TitleStyle.Render("Chains") +
		ui.RenderScrollIndicator(last < len(m.chainNames), first > 0)

	return style.Render(title + "\n" + m.ViewContent())
}
