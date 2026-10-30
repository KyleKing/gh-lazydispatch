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
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

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
	// FinishedAfter is seconds from the push to this run's finish.
	FinishedAfter float64 `json:"finished_after_seconds"`
	ID            int64   `json:"id"`
}

// ciTiming is one push's CI wall-clock time: from the earliest triggered run's
// creation (the push reaching GitHub) to the last run's finish. CommittedAt is
// recorded for re-cutting but does not start the clock, because a commit made
// locally days before it was pushed would count the wait for the push.
type ciTiming struct {
	HeadSHA     string `json:"head_sha"`
	CommittedAt string `json:"committed_at,omitempty"`
	StartedAt   string `json:"started_at"`
	FinishedAt  string `json:"finished_at"`
	// Excluded names why the commit stays out of every percentile ("" when it counts).
	Excluded   string  `json:"excluded,omitempty"`
	RunDetails []ciRun `json:"run_details"`
	Seconds    float64 `json:"seconds"`
	Runs       int     `json:"runs"`
}

// ciTimingWindow is the summary of the commits whose timing began in
// [From, To).
type ciTimingWindow struct {
	// Excluded counts the commits left out of the percentiles, by reason.
	Excluded    map[string]int `json:"excluded,omitempty"`
	Label       string         `json:"label"`
	From        string         `json:"from"`
	To          string         `json:"to"`
	Workflows   []ciWorkflow   `json:"workflows,omitempty"`
	Jobs        []ciJob        `json:"jobs,omitempty"`
	Steps       []ciStep       `json:"steps,omitempty"`
	Commits     int            `json:"commits"`
	MinSeconds  float64        `json:"min_seconds"`
	P50Seconds  float64        `json:"p50_seconds"`
	MeanSeconds float64        `json:"mean_seconds"`
	P90Seconds  float64        `json:"p90_seconds"`
	MaxSeconds  float64        `json:"max_seconds"`
}

// ciTimingStats is a per-commit CI timing rollup with summary statistics
// across every completed commit in the sample.
type ciTimingStats struct {
	Commits     []ciTiming       `json:"commits"`
	Windows     []ciTimingWindow `json:"windows,omitempty"`
	Epochs      []ciEpoch        `json:"epochs,omitempty"`
	MinSeconds  float64          `json:"min_seconds"`
	P50Seconds  float64          `json:"p50_seconds"`
	MeanSeconds float64          `json:"mean_seconds"`
	P90Seconds  float64          `json:"p90_seconds"`
	MaxSeconds  float64          `json:"max_seconds"`
}

func exportCITiming(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("ci-timing")

	var branch, event, format, workflow string

	var windowDays, baselineOffsetDays, sample, epochDays int

	fs.StringVar(&branch, "branch", "", "branch to sample; default the repository's default branch")
	fs.StringVar(&event, "event", "push", "event that triggered the runs")
	fs.IntVar(&windowDays, "window-days", defaultWindowDays, "days of commits in each compared window")
	fs.IntVar(&baselineOffsetDays, "baseline-offset-days", defaultWindowDays, "days before now the baseline ends")
	fs.StringVar(&workflow, "workflow", "", "workflow name to break down into jobs and steps")
	fs.IntVar(&sample, "sample", defaultJobSample, "with --workflow, runs per window to read jobs for; 0 reads all")
	fs.IntVar(&epochDays, "epoch-days", 0, "days of history to split into epochs; 0 skips")
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

	branch, err := resolveBranch(branch)
	if err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	windows := compareWindows(now, windowDays, baselineOffsetDays)

	epochFrom := daysBefore(now, epochDays)
	from := earliest(windows[0].from, epochDays > 0, epochFrom)

	runs, err := loadRuns(client, branch, event, from, now, stderr)
	if err != nil {
		return err
	}

	stats := summarizeCITimings(runs)
	stats.Windows = summarizeWindows(stats.Commits, windows)

	if epochDays > 0 {
		if stats.Epochs, err = epochsSince(context.Background(), branch, epochFrom, stats.Commits); err != nil {
			return err
		}
	}

	notef(stderr, "%d commits from %d runs\n", len(stats.Commits), len(runs))

	if workflow != "" {
		if err := attachJobs(client, stats.Windows, stats.Commits, windows, workflow, sample, stderr); err != nil {
			return err
		}
	}

	if format == formatMarkdown {
		return writeCITimingMarkdown(stdout, branch, workflow, stats.Windows, stats.Epochs)
	}

	return writeJSON(stdout, stats)
}

