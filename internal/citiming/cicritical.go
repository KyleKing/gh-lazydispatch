package citiming

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	agstats "github.com/kyleking/aragonite/stats"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

const (
	topCritical = 4
	topRunners  = 4
	topCaches   = 3
	skipped     = "skipped"
)

// ciCriticalJob is a job that finished last in some sampled pushes. Headroom is
// how much sooner the push would have ended had the job been instant, which is
// the gap to the next job to finish.
type ciCriticalJob struct {
	Workflow       string `json:"workflow"`
	Job            string `json:"job"`
	Runner         string `json:"runner"`
	headroomValues []float64
	queueValues    []float64
	runValues      []float64
	Pushes         int     `json:"pushes"`
	LastShare      float64 `json:"last_share"`
	P50Headroom    float64 `json:"p50_headroom_seconds"`
	TotalHeadroom  float64 `json:"total_headroom_seconds"`
	P50Queue       float64 `json:"p50_queue_seconds"`
	P50Seconds     float64 `json:"p50_seconds"`
}

// ciRunnerQueue is how long jobs on one runner label waited to start.
type ciRunnerQueue struct {
	Runner     string  `json:"runner"`
	Jobs       int     `json:"jobs"`
	P50Seconds float64 `json:"p50_queue_seconds"`
	P90Seconds float64 `json:"p90_queue_seconds"`
}

// ciCacheStep is a step whose name mentions a cache. The API does not report
// hits, so the time and skip share stand in for it.
type ciCacheStep struct {
	Name        string  `json:"name"`
	Steps       int     `json:"steps"`
	SkippedRate float64 `json:"skipped_rate"`
	P50Seconds  float64 `json:"p50_seconds"`
	P90Seconds  float64 `json:"p90_seconds"`
	total       float64
}

// ciCriticalSpan splits the wall clock of the sampled pushes of one epoch into
// where it went, by the job that finished last.
type ciCriticalSpan struct {
	Start         string          `json:"start"`
	Jobs          []ciCriticalJob `json:"critical_jobs"`
	Runners       []ciRunnerQueue `json:"runner_queue"`
	Caches        []ciCacheStep   `json:"cache_steps"`
	Pushes        int             `json:"pushes"`
	Sampled       int             `json:"sampled"`
	P50Wall       float64         `json:"p50_wall_seconds"`
	P90Wall       float64         `json:"p90_wall_seconds"`
	MeanUpstream  float64         `json:"mean_waiting_on_jobs_seconds"`
	MeanQueue     float64         `json:"mean_runner_queue_seconds"`
	MeanRun       float64         `json:"mean_running_seconds"`
	MeanAfterLast float64         `json:"mean_after_last_job_seconds"`
}

type pathJob struct {
	created, started, completed time.Time
	workflow, name, runner      string
	steps                       []github.StepTiming
}

// sampledPushes returns at most n counted pushes per
// span, chosen by a hash of each push's first run so the same pushes are read
// as the range grows.
func sampledPushes(commits []ciTiming, spans []timeWindow, n int) [][]ciTiming {
	picked := make([][]ciTiming, len(spans))

	for i, span := range spans {
		in := counted(commitsBetween(commits, span.from, span.to))
		if len(in) == 0 {
			continue
		}

		firstRun := make(map[int64]int, len(in))
		ids := make([]int64, 0, len(in))

		for c := range in {
			id := in[c].RunDetails[0].ID
			firstRun[id] = c
			ids = append(ids, id)
		}

		for _, id := range sampleRuns(ids, n) {
			picked[i] = append(picked[i], in[firstRun[id]])
		}
	}

	return picked
}

func pushRunIDs(pushes [][]ciTiming) []int64 {
	var ids []int64

	for _, span := range pushes {
		for c := range span {
			for _, run := range span[c].RunDetails {
				if !run.Retrigger && run.Conclusion != conclusionCanceled {
					ids = append(ids, run.ID)
				}
			}
		}
	}

	return ids
}

