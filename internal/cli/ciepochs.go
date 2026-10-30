package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/git"
)

// epochsFile lists pipeline changes the git history cannot show, such as a
// runner swap made in the GitHub UI. Each line is "YYYY-MM-DD label".
const epochsFile = ".github/ci-epochs"

// minEpochCommits is the fewest counted commits an epoch needs for its P90 to mean anything.
const minEpochCommits = 30

// ciEpoch is a span of commits that ran the same pipeline.
type ciEpoch struct {
	Start   string `json:"start"`
	Label   string `json:"label"`
	Changes int    `json:"changes"`
	Manual  bool   `json:"manual,omitempty"`
	// Few marks an epoch under minEpochCommits, whose percentiles are noise.
	Few        bool    `json:"few_commits,omitempty"`
	Commits    int     `json:"commits"`
	P50Seconds float64 `json:"p50_seconds"`
	P90Seconds float64 `json:"p90_seconds"`
	MaxSeconds float64 `json:"max_seconds"`
}

type epochBoundary struct {
	at      time.Time
	label   string
	manual  bool
	changes int
}

// epochBoundaries merges the workflow-file history with the manual epochs file.
func epochBoundaries(ctx context.Context, branch string, since time.Time) ([]epochBoundary, error) {
	bounds, err := pathBoundaries(ctx, branch, since, ".github/workflows", ".github/actions")
	if err != nil {
		return nil, err
	}

	manual, err := readManualEpochs(epochsFile)
	if err != nil {
		return nil, err
	}

	bounds = append(bounds, manual...)
	sort.SliceStable(bounds, func(i, j int) bool { return bounds[i].at.Before(bounds[j].at) })

	return bounds, nil
}

// pathBoundaries lists each commit touching paths since the start, behind an
// opening boundary at since so the span before the first change is an epoch too.
func pathBoundaries(ctx context.Context, branch string, since time.Time, paths ...string) ([]epochBoundary, error) {
	changes, err := git.PathChanges(ctx, "origin/"+branch, since, paths...)
	if err != nil {
		return nil, fmt.Errorf("reading the pipeline history (run from a checkout of the repository): %w", err)
	}

	bounds := make([]epochBoundary, 0, len(changes)+1)
	bounds = append(bounds, epochBoundary{at: since, label: "start of range"})

	for _, c := range changes {
		bounds = append(bounds, epochBoundary{at: c.At, label: c.Subject, changes: 1})
	}

	return bounds, nil
}

func epochsSince(ctx context.Context, branch string, since time.Time, commits []ciTiming) ([]ciEpoch, error) {
	bounds, err := epochBoundaries(ctx, branch, since)
	if err != nil {
		return nil, err
	}

	return summarizeEpochs(commits, bounds, minEpochCommits), nil
}

// boundsOf turns summarized epochs back into boundaries, so another set of
// commits can be cut at the same dates.
func boundsOf(epochs []ciEpoch) []epochBoundary {
	bounds := make([]epochBoundary, 0, len(epochs))

	for i := range epochs {
		at, err := time.Parse(timeFormat, epochs[i].Start)
		if err != nil {
			continue
		}

		e := &epochs[i]
		bounds = append(bounds, epochBoundary{at: at, label: e.Label, manual: e.Manual, changes: e.Changes})
	}

	return bounds
}

func readManualEpochs(path string) ([]epochBoundary, error) {
	data, err := os.ReadFile(path) //nolint:gosec // fixed relative path inside the checkout
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	return parseManualEpochs(strings.NewReader(string(data)), path)
}

func parseManualEpochs(r io.Reader, name string) ([]epochBoundary, error) {
	var bounds []epochBoundary

	scanner := bufio.NewScanner(r)
	for n := 1; scanner.Scan(); n++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		stamp, label, _ := strings.Cut(line, " ")

		at, err := time.Parse(time.DateOnly, stamp)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: want \"YYYY-MM-DD label\": %w", name, n, err)
		}

		bounds = append(bounds, epochBoundary{at: at, label: strings.TrimSpace(label), manual: true, changes: 1})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}

	return bounds, nil
}

// summarizeEpochs assigns each counted commit to the latest boundary before it.
// A boundary whose epoch holds fewer than minCommits counted commits folds into
// the next one, unless it is manual or the last, so the table never shows
// percentiles over a handful of commits except where nothing newer exists.
func summarizeEpochs(commits []ciTiming, bounds []epochBoundary, minCommits int) []ciEpoch {
	kept := counted(commits)

	var epochs []ciEpoch

	var pending *epochBoundary

	for i, b := range bounds {
		if pending != nil {
			b.at = pending.at
			b.changes += pending.changes
			if !b.manual {
				b.manual = pending.manual
			}
		}

		end := time.Time{}
		if i+1 < len(bounds) {
			end = bounds[i+1].at
		}

		in := commitsBetween(kept, b.at, end)
		if len(in) < minCommits && !b.manual && i+1 < len(bounds) {
			pending = &b

			continue
		}

		pending = nil

		epochs = append(epochs, ciEpoch{
			Start:      b.at.Format(timeFormat),
			Label:      b.label,
			Changes:    b.changes,
			Manual:     b.manual,
			Few:        len(in) < minCommits,
			Commits:    len(in),
			P50Seconds: secondsPercentile(in, percentileMedian),
			P90Seconds: secondsPercentile(in, percentileP90),
			MaxSeconds: secondsPercentile(in, 1),
		})
	}

	return epochs
}

func commitsBetween(commits []ciTiming, from, to time.Time) []ciTiming {
	var in []ciTiming

	for i := range commits {
		started, err := time.Parse(timeFormat, commits[i].StartedAt)
		if err == nil && !started.Before(from) && (to.IsZero() || started.Before(to)) {
			in = append(in, commits[i])
		}
	}

	return in
}

func writeEpochTable(b *strings.Builder, epochs []ciEpoch) {
	if len(epochs) == 0 {
		return
	}

	b.WriteString("\nEpochs (a pipeline change starts one, small epochs fold into the next).\n\n")
	b.WriteString("| Start | Change | Commits | P50 | P90 | Max |\n| --- | --- | ---: | ---: | ---: | ---: |\n")

	for i := range epochs {
		e := &epochs[i]

		label, commits := epochLabel(e), epochCount(e)

		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s |\n", e.Start[:len("2006-01-02")], label, commits,
			formatSeconds(e.P50Seconds), formatSeconds(e.P90Seconds), formatSeconds(e.MaxSeconds))
	}
}

func epochLabel(e *ciEpoch) string {
	if e.Changes > 1 {
		return fmt.Sprintf("%s (+%d earlier)", e.Label, e.Changes-1)
	}

	return e.Label
}

func epochCount(e *ciEpoch) string {
	if e.Few {
		return fmt.Sprintf("%d (under %d)", e.Commits, minEpochCommits)
	}

	return strconv.Itoa(e.Commits)
}
