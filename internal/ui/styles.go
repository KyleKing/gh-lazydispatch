package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-wfd/internal/ui/theme"
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
)

// Styles for the application (initialized in ApplyTheme).
var (
	BorderStyle        lipgloss.Style
	CLIPreviewStyle    lipgloss.Style
	FocusedBorderStyle lipgloss.Style
	HelpStyle          lipgloss.Style
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

	BorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(SecondaryColor)

	CLIPreviewStyle = lipgloss.NewStyle().
		Foreground(MutedColor).
		Italic(true)

	FocusedBorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor)

	HelpStyle = lipgloss.NewStyle().
		Foreground(SoftMutedColor)

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
