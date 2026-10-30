package citiming

import (
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

// jobRec is one job of a sampled push, kept with its run and push so every
// job-level table reads from the same rows.
type jobRec struct {
	created, started, completed time.Time
	push                        *ciTiming
	workflow, name, runner      string
	conclusion                  string
	steps                       []github.StepTiming
	runID                       int64
}

func (j *jobRec) ran() bool {
	return countable(j.conclusion) && !j.started.IsZero() && !j.completed.IsZero()
}

func (j *jobRec) seconds() float64 { return j.completed.Sub(j.started).Seconds() }

func (j *jobRec) queue() float64 { return nonNegative(j.started.Sub(j.created)) }

func (j *jobRec) label() string { return j.workflow + " / " + j.name }

// runnerFamily groups runner labels by the vendor that serves them.
func runnerFamily(label string) string {
	switch {
	case strings.HasPrefix(label, "blacksmith"):
		return "blacksmith"
	case strings.HasPrefix(label, "ubicloud"):
		return "ubicloud"
	case strings.HasPrefix(label, "ubuntu"), strings.HasPrefix(label, "macos"), strings.HasPrefix(label, "windows"):
		return "github"
	default:
		return label
	}
}

// collectJobs lists every job, skipped ones included, of the primary runs of
// pushes. Canceled runs are left out because their jobs stop partway.
func collectJobs(pushes []ciTiming, jobs runcache.Jobs) []jobRec {
	var out []jobRec

	for c := range pushes {
		for _, run := range pushes[c].RunDetails {
			if run.Retrigger || run.Conclusion == conclusionCanceled {
				continue
			}

			for k := range jobs[run.ID] {
				job := &jobs[run.ID][k]
				out = append(out, jobRec{
					push:       &pushes[c],
					workflow:   run.Name,
					name:       job.Name,
					runner:     strings.Join(job.Labels, ","),
					conclusion: job.Conclusion,
					created:    job.CreatedAt,
					started:    job.StartedAt,
					completed:  job.CompletedAt,
					steps:      job.Steps,
					runID:      run.ID,
				})
			}
		}
	}

	return out
}

func recsBetween(recs []jobRec, from, to time.Time) []jobRec {
	var in []jobRec

	for i := range recs {
		at := recs[i].created
		if !at.Before(from) && (to.IsZero() || at.Before(to)) {
			in = append(in, recs[i])
		}
	}

	return in
}

func daySpans(from, to time.Time) []timeWindow {
	var spans []timeWindow

	for day := from; day.Before(to); day = day.Add(hoursPerDay * time.Hour) {
		next := day.Add(hoursPerDay * time.Hour)
		spans = append(spans, timeWindow{from: day, to: next, label: day.Format(time.DateOnly)})
	}

	return spans
}
