package theme

import "github.com/charmbracelet/lipgloss"

// Theme defines semantic color roles for the UI.
type Theme struct {
	Primary   lipgloss.Color // Mauve - titles, focused borders
	Secondary lipgloss.Color // Surface2 - unfocused borders
	Accent    lipgloss.Color // Teal - selected items
	Muted     lipgloss.Color // Overlay2 - subtitles, help text
	SoftMuted lipgloss.Color // Overlay1 - default values, less critical info
	Text      lipgloss.Color // Text - normal text
	ModalBg   lipgloss.Color // Mantle - modal background
}

// Latte returns the Catppuccin Latte (light) theme.
func Latte() Theme {
	return Theme{
		Primary:   lipgloss.Color("#8839ef"), // Mauve
		Secondary: lipgloss.Color("#acb0be"), // Surface2
		Accent:    lipgloss.Color("#179299"), // Teal
		Muted:     lipgloss.Color("#7c7f93"), // Overlay2
		SoftMuted: lipgloss.Color("#8c8fa1"), // Overlay1
		Text:      lipgloss.Color("#4c4f69"), // Text
		ModalBg:   lipgloss.Color("#e6e9ef"), // Mantle
	}
}

// Macchiato returns the Catppuccin Macchiato (medium-dark) theme.
func Macchiato() Theme {
	return Theme{
		Primary:   lipgloss.Color("#c6a0f6"), // Mauve
		Secondary: lipgloss.Color("#5b6078"), // Surface2
		Accent:    lipgloss.Color("#8bd5ca"), // Teal
		Muted:     lipgloss.Color("#939ab7"), // Overlay2
		SoftMuted: lipgloss.Color("#a5adcb"), // Overlay1
		Text:      lipgloss.Color("#cad3f5"), // Text
		ModalBg:   lipgloss.Color("#1e2030"), // Mantle
	}
}
