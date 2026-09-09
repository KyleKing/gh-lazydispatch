package cli

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/git"
	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// defaultCITimingRunLimit samples enough runs to cover several dozen commits
// on a repository with a dozen or so workflows per push.
const defaultCITimingRunLimit = 500

// Percentiles read off the sorted per-commit durations.
const (
	percentileMedian = 0.5
	percentileP90    = 0.9
)

// ciTiming is one push's CI wall-clock time: from the earliest triggered run's
// start to the last one's finish.
type ciTiming struct {
	HeadSHA    string  `json:"head_sha"`
	StartedAt  string  `json:"started_at"`
	FinishedAt string  `json:"finished_at"`
	Seconds    float64 `json:"seconds"`
	Runs       int     `json:"runs"`
}

// ciTimingStats is a per-commit CI timing rollup with summary statistics
// across every completed commit in the sample.
type ciTimingStats struct {
	Commits     []ciTiming `json:"commits"`
	MinSeconds  float64    `json:"min_seconds"`
	P50Seconds  float64    `json:"p50_seconds"`
	MeanSeconds float64    `json:"mean_seconds"`
	P90Seconds  float64    `json:"p90_seconds"`
	MaxSeconds  float64    `json:"max_seconds"`
}

func exportCITiming(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("ci-timing")

	var branch, event string

	var limit int

	fs.StringVar(&branch, "branch", "", "branch to sample; default the repository's default branch")
	fs.StringVar(&event, "event", "push", "event that triggered the runs")
	fs.IntVar(&limit, "limit", defaultCITimingRunLimit, "runs to sample, not commits")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	if branch == "" {
		branch = git.GetDefaultBranch(context.Background())
	}

	if branch == "" {
		return fmt.Errorf("%w: --branch is required (could not detect a default branch)", ErrUsage)
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	runs, err := client.ListRunTimings(branch, event, limit)
	if err != nil {
		return fmt.Errorf("listing runs: %w", err)
	}

	stats := summarizeCITimings(runs)

	notef(stderr, "%d commits with every run finished (from %d runs sampled)\n", len(stats.Commits), len(runs))

	return writeJSON(stdout, stats)
}

// summarizeCITimings groups runs by the commit they ran against and reports,
// for each commit where every triggered run has finished, how long CI took
// end to end: from the earliest run's creation to the last run's completion.
// A commit still in flight is dropped, since its span is not the real one yet.
func summarizeCITimings(runs []github.RunTiming) ciTimingStats {
	byCommit := make(map[string][]github.RunTiming)
	for _, run := range runs {
		byCommit[run.HeadSHA] = append(byCommit[run.HeadSHA], run)
	}

	commits := make([]ciTiming, 0, len(byCommit))

	for sha, group := range byCommit {
		if sha == "" || !allComplete(group) {
			continue
		}

		start, end := commitSpan(group)
		commits = append(commits, ciTiming{
			HeadSHA:    sha,
			StartedAt:  start.Format(timeFormat),
			FinishedAt: end.Format(timeFormat),
			Seconds:    end.Sub(start).Seconds(),
			Runs:       len(group),
		})
	}

	sort.Slice(commits, func(i, j int) bool { return commits[i].StartedAt < commits[j].StartedAt })

	return ciTimingStats{
		Commits:     commits,
		MinSeconds:  secondsPercentile(commits, 0),
		P50Seconds:  secondsPercentile(commits, percentileMedian),
		MeanSeconds: secondsMean(commits),
		P90Seconds:  secondsPercentile(commits, percentileP90),
		MaxSeconds:  secondsPercentile(commits, 1),
	}
}

// allComplete reports whether every run in a commit's group has finished, so a
// push still in progress does not report a partial, misleadingly short span.
func allComplete(group []github.RunTiming) bool {
	for _, run := range group {
		if !run.IsComplete() {
			return false
		}
	}

	return true
}

// commitSpan returns the earliest creation and latest completion across a
// commit's runs, which is the wall-clock window a human waited on CI for.
func commitSpan(group []github.RunTiming) (time.Time, time.Time) {
	start, end := group[0].CreatedAt, group[0].UpdatedAt

	for _, run := range group[1:] {
		if run.CreatedAt.Before(start) {
			start = run.CreatedAt
		}

		if run.UpdatedAt.After(end) {
			end = run.UpdatedAt
		}
	}

	return start, end
}

func secondsMean(commits []ciTiming) float64 {
	if len(commits) == 0 {
		return 0
	}

	var sum float64
	for _, c := range commits {
		sum += c.Seconds
	}

	return sum / float64(len(commits))
}

// secondsPercentile reads p (0-1) out of commits' Seconds by linear
// interpolation between the two nearest ranks, sorting a private copy so the
// caller's chronological order survives.
func secondsPercentile(commits []ciTiming, p float64) float64 {
	n := len(commits)
	if n == 0 {
		return 0
	}

	seconds := make([]float64, n)
	for i, c := range commits {
		seconds[i] = c.Seconds
	}

	sort.Float64s(seconds)

	if n == 1 {
		return seconds[0]
	}

	rank := p * float64(n-1)
	lo, hi := int(math.Floor(rank)), int(math.Ceil(rank))

	if lo == hi {
		return seconds[lo]
	}

	frac := rank - float64(lo)

	return seconds[lo] + frac*(seconds[hi]-seconds[lo])
}
