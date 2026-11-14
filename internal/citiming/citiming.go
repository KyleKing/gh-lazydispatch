package citiming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	agstats "github.com/kyleking/aragonite/stats"

	"github.com/kyleking/gh-lazydispatch/v2/internal/git"
	"github.com/kyleking/gh-lazydispatch/v2/internal/github"
	"github.com/kyleking/gh-lazydispatch/v2/internal/runcache"
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
	// Retrigger marks a run created long after the push (a title edit, a label, a rerun),
	// which the commit's span leaves out.
	Retrigger bool `json:"retrigger,omitempty"`
	// Attempt is above 1 when someone re-ran the workflow.
	Attempt int `json:"attempt,omitempty"`
}

// ciTiming is one push's CI wall-clock time: from the earliest triggered run's
// creation (the push reaching GitHub) to the last run's finish. CommittedAt is
// recorded for re-cutting but does not start the clock, because a commit made
// locally days before it was pushed would count the wait for the push.
type ciTiming struct {
	HeadSHA     string `json:"head_sha"`
	Actor       string `json:"actor,omitempty"`
	Branch      string `json:"branch,omitempty"`
	PRs         []int  `json:"prs,omitempty"`
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
	Insights    *ciInsights        `json:"insights,omitempty"`
	Flakes      *ciFlakes          `json:"same_sha_flakes,omitempty"`
	Ignored     map[string]int     `json:"ignored_workflows,omitempty"`
	RangeFrom   string             `json:"range_from,omitempty"`
	RangeTo     string             `json:"range_to,omitempty"`
	Commits     []ciTiming         `json:"commits"`
	Windows     []ciTimingWindow   `json:"windows,omitempty"`
	Epochs      []ciEpoch          `json:"epochs,omitempty"`
	Authors     []ciAuthor         `json:"authors,omitempty"`
	Workflows   []ciWorkflowEpochs `json:"workflow_epochs,omitempty"`
	PRSamples   []ciPR             `json:"pull_requests,omitempty"`
	Outcomes    []ciOutcome        `json:"outcomes,omitempty"`
	Trends      []ciTrend          `json:"workflow_trends,omitempty"`
	Critical    []ciCriticalSpan   `json:"critical_path,omitempty"`
	GateJobs    []string           `json:"gate_jobs,omitempty"`
	Flaky       []ciFlaky          `json:"flaky_workflows,omitempty"`
	Queue       []ciQueue          `json:"merge_queue,omitempty"`
	QueueUnread []int              `json:"merge_queue_unread_prs,omitempty"`
	FlakeTarget float64            `json:"flake_target_percent,omitempty"`
	MinSeconds  float64            `json:"min_seconds"`
	P50Seconds  float64            `json:"p50_seconds"`
	MeanSeconds float64            `json:"mean_seconds"`
	P90Seconds  float64            `json:"p90_seconds"`
	MaxSeconds  float64            `json:"max_seconds"`
}

// Options selects what ci-timing reads and which tables it builds.
type Options struct {
	Split, Until, BaselineUntil, Since                                                                time.Time
	Branch, Event, Format, Workflow, Authors, PRs, IgnoreJobs, SplitAt, IgnoreWorkflows               string
	WindowDays, BaselineOffsetDays, Sample, EpochDays, SamplePRs, CriticalSample, EpochMin, CacheLogs int
	// Flakes reads every re-run attempt's jobs to count same-SHA flakes against FlakeTarget (percent of SHAs).
	Flakes      bool
	FlakeTarget float64
}

// eventPush is the GitHub event name of a push to a branch.
const eventPush = "push"

// Events and conclusions the GitHub API spells out.
const (
	eventMergeGroup   = "merge_group"
	eventPullRequest  = "pull_request"
	conclusionStartup = "startup_failure"
)

// Output formats.
const (
	FormatJSON     = "json"
	FormatMarkdown = "md"
)

// ErrBadOptions marks options the caller should report as a usage error.
var ErrBadOptions = errors.New("bad options")

// Run reads CI timing for the repository client points at and writes it to stdout.
func Run(client *github.Client, opts Options, stdout, stderr io.Writer) error {
	stats, filter, err := collectCITiming(client, opts, stderr)
	if err != nil {
		return err
	}

	if opts.Format == FormatMarkdown {
		return writeCITimingMarkdown(stdout, filter, opts.Workflow, stats)
	}

	return writeJSON(stdout, stats)
}

