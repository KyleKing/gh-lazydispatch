package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/kyleking/gh-wfr/internal/ui/theme"
)

var currentTheme theme.Theme

// Colors used throughout the UI.
var (
	PrimaryColor   lipgloss.Color
	SecondaryColor lipgloss.Color
	AccentColor    lipgloss.Color
	MutedColor     lipgloss.Color
	TextColor      lipgloss.Color
	ModalBgColor   lipgloss.Color
)

// Styles for the application (initialized in ApplyTheme).
var (
	TitleStyle         lipgloss.Style
	SubtitleStyle      lipgloss.Style
	SelectedStyle      lipgloss.Style
	NormalStyle        lipgloss.Style
	HelpStyle          lipgloss.Style
	BorderStyle        lipgloss.Style
	FocusedBorderStyle lipgloss.Style
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
	TextColor = currentTheme.Text
	ModalBgColor = currentTheme.ModalBg

	TitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(PrimaryColor)

	SubtitleStyle = lipgloss.NewStyle().
		Foreground(MutedColor)

	SelectedStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(AccentColor)

	NormalStyle = lipgloss.NewStyle().
		Foreground(TextColor)

	HelpStyle = lipgloss.NewStyle().
		Foreground(MutedColor)

	BorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(SecondaryColor)

	FocusedBorderStyle = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(PrimaryColor)
}

// PaneStyle returns a style for a pane with optional focus.
func PaneStyle(width, height int, focused bool) lipgloss.Style {
	style := BorderStyle
	if focused {
		style = FocusedBorderStyle
	}
	return style.Width(width - 2).Height(height - 2)
}
