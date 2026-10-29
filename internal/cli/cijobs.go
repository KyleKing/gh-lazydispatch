package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

const (
	jobFetchConcurrency = 4
	topSteps            = 15
)

// ciJob is one job of the drilled-down workflow, across its counted runs.
// QueueSeconds is how long GitHub took to hand the job to a runner.
type ciJob struct {
	Name            string   `json:"name"`
	Runners         []string `json:"runners"`
	Runs            int      `json:"runs"`
	P50Queue        float64  `json:"p50_queue_seconds"`
	P90Queue        float64  `json:"p90_queue_seconds"`
	P50Seconds      float64  `json:"p50_seconds"`
	P90Seconds      float64  `json:"p90_seconds"`
	P90FinishedPush float64  `json:"p90_finished_after_seconds"`
}

// ciStep is one step of one job, across the workflow's counted runs.
type ciStep struct {
	Job        string  `json:"job"`
	Name       string  `json:"name"`
	Runs       int     `json:"runs"`
	P50Seconds float64 `json:"p50_seconds"`
	P90Seconds float64 `json:"p90_seconds"`
}

// workflowRunIDs lists the runs of workflow that a job rollup reads: those in
// counted commits that were not canceled, whose timings are cut short.
func workflowRunIDs(commits []ciTiming, workflow string) []int64 {
	var ids []int64

	for i := range commits {
		for _, run := range commits[i].RunDetails {
			if run.Name == workflow && run.Conclusion != conclusionCanceled {
				ids = append(ids, run.ID)
			}
		}
	}

	return ids
}

// loadJobs returns the jobs of every run in ids, fetching the ones the cache
// lacks.
func loadJobs(client *github.Client, ids []int64, stderr io.Writer) (runcache.Jobs, error) {
	path := runcache.JobsPath(client.Owner() + "/" + client.Repo())

	cached, err := runcache.LoadJobs(path)
	if err != nil {
		return nil, fmt.Errorf("loading the job cache: %w", err)
	}

	var missing []int64

	for _, id := range ids {
		if _, ok := cached[id]; !ok {
			missing = append(missing, id)
		}
	}

	notef(stderr, "%d runs need their jobs fetched, %d cached (%s)\n", len(missing), len(ids)-len(missing), path)

	fetched := make([][]github.JobTiming, len(missing))

	var group errgroup.Group

	group.SetLimit(jobFetchConcurrency)

	for i, id := range missing {
		group.Go(func() error {
			jobs, err := client.ListRunJobTimings(id)
			if err != nil {
				return fmt.Errorf("fetching jobs: %w", err)
			}

			fetched[i] = jobs

			return nil
		})
	}

	if err := group.Wait(); err != nil {
		return nil, fmt.Errorf("listing jobs: %w", err)
	}

	for i, id := range missing {
		cached[id] = fetched[i]
	}

	if len(missing) > 0 {
		if err := runcache.SaveJobs(path, cached); err != nil {
			notef(stderr, "warning: the job cache was not saved: %v\n", err)
		}
	}

	return cached, nil
}

type jobSamples struct {
	runners                   map[string]bool
	queue, duration, finished []float64
}

type stepKey struct{ job, step string }

// summarizeJobs rolls the jobs and steps of workflow's runs in commits up by
// name, slowest P90 first. A skipped or canceled job or step has no real
// duration, so it is left out.
func summarizeJobs(commits []ciTiming, workflow string, jobs runcache.Jobs) ([]ciJob, []ciStep) {
	byJob := make(map[string]*jobSamples)
	bySteps := make(map[stepKey][]float64)

	for i := range commits {
		start, err := time.Parse(timeFormat, commits[i].StartedAt)
		if err != nil {
			continue
		}

		for _, run := range commits[i].RunDetails {
			if run.Name != workflow || run.Conclusion == conclusionCanceled {
				continue
			}

			runJobs := jobs[run.ID]
			for k := range runJobs {
				addJob(byJob, bySteps, &runJobs[k], start)
			}
		}
	}

	return jobRows(byJob), stepRows(bySteps)
}

func countable(conclusion string) bool {
	return conclusion == github.ConclusionSuccess || conclusion == github.ConclusionFailure
}

func addJob(byJob map[string]*jobSamples, bySteps map[stepKey][]float64, job *github.JobTiming, start time.Time) {
	if !countable(job.Conclusion) || job.StartedAt.IsZero() || job.CompletedAt.IsZero() {
		return
	}

	s := byJob[job.Name]
	if s == nil {
		s = &jobSamples{runners: make(map[string]bool)}
		byJob[job.Name] = s
	}

	s.runners[strings.Join(job.Labels, ",")] = true
	s.queue = append(s.queue, job.StartedAt.Sub(job.CreatedAt).Seconds())
	s.duration = append(s.duration, job.CompletedAt.Sub(job.StartedAt).Seconds())
	s.finished = append(s.finished, job.CompletedAt.Sub(start).Seconds())

	for i := range job.Steps {
		step := &job.Steps[i]
		if countable(step.Conclusion) && !step.StartedAt.IsZero() && !step.CompletedAt.IsZero() {
			key := stepKey{job: job.Name, step: step.Name}
			bySteps[key] = append(bySteps[key], step.CompletedAt.Sub(step.StartedAt).Seconds())
		}
	}
}