// resolveRange fixes the end of the range at --until or now, the start of the epochs, and the two compared
// windows. A --since turns epochs on, so opts.EpochDays is raised to cover it.
func resolveRange(opts *Options) (time.Time, time.Time, [2]timeWindow) {
	now := time.Now().UTC()
	if !opts.Until.IsZero() && opts.Until.Before(now) {
		now = opts.Until
	}

	windows := compareWindows(now, opts.WindowDays, opts.BaselineOffsetDays)
	if !opts.Split.IsZero() {
		windows = splitWindows(now, opts.Split, opts.Until, opts.BaselineUntil)
	}

	epochFrom := daysBefore(now, opts.EpochDays)
	if !opts.Since.IsZero() {
		epochFrom = opts.Since
		opts.EpochDays = max(int(now.Sub(opts.Since)/(hoursPerDay*time.Hour))+1, 1)
	}

	return now, epochFrom, windows
}

// collectCITiming reads the runs, and whatever else opts asks for, and
// summarizes them.
func collectCITiming(client *github.Client, opts Options, stderr io.Writer) (ciTimingStats, github.RunFilter, error) {
	prs, err := parseNumbers(opts.PRs)
	if err != nil {
		return ciTimingStats{}, github.RunFilter{}, err
	}

	gitBranch := opts.Branch
	if opts.Event == eventPush || opts.EpochDays > 0 {
		gitBranch, err = resolveBranch(opts.Branch)
		if err != nil {
			return ciTimingStats{}, github.RunFilter{}, err
		}
	}

	filter := github.RunFilter{Branch: opts.Branch, Event: opts.Event}
	if opts.Event == eventPush {
		filter.Branch = gitBranch
	}

	now, epochFrom, windows := resolveRange(&opts)

	logins := splitList(opts.Authors)

	from := earliest(windows[0].from, opts.EpochDays > 0, epochFrom)

	loaded, err := loadAuthorRuns(client, filter, logins, from, now, stderr)
	if err != nil {
		return ciTimingStats{}, filter, err
	}

	runs, ignored := dropIgnored(loaded, splitList(opts.IgnoreWorkflows))

	stats := summarizeCITimings(runs)
	stats.Ignored = ignored.counts
	stats.FlakeTarget = opts.FlakeTarget
	stats.Windows = summarizeWindows(stats.Commits, windows)
	stats.RangeFrom, stats.RangeTo = epochFrom.Format(timeFormat), now.Format(timeFormat)

	notef(stderr, "%d commits from %d runs\n", len(stats.Commits), len(runs))

	cuts := epochCuts{SplitAt: splitList(opts.SplitAt), IgnoredPaths: ignored.paths}
	if err := addHistory(&stats, opts, gitBranch, epochFrom, cuts, windows, logins, runs); err != nil {
		return ciTimingStats{}, filter, err
	}

	if opts.SamplePRs > 0 || len(prs) > 0 {
		if stats.PRSamples, err = samplePRs(client, stats.Commits, logins, prs, opts.SamplePRs); err != nil {
			return ciTimingStats{}, filter, err
		}
	}

	if err := addJobData(client, &stats, opts, windows, now, stderr); err != nil {
		return ciTimingStats{}, filter, err
	}

	if opts.Workflow != "" {
		workflow := workflowRunName(runs, opts.Workflow, stderr)

		err := attachJobs(client, stats.Windows, stats.Commits, windows, workflow, opts.Sample, stderr)
		if err != nil {
			return ciTimingStats{}, filter, err
		}
	}

	return stats, filter, nil
}

// addJobData fills the tables that read each run's jobs or the merge queue.
func addJobData(
	client *github.Client, stats *ciTimingStats, opts Options, windows [2]timeWindow, now time.Time,
	stderr io.Writer,
) error {
	if opts.EpochDays > 0 {
		stats.Flaky = summarizeFlaky(stats.Commits)
	}

	if opts.Flakes {
		if err := addFlakes(client, stats, opts, splitList(opts.IgnoreJobs), stderr); err != nil {
			return err
		}
	}

	if opts.Event == eventMergeGroup {
		addMergeQueue(client, stats, windows, now, stderr)
	}

	if opts.CriticalSample > 0 {
		ignore := splitList(opts.IgnoreJobs)

		return addCriticalPath(client, stats, windows, opts.CriticalSample, ignore, opts.CacheLogs, now, stderr)
	}

	return nil
}

