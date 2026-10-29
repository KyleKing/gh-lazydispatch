package cli

import (
	"context"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
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

const hoursPerDay = 24

// ciRun is one workflow run behind a commit's timing, kept so the raw data can
// be re-cut by workflow without another API call.
type ciRun struct {
	Name       string  `json:"name"`
	Conclusion string  `json:"conclusion"`
	CreatedAt  string  `json:"created_at"`
	FinishedAt string  `json:"finished_at"`
	Seconds    float64 `json:"seconds"`
	ID         int64   `json:"id"`
}

// ciTiming is one push's CI wall-clock time: from the earliest triggered run's
// creation (the push reaching GitHub) to the last run's finish. CommittedAt is
// recorded for re-cutting but does not start the clock, because a commit made
// locally days before it was pushed would count the wait for the push.
type ciTiming struct {
	HeadSHA     string  `json:"head_sha"`
	CommittedAt string  `json:"committed_at,omitempty"`
	StartedAt   string  `json:"started_at"`
	FinishedAt  string  `json:"finished_at"`
	RunDetails  []ciRun `json:"run_details"`
	Seconds     float64 `json:"seconds"`
	Runs        int     `json:"runs"`
}

// ciTimingWindow is the summary of the commits whose timing began in
// [From, To).
type ciTimingWindow struct {
	Label       string  `json:"label"`
	From        string  `json:"from"`
	To          string  `json:"to"`
	Commits     int     `json:"commits"`
	MinSeconds  float64 `json:"min_seconds"`
	P50Seconds  float64 `json:"p50_seconds"`
	MeanSeconds float64 `json:"mean_seconds"`
	P90Seconds  float64 `json:"p90_seconds"`
	MaxSeconds  float64 `json:"max_seconds"`
}

// ciTimingStats is a per-commit CI timing rollup with summary statistics
// across every completed commit in the sample.
type ciTimingStats struct {
	Commits     []ciTiming       `json:"commits"`
	Windows     []ciTimingWindow `json:"windows,omitempty"`
	MinSeconds  float64          `json:"min_seconds"`
	P50Seconds  float64          `json:"p50_seconds"`
	MeanSeconds float64          `json:"mean_seconds"`
	P90Seconds  float64          `json:"p90_seconds"`
	MaxSeconds  float64          `json:"max_seconds"`
}

func exportCITiming(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("ci-timing")

	var branch, event, format string

	var limit, windowDays, baselineOffsetDays int

	fs.StringVar(&branch, "branch", "", "branch to sample; default the repository's default branch")
	fs.StringVar(&event, "event", "push", "event that triggered the runs")
	fs.IntVar(&limit, "limit", defaultCITimingRunLimit, "runs to sample, not commits")
	fs.IntVar(&windowDays, "window-days", defaultWindowDays, "days of commits in each compared window")
	fs.IntVar(&baselineOffsetDays, "baseline-offset-days", defaultWindowDays, "days before now the baseline ends")
	fs.StringVar(&format, "format", formatJSON, "json (raw rows and summaries) or md (comparison table)")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	if format != formatJSON && format != formatMarkdown {
		return fmt.Errorf("%w: --format must be json or md, got %q", ErrUsage, format)
	}

	if windowDays <= 0 || baselineOffsetDays < 0 {
		return fmt.Errorf("%w: --window-days must be positive and --baseline-offset-days not negative", ErrUsage)
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

	now := time.Now().UTC()
	windows := compareWindows(now, windowDays, baselineOffsetDays)

	runs, err := client.ListRunTimings(branch, event, windows[0].from, limit)
	if err != nil {
		return fmt.Errorf("listing runs: %w", err)
	}

	stats := summarizeCITimings(runs)
	stats.Windows = summarizeWindows(stats.Commits, windows)

	notef(stderr, "%d commits with every run finished (from %d runs sampled)\n", len(stats.Commits), len(runs))

	if len(runs) >= limit {
		notef(stderr, "warning: the %d-run limit was reached; the baseline may be short; raise --limit\n", limit)
	}

	if format == formatMarkdown {
		return writeCITimingMarkdown(stdout, branch, stats.Windows)
	}

	return writeJSON(stdout, stats)
}

// defaultWindowDays compares the last week of commits with the week before.
const defaultWindowDays = 7

const (
	formatJSON     = "json"
	formatMarkdown = "md"
)

type timeWindow struct {
	from, to time.Time
	label    string
}

// compareWindows returns the baseline window (ending offsetDays before now)
// first, then the current window (ending now), each windowDays long.
func compareWindows(now time.Time, windowDays, offsetDays int) [2]timeWindow {
	day := hoursPerDay * time.Hour
	baselineTo := now.Add(-time.Duration(offsetDays) * day)

	return [2]timeWindow{
		{label: "baseline", from: baselineTo.Add(-time.Duration(windowDays) * day), to: baselineTo},
		{label: "current", from: now.Add(-time.Duration(windowDays) * day), to: now},
	}
}

// summarizeCITimings groups runs by the commit they ran against and reports,
// for each commit where every triggered run has finished, how long CI took
// end to end: from the earliest run's creation to the last run's completion. A commit still in
// flight is dropped, since its span is not the real one yet.
func summarizeCITimings(runs []github.RunTiming) ciTimingStats {
	byCommit := make(map[string][]github.RunTiming)
	for i := range runs {
		byCommit[runs[i].HeadSHA] = append(byCommit[runs[i].HeadSHA], runs[i])
	}

	commits := make([]ciTiming, 0, len(byCommit))

	for sha, group := range byCommit {
		if sha == "" || !allComplete(group) {
			continue
		}

		commits = append(commits, timeCommit(sha, group))
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

func timeCommit(sha string, group []github.RunTiming) ciTiming {
	start, end := commitSpan(group)

	timing := ciTiming{
		HeadSHA:    sha,
		StartedAt:  start.Format(timeFormat),
		FinishedAt: end.Format(timeFormat),
		Seconds:    end.Sub(start).Seconds(),
		Runs:       len(group),
		RunDetails: make([]ciRun, 0, len(group)),
	}

	if committed := earliestCommit(group); !committed.IsZero() {
		timing.CommittedAt = committed.Format(timeFormat)
	}

	for i := range group {
		run := &group[i]
		timing.RunDetails = append(timing.RunDetails, ciRun{
			Name:       run.Name,
			Conclusion: run.Conclusion,
			CreatedAt:  run.CreatedAt.Format(timeFormat),
			FinishedAt: run.UpdatedAt.Format(timeFormat),
			Seconds:    run.UpdatedAt.Sub(run.CreatedAt).Seconds(),
			ID:         run.ID,
		})
	}

	return timing
}

func earliestCommit(group []github.RunTiming) time.Time {
	var earliest time.Time

	for i := range group {
		if at := group[i].CommittedAt; !at.IsZero() && (earliest.IsZero() || at.Before(earliest)) {
			earliest = at
		}
	}

	return earliest
}

// summarizeWindows rolls up the commits that began inside each window.
func summarizeWindows(commits []ciTiming, windows [2]timeWindow) []ciTimingWindow {
	out := make([]ciTimingWindow, 0, len(windows))

	for _, w := range windows {
		var inWindow []ciTiming

		for _, c := range commits {
			started, err := time.Parse(timeFormat, c.StartedAt)
			if err == nil && !started.Before(w.from) && started.Before(w.to) {
				inWindow = append(inWindow, c)
			}
		}

		out = append(out, ciTimingWindow{
			Label:       w.label,
			From:        w.from.Format(timeFormat),
			To:          w.to.Format(timeFormat),
			Commits:     len(inWindow),
			MinSeconds:  secondsPercentile(inWindow, 0),
			P50Seconds:  secondsPercentile(inWindow, percentileMedian),
			MeanSeconds: secondsMean(inWindow),
			P90Seconds:  secondsPercentile(inWindow, percentileP90),
			MaxSeconds:  secondsPercentile(inWindow, 1),
		})
	}

	return out
}

func writeCITimingMarkdown(w io.Writer, branch string, windows []ciTimingWindow) error {
	var b strings.Builder

	fmt.Fprintf(&b, "CI wall clock per commit on `%s`, push to last run finished.\n\n", branch)
	b.WriteString("| Window | Commits | P50 | P90 | Max |\n| --- | ---: | ---: | ---: | ---: |\n")

	for _, win := range windows {
		fmt.Fprintf(&b, "| %s (%s to %s) | %d | %s | %s | %s |\n",
			win.Label, win.From[:len("2006-01-02")], win.To[:len("2006-01-02")], win.Commits,
			formatSeconds(win.P50Seconds), formatSeconds(win.P90Seconds), formatSeconds(win.MaxSeconds))
	}

	if len(windows) == 2 && windows[0].P90Seconds > 0 && windows[1].Commits > 0 {
		fmt.Fprintf(&b, "\nP90 change: %+.0f%%\n", (windows[1].P90Seconds/windows[0].P90Seconds-1)*percentScale)
	}

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("writing markdown: %w", err)
	}

	return nil
}

const percentScale = 100

func formatSeconds(seconds float64) string {
	return time.Duration(seconds * float64(time.Second)).Round(time.Second).String()
}

// allComplete reports whether every run in a commit's group has finished, so a
// push still in progress does not report a partial, misleadingly short span.
func allComplete(group []github.RunTiming) bool {
	for i := range group {
		if !group[i].IsComplete() {
			return false
		}
	}

	return true
}

// commitSpan returns the earliest creation and latest completion across a
// commit's runs, which is the wall-clock window a human waited on CI for.
func commitSpan(group []github.RunTiming) (time.Time, time.Time) {
	start, end := group[0].CreatedAt, group[0].UpdatedAt

	for i := 1; i < len(group); i++ {
		if group[i].CreatedAt.Before(start) {
			start = group[i].CreatedAt
		}

		if group[i].UpdatedAt.After(end) {
			end = group[i].UpdatedAt
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
