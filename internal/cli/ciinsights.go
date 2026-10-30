package cli

import (
	"io"
	"slices"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// ciInsights holds the job-level tables read from the sampled pushes' jobs.
type ciInsights struct {
	Runners  *ciRunnerComparison `json:"runner_comparison,omitempty"`
	Families []ciRunnerRow       `json:"runner_families,omitempty"`
	Variable []ciVariableJob     `json:"variable_jobs,omitempty"`
	Spend    []ciSpend           `json:"spend,omitempty"`
	Skipped  []ciSkipped         `json:"skipped_jobs,omitempty"`
	Handoffs []ciHandoff         `json:"handoffs,omitempty"`
	Failing  []ciJobFailure      `json:"failing_jobs,omitempty"`
}

// vendorTrial is the runner vendor whose trial the comparison looks for.
const vendorTrial = "blacksmith"

// addCriticalPath samples sample pushes a day, reads their jobs, and fills the
// per-epoch (or per-window when there are no epochs) job-level tables.
func addCriticalPath(
	client *github.Client, stats *ciTimingStats, windows [2]timeWindow, sample int, ignore []string,
	now time.Time, stderr io.Writer,
) error {
	spans := epochSpans(stats.Epochs, now)
	if len(spans) == 0 {
		spans = windows[:]
	}

	days := daySpans(spans[0].from, now)
	picked := slices.Concat(sampledPushes(stats.Commits, days, sample)...)

	jobs, err := loadJobs(client, pushRunIDs([][]ciTiming{picked}), stderr)
	if err != nil {
		return err
	}

	grouped := make([][]ciTiming, len(spans))
	for i, span := range spans {
		grouped[i] = commitsBetween(picked, span.from, span.to)
	}

	stats.Critical = summarizeCritical(grouped, spans, stats.Commits, jobs, ignore)

	recs := collectJobs(picked, jobs)
	cmp := compareRunners(recs, vendorTrial)

	stats.Insights = &ciInsights{
		Runners:  cmp,
		Families: familyRows(recs, cmp),
		Variable: variableJobs(recs),
		Spend:    summarizeSpend(recs, spans),
		Skipped:  skippedJobs(recs),
		Handoffs: summarizeHandoffs(recs, spans, readNeeds(), ignore),
		Failing:  failingJobs(recs),
	}

	return nil
}

// addMergeQueue reads the queue events of the pull requests behind the merge
// groups in stats.
func addMergeQueue(
	client *github.Client, stats *ciTimingStats, windows [2]timeWindow, now time.Time, stderr io.Writer,
) error {
	spans := epochSpans(stats.Epochs, now)
	if len(spans) == 0 {
		spans = windows[:]
	}

	prs := queuePRs(stats.Commits)
	notef(stderr, "%d pull requests entered the merge queue; reading their events\n", len(prs))

	events, err := client.ListMergeQueueEvents(prs)
	if err != nil {
		return err //nolint:wrapcheck // already names the failed read
	}

	stats.Queue = summarizeQueue(events, spans)

	return nil
}

func writeInsights(b *strings.Builder, in *ciInsights) {
	if in == nil {
		return
	}

	writeRunnerComparison(b, in.Runners, in.Families)
	writeHandoffs(b, in.Handoffs)
	writeVariableJobs(b, in.Variable)
	writeSpend(b, in.Spend, in.Skipped)
	writeFlakyJobs(b, in.Failing)
}

func writeFlakyJobs(b *strings.Builder, failing []ciJobFailure) {
	writeFlaky(b, nil, failing)
}
