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

// chainColumns describes the chain table. Description is what a narrow pane
// drops first, then the variable count.
func chainColumns() []table.Column {
	return []table.Column{
		{Key: ui.ColKeyName, Title: ui.ColTitleName, Min: ui.ColMinName, Weight: ui.WeightMid},
		{
			Key: "steps", Title: "Steps", Min: ui.ColMaxSteps, Max: ui.ColMaxSteps,
			Align: table.AlignRight, Priority: ui.PrioThirdToGo,
		},
		{
			Key: "vars", Title: "Vars", Min: ui.ColMinCount, Max: ui.ColMaxCount,
			Align: table.AlignRight, Priority: ui.PrioSecondToGo,
		},
		{Key: "desc", Title: "Description", Min: ui.ColMinName, Weight: ui.WeightHigh, Priority: ui.PrioFirstToGo},
	}
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

// ViewContent renders the chain list content without the pane border.
func (m ChainListModel) ViewContent() string {
	if len(m.chainNames) == 0 {
		var content strings.Builder

		content.WriteString(ui.SubtitleStyle.Render("No chains configured"))
		content.WriteString("\n\n")
		content.WriteString(ui.NormalStyle.Render("Chains let you run multiple"))
		content.WriteString("\n")
		content.WriteString(ui.NormalStyle.Render("workflows in sequence."))
		content.WriteString("\n\n")
		content.WriteString(ui.HelpStyle.Render("Create .github/lazydispatch.yml:"))
		content.WriteString("\n\n")
		content.WriteString(ui.CLIPreviewStyle.Render("  chains:"))
		content.WriteString("\n")
		content.WriteString(ui.CLIPreviewStyle.Render("    deploy:"))
		content.WriteString("\n")
		content.WriteString(ui.CLIPreviewStyle.Render("      steps:"))
		content.WriteString("\n")
		content.WriteString(ui.CLIPreviewStyle.Render("        - workflow: build.yml"))
		content.WriteString("\n")
		content.WriteString(ui.CLIPreviewStyle.Render("        - workflow: deploy.yml"))

		return content.String()
	}

	var content strings.Builder

	layout := ui.FitColumns(chainColumns(), m.width, ui.RowGutterWidth)

	content.WriteString(ui.TableHeader(layout, ui.RowGutterWidth))
	content.WriteString("\n")

	for i, name := range m.chainNames {
		chain := m.chains[name]
		stepCount := len(chain.Steps)
		varCount := len(chain.Variables)

		desc := chain.Description
		if desc == "" {
			desc = "(no description)"
		}

		indicator := "  "
		if i == m.selectedIndex {
			indicator = "> "
		}

		cells := ui.TableRow(layout, map[string]string{
			ui.ColKeyName: name,
			"steps":       strconv.Itoa(stepCount),
			"vars":        strconv.Itoa(varCount),
			"desc":        desc,
		})

		rowStyle := ui.TableRowStyle
		if i == m.selectedIndex {
			rowStyle = ui.TableSelectedStyle
		}

		content.WriteString(rowStyle.Render(indicator + cells))

		if i < len(m.chainNames)-1 {
			content.WriteString("\n")
		}
	}

	return content.String()
}

// View renders the chain list pane with border.
func (m ChainListModel) View() string {
	style := ui.PaneStyle(m.width, m.height, m.focused)
	title := ui.TitleStyle.Render("Chains")

	return style.Render(title + "\n" + m.ViewContent())
}