func resolveBranch(branch string) (string, error) {
	if branch == "" {
		branch = git.GetDefaultBranch(context.Background())
	}

	if branch == "" {
		return "", fmt.Errorf("%w: --branch is required (could not detect a default branch)", ErrUsage)
	}

	return branch, nil
}

func earliest(base time.Time, useOther bool, other time.Time) time.Time {
	if useOther && other.Before(base) {
		return other
	}

	return base
}

func daysBefore(now time.Time, days int) time.Time {
	return now.Add(-time.Duration(days) * hoursPerDay * time.Hour)
}

// defaultJobSample is enough runs per window for a stable P90 of a job's time.
const defaultJobSample = 150

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
		if sha == "" {
			continue
		}

		commits = append(commits, timeCommit(sha, group))
	}

	sort.Slice(commits, func(i, j int) bool { return commits[i].StartedAt < commits[j].StartedAt })

	kept := counted(commits)

	return ciTimingStats{
		Commits:     commits,
		MinSeconds:  secondsPercentile(kept, 0),
		P50Seconds:  secondsPercentile(kept, percentileMedian),
		MeanSeconds: secondsMean(kept),
		P90Seconds:  secondsPercentile(kept, percentileP90),
		MaxSeconds:  secondsPercentile(kept, 1),
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
		Excluded:   exclusion(group),
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

			FinishedAfter: run.UpdatedAt.Sub(start).Seconds(),
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

func windowCommits(commits []ciTiming, w timeWindow) []ciTiming {
	var in []ciTiming

	for _, c := range commits {
		started, err := time.Parse(timeFormat, c.StartedAt)
		if err == nil && !started.Before(w.from) && started.Before(w.to) {
			in = append(in, c)
		}
	}

	return in
}

// attachJobs fills each window with the job and step rollup of workflow.
func attachJobs(
	client *github.Client, windows []ciTimingWindow, commits []ciTiming, spans [2]timeWindow, workflow string,
	sample int, stderr io.Writer,
) error {
	ids := make([]int64, 0, len(commits))
	inWindow := make([][]ciTiming, len(spans))

	for i, span := range spans {
		inWindow[i] = counted(windowCommits(commits, span))
		ids = append(ids, sampleRuns(workflowRunIDs(inWindow[i], workflow), sample)...)
	}

	jobs, err := loadJobs(client, ids, stderr)
	if err != nil {
		return err
	}

	for i := range windows {
		windows[i].Jobs, windows[i].Steps = summarizeJobs(inWindow[i], workflow, jobs)
	}

	return nil
}

// summarizeWindows rolls up the commits that began inside each window.
func summarizeWindows(commits []ciTiming, windows [2]timeWindow) []ciTimingWindow {
	out := make([]ciTimingWindow, 0, len(windows))

	for _, w := range windows {
		all := windowCommits(commits, w)
		inWindow := counted(all)

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
			Excluded:    excludedCounts(all),
			Workflows:   summarizeWorkflows(inWindow),
		})
	}

	return out
}

func writeCITimingMarkdown(w io.Writer, branch, workflow string, windows []ciTimingWindow, epochs []ciEpoch) error {
	var b strings.Builder

	fmt.Fprintf(&b, "CI wall clock per commit on `%s`, push to last run finished.\n", branch)
	b.WriteString("Commits are left out (and counted) when a run timed out, was unfinished,\n")
	b.WriteString("or was canceled after every completed run.\n\n")
	b.WriteString("| Window | Commits | Left out | P50 | P90 | Max |\n| --- | ---: | --- | ---: | ---: | ---: |\n")

	for i := range windows {
		win := &windows[i]
		fmt.Fprintf(&b, "| %s (%s to %s) | %d | %s | %s | %s | %s |\n",
			win.Label, win.From[:len("2006-01-02")], win.To[:len("2006-01-02")], win.Commits, formatExcluded(win),
			formatSeconds(win.P50Seconds), formatSeconds(win.P90Seconds), formatSeconds(win.MaxSeconds))
	}

	if len(windows) == 2 && windows[0].P90Seconds > 0 && windows[1].Commits > 0 {
		fmt.Fprintf(&b, "\nP90 change: %+.0f%%\n", (windows[1].P90Seconds/windows[0].P90Seconds-1)*percentScale)
		writeWorkflowTable(&b, windows[0], windows[1])

		if workflow != "" {
			writeJobTables(&b, workflow, windows[0], windows[1])
		}
	}

	writeEpochTable(&b, epochs)

	if _, err := io.WriteString(w, b.String()); err != nil {
		return fmt.Errorf("writing markdown: %w", err)
	}

	return nil
}

