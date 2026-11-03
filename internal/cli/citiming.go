package cli

import (
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/citiming"
)

const defaultJobFailureRuns = 100

func exportCITiming(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("ci-timing")

	var opts citiming.Options

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

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	if opts.Format != citiming.FormatJSON && opts.Format != citiming.FormatMarkdown {
		return fmt.Errorf("%w: --format must be json or md, got %q", ErrUsage, opts.Format)
	}

	if opts.WindowDays <= 0 || opts.BaselineOffsetDays < 0 {
		return fmt.Errorf("%w: --window-days must be positive and --baseline-offset-days not negative", ErrUsage)
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	return usageError(citiming.Run(client, opts, stdout, stderr))
}

func parseSplit(value string) (time.Time, error) {
	for _, layout := range []string{time.RFC3339, time.DateOnly} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("%w: --split %q is not RFC 3339 or YYYY-MM-DD", ErrUsage, value)
}

func exportJobFailures(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("job-failures")

	var opts citiming.JobFailureOptions

	fs.StringVar(&opts.Workflow, "workflow", "", "workflow filename, e.g. post-deploy-smoke.yml")
	fs.StringVar(&opts.Branch, "branch", "", "branch name")
	fs.StringVar(&opts.Event, "event", "", "triggering event")
	fs.IntVar(&opts.Runs, "runs", defaultJobFailureRuns, "newest finished runs to read")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	return usageError(citiming.JobFailures(client, opts, stdout, stderr))
}

func exportCIReport(args []string, stdout, stderr io.Writer) error {
	fs := newFlagSet("ci-report")

	var opts citiming.ReportOptions

	fs.StringVar(&opts.Authors, "author", "", "limit pull request runs to these GitHub logins (comma-separated)")
	fs.StringVar(&opts.IgnoreJobs, "ignore-job", "", "job names to leave out of the path; default detects gate jobs")
	fs.StringVar(&opts.Out, "out", "", "write the report to this file instead of stdout")
	fs.IntVar(&opts.Days, "days", citiming.ReportDays, "days of history to report on")
	fs.IntVar(&opts.WindowDays, "window-days", citiming.DefaultWindowDays, "days in each compared window")
	fs.IntVar(&opts.Sample, "sample", citiming.ReportSample, "pushes per day whose jobs are read")
	fs.IntVar(&opts.EpochMinimum, "epoch-min", citiming.ReportEpochMin, "fewest counted commits per epoch")
	fs.IntVar(&opts.CacheLogs, "cache-logs", citiming.ReportCacheLogs,
		"job logs to read per job with a cache step, for cache hit rates; 0 skips")

	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	if opts.Days <= 0 || opts.WindowDays <= 0 {
		return fmt.Errorf("%w: --days and --window-days must be positive", ErrUsage)
	}

	client, err := newClient()
	if err != nil {
		return err
	}

	return usageError(citiming.Report(client, opts, stdout, stderr))
}

// usageError reports options the citiming package rejected as a usage error.
func usageError(err error) error {
	if errors.Is(err, citiming.ErrBadOptions) {
		return fmt.Errorf("%w: %w", ErrUsage, err)
	}

	return err
}