// addHistory fills the epoch, per-author, and per-workflow cuts, which need the
// repository's git history.
func addHistory(
	stats *ciTimingStats, opts Options, branch string, since time.Time, cuts epochCuts, windows [2]timeWindow,
	logins []string, runs []github.RunTiming,
) error {
	var err error

	if opts.EpochDays > 0 {
		ctx := context.Background()

		if stats.Epochs, err = epochsSince(ctx, branch, since, stats.Commits, opts.EpochMin, cuts); err != nil {
			return err
		}

		if stats.Workflows, err = workflowEpochs(ctx, branch, since, runs); err != nil {
			return err
		}

		spans := epochSpans(stats.Epochs, windows[1].to)
		stats.Outcomes = summarizeOutcomes(stats.Commits, spans)
		stats.Trends = summarizeTrends(stats.Commits, spans)
	}

	if len(logins) > 0 {
		stats.Authors = summarizeAuthors(stats.Commits, logins, windows, stats.Epochs, since)
	}

	return nil
}

func loadAuthorRuns(
	client *github.Client, filter github.RunFilter, logins []string, from, to time.Time, stderr io.Writer,
) ([]github.RunTiming, error) {
	if len(logins) == 0 {
		return loadRuns(client, filter, from, to, stderr)
	}

	var all []github.RunTiming

	for _, login := range logins {
		filter.Actor = login

		runs, err := loadRuns(client, filter, from, to, stderr)
		if err != nil {
			return nil, err
		}

		all = append(all, runs...)
	}

	return all, nil
}

func splitList(list string) []string {
	var out []string

	for _, item := range strings.Split(list, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}

	return out
}

func parseNumbers(list string) ([]int, error) {
	var out []int

	for _, item := range splitList(list) {
		n, err := strconv.Atoi(strings.TrimPrefix(item, "#"))
		if err != nil {
			return nil, fmt.Errorf("%w: --pr wants numbers, got %q", ErrBadOptions, item)
		}

		out = append(out, n)
	}

	return out, nil
}

