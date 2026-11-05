package citiming

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/logs"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

// JobFailureOptions selects the runs whose jobs are counted. Runs is how many of
// the workflow's newest finished runs to read, with no sampling.
type JobFailureOptions struct {
	Workflow, Branch, Event string
	Runs                    int
}

// jobFailureRow is one job's record across the examined runs. FirstError comes
// from the log of the newest failed run in which TopStep failed. FlakedSHAs are
// the head SHAs where the job failed and a later attempt or run passed it, and
// PersistedSHAs those where its last outcome failed.
type jobFailureRow struct {
	instance        *failedInstance
	Job             string   `json:"job"`
	TopStep         string   `json:"top_failing_step,omitempty"`
	FirstError      string   `json:"first_error,omitempty"`
	FlakedSHAs      []string `json:"flaked_shas,omitempty"`
	PersistedSHAs   []string `json:"persisted_shas,omitempty"`
	Runs            int      `json:"runs"`
	Failed          int      `json:"failed"`
	FailedShare     float64  `json:"failed_share"`
	TopStepFailed   int      `json:"top_failing_step_failures,omitempty"`
	LatestFailedRun int64    `json:"latest_failed_run,omitempty"`
}

type failedInstance struct {
	step  github.StepTiming
	jobID int64
	runID int64
}

type jobFailureReport struct {
	Workflow string          `json:"workflow"`
	From     string          `json:"from,omitempty"`
	To       string          `json:"to,omitempty"`
	Jobs     []jobFailureRow `json:"jobs"`
	Runs     int             `json:"runs"`
	Failed   int             `json:"failed_runs"`
}

// maxFirstErrorRunes keeps a first-error line readable in a table.
const maxFirstErrorRunes = 200

// JobFailures writes the failure rate of every job of one workflow across its
// newest runs, with each job's most-failing step and that step's first error line.
func JobFailures(client *github.Client, opts JobFailureOptions, stdout, stderr io.Writer) error {
	if opts.Workflow == "" || opts.Runs <= 0 {
		return fmt.Errorf("%w: job-failures needs --workflow and a positive --runs", ErrBadOptions)
	}

	runs, err := client.ListRunsPaged(github.RunQuery{
		Workflow: opts.Workflow, Branch: opts.Branch, Event: opts.Event,
		Status: github.StatusCompleted, Limit: opts.Runs,
	})
	if err != nil {
		return fmt.Errorf("listing runs: %w", err)
	}

	ids := make([]int64, 0, len(runs))

	for i := range runs {
		if runs[i].Conclusion != conclusionCanceled {
			ids = append(ids, runs[i].ID)
		}
	}

	jobs, err := loadJobs(client, ids, stderr)
	if err != nil {
		return err
	}

	attempts, err := loadAttempts(client, rerunAttemptKeys(runs), stderr)
	if err != nil {
		return err
	}

	report := rollUpJobFailures(opts.Workflow, ids, jobs)
	markShaVerdicts(&report, runs, jobs, attempts)
	if len(runs) > 0 {
		report.To = runs[0].CreatedAt.Format(timeFormat)
		report.From = runs[len(runs)-1].CreatedAt.Format(timeFormat)
	}

	for i := range report.Jobs {
		row := &report.Jobs[i]
		if row.instance == nil {
			continue
		}

		raw, err := client.JobLog(row.instance.jobID)
		if err != nil {
			notef(stderr, "warning: first error of %q unavailable: %v\n", row.Job, err)

			continue
		}

		line, named := firstErrorInStep(raw, row.instance.step)
		if !named {
			if siblings := failedSiblings(row.Job, jobs[row.instance.runID]); siblings != "" {
				line = siblings
			}
		}

		row.FirstError = line
	}

	return writeJSON(stdout, report)
}

// jobTally accumulates one job name's outcomes across runs.
type jobTally struct {
	steps    map[string]int
	failures []failedInstance
	row      jobFailureRow
}

func (t *jobTally) add(runID int64, job *github.JobTiming) {
	t.row.Runs++

	if job.Conclusion != github.ConclusionFailure {
		return
	}

	t.row.Failed++

	if t.row.LatestFailedRun == 0 {
		t.row.LatestFailedRun = runID
	}

	if step := failingStep(job); step != nil {
		t.steps[step.Name]++
		t.failures = append(t.failures, failedInstance{step: *step, jobID: job.ID, runID: runID})
	}
}

func (t *jobTally) finish() jobFailureRow {
	row := t.row
	row.FailedShare = float64(row.Failed) / float64(row.Runs)
	row.TopStep, row.TopStepFailed = topStep(t.steps)

	for i := range t.failures {
		if t.failures[i].step.Name == row.TopStep {
			row.instance = &t.failures[i]

			break
		}
	}

	return row
}

