package modal

import (
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// resetAlternate is the reset lipgloss emits where it writes the parameter out
// rather than eliding it, which ansi.ResetStyle does not cover.
const resetAlternate = "\x1b[0m"

// ground paints bg behind every cell of content, including the cells a short
// line pads to reach the width of the widest.
//
// A styled segment ends in an SGR reset, which clears the frame's background
// along with the segment's own colors. So a background set on the frame alone
// survives only as far as the first styled segment of each line, and the rest
// of that line draws on the terminal's ground instead.
func ground(content string, bg color.Color) string {
	set := ansi.Style{}.BackgroundColor(bg).String()
	keep := strings.NewReplacer(
		ansi.ResetStyle, ansi.ResetStyle+set,
		resetAlternate, resetAlternate+set,
	)

	lines := strings.Split(content, "\n")

	widest := 0
	for _, line := range lines {
		widest = max(widest, lipgloss.Width(line))
	}

	for i, line := range lines {
		padded := line + strings.Repeat(" ", widest-lipgloss.Width(line))
		lines[i] = set + keep.Replace(padded) + ansi.ResetStyle
	}

	return strings.Join(lines, "\n")
}
