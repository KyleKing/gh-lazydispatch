package cli

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

const (
	windowPair        = 2
	reportEpochMin    = 120
	reportSample      = 6
	reportDays        = 30
	reportFilePerm    = 0o644
	changeThreshold   = 0.10
	costThreshold     = 0.30
	leftOutThreshold  = 0.15
	minWindowCommits  = 20
	minGreenReruns    = 10
	minEvictionSample = 20
	evictionThreshold = 0.05
	skippedMostly     = 0.9
	chainThreshold    = 0.3
	topFindings       = 3
	slowSpreadShare   = 0.1
	spreadFactor      = 2
)

// reportEvent is one trigger's collected statistics.
type reportEvent struct {
	label  string
	filter github.RunFilter
	stats  ciTimingStats
}

type ciReportOptions struct {
	authors, ignoreJobs, out               string
	days, windowDays, sample, epochMinimum int
}

func exportCIReport(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("ci-report")

	var opts ciReportOptions

	fs.StringVar(&opts.authors, "author", "", "limit pull request runs to these GitHub logins (comma-separated)")
	fs.StringVar(&opts.ignoreJobs, "ignore-job", "", "job names to leave out of the path; default detects gate jobs")
	fs.StringVar(&opts.out, "out", "", "write the report to this file instead of stdout")
	fs.IntVar(&opts.days, "days", reportDays, "days of history to report on")
	fs.IntVar(&opts.windowDays, "window-days", defaultWindowDays, "days in each compared window")
	fs.IntVar(&opts.sample, "sample", reportSample, "pushes per day whose jobs are read")
	fs.IntVar(&opts.epochMinimum, "epoch-min", reportEpochMin, "fewest counted commits per epoch")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	if opts.days <= 0 || opts.windowDays <= 0 {
		return fmt.Errorf("%w: --days and --window-days must be positive", ErrUsage)
	}

	events, err := collectReportEvents(opts, stderr)
	if err != nil {
		return err
	}

	report, err := renderReport(events, opts)
	if err != nil {
		return err
	}

	if opts.out == "" {
		_, err := io.WriteString(stdout, report)
		if err != nil {
			return fmt.Errorf("writing the report: %w", err)
		}

		return nil
	}

	err = os.WriteFile(opts.out, []byte(report), reportFilePerm)
	if err != nil {
		return fmt.Errorf("writing %s: %w", opts.out, err)
	}

	notef(stderr, "wrote %s\n", opts.out)

	return nil
}

// collectReportEvents reads pull requests, the default branch, and the merge
// queue. A trigger with no runs in the range is left out.
func collectReportEvents(opts ciReportOptions, stderr io.Writer) ([]reportEvent, error) {
	triggers := []struct{ label, event, authors string }{
		{"Pull requests", "pull_request", opts.authors},
		{"Pushes to the default branch", "push", ""},
		{"Merge queue", "merge_group", ""},
	}

	var events []reportEvent

	for _, t := range triggers {
		stats, filter, err := collectCITiming(ciTimingOptions{
			event: t.event, authors: t.authors, ignoreJobs: opts.ignoreJobs,
			windowDays: opts.windowDays, baselineOffsetDays: opts.windowDays,
			epochDays: opts.days, epochMin: opts.epochMinimum, criticalSample: opts.sample,
		}, stderr)
		if err != nil {
			return nil, err
		}

		if len(stats.Commits) > 0 {
			events = append(events, reportEvent{label: t.label, filter: filter, stats: stats})
		}
	}

	return events, nil
}

func renderReport(events []reportEvent, opts ciReportOptions) (string, error) {
	var b strings.Builder

	fmt.Fprintf(&b, "# CI performance report\n\nGenerated %s over the last %d days. Wall clock is the span from "+
		"a push's first workflow run being created to its last run finishing. Job-level tables read a "+
		"hash-chosen sample of %d pushes a day.\n", time.Now().UTC().Format(time.DateOnly), opts.days, opts.sample)

	b.WriteString("\n## Findings\n\nRule-based readings of the tables below. Each is a lead to check.\n\n")

	findings := reportFindings(events)
	if len(findings) == 0 {
		b.WriteString("- Nothing crossed a threshold.\n")
	}

	for _, f := range findings {
		b.WriteString("- " + f + "\n")
	}

	for i := range events {
		e := &events[i]
		fmt.Fprintf(&b, "\n## %s\n\n", e.label)

		if err := writeCITimingMarkdown(&b, e.filter, "", e.stats); err != nil {
			return "", err
		}
	}

	return b.String(), nil
}