func resolveBranch(branch string) (string, error) {
	if branch == "" {
		branch = git.GetDefaultBranch(context.Background())
	}

	if branch == "" {
		return "", fmt.Errorf("%w: --branch is required (could not detect a default branch)", ErrBadOptions)
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

// DefaultJobSample is enough runs per window for a stable P90 of a job's time.
const DefaultJobSample = 150

// DefaultWindowDays compares the last week of commits with the week before.
const DefaultWindowDays = 7

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

// splitWindows returns the baseline window ending at split and the current
// window running from split to until (or to now when until is zero or later),
// both as long as the time from split to the end of the current window. A
// nonzero baselineUntil moves the baseline to end there, leaving a gap before
// split for the runs that straddle the change.
func splitWindows(now, split, until, baselineUntil time.Time) [2]timeWindow {
	end := now
	if !until.IsZero() && until.Before(now) {
		end = until
	}

	span := end.Sub(split)

	baselineTo := split
	if !baselineUntil.IsZero() {
		baselineTo = baselineUntil
	}

	return [2]timeWindow{
		{label: "baseline", from: baselineTo.Add(-span), to: baselineTo},
		{label: "current", from: split, to: end},
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

// retriggerGap is how long after a push's first run a later run counts as a
// retrigger (an edited title, a label, a rerun) and not part of the push.
const retriggerGap = 10 * time.Minute

func timeCommit(sha string, group []github.RunTiming) ciTiming {
	primary := pushRuns(group)
	start, end := commitSpan(primary)

	timing := ciTiming{
		HeadSHA:    sha,
		StartedAt:  start.Format(timeFormat),
		FinishedAt: end.Format(timeFormat),
		Seconds:    end.Sub(start).Seconds(),
		Runs:       len(primary),
		RunDetails: make([]ciRun, 0, len(group)),
		Excluded:   exclusion(primary),
		Actor:      primary[0].Actor,
		Branch:     primary[0].HeadBranch,
	}

	for i := range primary {
		if len(primary[i].PRs) > 0 {
			timing.PRs = primary[i].PRs

			break
		}
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
			Retrigger:  run.CreatedAt.Sub(start) > retriggerGap,
			Attempt:    run.RunAttempt,

			FinishedAfter: run.UpdatedAt.Sub(start).Seconds(),
		})
	}

	return timing
}

// pushRuns returns the runs created within retriggerGap of the group's first.
func pushRuns(group []github.RunTiming) []github.RunTiming {
	first := group[0].CreatedAt
	for i := range group {
		if group[i].CreatedAt.Before(first) {
			first = group[i].CreatedAt
		}
	}

	primary := make([]github.RunTiming, 0, len(group))

	for i := range group {
		if group[i].CreatedAt.Sub(first) <= retriggerGap {
			primary = append(primary, group[i])
		}
	}

	return primary
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

	for i := range commits {
		started, err := time.Parse(timeFormat, commits[i].StartedAt)
		if err == nil && !started.Before(w.from) && started.Before(w.to) {
			in = append(in, commits[i])
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

func writeCITimingMarkdown(w io.Writer, filter github.RunFilter, workflow string, stats ciTimingStats) error {
	var b strings.Builder

	windows := stats.Windows

	scope := filter.Branch
	if scope == "" {
		scope = "any branch"
	}

	fmt.Fprintf(&b, "CI wall clock per commit (%s on `%s`), first run to last run finished.\n", filter.Event, scope)
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

	writeIgnored(&b, stats.Ignored)
	writeEpochTable(&b, stats.Epochs)
	writeOutcomeTable(&b, stats.Outcomes)
	writeTrendTable(&b, stats.Trends)
	writeCriticalTables(&b, stats.Critical)
	writeFlaky(&b, stats.Flaky, nil)
	writeFlakes(&b, stats.Flakes, flakeTargetOr(stats.FlakeTarget))
	writeInsights(&b, stats.Insights)
	writeQueue(&b, stats.Queue, len(stats.QueueUnread))
	writeAuthorTables(&b, stats.Authors)
	writeWorkflowEpochTable(&b, stats.Workflows)
	writePRTable(&b, stats.PRSamples)

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
		if !github.IsComplete(&group[i]) {
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
	for i := range commits {
		sum += commits[i].Seconds
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

// percentile reads p (0-1) out of values by linear interpolation.
func percentile(seconds []float64, p float64) float64 {
	return agstats.Percentile(seconds, p)
}

// loadRuns returns the runs created in [from, to), reading finished ones from
// the on-disk cache and fetching only the ranges it does not cover. A fetch
// that hit the API's result cap is reported and not cached, so a later run
// retries it.
func loadRuns(
	client *github.Client, filter github.RunFilter, from, to time.Time, stderr io.Writer,
) ([]github.RunTiming, error) {
	path := runcache.Path(client.Owner()+"/"+client.Repo(), filter)

	entry, err := runcache.Load(path)
	if err != nil {
		return nil, fmt.Errorf("loading the run cache: %w", err)
	}

	var (
		fetched []github.RunTiming
		capped  []time.Time
	)

	for _, missing := range entry.Missing(from, to) {
		sample, err := client.ListRunTimings(filter, missing.From, missing.To)
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

	return nameByWorkflowFile(runcache.Within(merged.Runs, fetched, from, to)), nil
}

// nameByWorkflowFile replaces the name of every run of a workflow file whose
// runs carry different names with the file's base name, because a `run-name`
// expression ("Preview jobs (PR 2136)") would otherwise make each run its own
// workflow in every per-workflow table.
func nameByWorkflowFile(runs []github.RunTiming) []github.RunTiming {
	names := make(map[string]map[string]bool)

	for i := range runs {
		if runs[i].Path == "" {
			continue
		}

		if names[runs[i].Path] == nil {
			names[runs[i].Path] = make(map[string]bool)
		}

		names[runs[i].Path][runs[i].Name] = true
	}

	for i := range runs {
		if len(names[runs[i].Path]) > 1 {
			runs[i].Name = workflowFileStem(runs[i].Path)
		}
	}

	return runs
}

// workflowRunName maps a --workflow value to the name its runs carry, because
// the per-job tables match on run name while the rest of the export takes a
// file name. A value matching neither is returned as is, with a warning.
func workflowRunName(runs []github.RunTiming, workflow string, stderr io.Writer) string {
	for i := range runs {
		if runs[i].Name == workflow {
			return workflow
		}
	}

	for i := range runs {
		if runs[i].Path != "" && workflowFileBase(runs[i].Path) == workflow {
			return runs[i].Name
		}
	}

	notef(stderr, "warning: no run in range is named or filed as %q; the per-job tables will be empty\n", workflow)

	return workflow
}

func workflowFileBase(workflowPath string) string {
	return workflowPath[strings.LastIndex(workflowPath, "/")+1:]
}

func workflowFileStem(workflowPath string) string {
	return strings.TrimSuffix(strings.TrimSuffix(workflowFileBase(workflowPath), ".yml"), ".yaml")
}

func flakeTargetOr(target float64) float64 {
	if target > 0 {
		return target
	}

	return DefaultFlakeTarget
}

// span is the range the stats cover, or zero times when none was recorded.
func (s *ciTimingStats) span() (time.Time, time.Time) {
	from, err := time.Parse(timeFormat, s.RangeFrom)
	if err != nil {
		return time.Time{}, time.Time{}
	}

	to, err := time.Parse(timeFormat, s.RangeTo)
	if err != nil {
		return time.Time{}, time.Time{}
	}

	return from, to
}