func jobRows(byJob map[string]*jobSamples) []ciJob {
	out := make([]ciJob, 0, len(byJob))

	for name, s := range byJob {
		runners := make([]string, 0, len(s.runners))
		for r := range s.runners {
			runners = append(runners, r)
		}

		sort.Strings(runners)

		out = append(out, ciJob{
			Name:            name,
			Runners:         runners,
			Runs:            len(s.duration),
			P50Queue:        percentile(s.queue, percentileMedian),
			P90Queue:        percentile(s.queue, percentileP90),
			P50Seconds:      percentile(s.duration, percentileMedian),
			P90Seconds:      percentile(s.duration, percentileP90),
			P90FinishedPush: percentile(s.finished, percentileP90),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].P90FinishedPush != out[j].P90FinishedPush {
			return out[i].P90FinishedPush > out[j].P90FinishedPush
		}

		return out[i].Name < out[j].Name
	})

	return out
}

func stepRows(bySteps map[stepKey][]float64) []ciStep {
	out := make([]ciStep, 0, len(bySteps))

	for key, durations := range bySteps {
		out = append(out, ciStep{
			Job:        key.job,
			Name:       key.step,
			Runs:       len(durations),
			P50Seconds: percentile(durations, percentileMedian),
			P90Seconds: percentile(durations, percentileP90),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].P90Seconds != out[j].P90Seconds {
			return out[i].P90Seconds > out[j].P90Seconds
		}

		return out[i].Job+out[i].Name < out[j].Job+out[j].Name
	})

	return out
}

// writeJobTables compares the drilled-down workflow's jobs and slowest steps
// across the two windows. A change in runner labels between windows is the
// sign of a pipeline change that percentiles alone would average away.
func writeJobTables(b *strings.Builder, workflow string, baseline, current ciTimingWindow) {
	prevJobs := make(map[string]ciJob, len(baseline.Jobs))
	for _, j := range baseline.Jobs {
		prevJobs[j.Name] = j
	}

	fmt.Fprintf(b, "\nJobs in %s (current window, slowest first).\n\n", workflow)
	b.WriteString("| Job | Runner | Queue P50 | Queue P90 | Run P50 | Run P90 | Baseline run P90 | Change |\n")
	b.WriteString("| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")

	for _, job := range current.Jobs {
		prev, change, runner := "-", "-", strings.Join(job.Runners, " | ")

		if old, ok := prevJobs[job.Name]; ok && old.P90Seconds > 0 {
			prev = formatSeconds(old.P90Seconds)
			change = fmt.Sprintf("%+.0f%%", (job.P90Seconds/old.P90Seconds-1)*percentScale)

			if !sameStrings(old.Runners, job.Runners) {
				runner += " (was " + strings.Join(old.Runners, " | ") + ")"
			}
		}

		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s | %s | %s |\n", job.Name, runner,
			formatSeconds(job.P50Queue), formatSeconds(job.P90Queue),
			formatSeconds(job.P50Seconds), formatSeconds(job.P90Seconds), prev, change)
	}

	writeStepTable(b, baseline, current)
}

func writeStepTable(b *strings.Builder, baseline, current ciTimingWindow) {
	prev := make(map[stepKey]ciStep, len(baseline.Steps))
	for _, s := range baseline.Steps {
		prev[stepKey{s.Job, s.Name}] = s
	}

	b.WriteString("\nSlowest steps (current window).\n\n")
	b.WriteString("| Job | Step | P50 | P90 | Baseline P90 | Change |\n| --- | --- | ---: | ---: | ---: | ---: |\n")

	for i := range current.Steps {
		if i == topSteps {
			break
		}

		step := &current.Steps[i]
		before, change := "-", "-"

		if old, ok := prev[stepKey{step.Job, step.Name}]; ok && old.P90Seconds > 0 {
			before = formatSeconds(old.P90Seconds)
			change = fmt.Sprintf("%+.0f%%", (step.P90Seconds/old.P90Seconds-1)*percentScale)
		}

		fmt.Fprintf(b, "| %s | %s | %s | %s | %s | %s |\n", step.Job, step.Name,
			formatSeconds(step.P50Seconds), formatSeconds(step.P90Seconds), before, change)
	}
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}