// reportFindings applies the thresholds to each trigger's statistics.
func reportFindings(events []reportEvent) []string {
	var out []string

	for i := range events {
		e := &events[i]
		for _, find := range []func(*reportEvent) []string{
			wallClockFinding, leftOutFinding, pathFinding, costFinding, skippedFinding,
			flakyFinding, spreadFinding, runnerFinding, queueFinding,
		} {
			out = append(out, find(e)...)
		}

		if len(e.stats.GateJobs) > 0 {
			out = append(out, fmt.Sprintf("%s: left out of the path as gate jobs, which only wait on other jobs: %s.",
				e.label, strings.Join(e.stats.GateJobs, ", ")))
		}
	}

	return out
}

func wallClockFinding(e *reportEvent) []string {
	w := e.stats.Windows
	if len(w) != 2 || w[0].Commits < minWindowCommits || w[1].Commits < minWindowCommits || w[0].P90Seconds == 0 {
		return nil
	}

	change := w[1].P90Seconds/w[0].P90Seconds - 1
	if change > -changeThreshold && change < changeThreshold {
		return nil
	}

	direction := "rose"
	if change < 0 {
		direction = "fell"
	}

	return []string{fmt.Sprintf("%s: P90 wall clock %s %.0f%% between the windows (%s over %d pushes to %s over %d).",
		e.label, direction, change*percentScale, formatSeconds(w[0].P90Seconds), w[0].Commits,
		formatSeconds(w[1].P90Seconds), w[1].Commits)}
}

func leftOutFinding(e *reportEvent) []string {
	w := e.stats.Windows
	if len(w) < windowPair {
		return nil
	}

	var left int
	for _, n := range w[1].Excluded {
		left += n
	}

	total := left + w[1].Commits
	if total == 0 || float64(left)/float64(total) < leftOutThreshold {
		return nil
	}

	return []string{fmt.Sprintf("%s: %.0f%% of pushes in the current window were canceled, re-run, or unfinished, "+
		"so the wall clock a person waits is longer than the percentiles show.",
		e.label, float64(left)/float64(total)*percentScale)}
}

// pathFinding names the job and chain that most often end a push in the latest
// epoch with enough sampled pushes.
func pathFinding(e *reportEvent) []string {
	var out []string

	crit := e.stats.Critical
	for i := len(crit) - 1; i >= 0; i-- {
		if crit[i].Sampled < topFindings || len(crit[i].Jobs) == 0 {
			continue
		}

		j := &crit[i].Jobs[0]
		out = append(out, fmt.Sprintf("%s: from %s, %s (%s) ended %.0f%% of sampled pushes, with a median %s of "+
			"headroom over the next job and a median run of %s. Speeding it up alone gains at most that headroom.",
			e.label, crit[i].Start, j.Job, j.Workflow, j.LastShare*percentScale,
			formatSeconds(j.P50Headroom), formatSeconds(j.P50Seconds)))

		break
	}

	if e.stats.Insights == nil {
		return out
	}

	h := e.stats.Insights.Handoffs
	for i := len(h) - 1; i >= 0; i-- {
		for _, c := range h[i].Chains {
			if strings.Contains(c.Chain, chainArrow) && c.Share >= chainThreshold {
				return append(out, fmt.Sprintf("%s: from %s, %.0f%% of pushes ended on the chain %s.",
					e.label, h[i].Start, c.Share*percentScale, c.Chain))
			}
		}
	}

	return out
}

func costFinding(e *reportEvent) []string {
	if e.stats.Insights == nil || len(e.stats.Insights.Spend) < 2 {
		return nil
	}

	spend := e.stats.Insights.Spend
	first, last := &spend[0], &spend[len(spend)-1]

	lo, hi := first, first
	for i := range spend {
		if spend[i].BilledMinutes < lo.BilledMinutes {
			lo = &spend[i]
		}

		if spend[i].BilledMinutes > hi.BilledMinutes {
			hi = &spend[i]
		}
	}

	if lo.BilledMinutes == 0 || hi.BilledMinutes/lo.BilledMinutes-1 < costThreshold || last.Pushes == 0 {
		return nil
	}

	return []string{fmt.Sprintf("%s: billed minutes per push ranged from %.0f (epoch %s) to %.0f (epoch %s), "+
		"with jobs per push %.0f and %.0f. Compare the workflow changes between those epochs.",
		e.label, lo.BilledMinutes, lo.Start, hi.BilledMinutes, hi.Start, lo.JobsPerPush, hi.JobsPerPush)}
}