// rollUpJobFailures counts, for every job name, the runs it finished in and the
// ones it failed, newest run first in ids. A canceled or skipped job has no
// outcome, so it counts as neither.
func rollUpJobFailures(workflow string, ids []int64, jobs runcache.Jobs) jobFailureReport {
	tallies := make(map[string]*jobTally)
	failedRuns := make(map[int64]bool)

	for _, id := range ids {
		for k := range jobs[id] {
			job := &jobs[id][k]
			if !countable(job.Conclusion) {
				continue
			}

			if tallies[job.Name] == nil {
				tallies[job.Name] = &jobTally{steps: make(map[string]int), row: jobFailureRow{Job: job.Name}}
			}

			tallies[job.Name].add(id, job)

			if job.Conclusion == github.ConclusionFailure {
				failedRuns[id] = true
			}
		}
	}

	out := make([]jobFailureRow, 0, len(tallies))
	for _, t := range tallies {
		out = append(out, t.finish())
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Failed != out[j].Failed {
			return out[i].Failed > out[j].Failed
		}

		return out[i].Job < out[j].Job
	})

	return jobFailureReport{Workflow: workflow, Runs: len(ids), Failed: len(failedRuns), Jobs: out}
}

func failingStep(job *github.JobTiming) *github.StepTiming {
	for i := range job.Steps {
		if job.Steps[i].Conclusion == github.ConclusionFailure {
			return &job.Steps[i]
		}
	}

	return nil
}

// topStep picks the step with the most failures, the alphabetically first on a tie.
func topStep(counts map[string]int) (string, int) {
	best, most := "", 0

	for name, n := range counts {
		if n > most || (n == most && name < best) {
			best, most = name, n
		}
	}

	return best, most
}

// firstErrorInStep returns the line that names why the step failed, from the
// lines the job log wrote while step ran, skipping the echoed script. Log lines
// carry finer timestamps than the step's whole seconds, so the step's end gets
// a second of slack. The bool reports whether the line names a failed test
// rather than being the first error-level line, which can be console noise.
func firstErrorInStep(rawLog string, step github.StepTiming) (string, bool) {
	if step.StartedAt.IsZero() || step.CompletedAt.IsZero() {
		return "", false
	}

	var (
		echo     logs.CommandEcho
		contents []string
		fallback string
	)

	end := step.CompletedAt.Add(time.Second)

	for _, entry := range logs.ParseLogOutput(rawLog, step.Name) {
		if entry.Timestamp.Before(step.StartedAt) || !entry.Timestamp.Before(end) || echo.Echoed(entry.Content) {
			continue
		}

		content := strings.TrimSpace(entry.Content)
		contents = append(contents, content)

		if fallback == "" && entry.Level == logs.LogLevelError {
			fallback = content
		}
	}

	if id := pytestFailureID(contents); id != "" {
		return truncateRunes(id), true
	}

	if line := vitestFailureLine(contents); line != "" {
		return truncateRunes(line), true
	}

	return truncateRunes(fallback), false
}

const pytestSummaryHeader = "short test summary info"

var pytestFailurePrefixes = []string{"FAILED ", "SUBFAILED", "ERROR "} //nolint:gochecknoglobals // read-only

// pytestFailureID returns the first FAILED, SUBFAILED, or ERROR entry of
// pytest's short test summary, cut to the test id.
func pytestFailureID(contents []string) string {
	inSummary := false

	for _, line := range contents {
		if strings.Contains(line, pytestSummaryHeader) {
			inSummary = true

			continue
		}

		if !inSummary {
			continue
		}

		for _, prefix := range pytestFailurePrefixes {
			if strings.HasPrefix(line, prefix) {
				return cutMessage(line)
			}
		}
	}

	return ""
}

// cutMessage drops the " - message" pytest appends to a summary entry, ignoring
// separators inside brackets or parentheses of a parametrized id.
func cutMessage(line string) string {
	depth := 0

	for i, r := range line {
		switch r {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		}

		if depth <= 0 && strings.HasPrefix(line[i:], " - ") {
			return line[:i]
		}
	}

	return line
}

// vitestFailureLine returns the first FAIL line, which names the file and the
// test or story, else the first line marking a failed test.
func vitestFailureLine(contents []string) string {
	marked := ""

	for _, line := range contents {
		switch {
		case strings.HasPrefix(line, "FAIL "):
			return strings.Join(strings.Fields(line), " ")
		case marked == "" && (strings.HasPrefix(line, "× ") || strings.HasPrefix(line, "✗")):
			marked = line
		}
	}

	return marked
}

// failedSiblings names the jobs of the same run that failed under this job's
// name as a prefix, which is how a gate job over a matrix of shards fails.
func failedSiblings(job string, runJobs []github.JobTiming) string {
	var names []string

	for i := range runJobs {
		if runJobs[i].Conclusion == github.ConclusionFailure && strings.HasPrefix(runJobs[i].Name, job+" ") {
			names = append(names, runJobs[i].Name)
		}
	}

	if len(names) == 0 {
		return ""
	}

	return truncateRunes("failed jobs: " + strings.Join(names, ", "))
}

func truncateRunes(s string) string {
	if runes := []rune(s); len(runes) > maxFirstErrorRunes {
		return string(runes[:maxFirstErrorRunes]) + "..."
	}

	return s
}
