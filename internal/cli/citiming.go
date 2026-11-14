package cli

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/v2/internal/citiming"
)

const defaultJobFailureRuns = 100

func ciTimingFlags(fs *flag.FlagSet) *citiming.Options {
	opts := &citiming.Options{}

	fs.StringVar(&opts.Branch, "branch", "", "branch to sample; default branch for push, any branch for other events")
	fs.StringVar(&opts.Event, "event", "push", "event that triggered the runs (push, pull_request, merge_group)")
	fs.IntVar(&opts.WindowDays, "window-days", citiming.DefaultWindowDays, "days of commits in each compared window")
	fs.IntVar(&opts.BaselineOffsetDays, "baseline-offset-days", citiming.DefaultWindowDays,
		"days before now the baseline ends")
	fs.Func("split", "RFC 3339 time or YYYY-MM-DD: compare the time since it with an equal span before it",
		func(value string) error {
			t, err := parseSplit(value)
			opts.Split = t

			return err
		})
	fs.Func("until", "RFC 3339 time or YYYY-MM-DD: with --split, end the current window here instead of now",
		func(value string) error {
			t, err := parseSplit(value)
			opts.Until = t

			return err
		})
	fs.Func("baseline-until", "RFC 3339 time or YYYY-MM-DD: with --split, end the baseline here, leaving a gap",
		func(value string) error {
			t, err := parseSplit(value)
			opts.BaselineUntil = t

			return err
		})
	fs.Func("since", "RFC 3339 time or YYYY-MM-DD: with --epoch-days, start the epoch range here",
		func(value string) error {
			t, err := parseSplit(value)
			opts.Since = t

			return err
		})
	fs.StringVar(&opts.SplitAt, "split-at", "",
		"comma-separated commits or dates that replace the automatic epoch cuts")
	fs.StringVar(&opts.IgnoreWorkflows, "ignore-workflow", "",
		"comma-separated workflow names or files that never gate a push; left out of wall clock and epoch cuts")
	fs.BoolVar(&opts.Flakes, "flakes", false, "with --epoch-days, count same-SHA flakes (reads re-run attempts' jobs)")
	fs.Float64Var(&opts.FlakeTarget, "flake-target", citiming.DefaultFlakeTarget,
		"with --flakes, the most head SHAs, in percent, that may hold a flake")
	fs.StringVar(&opts.Workflow, "workflow", "", "workflow name to break down into jobs and steps")
	fs.IntVar(&opts.Sample, "sample", citiming.DefaultJobSample,
		"with --workflow, runs per window to read jobs of; 0 reads all")
	fs.IntVar(&opts.EpochDays, "epoch-days", 0, "days of history to split into epochs; 0 skips")
	fs.IntVar(&opts.EpochMin, "epoch-min", citiming.MinEpochCommits,
		"fewest counted commits per epoch before it stands alone")
	fs.StringVar(&opts.IgnoreJobs, "ignore-job", "", "with --critical-sample, job names to leave out of the path")
	fs.IntVar(&opts.CriticalSample, "critical-sample", 0,
		"with --epoch-days, pushes per day whose jobs are read for job-level tables; 0 skips")
	fs.IntVar(&opts.CacheLogs, "cache-logs", 0,
		"with --critical-sample, job logs to read per job with a cache step; 0 skips")
	fs.StringVar(&opts.Authors, "author", "", "comma-separated GitHub logins to compare; only their runs are read")
	fs.IntVar(&opts.SamplePRs, "sample-prs", 0, "with --author, largest pull requests to spot-check per author")
	fs.StringVar(&opts.PRs, "pr", "", "comma-separated pull request numbers to spot-check")
	fs.StringVar(&opts.Format, "format", citiming.FormatJSON, "json (raw rows and summaries) or md (comparison table)")

	return opts
}

func exportCITiming(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("ci-timing")

	opts := ciTimingFlags(fs)

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	if err := checkCITimingOptions(opts); err != nil {
		return err
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	return usageError(citiming.Run(client, *opts, stdout, stderr))
}

func checkCITimingOptions(opts *citiming.Options) error {
	if opts.Format != citiming.FormatJSON && opts.Format != citiming.FormatMarkdown {
		return fmt.Errorf("%w: --format must be json or md, got %q", ErrUsage, opts.Format)
	}

	if err := checkUntil(opts); err != nil {
		return err
	}

	if !opts.Since.IsZero() && opts.EpochDays <= 0 {
		return fmt.Errorf("%w: --since needs --epoch-days", ErrUsage)
	}

	if !opts.BaselineUntil.IsZero() && (opts.Split.IsZero() || opts.BaselineUntil.After(opts.Split)) {
		return fmt.Errorf("%w: --baseline-until needs --split and a time no later than it", ErrUsage)
	}

	if opts.WindowDays <= 0 || opts.BaselineOffsetDays < 0 {
		return fmt.Errorf("%w: --window-days must be positive and --baseline-offset-days not negative", ErrUsage)
	}

	return nil
}

func checkUntil(opts *citiming.Options) error {
	if opts.Until.IsZero() {
		return nil
	}

	if opts.Split.IsZero() && opts.Since.IsZero() {
		return fmt.Errorf("%w: --until needs --split or --since", ErrUsage)
	}

	if !opts.Split.IsZero() && !opts.Until.After(opts.Split) {
		return fmt.Errorf("%w: --until must be later than --split", ErrUsage)
	}

	return nil
}

func parseSplit(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, time.DateOnly} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("%w: --split %q is not RFC 3339 or YYYY-MM-DD", ErrUsage, value)
}

