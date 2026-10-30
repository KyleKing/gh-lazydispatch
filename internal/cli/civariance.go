package cli

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

const (
	minVarianceRuns = 8
	topVariable     = 10
	// A cache post step runs cacheSaveSeconds or longer when it uploads an entry, which follows a miss.
	cacheSaveSeconds = 3
	slowFactor       = 2
	minCacheGroup    = 3
)

// ciVariableJob is a job whose run time spreads widely, with the step that
// accounts for most of the spread and what a cache save in the job says about
// misses. Spread is P90 minus P50.
type ciVariableJob struct {
	Job          string  `json:"job"`
	WidestStep   string  `json:"widest_step,omitempty"`
	Runs         int     `json:"runs"`
	P50Seconds   float64 `json:"p50_seconds"`
	P90Seconds   float64 `json:"p90_seconds"`
	SlowShare    float64 `json:"slow_share"`
	StepP50      float64 `json:"widest_step_p50_seconds,omitempty"`
	StepP90      float64 `json:"widest_step_p90_seconds,omitempty"`
	MissShare    float64 `json:"cache_save_share,omitempty"`
	P50AfterHit  float64 `json:"p50_without_save_seconds,omitempty"`
	P50AfterMiss float64 `json:"p50_with_save_seconds,omitempty"`
}

func variableJobs(recs []jobRec) []ciVariableJob {
	byJob := make(map[string][]*jobRec)

	for i := range recs {
		if recs[i].ran() {
			byJob[recs[i].label()] = append(byJob[recs[i].label()], &recs[i])
		}
	}

	var out []ciVariableJob

	for label, group := range byJob {
		if len(group) < minVarianceRuns {
			continue
		}

		out = append(out, variability(label, group))
	}

	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].P90Seconds-out[i].P50Seconds, out[j].P90Seconds-out[j].P50Seconds
		if a != b {
			return a > b
		}

		return out[i].Job < out[j].Job
	})

	if len(out) > topVariable {
		out = out[:topVariable]
	}

	return out
}

func variability(label string, group []*jobRec) ciVariableJob {
	durations := make([]float64, len(group))
	for i, r := range group {
		durations[i] = r.seconds()
	}

	p50 := percentile(durations, percentileMedian)
	slow := 0

	for _, d := range durations {
		if d > slowFactor*p50 {
			slow++
		}
	}

	job := ciVariableJob{
		Job: label, Runs: len(group), P50Seconds: p50, P90Seconds: percentile(durations, percentileP90),
		SlowShare: float64(slow) / float64(len(group)),
	}

	job.WidestStep, job.StepP50, job.StepP90 = widestStep(group)
	job.MissShare, job.P50AfterHit, job.P50AfterMiss = cacheSaves(group)

	return job
}

func widestStep(group []*jobRec) (string, float64, float64) {
	steps := make(map[string][]float64)

	for _, r := range group {
		for i := range r.steps {
			s := &r.steps[i]
			if s.Conclusion == github.ConclusionSuccess && !s.StartedAt.IsZero() && !s.CompletedAt.IsZero() {
				steps[s.Name] = append(steps[s.Name], s.CompletedAt.Sub(s.StartedAt).Seconds())
			}
		}
	}

	var (
		name     string
		p50, p90 float64
	)

	for step, d := range steps {
		if len(d) < minVarianceRuns {
			continue
		}

		lo, hi := percentile(d, percentileMedian), percentile(d, percentileP90)
		if hi-lo > p90-p50 || (hi-lo == p90-p50 && step < name) {
			name, p50, p90 = step, lo, hi
		}
	}

	return name, p50, p90
}

// cacheSaves splits a job's runs by whether a cache post step uploaded an
// entry, which an action only does after a miss. It reports 0s when the job
// has no such step or one side has too few runs to compare.
func cacheSaves(group []*jobRec) (float64, float64, float64) {
	var saved, kept []float64

	for _, r := range group {
		hasCache, didSave := false, false

		for i := range r.steps {
			s := &r.steps[i]
			if strings.HasPrefix(s.Name, "Post ") && strings.Contains(strings.ToLower(s.Name), "cache") &&
				s.Conclusion == github.ConclusionSuccess {
				hasCache = true
				didSave = didSave || s.CompletedAt.Sub(s.StartedAt) >= cacheSaveSeconds*time.Second
			}
		}

		switch {
		case !hasCache:
			continue
		case didSave:
			saved = append(saved, r.seconds())
		default:
			kept = append(kept, r.seconds())
		}
	}

	if len(saved)+len(kept) == 0 {
		return 0, 0, 0
	}

	share := float64(len(saved)) / float64(len(saved)+len(kept))
	if len(saved) < minCacheGroup || len(kept) < minCacheGroup {
		return share, 0, 0
	}

	return share, percentile(kept, percentileMedian), percentile(saved, percentileMedian)
}

