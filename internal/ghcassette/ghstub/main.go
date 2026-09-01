// Command ghstub stands in for the gh binary while tests run. It is placed on
// PATH ahead of the real gh, so every gh call gh-lazydispatch makes goes
// through it: the run and job reads, the log downloads, and a dispatch alike.
//
// In record mode it runs the real gh and appends what happened to the
// cassette. In replay mode it answers from the cassette and fails loudly on a
// call nobody recorded, because a silent empty response would look like a run
// that produced no logs.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/ghcassette"
)

// exitHarness is returned for a failure of the stub itself, kept clear of the
// exit codes gh uses so a broken cassette is never read as a gh error.
const (
	exitHarness = 97
	journalPerm = 0o600
)

var (
	errNoCassette = errors.New("GH_CASSETTE is unset")
	errNoRealGH   = errors.New("GH_CASSETTE_REAL is unset, so record mode has no gh to run")
)

func main() {
	code, err := run(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ghstub:", err)
		os.Exit(exitHarness)
	}

	os.Exit(code)
}

func run(args []string) (int, error) {
	path := os.Getenv("GH_CASSETTE")
	if path == "" {
		return 0, errNoCassette
	}

	if os.Getenv("GH_CASSETTE_MODE") == "record" {
		return record(path, args)
	}

	return replay(path, args)
}

func replay(path string, args []string) (int, error) {
	c, err := ghcassette.Load(path)
	if err != nil {
		return 0, fmt.Errorf("loading the cassette: %w", err)
	}

	journal := os.Getenv("GH_CASSETTE_JOURNAL")

	cursor, err := nextIndex(journal)
	if err != nil {
		return 0, err
	}

	i, err := c.Match(cursor, args)
	if err != nil {
		return 0, fmt.Errorf("matching the call: %w", err)
	}

	if err := appendLine(journal, strconv.Itoa(i)); err != nil {
		return 0, err
	}

	it := c.Interactions[i]
	if _, err := os.Stdout.WriteString(it.Stdout); err != nil {
		return 0, fmt.Errorf("writing stdout: %w", err)
	}

	if _, err := os.Stderr.WriteString(it.Stderr); err != nil {
		return 0, fmt.Errorf("writing stderr: %w", err)
	}

	return it.Exit, nil
}

func record(path string, args []string) (int, error) {
	ghPath := os.Getenv("GH_CASSETTE_REAL")
	if ghPath == "" {
		return 0, errNoRealGH
	}

	//nolint:gosec,noctx // the arguments are whatever the program under test passed to gh
	cmd := exec.Command(ghPath, args...)

	var out, errOut strings.Builder

	cmd.Stdout = &out
	cmd.Stderr = &errOut

	code := 0

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) {
			return 0, fmt.Errorf("running %s: %w", ghPath, err)
		}

		code = exitErr.ExitCode()
	}

	if _, err := os.Stdout.WriteString(out.String()); err != nil {
		return 0, fmt.Errorf("writing stdout: %w", err)
	}

	if _, err := os.Stderr.WriteString(errOut.String()); err != nil {
		return 0, fmt.Errorf("writing stderr: %w", err)
	}

	c, err := ghcassette.Load(path)
	if err != nil {
		c = &ghcassette.Cassette{}
	}

	c.Interactions = append(c.Interactions, ghcassette.Interaction{
		Args:   args,
		Stdout: out.String(),
		Stderr: errOut.String(),
		Exit:   code,
	})

	if err := ghcassette.Save(path, c); err != nil {
		return 0, fmt.Errorf("saving the cassette: %w", err)
	}

	return code, nil
}

// nextIndex reads the journal of already-replayed interactions and returns the
// index replay resumes from.
func nextIndex(journal string) (int, error) {
	if journal == "" {
		return 0, nil
	}

	raw, err := os.ReadFile(journal) // #nosec G304,G703 -- a path the harness wrote
	if err != nil {
		return 0, nil //nolint:nilerr // an absent journal means nothing has replayed yet
	}

	lines := strings.Fields(string(raw))
	if len(lines) == 0 {
		return 0, nil
	}

	last, err := strconv.Atoi(lines[len(lines)-1])
	if err != nil {
		return 0, fmt.Errorf("reading journal %s: %w", journal, err)
	}

	return last + 1, nil
}

func appendLine(path, s string) error {
	if path == "" {
		return nil
	}

	// #nosec G304,G703 -- a path the harness wrote
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, journalPerm)
	if err != nil {
		return fmt.Errorf("opening journal %s: %w", path, err)
	}

	if _, err := fmt.Fprintln(f, s); err != nil {
		_ = f.Close() //nolint:errcheck // the write failure is the one worth reporting

		return fmt.Errorf("writing journal %s: %w", path, err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("closing journal %s: %w", path, err)
	}

	return nil
}
