package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
)

// defaultWatchInterval is how often watch polls a run it has not seen finish.
// Gentler than the TUI's watcher.PollInterval: a foreground command spends a
// human's attention on every line it prints, where the TUI's ticks are silent.
const defaultWatchInterval = 15 * time.Second

// digestFilePerm is the digest's file mode: readable by the user running the
// command, nothing else, matching a file that may end up quoted into a prompt.
const digestFilePerm = 0o600

// RunGetter fetches one workflow run by ID. The subset of *github.Client that
// watch's poll loop needs, so the loop is testable without a live repository.
type RunGetter interface {
	GetWorkflowRun(runID int64) (*github.WorkflowRun, error)
}

func runWatch(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("watch")

	var (
		intervalSecs int
		outPath      string
		fixCmd       string
		fix          bool
	)

	fs.IntVar(&intervalSecs, "interval", int(defaultWatchInterval/time.Second), "seconds between polls")
	fs.StringVar(&outPath, "out", "", "where to write the digest (default: ./lazydispatch-run-<id>.md)")
	fs.BoolVar(&fix, "fix", false,
		"on failure, hand the digest to an interactive Claude Code session to investigate")
	fs.StringVar(&fixCmd, "fix-cmd", "claude", "the command --fix spawns")

	runID, err := parseRunID(fs, args)
	if err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	run, err := pollUntilDone(client, runID, time.Duration(intervalSecs)*time.Second, stderr)
	if err != nil {
		return err
	}

	path, err := writeDigest(client, runID, outPath, stderr)
	if err != nil {
		return err
	}

	notef(stderr, "run %d finished: %s (%s)\n", runID, run.Status, run.Conclusion)

	if _, err := fmt.Fprintln(stdout, path); err != nil {
		return fmt.Errorf("writing the digest path: %w", err)
	}

	if !fix || run.Conclusion == github.ConclusionSuccess {
		return nil
	}

	return investigate(fixCmd, path, run)
}

// pollUntilDone asks client about runID until it reports completed, waiting
// interval between polls. A poll that already finds it done never sleeps at
// all, which is what keeps a recorded test instant.
func pollUntilDone(
	client RunGetter, runID int64, interval time.Duration, stderr io.Writer,
) (*github.WorkflowRun, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	for {
		run, err := client.GetWorkflowRun(runID)
		if err != nil {
			return nil, fmt.Errorf("fetching run %d: %w", runID, err)
		}

		if run.Status == github.StatusCompleted {
			return run, nil
		}

		notef(stderr, "run %d: %s...\n", runID, run.Status)

		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("watch stopped: %w", ctx.Err())
		case <-time.After(interval):
		}
	}
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
