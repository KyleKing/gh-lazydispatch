package cli

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/listener"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// defaultWatchInterval is how often watch polls a run it has not seen finish.
// Gentler than the TUI's watcher.PollInterval: a foreground command spends a
// human's attention on every line it prints, where the TUI's ticks are silent.
const defaultWatchInterval = 15 * time.Second

// digestFilePerm is the digest's file mode: readable by the user running the
// command, nothing else, matching a file that may end up quoted into a prompt.
const digestFilePerm = 0o600

func runWatch(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("watch")

	var (
		intervalSecs, pr int
		outPath, fixCmd  string
		notifyCmd        string
		fix              bool
	)

	fs.IntVar(&intervalSecs, "interval", int(defaultWatchInterval/time.Second), "seconds between polls")
	fs.StringVar(&outPath, "out", "", "where to write the digest (default: ./lazydispatch-run-<id>.md)")
	fs.IntVar(&pr, "pr", 0, "watch a pull request's checks instead of one run")
	fs.StringVar(&notifyCmd, "notify-cmd", "", "a command run with the outcome as its argument")
	fs.BoolVar(&fix, "fix", false,
		"on failure, hand the digest to an interactive Claude Code session to investigate")
	fs.StringVar(&fixCmd, "fix-cmd", "claude", "the command --fix spawns")

	runID, err := parseWatchTarget(fs, args, &pr)
	if err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	cond := listener.RunDone(client, runID)
	if pr > 0 {
		cond = listener.ChecksSettled(client, pr)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	outcome, err := listener.Wait(ctx, cond, time.Duration(intervalSecs)*time.Second,
		func(line string) { notef(stderr, "%s\n", line) })
	if err != nil {
		return fmt.Errorf("waiting: %w", err)
	}

	notef(stderr, "%s\n", outcome.Summary)

	paths, err := writeDigests(client, digestRuns(runID, outcome), outPath, stderr)
	if err != nil {
		return err
	}

	for _, path := range paths {
		if _, err := fmt.Fprintln(stdout, path); err != nil {
			return fmt.Errorf("writing the digest path: %w", err)
		}
	}

	if err := listener.Notify(ctx, stderr, notifyCmd, outcome); err != nil {
		return fmt.Errorf("notifying: %w", err)
	}

	if !fix || !outcome.Failed || len(paths) == 0 || len(outcome.Runs) == 0 {
		return nil
	}

	run, err := client.GetWorkflowRun(outcome.Runs[0])
	if err != nil {
		return fmt.Errorf("fetching run %d: %w", outcome.Runs[0], err)
	}

	return investigate(fixCmd, paths[0], run)
}

// parseWatchTarget reads the run ID that watch waits on, or none when --pr
// names a pull request instead.
func parseWatchTarget(fs *flag.FlagSet, args []string, pr *int) (int64, error) {
	runID, err := parseRunID(fs, args)
	if err == nil {
		if *pr > 0 {
			return 0, fmt.Errorf("%w: give a run ID or --pr, not both", ErrUsage)
		}

		return runID, nil
	}

	if !errors.Is(err, errNoRunID) {
		return 0, err
	}

	if parseErr := fs.Parse(args); parseErr != nil {
		return 0, fmt.Errorf("%w: %w", ErrUsage, parseErr)
	}

	if *pr <= 0 || fs.NArg() > 0 {
		return 0, err
	}

	return 0, nil
}

// digestRuns names the runs to write a digest for. A watched run always gets one,
// since the digest of a green run is still the record of what it printed; a
// pull request gets one per failing run.
func digestRuns(runID int64, outcome listener.Outcome) []int64 {
	if runID != 0 {
		return []int64{runID}
	}

	return outcome.Runs
}

func writeDigests(client *github.Client, ids []int64, outPath string, stderr io.Writer) ([]string, error) {
	paths := make([]string, 0, len(ids))

	for _, id := range ids {
		path := ""
		if len(ids) == 1 {
			path = outPath
		}

		written, err := writeDigest(client, id, path, stderr)
		if err != nil {
			return nil, err
		}

		paths = append(paths, written)
	}

	return paths, nil
}

// writeDigest fetches runID's logs and writes them, errors-only, as markdown
// to path (or the default ./lazydispatch-run-<id>.md), and reports how much
// of the run that kept.
func writeDigest(client *github.Client, runID int64, path string, stderr io.Writer) (string, error) {
	runLogs, err := fetchRunLogs(client, runID)
	if err != nil {
		return "", err
	}

	cfg := logs.NewFilterConfig()
	cfg.Level = logs.FilterErrors

	doc, err := logs.ExportAsMarkdown(runLogs, cfg)
	if err != nil {
		return "", fmt.Errorf("rendering the digest: %w", err)
	}

	reportKept(stderr, countLines(runLogs), strings.Count(doc, "\n"))

	if path == "" {
		path = fmt.Sprintf("lazydispatch-run-%d.md", runID)
	}

	if err := os.WriteFile(path, []byte(doc), digestFilePerm); err != nil {
		return "", fmt.Errorf("writing the digest to %s: %w", path, err)
	}

	return path, nil
}

// investigate hands the digest to an interactive Claude Code session in the
// foreground, the same way `git commit` hands the terminal to $EDITOR. The
// prompt asks it to fix and commit locally; it never tells it to push, and
// nothing here runs git on the caller's behalf.
func investigate(fixCmd, digestPath string, run *github.WorkflowRun) error {
	prompt := fmt.Sprintf(
		"Run %d (%s) failed on branch %q: %s\n\n"+
			"Its failure digest (errors-only log excerpts and matched failure signatures) is at %s.\n"+
			"Investigate the cause and fix it. If you commit the fix, commit it locally on its own"+
			" branch. Do not push.",
		run.ID, run.Name, run.HeadBranch, run.URL, digestPath,
	)

	//nolint:gosec // fixCmd is an operator-supplied flag, same trust boundary as $EDITOR
	cmd := exec.CommandContext(context.Background(), fixCmd, prompt)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running %s: %w", fixCmd, err)
	}

	return nil
}
