// Package ui provides shared UI styling and components for the TUI application.
package ui

import (
	"image/color"
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/kyleking/aragonite/tui/theme"
	"github.com/sahilm/fuzzy"
)

var currentPalette theme.Palette

// A zero-value lipgloss.Style draws no border and no color, so a consumer that
// never applies a theme renders a pane invisibly different from a real one.
// Every style is built here first, and InitTheme replaces them.
//
//nolint:gochecknoinits // styles are package state, and a zero one renders wrong rather than failing
func init() {
	InitTheme(theme.Macchiato())
}

// Colors used throughout the UI.
var (
	PrimaryColor   color.Color
	SecondaryColor color.Color
	AccentColor    color.Color
	MutedColor     color.Color
	SoftMutedColor color.Color
	TextColor      color.Color
	ModalBgColor   color.Color
	ErrorColor     color.Color
	LinkColor      color.Color
)

// Styles for the application (initialized in ApplyTheme).
var (
	BorderStyle        lipgloss.Style
	CLIPreviewStyle    lipgloss.Style
	ErrorStyle         lipgloss.Style
	ErrorTitleStyle    lipgloss.Style
	FocusedBorderStyle lipgloss.Style
	HelpStyle          lipgloss.Style
	LinkStyle          lipgloss.Style
	NormalStyle        lipgloss.Style
	SelectedStyle      lipgloss.Style
	SubtitleStyle      lipgloss.Style
	TabActiveStyle     lipgloss.Style
	TabInactiveStyle   lipgloss.Style
	TableDefaultStyle  lipgloss.Style
	TableDimmedStyle   lipgloss.Style
	TableHeaderStyle   lipgloss.Style
	TableItalicStyle   lipgloss.Style
	TableRowStyle      lipgloss.Style
	TableSelectedStyle lipgloss.Style
	TitleStyle         lipgloss.Style
)

// InitTheme sets the palette and applies colors.
func InitTheme(p theme.Palette) {
	currentPalette = p

	ApplyTheme()
}

// ApplyTheme updates all colors and styles from the current palette. The three
// roles beyond theme.Semantic are named here because only this application
// needs them: a softer muted for defaults, a modal ground, and a link color.
func ApplyTheme() {
	semantic := currentPalette.Semantic()

	PrimaryColor = semantic.Primary
	SecondaryColor = semantic.Secondary
	AccentColor = semantic.Accent
	MutedColor = semantic.Muted
	SoftMutedColor = currentPalette.Overlay1
	TextColor = semantic.Text
	ModalBgColor = currentPalette.Mantle
	ErrorColor = semantic.Error
	LinkColor = currentPalette.Blue

	BorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(SecondaryColor)

	CLIPreviewStyle = lipgloss.NewStyle().
		Foreground(MutedColor).
		Italic(true)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(ErrorColor)

	ErrorTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ErrorColor)

	// The focused pane carries a heavier border as well as a brighter one, so
	// focus survives NO_COLOR and a monochrome terminal.
	FocusedBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.ThickBorder()).
		BorderForeground(PrimaryColor)

	HelpStyle = lipgloss.NewStyle().
		Foreground(SoftMutedColor)

	LinkStyle = lipgloss.NewStyle().
		Foreground(LinkColor).
		Underline(true)

	NormalStyle = lipgloss.NewStyle().
		Foreground(TextColor)

	SelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(AccentColor)

	SubtitleStyle = lipgloss.NewStyle().
		Foreground(SoftMutedColor)

	// The active tab is a filled segment rather than a bracketed name: every
	// other bracket in this UI is a key you can press, and one that is not
	// reads as a broken hint.
	TabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ModalBgColor).
		Background(PrimaryColor).
		Padding(0, 1)

	TabInactiveStyle = lipgloss.NewStyle().
		Foreground(MutedColor).
		Padding(0, 1)

	TableDefaultStyle = lipgloss.NewStyle().
		Foreground(SoftMutedColor)

	TableDimmedStyle = lipgloss.NewStyle().
		Foreground(MutedColor)

	TableHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(SecondaryColor)

	TableItalicStyle = lipgloss.NewStyle().
		Italic(true).
		Foreground(MutedColor)

	TableRowStyle = lipgloss.NewStyle().
		Foreground(TextColor)

	TableSelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(AccentColor)

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor)
}

