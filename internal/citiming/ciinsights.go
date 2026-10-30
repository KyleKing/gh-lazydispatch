package citiming

import (
	"io"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
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
	Caches   []ciCacheOutcome    `json:"cache_logs,omitempty"`
}

// vendorTrial is the runner vendor whose trial the comparison looks for.
const vendorTrial = "blacksmith"

// addCriticalPath samples sample pushes a day, reads their jobs, and fills the
// per-epoch (or per-window when there are no epochs) job-level tables.
func addCriticalPath(
	client *github.Client, stats *ciTimingStats, windows [2]timeWindow, sample int, ignore []string,
	cacheLogs int, now time.Time, stderr io.Writer,
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

	if len(ignore) == 0 {
		ignore = detectGateJobs(picked, jobs)
		if len(ignore) > 0 {
			notef(stderr, "leaving out gate jobs that only wait on others: %s\n", strings.Join(ignore, ", "))
		}
	}

	stats.GateJobs = ignore

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

	if cacheLogs > 0 {
		return addCacheLogs(client, stats.Insights, recs, cacheLogs, stderr)
	}

	return nil
}

// addCacheLogs reads a few job logs for each job that has a cache step.
func addCacheLogs(client *github.Client, in *ciInsights, recs []jobRec, perJob int, stderr io.Writer) error {
	ids := make(map[int64]*jobRec)
	picked := cacheJobCandidates(recs, ids, perJob)

	logs, err := readCacheLogs(client, picked, stderr)
	if err != nil {
		return err
	}

	in.Caches = summarizeCacheLogs(picked, ids, logs)

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
	writeCacheLogs(b, in.Caches)
	writeFlakyJobs(b, in.Failing)
}

func writeFlakyJobs(b *strings.Builder, failing []ciJobFailure) {
	writeFlaky(b, nil, failing)
}

const (
	gateLastShare    = 0.25
	gateHeadroom     = 15
	gateDetectRounds = 3
)

// detectGateJobs names the jobs that end a quarter or more of the sampled
// pushes yet finish within gateHeadroom seconds of the job before them, which
// is a job that only waits on others (a gate or an aggregator). Each round
// leaves the names found out and looks again, because the job behind a gate
// may be another aggregator.
func detectGateJobs(pushes []ciTiming, jobs runcache.Jobs) []string {
	var found []string

	for range gateDetectRounds {
		lasts := make(map[string][]float64)

		for c := range pushes {
			all := pushJobs(&pushes[c], jobs, found)
			if len(all) == 0 {
				continue
			}

			headroom := 0.0
			if len(all) > 1 {
				headroom = all[0].completed.Sub(all[1].completed).Seconds()
			}

			lasts[all[0].name] = append(lasts[all[0].name], headroom)
		}

		before := len(found)

		for name, headrooms := range lasts {
			if float64(len(headrooms)) >= gateLastShare*float64(len(pushes)) &&
				percentile(headrooms, percentileMedian) <= gateHeadroom {
				found = append(found, name)
			}
		}

		if len(found) == before {
			break
		}
	}

	sort.Strings(found)

	return found
}
