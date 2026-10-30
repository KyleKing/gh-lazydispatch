package citiming

import (
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

const (
	topCacheJobs     = 8
	logFetchParallel = 4
)

var (
	cacheHitLine  = regexp.MustCompile(`(?i)cache restored (from key|successfully)|cache hit occurred|cache hit for`)
	cacheMissLine = regexp.MustCompile(`(?i)cache not found for input keys|no (github actions )?cache found|cache miss`)
)

// classifyCacheLog counts the cache restores and misses a job log reports. One
// restore prints two matching lines in actions/cache, so a restore counts once
// per "restored from key" line when present.
func classifyCacheLog(log string) runcache.CacheLog {
	var out runcache.CacheLog

	for line := range strings.SplitSeq(log, "\n") {
		switch {
		case cacheMissLine.MatchString(line):
			out.Misses++
		case strings.Contains(strings.ToLower(line), "cache restored from key") ||
			(cacheHitLine.MatchString(line) && !strings.Contains(strings.ToLower(line), "restored successfully")):
			out.Hits++
		}
	}

	return out
}

// ciCacheOutcome is what the logs of one job's sampled runs say about its
// caches: in how many a restore missed, and what the run cost with and without.
type ciCacheOutcome struct {
	Job         string `json:"job"`
	allHit      []float64
	missed      []float64
	Logs        int     `json:"logs"`
	WithMiss    int     `json:"runs_with_a_miss"`
	P50AllHit   float64 `json:"p50_all_hit_seconds"`
	P50WithMiss float64 `json:"p50_with_miss_seconds"`
}

// cacheJobCandidates picks, for the busiest jobs that have a cache step, up to
// perJob job IDs each, chosen by a hash of the ID so the same logs return.
func cacheJobCandidates(recs []jobRec, ids map[int64]*jobRec, perJob int) map[string][]int64 {
	byJob := make(map[string][]int64)

	for i := range recs {
		r := &recs[i]
		if !r.ran() || r.jobID == 0 || !hasCacheStep(r.steps) {
			continue
		}

		byJob[r.label()] = append(byJob[r.label()], r.jobID)
		ids[r.jobID] = r
	}

	labels := make([]string, 0, len(byJob))
	for label := range byJob {
		labels = append(labels, label)
	}

	sort.Slice(labels, func(i, j int) bool {
		if len(byJob[labels[i]]) != len(byJob[labels[j]]) {
			return len(byJob[labels[i]]) > len(byJob[labels[j]])
		}

		return labels[i] < labels[j]
	})

	if len(labels) > topCacheJobs {
		labels = labels[:topCacheJobs]
	}

	picked := make(map[string][]int64, len(labels))
	for _, label := range labels {
		picked[label] = sampleRuns(byJob[label], perJob)
	}

	return picked
}

func hasCacheStep(steps []github.StepTiming) bool {
	for i := range steps {
		if strings.Contains(strings.ToLower(steps[i].Name), "cache") {
			return true
		}
	}

	return false
}

// readCacheLogs fetches the logs of the picked jobs that the cache file lacks,
// stopping at the first failure and keeping what was read.
func readCacheLogs(
	client *github.Client, picked map[string][]int64, stderr io.Writer,
) (runcache.CacheLogs, error) {
	path := runcache.CacheLogsPath(client.Owner() + "/" + client.Repo())

	cached, err := runcache.LoadCacheLogs(path)
	if err != nil {
		return nil, fmt.Errorf("loading the cache-log file: %w", err)
	}

	var missing []int64

	for _, ids := range picked {
		for _, id := range ids {
			if _, ok := cached[id]; !ok {
				missing = append(missing, id)
			}
		}
	}

	notef(stderr, "%d job logs to read for cache hits, %d cached\n", len(missing), len(cached))

	missing, shortfall := affordable(missing, stderr)

	var (
		mu    sync.Mutex
		group errgroup.Group
	)

	group.SetLimit(logFetchParallel)

	for _, id := range missing {
		group.Go(func() error {
			log, err := client.JobLog(id)
			if err != nil {
				return err //nolint:wrapcheck // already names the job
			}

			mu.Lock()
			cached[id] = classifyCacheLog(log)
			mu.Unlock()

			return nil
		})
	}

	fetchErr := group.Wait()

	if len(missing) > 0 {
		if err := runcache.SaveCacheLogs(path, cached); err != nil {
			notef(stderr, "warning: the cache-log file was not saved: %v\n", err)
		}
	}

	if fetchErr != nil {
		return nil, fmt.Errorf("reading job logs (rerun to resume): %w", fetchErr)
	}

	if shortfall != nil {
		return nil, shortfall
	}

	return cached, nil
}

func summarizeCacheLogs(picked map[string][]int64, ids map[int64]*jobRec, logs runcache.CacheLogs) []ciCacheOutcome {
	out := make([]ciCacheOutcome, 0, len(picked))

	for label, jobIDs := range picked {
		o := ciCacheOutcome{Job: label}

		for _, id := range jobIDs {
			log, ok := logs[id]
			if !ok || log.Hits+log.Misses == 0 {
				continue
			}

			o.Logs++

			if log.Misses > 0 {
				o.WithMiss++
				o.missed = append(o.missed, ids[id].seconds())
			} else {
				o.allHit = append(o.allHit, ids[id].seconds())
			}
		}

		if o.Logs > 0 {
			o.P50AllHit = percentile(o.allHit, percentileMedian)
			o.P50WithMiss = percentile(o.missed, percentileMedian)
			out = append(out, o)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].WithMiss != out[j].WithMiss {
			return out[i].WithMiss > out[j].WithMiss
		}

		return out[i].Job < out[j].Job
	})

	return out
}

func writeCacheLogs(b *strings.Builder, outcomes []ciCacheOutcome) {
	if len(outcomes) == 0 {
		return
	}

	writeTableHead(b, "Cache restores read from job logs (a sample of each job's runs).",
		"<Job", "Logs", "With a miss", "Run P50 all hit", "Run P50 with a miss")

	for i := range outcomes {
		o := &outcomes[i]

		hit, miss := "-", "-"
		if len(o.allHit) > 0 {
			hit = formatSeconds(o.P50AllHit)
		}

		if len(o.missed) > 0 {
			miss = formatSeconds(o.P50WithMiss)
		}

		fmt.Fprintf(b, "| %s | %d | %d (%.0f%%) | %s | %s |\n", o.Job, o.Logs, o.WithMiss,
			float64(o.WithMiss)/float64(o.Logs)*percentScale, hit, miss)
	}
}