// PaneBorderSize is the space a pane's border occupies on each axis, which is
// what separates the width a pane is given from the width its content gets.
const PaneBorderSize = 2

// PaneBox draws content inside a pane of exactly width by height cells.
// Content is truncated rather than wrapped, because a wrapped line costs the
// pane the bottom border MaxHeight then clips in its place.
func PaneBox(width, height int, focused bool, content string) string {
	style := BorderStyle
	if focused {
		style = FocusedBorderStyle
	}

	inner := max(width-PaneBorderSize, 0)
	rows := max(height-PaneBorderSize, 0)

	lines := strings.Split(content, "\n")
	if len(lines) > rows {
		lines = lines[:rows]
	}

	for i, line := range lines {
		lines[i] = ansi.Truncate(line, inner, "…")
	}

	return style.Width(width).Height(height).MaxHeight(height).Render(strings.Join(lines, "\n"))
}

// FormatEmptyValue returns the display string for a value, showing ("") for empty strings.
func FormatEmptyValue(val string) string {
	if val == "" {
		return `("")`
	}

	return val
}

// RenderEmptyValue returns a styled string for a value, using italic style for empty strings.
func RenderEmptyValue(val string) string {
	if val == "" {
		return TableItalicStyle.Render(`("")`)
	}

	return NormalStyle.Render(val)
}

// ApplyFuzzyFilter returns items filtered by query using fuzzy matching.
// Returns original items if query is empty.
func ApplyFuzzyFilter(query string, items []string) []string {
	if query == "" {
		return items
	}

	matches := fuzzy.Find(query, items)
	results := make([]string, len(matches))

	for i, match := range matches {
		results[i] = match.Str
	}

	return results
}

// RemoveListBackgrounds removes all backgrounds from a list.Model for modal overlay.
func RemoveListBackgrounds(l list.Model) list.Model {
	l.Styles.Title = l.Styles.Title.UnsetBackground()
	l.Styles.HelpStyle = l.Styles.HelpStyle.UnsetBackground()
	l.Styles.TitleBar = l.Styles.TitleBar.UnsetBackground()
	l.Styles.Spinner = l.Styles.Spinner.UnsetBackground()
	l.Styles.Filter.Focused.Prompt = l.Styles.Filter.Focused.Prompt.UnsetBackground()
	l.Styles.Filter.Blurred.Prompt = l.Styles.Filter.Blurred.Prompt.UnsetBackground()
	l.Styles.DefaultFilterCharacterMatch = l.Styles.DefaultFilterCharacterMatch.UnsetBackground()
	l.Styles.StatusBar = l.Styles.StatusBar.UnsetBackground()
	l.Styles.StatusEmpty = l.Styles.StatusEmpty.UnsetBackground()
	l.Styles.StatusBarActiveFilter = l.Styles.StatusBarActiveFilter.UnsetBackground()
	l.Styles.StatusBarFilterCount = l.Styles.StatusBarFilterCount.UnsetBackground()
	l.Styles.NoItems = l.Styles.NoItems.UnsetBackground()
	l.Styles.PaginationStyle = l.Styles.PaginationStyle.UnsetBackground()
	l.Styles.ActivePaginationDot = l.Styles.ActivePaginationDot.UnsetBackground()
	l.Styles.InactivePaginationDot = l.Styles.InactivePaginationDot.UnsetBackground()
	l.Styles.ArabicPagination = l.Styles.ArabicPagination.UnsetBackground()
	l.Styles.DividerDot = l.Styles.DividerDot.UnsetBackground()

	return l
}

// RenderScrollIndicator renders scroll arrows (^ and v) for lists.
func RenderScrollIndicator(hasMore, hasLess bool) string {
	indicator := ""
	if hasLess {
		indicator += "^"
	} else {
		indicator += " "
	}

	indicator += " "
	if hasMore {
		indicator += "v"
	}

	return SubtitleStyle.Render(indicator)
}
