package modal

import (
	"image/color"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// unpaintedCell reports the first cell of line drawn with no background, or -1
// where every cell carries one. It reads the SGR state the way a terminal
// does, because a background the frame sets is lost at the next reset rather
// than at the end of the line.
func unpaintedCell(line string) int {
	painted, cell := false, 0

	for i := 0; i < len(line); i++ {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			end := strings.IndexByte(line[i:], 'm')
			if end < 0 {
				break
			}

			params := line[i+2 : i+end]
			switch {
			case params == "" || params == "0":
				painted = false
			case strings.Contains(params, "48;") || strings.HasPrefix(params, "4"):
				painted = true
			}

			i += end

			continue
		}

		if !painted {
			return cell
		}

		cell++
	}

	return -1
}

// A modal draws on one ground: the frame's background has to reach the cells
// past a styled segment and the cells a short line pads with, both of which a
// segment's own SGR reset clears.
func TestGround_PaintsEveryCellAModalDraws(t *testing.T) {
	t.Parallel()

	red := lipgloss.NewStyle().Foreground(color.RGBA{R: 0xff, A: 0xff})
	ownGround := lipgloss.NewStyle().Background(color.RGBA{B: 0xff, A: 0xff})

	cases := map[string]string{
		"a styled segment followed by nothing": red.Render("Title"),
		"a styled segment then plain text":     red.Render("Chain: ") + "deploy-prod",
		"lines of differing width": strings.Join([]string{
			red.Render("Steps:"),
			"  1. docker-build.yml",
			red.Render("  2. deploy-prod.yml ") + "(wait: none)",
		}, "\n"),
		"a segment carrying its own ground": ownGround.Render(" tab ") + red.Render(" other "),
	}

	for name, content := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			widths := map[int]bool{}

			for i, line := range strings.Split(ground(content, color.RGBA{R: 0x1e, G: 0x20, B: 0x2f, A: 0xff}), "\n") {
				if cell := unpaintedCell(line); cell >= 0 {
					t.Errorf("row %d draws cell %d on the terminal's ground rather than the modal's", i, cell)
				}

				widths[lipgloss.Width(line)] = true
			}

			if len(widths) > 1 {
				t.Errorf("rows carry %d widths, so the frame pads the short ones itself: %v", len(widths), widths)
			}
		})
	}
}