func pushJobs(c *ciTiming, jobs runcache.Jobs, ignore []string) []pathJob {
	var out []pathJob

	for _, run := range c.RunDetails {
		if run.Retrigger || run.Conclusion == conclusionCanceled {
			continue
		}

		for k := range jobs[run.ID] {
			job := &jobs[run.ID][k]
			if !countable(job.Conclusion) || job.StartedAt.IsZero() || job.CompletedAt.IsZero() ||
				slices.Contains(ignore, job.Name) {
				continue
			}

			out = append(out, pathJob{
				workflow:  run.Name,
				name:      job.Name,
				runner:    strings.Join(job.Labels, ","),
				created:   job.CreatedAt,
				started:   job.StartedAt,
				completed: job.CompletedAt,
				steps:     job.Steps,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].completed.After(out[j].completed) })

	return out
}

type spanSamples struct {
	critical map[string]*ciCriticalJob
	runners  map[string][]float64
	caches   map[string]*cacheSamples
	wall     []float64
	upstream []float64
	queue    []float64
	run      []float64
	after    []float64
	sampled  int
}

type cacheSamples struct {
	durations []float64
	steps     int
	skipped   int
}

func summarizeCritical(
	pushes [][]ciTiming, spans []timeWindow, commits []ciTiming, jobs runcache.Jobs, ignore []string,
) []ciCriticalSpan {
	out := make([]ciCriticalSpan, 0, len(spans))

	for i, span := range spans {
		s := &spanSamples{
			critical: make(map[string]*ciCriticalJob),
			runners:  make(map[string][]float64),
			caches:   make(map[string]*cacheSamples),
		}

		for c := range pushes[i] {
			s.add(&pushes[i][c], jobs, ignore)
		}

		out = append(out, s.span(span, len(counted(commitsBetween(commits, span.from, span.to)))))
	}

	return out
}

func (s *spanSamples) add(c *ciTiming, jobs runcache.Jobs, ignore []string) {
	all := pushJobs(c, jobs, ignore)
	if len(all) == 0 {
		return
	}

	start, errStart := time.Parse(timeFormat, c.StartedAt)
	finish, errFinish := time.Parse(timeFormat, c.FinishedAt)

	if errStart != nil || errFinish != nil {
		return
	}

	last := all[0]
	headroom := 0.0

	if len(all) > 1 {
		headroom = last.completed.Sub(all[1].completed).Seconds()
	}

	s.sampled++
	s.wall = append(s.wall, c.Seconds)
	s.upstream = append(s.upstream, nonNegative(last.created.Sub(start)))
	s.queue = append(s.queue, nonNegative(last.started.Sub(last.created)))
	s.run = append(s.run, nonNegative(last.completed.Sub(last.started)))
	s.after = append(s.after, nonNegative(finish.Sub(last.completed)))

	key := last.workflow + "/" + last.name

	job := s.critical[key]
	if job == nil {
		job = &ciCriticalJob{Workflow: last.workflow, Job: last.name, Runner: last.runner}
		s.critical[key] = job
	}

	job.Pushes++
	job.TotalHeadroom += headroom
	job.headroomValues = append(job.headroomValues, headroom)
	job.queueValues = append(job.queueValues, last.started.Sub(last.created).Seconds())
	job.runValues = append(job.runValues, last.completed.Sub(last.started).Seconds())

	for i := range all {
		s.runners[all[i].runner] = append(s.runners[all[i].runner], nonNegative(all[i].started.Sub(all[i].created)))
		s.addCacheSteps(all[i].steps)
	}
}

func (s *spanSamples) addCacheSteps(steps []github.StepTiming) {
	for i := range steps {
		step := &steps[i]
		if !strings.Contains(strings.ToLower(step.Name), "cache") {
			continue
		}

		c := s.caches[step.Name]
		if c == nil {
			c = &cacheSamples{}
			s.caches[step.Name] = c
		}

		c.steps++

		switch {
		case step.Conclusion == skipped:
			c.skipped++
		case step.Conclusion == github.ConclusionSuccess && !step.StartedAt.IsZero() && !step.CompletedAt.IsZero():
			c.durations = append(c.durations, step.CompletedAt.Sub(step.StartedAt).Seconds())
		}
	}
}

func nonNegative(d time.Duration) float64 {
	return max(d.Seconds(), 0)
}

func mean(values []float64) float64 {
	return agstats.Mean(values)
}

func (s *spanSamples) span(span timeWindow, pushes int) ciCriticalSpan {
	out := ciCriticalSpan{
		Start:         span.label,
		Pushes:        pushes,
		Sampled:       s.sampled,
		P50Wall:       percentile(s.wall, percentileMedian),
		P90Wall:       percentile(s.wall, percentileP90),
		MeanUpstream:  mean(s.upstream),
		MeanQueue:     mean(s.queue),
		MeanRun:       mean(s.run),
		MeanAfterLast: mean(s.after),
	}

	for _, job := range s.critical {
		job.LastShare = float64(job.Pushes) / float64(s.sampled)
		job.P50Headroom = percentile(job.headroomValues, percentileMedian)
		job.P50Queue = percentile(job.queueValues, percentileMedian)
		job.P50Seconds = percentile(job.runValues, percentileMedian)
		out.Jobs = append(out.Jobs, *job)
	}

	sort.Slice(out.Jobs, func(i, j int) bool {
		if out.Jobs[i].TotalHeadroom != out.Jobs[j].TotalHeadroom {
			return out.Jobs[i].TotalHeadroom > out.Jobs[j].TotalHeadroom
		}

		return out.Jobs[i].Job < out.Jobs[j].Job
	})

	for runner, queue := range s.runners {
		out.Runners = append(out.Runners, ciRunnerQueue{
			Runner:     runner,
			Jobs:       len(queue),
			P50Seconds: percentile(queue, percentileMedian),
			P90Seconds: percentile(queue, percentileP90),
		})
	}

	sort.Slice(out.Runners, func(i, j int) bool {
		if out.Runners[i].Jobs != out.Runners[j].Jobs {
			return out.Runners[i].Jobs > out.Runners[j].Jobs
		}

		return out.Runners[i].Runner < out.Runners[j].Runner
	})

	for name, c := range s.caches {
		var total float64
		for _, d := range c.durations {
			total += d
		}

		out.Caches = append(out.Caches, ciCacheStep{
			Name:        name,
			Steps:       c.steps,
			SkippedRate: float64(c.skipped) / float64(c.steps),
			P50Seconds:  percentile(c.durations, percentileMedian),
			P90Seconds:  percentile(c.durations, percentileP90),
			total:       total,
		})
	}

	sort.Slice(out.Caches, func(i, j int) bool {
		if out.Caches[i].total != out.Caches[j].total {
			return out.Caches[i].total > out.Caches[j].total
		}

		return out.Caches[i].Name < out.Caches[j].Name
	})

	return out
}

func writeCriticalTables(b *strings.Builder, spans []ciCriticalSpan) {
	if len(spans) == 0 {
		return
	}

	writeTableHead(b, "Where the wall clock goes (sampled pushes; mean seconds on the path of the last job to finish).",
		"<Epoch", "Sampled", "Wall P50", "Wall P90", "Waiting on earlier jobs", "Runner queue",
		"Last job running", "After last job")

	for i := range spans {
		s := &spans[i]
		fmt.Fprintf(b, "| %s | %d of %d | %s | %s | %s | %s | %s | %s |\n", s.Start, s.Sampled, s.Pushes,
			formatSeconds(s.P50Wall), formatSeconds(s.P90Wall), formatSeconds(s.MeanUpstream),
			formatSeconds(s.MeanQueue), formatSeconds(s.MeanRun), formatSeconds(s.MeanAfterLast))
	}

	writeTableHead(b, "Jobs that end the push, by total headroom (time saved in the sample if the job were instant).",
		"<Epoch", "<Workflow / job", "<Runner", "Last in", "Headroom P50", "Headroom total", "Queue P50", "Run P50")

	for i := range spans {
		for k := range spans[i].Jobs {
			if k == topCritical {
				break
			}

			j := &spans[i].Jobs[k]
			fmt.Fprintf(b, "| %s | %s / %s | %s | %.0f%% | %s | %s | %s | %s |\n", spans[i].Start, j.Workflow, j.Job,
				j.Runner, j.LastShare*percentScale, formatSeconds(j.P50Headroom), formatSeconds(j.TotalHeadroom),
				formatSeconds(j.P50Queue), formatSeconds(j.P50Seconds))
		}
	}

	writeRunnerTable(b, spans)
	writeCacheTable(b, spans)
}

func writeRunnerTable(b *strings.Builder, spans []ciCriticalSpan) {
	writeTableHead(b, "Runner queue wait (every job of the sampled pushes).",
		"<Epoch", "<Runner", "Jobs", "Queue P50", "Queue P90")

	for i := range spans {
		for k := range spans[i].Runners {
			if k == topRunners {
				break
			}

			r := &spans[i].Runners[k]
			fmt.Fprintf(b, "| %s | %s | %d | %s | %s |\n", spans[i].Start, r.Runner, r.Jobs,
				formatSeconds(r.P50Seconds), formatSeconds(r.P90Seconds))
		}
	}
}

func writeCacheTable(b *strings.Builder, spans []ciCriticalSpan) {
	writeTableHead(b, "Cache steps (name contains \"cache\"; the API reports time and skips, not hits).",
		"<Epoch", "<Step", "Steps", "Skipped", "P50", "P90")

	for i := range spans {
		for k := range spans[i].Caches {
			if k == topCaches {
				break
			}

			c := &spans[i].Caches[k]
			fmt.Fprintf(b, "| %s | %s | %d | %.0f%% | %s | %s |\n", spans[i].Start, c.Name, c.Steps,
				c.SkippedRate*percentScale, formatSeconds(c.P50Seconds), formatSeconds(c.P90Seconds))
		}
	}
}
