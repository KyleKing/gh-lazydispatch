package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Theme defines semantic color roles for the UI.
type Theme struct {
	Primary   color.Color // Mauve - titles, focused borders
	Secondary color.Color // Surface2 - unfocused borders
	Accent    color.Color // Teal - selected items
	Muted     color.Color // Overlay2 - subtitles, help text
	SoftMuted color.Color // Overlay1 - default values, less critical info
	Text      color.Color // Text - normal text
	ModalBg   color.Color // Mantle - modal background
	Error     color.Color // Red - error messages
	Link      color.Color // Blue - URLs and links
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
		Error:     lipgloss.Color("#d20f39"), // Red
		Link:      lipgloss.Color("#1e66f5"), // Blue
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
		Error:     lipgloss.Color("#ed8796"), // Red
		Link:      lipgloss.Color("#8aadf4"), // Blue
	}
}