func formatExcluded(win *ciTimingWindow) string {
	total := win.Commits
	for _, n := range win.Excluded {
		total += n
	}

	if len(win.Excluded) == 0 {
		return "0"
	}

	reasons := make([]string, 0, len(win.Excluded))
	left := 0

	for reason, n := range win.Excluded {
		reasons = append(reasons, fmt.Sprintf("%d %s", n, reason))
		left += n
	}

	sort.Strings(reasons)

	rate := float64(left) / float64(total) * percentScale

	return fmt.Sprintf("%d of %d (%.0f%%: %s)", left, total, rate, strings.Join(reasons, ", "))
}

// writeWorkflowTable compares each workflow's P90 finish time after the push
// across the two windows, since the slowest workflow sets a commit's wall clock.
func writeWorkflowTable(b *strings.Builder, baseline, current ciTimingWindow) {
	before := make(map[string]ciWorkflow, len(baseline.Workflows))
	for _, wf := range baseline.Workflows {
		before[wf.Name] = wf
	}

	b.WriteString("\nWorkflow finish time after the push (current window, slowest P90 first).\n\n")
	b.WriteString("| Workflow | Last | P50 | P90 | Baseline P90 | Change | Run P90 |\n")
	b.WriteString("| --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")

	for _, wf := range current.Workflows {
		prev, change := "-", "-"

		if old, ok := before[wf.Name]; ok && old.P90FinishedAfter > 0 {
			prev = formatSeconds(old.P90FinishedAfter)
			change = fmt.Sprintf("%+.0f%%", (wf.P90FinishedAfter/old.P90FinishedAfter-1)*percentScale)
		}

		fmt.Fprintf(b, "| %s | %.0f%% | %s | %s | %s | %s | %s |\n", wf.Name, wf.LastShare*percentScale,
			formatSeconds(wf.P50FinishedAfter), formatSeconds(wf.P90FinishedAfter),
			prev, change, formatSeconds(wf.P90Seconds))
	}
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
	seconds := make([]float64, len(commits))
	for i := range commits {
		seconds[i] = commits[i].Seconds
	}

	return percentile(seconds, p)
}

// percentile reads p (0-1) out of values by linear interpolation, sorting
// them in place.
func percentile(seconds []float64, p float64) float64 {
	n := len(seconds)
	if n == 0 {
		return 0
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

// loadRuns returns the runs created in [from, to), reading finished ones from
// the on-disk cache and fetching only the ranges it does not cover. A fetch
// that hit the API's result cap is reported and not cached, so a later run
// retries it.
func loadRuns(
	client *github.Client, branch, event string, from, to time.Time, stderr io.Writer,
) ([]github.RunTiming, error) {
	path := runcache.Path(client.Owner()+"/"+client.Repo(), event, branch)

	entry, err := runcache.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading the run cache: %w", err)
	}

	var (
		fetched []github.RunTiming
		capped  []time.Time
	)

	for _, missing := range entry.Missing(from, to) {
		sample, err := client.ListRunTimings(branch, event, missing.From, missing.To)
		if err != nil {
			return nil, fmt.Errorf("listing runs: %w", err)
		}

		fetched = append(fetched, sample.Runs...)
		capped = append(capped, sample.Capped...)
	}

	notef(stderr, "%d runs fetched, %d cached (%s)\n", len(fetched), len(entry.Runs), path)

	for _, at := range capped {
		notef(stderr, "warning: runs from %s hit the 1000-result cap and are incomplete\n", at.Format(time.RFC3339))
	}

	merged := entry.Merge(fetched, from, to)

	if len(capped) == 0 {
		if err := runcache.Save(path, merged); err != nil {
			notef(stderr, "warning: the run cache was not saved: %v\n", err)
		}
	}

	return runcache.Within(merged.Runs, fetched, from, to), nil
}
