// Package ui provides shared UI styling and components for the TUI application.
package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-lazydispatch/internal/ui/theme"
	"github.com/sahilm/fuzzy"
)

var currentTheme theme.Theme

// Colors used throughout the UI.
var (
	PrimaryColor   lipgloss.Color
	SecondaryColor lipgloss.Color
	AccentColor    lipgloss.Color
	MutedColor     lipgloss.Color
	SoftMutedColor lipgloss.Color
	TextColor      lipgloss.Color
	ModalBgColor   lipgloss.Color
	ErrorColor     lipgloss.Color
	LinkColor      lipgloss.Color
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
	TableDefaultStyle  lipgloss.Style
	TableDimmedStyle   lipgloss.Style
	TableHeaderStyle   lipgloss.Style
	TableItalicStyle   lipgloss.Style
	TableRowStyle      lipgloss.Style
	TableSelectedStyle lipgloss.Style
	TitleStyle         lipgloss.Style
)

// InitTheme sets the theme and applies colors.
func InitTheme(t theme.Theme) {
	currentTheme = t

	ApplyTheme()
}

// ApplyTheme updates all colors and styles from current theme.
func ApplyTheme() {
	PrimaryColor = currentTheme.Primary
	SecondaryColor = currentTheme.Secondary
	AccentColor = currentTheme.Accent
	MutedColor = currentTheme.Muted
	SoftMutedColor = currentTheme.SoftMuted
	TextColor = currentTheme.Text
	ModalBgColor = currentTheme.ModalBg
	ErrorColor = currentTheme.Error
	LinkColor = currentTheme.Link

	BorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(SecondaryColor)

	CLIPreviewStyle = lipgloss.NewStyle().
		Foreground(MutedColor).
		Italic(true)

	ErrorStyle = lipgloss.NewStyle().
		Foreground(ErrorColor)

	ErrorTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(ErrorColor)

	FocusedBorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
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

// PaneStyle returns a style for a pane with optional focus.
func PaneStyle(width, height int, focused bool) lipgloss.Style {
	style := BorderStyle
	if focused {
		style = FocusedBorderStyle
	}

	return style.Width(width - 2).Height(height - 2)
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
	l.Styles.FilterPrompt = l.Styles.FilterPrompt.UnsetBackground()
	l.Styles.FilterCursor = l.Styles.FilterCursor.UnsetBackground()
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

// TruncateWithEllipsis truncates a string to maxLen, adding "..." if truncated.
func TruncateWithEllipsis(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}

	if maxLen <= 3 {
		return s[:maxLen]
	}

	return s[:maxLen-3] + "..."
}

// PadRight pads a string to a specific length with spaces.
func PadRight(s string, length int) string {
	if len(s) >= length {
		return s
	}

	return s + strings.Repeat(" ", length-len(s))
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
