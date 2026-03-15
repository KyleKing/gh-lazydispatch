// Package theme provides color theme detection and management for the TUI.
package theme

import (
	"os"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// Detect returns the appropriate theme based on terminal background and environment variables.
func Detect() Theme {
	if env := os.Getenv("CATPPUCCIN_THEME"); env != "" {
		switch strings.ToLower(env) {
		case "latte", "light":
			return Latte()
		case "macchiato", "dark":
			return Macchiato()
		}
	}

	if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
		return Macchiato()
	}

	return Latte()
}
