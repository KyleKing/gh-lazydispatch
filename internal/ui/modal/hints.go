package modal

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/kyleking/gh-lazydispatch/internal/ui"
)

// hintSeparator is the gap between two key hints on the same line.
const hintSeparator = "  "

// renderHints lays out a modal's key hints, wrapping onto as many lines as
// width allows. Stack.Render clips rather than wraps, so a single long line of
// hints loses its tail silently, and the tail is where "[esc] cancel" sits.
//
// A width of zero or less means the modal was never sized, so the hints go out
// on one line.
func renderHints(width int, hints ...string) string {
	if len(hints) == 0 {
		return ""
	}

	if width <= 0 {
		return ui.HelpStyle.Render(strings.Join(hints, hintSeparator))
	}

	var (
		lines []string
		line  string
	)

	for _, hint := range hints {
		switch {
		case line == "":
			line = hint
		case ansi.StringWidth(line+hintSeparator+hint) <= width:
			line += hintSeparator + hint
		default:
			lines = append(lines, line)
			line = hint
		}
	}

	return ui.HelpStyle.Render(strings.Join(append(lines, line), "\n"))
}