func skippedFinding(e *reportEvent) []string {
	if e.stats.Insights == nil {
		return nil
	}

	var names []string

	for _, s := range e.stats.Insights.Skipped {
		if s.Share >= 1 && len(names) < topFindings {
			names = append(names, s.Job)
		}
	}

	if len(names) == 0 {
		return nil
	}

	return []string{fmt.Sprintf("%s: jobs skipped on every sampled push include %s. A job that never runs may "+
		"be removable from this trigger.", e.label, strings.Join(names, ", "))}
}

func flakyFinding(e *reportEvent) []string {
	var out []string

	for i := range e.stats.Flaky {
		f := &e.stats.Flaky[i]
		if f.RerunGreen >= minGreenReruns && len(out) < topFindings {
			failed := int(f.FailedShare * float64(f.Runs))
			out = append(out, fmt.Sprintf("%s: %s had %d re-runs that passed (%d of %d runs failed), the closest "+
				"signal to a flake the API gives.", e.label, f.Workflow, f.RerunGreen, failed, f.Runs))
		}
	}

	return out
}

func spreadFinding(e *reportEvent) []string {
	if e.stats.Insights == nil {
		return nil
	}

	var out []string

	for _, j := range e.stats.Insights.Variable {
		if j.SlowShare >= slowSpreadShare && j.P90Seconds >= spreadFactor*j.P50Seconds && len(out) < topFindings {
			out = append(out, fmt.Sprintf("%s: %s ran a median %s but %s at P90 (%.0f%% of runs over twice "+
				"the median), mostly in the step %q.", e.label, j.Job, formatSeconds(j.P50Seconds),
				formatSeconds(j.P90Seconds), j.SlowShare*percentScale, j.WidestStep))
		}
	}

	return out
}

func runnerFinding(e *reportEvent) []string {
	in := e.stats.Insights
	if in == nil || in.Runners == nil {
		return nil
	}

	var vendor, other *ciRunnerRow

	for i := range in.Families {
		r := &in.Families[i]
		switch {
		case r.Phase == "2 during" && r.Runner == in.Runners.Vendor:
			vendor = r
		case r.Phase == "1 before" && usualRunner(r, in.Runners.Vendor, other):
			other = r
		}
	}

	if vendor == nil || other == nil {
		return nil
	}

	return []string{fmt.Sprintf("%s: %s runners were used from %s to %s. Their jobs queued a median %s against "+
		"%s on %s, and ran a median %s against %s.", e.label, in.Runners.Vendor, in.Runners.From, in.Runners.To,
		formatSeconds(vendor.P50Queue), formatSeconds(other.P50Queue), other.Runner,
		formatSeconds(vendor.P50Seconds), formatSeconds(other.P50Seconds))}
}

// usualRunner reports whether r is the busiest non-vendor, non-GitHub runner family so far.
func usualRunner(r *ciRunnerRow, vendor string, best *ciRunnerRow) bool {
	return r.Runner != vendor && r.Runner != "github" && (best == nil || r.Jobs > best.Jobs)
}

func queueFinding(e *reportEvent) []string {
	var out []string

	for i := len(e.stats.Queue) - 1; i >= 0; i-- {
		q := &e.stats.Queue[i]
		if q.Entries >= minEvictionSample && q.EvictionRate >= evictionThreshold {
			out = append(out, fmt.Sprintf("%s: %.0f%% of merge queue entries from %s were evicted (%d of %d), "+
				"and merged entries waited a median %s.", e.label, q.EvictionRate*percentScale, q.Start,
				q.Evicted, q.Entries, formatSeconds(q.P50Wait)))

			break
		}
	}

	if e.stats.Insights != nil && len(e.stats.Insights.Spend) > 0 {
		last := e.stats.Insights.Spend[len(e.stats.Insights.Spend)-1]
		if last.SkippedShare >= skippedMostly {
			out = append(out, fmt.Sprintf("%s: %.0f%% of jobs were skipped in the latest epoch, so this trigger "+
				"runs almost no checks.", e.label, last.SkippedShare*percentScale))
		}
	}

	return out
}
