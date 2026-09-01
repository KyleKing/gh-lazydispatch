// Package cli implements gh-lazydispatch's non-interactive commands. Every
// command here reads; nothing dispatches. The TUI and these commands sit on
// the same internal packages, so an agent and a human see the same parse of
// the same workflow run.
package cli

import (
	"errors"
	"fmt"
	"io"
)

// ErrUsage is returned when arguments do not name a command this package
// implements. Callers print usage and exit non-zero.
var ErrUsage = errors.New("usage")

// Process exit codes.
const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// IsCommand reports whether args name a CLI command rather than the TUI, so
// main can keep launching the TUI when it is given no command.
func IsCommand(args []string) bool {
	return len(args) > 0 && args[0] == "export"
}

// Run executes the command named by args (without the program name) and
// returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if !IsCommand(args) {
		notef(stderr, exportUsage+"\n")

		return exitUsage
	}

	err := runExport(args[1:], stdout, stderr)

	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, ErrUsage):
		notef(stderr, "gh-lazydispatch: %v\n\n%s\n", err, exportUsage)

		return exitUsage
	default:
		notef(stderr, "gh-lazydispatch: %v\n", err)

		return exitError
	}
}

// notef writes progress and errors to stderr. A caller redirecting stdout to a
// file has nowhere useful to report a broken stderr, so the write is
// deliberately unchecked.
func notef(stderr io.Writer, format string, args ...any) {
	//nolint:errcheck // diagnostics on stderr; nothing to do if they fail
	fmt.Fprintf(stderr, format, args...)
}
