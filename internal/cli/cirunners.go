package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

const (
	minRunnerSamples = 3
	topRunnerJobs    = 12
)

// ciRunnerRow is one job on one runner in one phase around a runner swap.
type ciRunnerRow struct {
	Job         string  `json:"job"`
	Phase       string  `json:"phase"`
	Runner      string  `json:"runner"`
	Jobs        int     `json:"jobs"`
	FailureRate float64 `json:"failure_rate"`
	P50Queue    float64 `json:"p50_queue_seconds"`
	P90Queue    float64 `json:"p90_queue_seconds"`
	P50Seconds  float64 `json:"p50_seconds"`
	P90Seconds  float64 `json:"p90_seconds"`
}

// ciRunnerComparison lines up the jobs that ran on a vendor's runners against
// the same jobs on the usual runners before, during, and after that trial.
type ciRunnerComparison struct {
	start  time.Time
	end    time.Time
	Vendor string        `json:"vendor"`
	From   string        `json:"from"`
	To     string        `json:"to"`
	Rows   []ciRunnerRow `json:"rows"`
}

type phaseKey struct{ job, phase, runner string }

type phaseSamples struct {
	queue, run []float64
	jobs, fail int
}

func phaseOf(at, from, to time.Time) string {
	switch {
	case at.Before(from):
		return "1 before"
	case at.After(to):
		return "3 after"
	default:
		return "2 during"
	}
}

// vendorSpan returns when the vendor's runners were in use and how often each
// job ran on them.
func vendorSpan(recs []jobRec, vendor string) (time.Time, time.Time, map[string]int) {
	var from, to time.Time

	onVendor := make(map[string]int)

	for i := range recs {
		r := &recs[i]
		if !r.ran() || runnerFamily(r.runner) != vendor {
			continue
		}

		onVendor[r.label()]++

		if from.IsZero() || r.started.Before(from) {
			from = r.started
		}

		if r.started.After(to) {
			to = r.started
		}
	}

	return from, to, onVendor
}

func (g *phaseSamples) add(r *jobRec) {
	g.jobs++
	g.queue = append(g.queue, r.queue())
	g.run = append(g.run, r.seconds())

	if r.conclusion != github.ConclusionSuccess {
		g.fail++
	}
}

func (g *phaseSamples) row(key phaseKey) ciRunnerRow {
	return ciRunnerRow{
		Job: key.job, Phase: key.phase, Runner: key.runner, Jobs: g.jobs,
		FailureRate: float64(g.fail) / float64(g.jobs),
		P50Queue:    percentile(g.queue, percentileMedian), P90Queue: percentile(g.queue, percentileP90),
		P50Seconds: percentile(g.run, percentileMedian), P90Seconds: percentile(g.run, percentileP90),
	}
}

// compareRunners finds the span the vendor's runners were in use and compares
// each job seen on them (at least minRunnerSamples times) across phases.
func compareRunners(recs []jobRec, vendor string) *ciRunnerComparison {
	from, to, onVendor := vendorSpan(recs, vendor)
	if from.IsZero() {
		return nil
	}

	groups := make(map[phaseKey]*phaseSamples)

	for i := range recs {
		r := &recs[i]
		if !r.ran() || onVendor[r.label()] < minRunnerSamples {
			continue
		}

		key := phaseKey{job: r.label(), phase: phaseOf(r.started, from, to), runner: runnerFamily(r.runner)}
		if groups[key] == nil {
			groups[key] = &phaseSamples{}
		}

		groups[key].add(r)
	}

	out := &ciRunnerComparison{
		Vendor: vendor, From: from.Format(time.DateOnly), To: to.Format(time.DateOnly), start: from, end: to,
	}

	for key, g := range groups {
		out.Rows = append(out.Rows, g.row(key))
	}

	sort.Slice(out.Rows, func(i, j int) bool {
		a, b := &out.Rows[i], &out.Rows[j]
		if a.Job != b.Job {
			return onVendor[a.Job] > onVendor[b.Job] || (onVendor[a.Job] == onVendor[b.Job] && a.Job < b.Job)
		}

		return a.Phase+a.Runner < b.Phase+b.Runner
	})

	return out
}

// familyRows is the all-jobs view of the same trial: queue and run time by
// vendor and phase.
func familyRows(recs []jobRec, cmp *ciRunnerComparison) []ciRunnerRow {
	if cmp == nil {
		return nil
	}

	end := cmp.end.Add(hoursPerDay * time.Hour)
	groups := make(map[phaseKey]*phaseSamples)

	for i := range recs {
		r := &recs[i]
		if !r.ran() {
			continue
		}

		key := phaseKey{job: "all jobs", phase: phaseOf(r.started, cmp.start, end), runner: runnerFamily(r.runner)}
		if groups[key] == nil {
			groups[key] = &phaseSamples{}
		}

		groups[key].add(r)
	}

	var out []ciRunnerRow

	for key, g := range groups {
		out = append(out, g.row(key))
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Phase+out[i].Runner < out[j].Phase+out[j].Runner })

	return out
}

func writeRunnerComparison(b *strings.Builder, cmp *ciRunnerComparison, families []ciRunnerRow) {
	if cmp == nil {
		return
	}

	writeTableHead(b, fmt.Sprintf("%s runners were used from %s to %s. Queue and run time by vendor, all jobs.",
		cmp.Vendor, cmp.From, cmp.To), "<Phase", "<Runner", "Jobs", "Queue P50", "Queue P90", "Run P50", "Run P90")

	for i := range families {
		r := &families[i]
		fmt.Fprintf(b, "| %s | %s | %d | %s | %s | %s | %s |\n", r.Phase[2:], r.Runner, r.Jobs,
			formatSeconds(r.P50Queue), formatSeconds(r.P90Queue),
			formatSeconds(r.P50Seconds), formatSeconds(r.P90Seconds))
	}

	writeTableHead(b, "The same jobs before, during, and after (jobs seen on the vendor's runners at least 3 times).",
		"<Job", "<Phase", "<Runner", "Jobs", "Failed", "Queue P50", "Run P50", "Run P90")

	seen := make(map[string]bool)

	for i := range cmp.Rows {
		r := &cmp.Rows[i]

		if !seen[r.Job] {
			if len(seen) == topRunnerJobs {
				break
			}

			seen[r.Job] = true
		}

		fmt.Fprintf(b, "| %s | %s | %s | %d | %.0f%% | %s | %s | %s |\n", r.Job, r.Phase[2:], r.Runner, r.Jobs,
			r.FailureRate*percentScale, formatSeconds(r.P50Queue),
			formatSeconds(r.P50Seconds), formatSeconds(r.P90Seconds))
	}
}
