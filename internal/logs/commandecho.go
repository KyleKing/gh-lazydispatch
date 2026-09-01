package logs

import "strings"

// GitHub folds the source of every `run:` step into the top of that step's
// output. A script that mentions a failure is not a run that hit one, so
// anything scanning for meaning skips the fold.
const (
	commandEchoHeader = "##[group]Run "
	groupFooter       = "##[endgroup]"
)

// CommandEcho tracks whether a step's lines are currently inside the command
// echo. Feed it a step's lines in order; a zero value starts outside.
type CommandEcho struct {
	inside bool
}

// Echoed advances over line and reports whether it is script source rather
// than the step's own output.
func (c *CommandEcho) Echoed(line string) bool {
	if c.inside {
		c.inside = !strings.HasPrefix(line, groupFooter)

		return true
	}

	if strings.HasPrefix(line, commandEchoHeader) {
		c.inside = true

		return true
	}

	return false
}
