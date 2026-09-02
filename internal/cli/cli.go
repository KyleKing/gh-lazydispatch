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

const usage = `gh-lazydispatch export - read GitHub Actions without reading its logs
gh-lazydispatch watch   - wait out a run and keep its failure digest local

Usage:
  gh-lazydispatch export workflows           Dispatchable workflows and their inputs
  gh-lazydispatch export chains              Chains defined in .github/lazydispatch.yml
  gh-lazydispatch export runs [flags]        Recent workflow runs, newest first
  gh-lazydispatch export logs <run-id>       One run's logs, parsed into steps
  gh-lazydispatch export diagnose <run-id>   Why a run failed, without its logs
  gh-lazydispatch watch <run-id> [flags]     Poll a run to completion and write its digest

Flags (runs):
  --workflow <file>   Filename, e.g. ci.yml
  --branch <name>
  --status <status>   queued|in_progress|completed|success|failure
  --limit <n>         Default 20
  --current           One row per workflow: the branch's state rather than its
                      history. Needs --branch, and ignores --status and --limit

Flags (logs):
  --errors-only       Keep only lines that read as errors
  --step <n>          One step, by index
  --grep <pattern>    Keep lines matching a regular expression
  --limit <n>         Cap lines per step
  --format json|md    Default json

Flags (watch):
  --interval <secs>   Seconds between polls. Default 15
  --out <path>        Where to write the digest. Default ./lazydispatch-run-<id>.md
  --fix               On failure, hand the digest to an interactive Claude Code
                      session to investigate. It may commit locally; it never pushes
  --fix-cmd <cmd>      The command --fix spawns. Default "claude"

export writes JSON to stdout and counts to stderr; nothing it does dispatches.
watch writes a markdown digest to disk and blocks until the run finishes.`

// IsCommand reports whether args name a CLI command rather than the TUI, so
// main can keep launching the TUI when it is given no command.
func IsCommand(args []string) bool {
	return len(args) > 0 && (args[0] == "export" || args[0] == "watch")
}

// Run executes the command named by args (without the program name) and
// returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	if !IsCommand(args) {
		notef(stderr, usage+"\n")

		return exitUsage
	}

	var err error

	switch args[0] {
	case "export":
		err = runExport(args[1:], stdout, stderr)
	case "watch":
		err = runWatch(args[1:], stdout, stderr)
	}

	switch {
	case err == nil:
		return exitOK
	case errors.Is(err, ErrUsage):
		notef(stderr, "gh-lazydispatch: %v\n\n%s\n", err, usage)

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