func writeVariableJobs(b *strings.Builder, jobs []ciVariableJob) {
	if len(jobs) == 0 {
		return
	}

	writeTableHead(b, "Jobs with the widest run-time spread (P90 minus P50). Slow is the share of runs over twice the "+
		"median. Cache saves is how often a cache post step uploaded an entry, which follows a miss.",
		"<Job", "Runs", "P50", "P90", "Slow", "<Widest step", "Step P50", "Step P90",
		"Cache saves", "Run P50 no save", "Run P50 save")

	for i := range jobs {
		j := &jobs[i]
		save, noSave, withSave := "-", "-", "-"

		if j.MissShare > 0 || j.P50AfterHit > 0 {
			save = fmt.Sprintf("%.0f%%", j.MissShare*percentScale)
		}

		if j.P50AfterHit > 0 {
			noSave, withSave = formatSeconds(j.P50AfterHit), formatSeconds(j.P50AfterMiss)
		}

		fmt.Fprintf(b, "| %s | %d | %s | %s | %.0f%% | %s | %s | %s | %s | %s | %s |\n", j.Job, j.Runs,
			formatSeconds(j.P50Seconds), formatSeconds(j.P90Seconds), j.SlowShare*percentScale, j.WidestStep,
			formatSeconds(j.StepP50), formatSeconds(j.StepP90), save, noSave, withSave)
	}
}

// ciSpend is the runner time one epoch's sampled pushes consumed.
type ciSpend struct {
	Start         string  `json:"start"`
	Pushes        int     `json:"pushes"`
	JobsPerPush   float64 `json:"jobs_per_push"`
	SkippedShare  float64 `json:"skipped_share"`
	RunnerMinutes float64 `json:"runner_minutes_per_push"`
	BilledMinutes float64 `json:"billed_minutes_per_push"`
}

// ciSkipped is a job that mostly does not run.
type ciSkipped struct {
	Job   string  `json:"job"`
	Jobs  int     `json:"jobs"`
	Share float64 `json:"skipped_share"`
}

func summarizeSpend(recs []jobRec, spans []timeWindow) []ciSpend {
	out := make([]ciSpend, 0, len(spans))

	for _, span := range spans {
		in := recsBetween(recs, span.from, span.to)
		pushes := make(map[string]bool)

		var skippedJobs int

		var seconds, billed float64

		for i := range in {
			pushes[in[i].push.HeadSHA] = true

			if in[i].conclusion == "skipped" {
				skippedJobs++

				continue
			}

			if in[i].ran() {
				seconds += in[i].seconds()
				billed += math.Ceil(in[i].seconds() / secondsPerMinute)
			}
		}

		n := float64(max(len(pushes), 1))
		out = append(out, ciSpend{
			Start: span.label, Pushes: len(pushes), JobsPerPush: float64(len(in)) / n,
			SkippedShare:  float64(skippedJobs) / float64(max(len(in), 1)),
			RunnerMinutes: seconds / secondsPerMinute / n, BilledMinutes: billed / n,
		})
	}

	return out
}

const (
	secondsPerMinute = 60
	topSkipped       = 8
)

func skippedJobs(recs []jobRec) []ciSkipped {
	counts := make(map[string][2]int)

	for i := range recs {
		c := counts[recs[i].label()]
		c[0]++

		if recs[i].conclusion == "skipped" {
			c[1]++
		}

		counts[recs[i].label()] = c
	}

	var out []ciSkipped

	for label, c := range counts {
		if c[1] > 0 && c[0] >= minVarianceRuns {
			out = append(out, ciSkipped{Job: label, Jobs: c[0], Share: float64(c[1]) / float64(c[0])})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Share != out[j].Share {
			return out[i].Share > out[j].Share
		}

		return out[i].Job < out[j].Job
	})

	if len(out) > topSkipped {
		out = out[:topSkipped]
	}

	return out
}

func writeSpend(b *strings.Builder, spend []ciSpend, skipped []ciSkipped) {
	if len(spend) == 0 {
		return
	}

	writeTableHead(b, "Runner time per push (sampled pushes; billed minutes round each job up to a whole minute).",
		"<Epoch", "Pushes", "Jobs per push", "Skipped", "Runner min", "Billed min")

	for i := range spend {
		s := &spend[i]
		fmt.Fprintf(b, "| %s | %d | %.1f | %.0f%% | %.1f | %.1f |\n", s.Start, s.Pushes, s.JobsPerPush,
			s.SkippedShare*percentScale, s.RunnerMinutes, s.BilledMinutes)
	}

	writeTableHead(b, "Jobs skipped most often (an `if:` or a path filter kept them from running).",
		"<Job", "Jobs", "Skipped")

	for i := range skipped {
		fmt.Fprintf(b, "| %s | %d | %.0f%% |\n", skipped[i].Job, skipped[i].Jobs, skipped[i].Share*percentScale)
	}
}