func jobFailureFlags(fs *flag.FlagSet) *citiming.JobFailureOptions {
	opts := &citiming.JobFailureOptions{}

	fs.StringVar(&opts.Workflow, "workflow", "", "workflow filename, e.g. post-deploy-smoke.yml")
	fs.StringVar(&opts.Branch, "branch", "", "branch name")
	fs.StringVar(&opts.Event, "event", "", "triggering event")
	fs.IntVar(&opts.Runs, "runs", defaultJobFailureRuns, "newest finished runs to read")

	return opts
}

func exportJobFailures(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("job-failures")

	opts := jobFailureFlags(fs)

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	return usageError(citiming.JobFailures(client, *opts, stdout, stderr))
}

type ciReportFlagValues struct {
	opts            *citiming.ReportOptions
	deployWorkflows *string
	postDeploy      *string
}

func ciReportFlags(fs *flag.FlagSet) ciReportFlagValues {
	f := ciReportFlagValues{opts: &citiming.ReportOptions{}}
	opts := f.opts
	opts.Targets = citiming.DefaultTargets()

	fs.Func("since", "RFC 3339 time or YYYY-MM-DD: start of the range, in place of --days",
		func(value string) error {
			t, err := parseSplit(value)
			opts.Since = t

			return err
		})
	fs.Func("until", "RFC 3339 time or YYYY-MM-DD: end of the range, default now",
		func(value string) error {
			t, err := parseSplit(value)
			opts.Until = t

			return err
		})
	fs.StringVar(&opts.SplitAt, "split-at", "",
		"comma-separated commits or dates that cut the range into windows, in place of the automatic epochs")
	fs.StringVar(&opts.IgnoreWorkflows, "ignore-workflow", "",
		"comma-separated workflow names or files that never gate a push; left out of wall clock and cuts")
	fs.DurationVar(&opts.Targets.Wall, "target-wall", opts.Targets.Wall, "target wall clock per pull request push")
	fs.Float64Var(&opts.Targets.Flake, "target-flake", opts.Targets.Flake,
		"target share of head SHAs, in percent, with a same-SHA flake")
	fs.DurationVar(&opts.Targets.Queue, "target-queue", opts.Targets.Queue, "target merge queue wait")
	f.deployWorkflows = fs.String("deploy-workflow", "", "comma-separated deploy workflow files to report")
	fs.StringVar(&opts.Deploy.Smoke, "smoke-workflow", "",
		"with --deploy-workflow, the workflow file that verifies a deploy")
	f.postDeploy = fs.String("post-deploy-workflow", "", "with --deploy-workflow, workflow files counted per SHA")
	fs.StringVar(&opts.Deploy.UpMarker, "deploy-up", "",
		"with --deploy-workflow, part of the job or step name that does the deploy; empty uses the whole run")
	fs.StringVar(&opts.Authors, "author", "", "limit pull request runs to these GitHub logins (comma-separated)")
	fs.StringVar(&opts.IgnoreJobs, "ignore-job", "",
		"job names to leave out of the path and flake count; default detects gate jobs")
	fs.StringVar(&opts.Out, "out", "", "write the report to this file instead of stdout")
	fs.IntVar(&opts.Days, "days", citiming.ReportDays, "days of history to report on")
	fs.IntVar(&opts.WindowDays, "window-days", citiming.DefaultWindowDays, "days in each compared window")
	fs.IntVar(&opts.Sample, "sample", citiming.ReportSample, "pushes per day whose jobs are read")
	fs.IntVar(&opts.EpochMinimum, "epoch-min", citiming.ReportEpochMin, "fewest counted commits per epoch")
	fs.IntVar(&opts.CacheLogs, "cache-logs", citiming.ReportCacheLogs,
		"job logs to read per job with a cache step, for cache hit rates; 0 skips")

	return f
}

func exportCIReport(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("ci-report")

	f := ciReportFlags(fs)
	opts := f.opts

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	if opts.Days <= 0 || opts.WindowDays <= 0 {
		return fmt.Errorf("%w: --days and --window-days must be positive", ErrUsage)
	}

	if !opts.Until.IsZero() && !opts.Since.IsZero() && !opts.Until.After(opts.Since) {
		return fmt.Errorf("%w: --until must be later than --since", ErrUsage)
	}

	opts.Deploy.Workflows = splitCSV(*f.deployWorkflows)
	opts.Deploy.PostDeploy = splitCSV(*f.postDeploy)

	if opts.Deploy.Smoke != "" && len(opts.Deploy.PostDeploy) == 0 {
		opts.Deploy.PostDeploy = []string{opts.Deploy.Smoke}
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	return usageError(citiming.Report(client, *opts, stdout, stderr))
}

// usageError reports options the citiming package rejected as a usage error.
func usageError(err error) error {
	if errors.Is(err, citiming.ErrBadOptions) {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	return err
}

func splitCSV(list string) []string {
	var out []string

	for _, item := range strings.Split(list, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}

	return out
}
