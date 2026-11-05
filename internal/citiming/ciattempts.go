package citiming

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/runcache"
)

// DefaultFlakeTarget is the share of head SHAs, in percent, that may hold a same-SHA flake.
const DefaultFlakeTarget = 0.5

// Failure classes readable from job and step conclusions, without logs.
const (
	classCancelled  = "canceled"
	classRunnerLost = "runner lost"
	classStepFailed = "step failed"
	classTimedOut   = "timed out"
)

const (
	conclusionTimedOut = "timed_out"
	topFlakeJobs       = 15
	firstRerun         = 2
)

// ciFlakeRow counts head SHAs for one workflow or job. SHAs is the number of
// head SHAs the workflow reached a verdict on, which job rows divide by too.
// FlakedMembers and FailedMembers name the SHAs behind Flaked and Failed.
type ciFlakeRow struct {
	Name          string   `json:"name"`
	FlakedMembers []string `json:"flaked_members,omitempty"`
	FailedMembers []string `json:"failed_members,omitempty"`
	SHAs          int      `json:"shas"`
	Flaked        int      `json:"flaked"`
	Failed        int      `json:"failed"`
}

// ciFlakes is the same-SHA view of reliability: a flake is a job that failed in
// an earlier attempt and passed in a later attempt of the same head SHA.
type ciFlakes struct {
	// shaSet, flakedSet, and failedSet let a caller re-count the SHAs inside a sub-range.
	shaSet, flakedSet, failedSet map[string]bool

	FlakedClasses map[string]int `json:"flaked_classes,omitempty"`
	FailedClasses map[string]int `json:"failed_classes,omitempty"`
	Workflows     []ciFlakeRow   `json:"workflows"`
	Jobs          []ciFlakeRow   `json:"jobs"`
	SHAs          int            `json:"shas"`
	Flaked        int            `json:"flaked_shas"`
	Failed        int            `json:"failed_shas"`
}

// verdict reports whether a run reached a pass or fail, as opposed to being
// canceled by a newer push or skipped.
func verdict(conclusion string) bool {
	switch conclusion {
	case github.ConclusionSuccess, github.ConclusionFailure, conclusionTimedOut, conclusionStartup:
		return true
	default:
		return false
	}
}

func failing(conclusion string) bool {
	return verdict(conclusion) && conclusion != github.ConclusionSuccess
}

// attemptKeysNeeded lists the attempts whose jobs the flake view reads: every
// attempt of a re-run run, and the only attempt of a run that failed.
func attemptKeysNeeded(commits []ciTiming) []string {
	seen := make(map[string]bool)

	var keys []string

	add := func(id int64, attempt int) {
		if key := runcache.AttemptKey(id, attempt); !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}

	for c := range commits {
		for _, run := range commits[c].RunDetails {
			switch {
			case !verdict(run.Conclusion):
			case run.Attempt > 1:
				for a := 1; a <= run.Attempt; a++ {
					add(run.ID, a)
				}
			case failing(run.Conclusion):
				add(run.ID, 1)
			}
		}
	}

	sort.Strings(keys)

	return keys
}

func addFlakes(client *github.Client, stats *ciTimingStats, opts Options, ignoreJobs []string, stderr io.Writer) error {
	if opts.EpochDays <= 0 {
		return nil
	}

	from, to := stats.span()
	inRange := commitsBetween(stats.Commits, from, to)

	attempts, err := loadAttempts(client, attemptKeysNeeded(inRange), stderr)
	if err != nil {
		return err
	}

	stats.Flakes = summarizeFlakes(inRange, attempts, ignoreJobs)

	return nil
}

// loadAttempts returns the jobs of each attempt in keys, fetching the ones the
// cache lacks. Like loadJobs, a failed fetch stops the rest and what was
// fetched is saved.
func loadAttempts(client *github.Client, keys []string, stderr io.Writer) (runcache.Attempts, error) {
	path := runcache.AttemptsPath(client.Owner() + "/" + client.Repo())

	cached, err := runcache.LoadAttempts(path)
	if err != nil {
		return nil, fmt.Errorf("loading the attempt cache: %w", err)
	}

	var missing []string

	for _, key := range keys {
		if _, ok := cached[key]; !ok {
			missing = append(missing, key)
		}
	}

	notef(stderr, "%d run attempts need their jobs fetched, %d cached (%s)\n",
		len(missing), len(keys)-len(missing), path)

	missing, shortfall := affordable(missing, stderr)

	var (
		mu    sync.Mutex
		group errgroup.Group
	)

	group.SetLimit(jobFetchConcurrency)

	failed := make(chan struct{})

	var once sync.Once

	for _, key := range missing {
		group.Go(func() error {
			select {
			case <-failed:
				return nil
			default:
			}

			jobs, err := fetchAttempt(client, key)
			if err != nil {
				once.Do(func() { close(failed) })

				return err
			}

			mu.Lock()
			cached[key] = jobs
			mu.Unlock()

			return nil
		})
	}

	fetchErr := group.Wait()

	if len(missing) > 0 {
		if err := runcache.SaveAttempts(path, cached); err != nil {
			notef(stderr, "warning: the attempt cache was not saved: %v\n", err)
		}
	}

	if fetchErr != nil {
		return nil, fmt.Errorf("listing attempt jobs (rerun to resume, finished attempts are cached): %w", fetchErr)
	}

	if shortfall != nil {
		return nil, shortfall
	}

	return cached, nil
}

func fetchAttempt(client *github.Client, key string) ([]runcache.AttemptJob, error) {
	var (
		id      int64
		attempt int
	)

	if _, err := fmt.Sscanf(key, "%d/%d", &id, &attempt); err != nil {
		return nil, fmt.Errorf("reading attempt key %q: %w", key, err)
	}

	var jobs []github.JobTiming

	err := withRetry(func() error {
		var readErr error

		jobs, readErr = client.ListRunAttemptJobs(id, attempt)

		return readErr //nolint:wrapcheck // withRetry returns it to the caller below, which wraps it
	})
	if err != nil {
		return nil, fmt.Errorf("fetching jobs: %w", err)
	}

	slim := make([]runcache.AttemptJob, 0, len(jobs))

	for i := range jobs {
		job := runcache.AttemptJob{Name: jobs[i].Name, Conclusion: jobs[i].Conclusion}

		for _, step := range jobs[i].Steps {
			job.StepFailed = job.StepFailed || step.Conclusion == github.ConclusionFailure
			job.StepTimedOut = job.StepTimedOut || step.Conclusion == conclusionTimedOut
		}

		slim = append(slim, job)
	}

	return slim, nil
}

// classify names why a failed job failed from its conclusions alone. A job that
// failed with no failed step lost its runner or was killed from outside.
func classify(job *runcache.AttemptJob) string {
	switch {
	case job.Conclusion == conclusionCanceled:
		return classCancelled
	case job.Conclusion == conclusionTimedOut || job.StepTimedOut:
		return classTimedOut
	case !job.StepFailed:
		return classRunnerLost
	default:
		return classStepFailed
	}
}

type flakeKey struct{ sha, workflow string }

type flakeTally struct {
	workflows map[string]*ciFlakeRow
	jobs      map[string]*ciFlakeRow
	flaked    map[string]int
	failed    map[string]int
	shaFlaked map[string]bool
	shaFailed map[string]bool
}

// summarizeFlakes counts, per head SHA, the jobs that failed and later passed,
// and the workflows whose last attempt failed. A SHA a workflow never reached a
// verdict on does not count toward its denominator.
func summarizeFlakes(commits []ciTiming, attempts runcache.Attempts, ignoreJobs []string) *ciFlakes {
	groups := make(map[flakeKey][]*ciRun)
	shas := make(map[string]bool)

	for c := range commits {
		if commits[c].Excluded == excludedIncomplete {
			continue
		}

		for r := range commits[c].RunDetails {
			run := &commits[c].RunDetails[r]
			if verdict(run.Conclusion) {
				shas[commits[c].HeadSHA] = true
				key := flakeKey{commits[c].HeadSHA, run.Name}
				groups[key] = append(groups[key], run)
			}
		}
	}

	tally := flakeTally{
		workflows: make(map[string]*ciFlakeRow), jobs: make(map[string]*ciFlakeRow),
		flaked: make(map[string]int), failed: make(map[string]int),
		shaFlaked: make(map[string]bool), shaFailed: make(map[string]bool),
	}

	for key, runs := range groups {
		tally.addGroup(key, runs, attempts, ignoreJobs)
	}

	return &ciFlakes{
		SHAs: len(shas), Flaked: len(tally.shaFlaked), Failed: len(tally.shaFailed),
		FlakedClasses: tally.flaked, FailedClasses: tally.failed,
		shaSet: shas, flakedSet: tally.shaFlaked, failedSet: tally.shaFailed,
		Workflows: sortedRows(tally.workflows), Jobs: sortedRows(tally.jobs),
	}
}

func rowOf(rows map[string]*ciFlakeRow, name string) *ciFlakeRow {
	if rows[name] == nil {
		rows[name] = &ciFlakeRow{Name: name}
	}

	return rows[name]
}

// addGroup tallies one workflow's runs on one head SHA.
func (t *flakeTally) addGroup(key flakeKey, runs []*ciRun, attempts runcache.Attempts, ignoreJobs []string) {
	wf := rowOf(t.workflows, key.workflow)
	wf.SHAs++

	flakedJobs := make(map[string]bool)

	for _, run := range runs {
		for name, class := range flakedJobsOf(run, attempts, ignoreJobs) {
			if !flakedJobs[name] {
				flakedJobs[name] = true
				t.flaked[class]++
			}
		}
	}

	for name := range flakedJobs {
		row := rowOf(t.jobs, key.workflow+" / "+name)
		row.Flaked++
		row.FlakedMembers = append(row.FlakedMembers, key.sha)
	}

	if len(flakedJobs) > 0 {
		wf.Flaked++
		wf.FlakedMembers = append(wf.FlakedMembers, key.sha)
		t.shaFlaked[key.sha] = true
	}

	latest := runs[0]
	for _, run := range runs {
		if run.CreatedAt > latest.CreatedAt {
			latest = run
		}
	}

	if failing(latest.Conclusion) {
		t.addFailure(key, latest, wf, attempts, ignoreJobs)
	}
}

// addFailure counts a run whose last attempt failed. When its jobs are known
// and every failed one is an ignored aggregator, the run only echoes a failure
// elsewhere and is left out.
func (t *flakeTally) addFailure(
	key flakeKey, run *ciRun, wf *ciFlakeRow, attempts runcache.Attempts, ignoreJobs []string,
) {
	jobs, known := attempts[runcache.AttemptKey(run.ID, max(run.Attempt, 1))]
	echo := known

	for i := range jobs {
		if isIgnored(jobs[i].Name, ignoreJobs) {
			continue
		}

		switch {
		case failing(jobs[i].Conclusion):
			echo = false
			row := rowOf(t.jobs, key.workflow+" / "+jobs[i].Name)
			row.Failed++
			row.FailedMembers = append(row.FailedMembers, key.sha)
			t.failed[classify(&jobs[i])]++
		case jobs[i].Conclusion == conclusionCanceled:
			t.failed[classCancelled]++
		}
	}

	if echo {
		return
	}

	wf.Failed++
	wf.FailedMembers = append(wf.FailedMembers, key.sha)
	t.shaFailed[key.sha] = true
}

// flakedJobsOf returns each job of a re-run run that failed in one attempt and
// passed in a later one, with the class of its last failure.
func flakedJobsOf(run *ciRun, attempts runcache.Attempts, ignoreJobs []string) map[string]string {
	if run.Attempt < firstRerun {
		return nil
	}

	lastFailure := make(map[string]string)
	flaked := make(map[string]string)

	for a := 1; a <= run.Attempt; a++ {
		jobs := attempts[runcache.AttemptKey(run.ID, a)]
		for i := range jobs {
			name := jobs[i].Name

			switch {
			case isIgnored(name, ignoreJobs):
			case failing(jobs[i].Conclusion):
				lastFailure[name] = classify(&jobs[i])
			case jobs[i].Conclusion == github.ConclusionSuccess && lastFailure[name] != "":
				flaked[name] = lastFailure[name]
			}
		}
	}

	return flaked
}

func isIgnored(job string, ignore []string) bool {
	for _, name := range ignore {
		if strings.EqualFold(job, name) {
			return true
		}
	}

	return false
}

func sortedRows(rows map[string]*ciFlakeRow) []ciFlakeRow {
	out := make([]ciFlakeRow, 0, len(rows))

	for _, r := range rows {
		if r.Flaked > 0 || r.Failed > 0 {
			sort.Strings(r.FlakedMembers)
			sort.Strings(r.FailedMembers)

			out = append(out, *r)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Flaked != out[j].Flaked {
			return out[i].Flaked > out[j].Flaked
		}

		if out[i].Failed != out[j].Failed {
			return out[i].Failed > out[j].Failed
		}

		return out[i].Name < out[j].Name
	})

	return out
}

func sharePercent(n, of int) float64 {
	if of == 0 {
		return 0
	}

	return float64(n) / float64(of) * percentScale
}

const (
	shortSHA     = 10
	maxListedSHA = 8
)

// writeFlakedSHAs lists the head SHAs behind each workflow's flakes. The JSON
// carries every member.
func writeFlakedSHAs(b *strings.Builder, rows []ciFlakeRow) {
	header := false

	for i := range rows {
		members := rows[i].FlakedMembers
		if len(members) == 0 {
			continue
		}

		if !header {
			writeTableHead(b, "Head SHAs that flaked, per workflow.", "<Workflow", "<SHAs")

			header = true
		}

		listed := make([]string, 0, maxListedSHA)
		for _, sha := range members[:min(len(members), maxListedSHA)] {
			listed = append(listed, sha[:min(len(sha), shortSHA)])
		}

		more := ""
		if len(members) > maxListedSHA {
			more = fmt.Sprintf(" (+%d more)", len(members)-maxListedSHA)
		}

		fmt.Fprintf(b, "| %s | %s%s |\n", rows[i].Name, strings.Join(listed, ", "), more)
	}
}

// writeFlakes prints the same-SHA tables for the workflows and jobs that flaked or failed.
func writeFlakes(b *strings.Builder, f *ciFlakes, target float64) {
	if f == nil {
		return
	}

	rate := sharePercent(f.Flaked, f.SHAs)

	fmt.Fprintf(b, "\nSame-SHA reliability over %d head SHAs. A flake is a job that failed in an earlier attempt "+
		"and passed in a later attempt of the same SHA. A re-run is a lower bound: a rebase or fix pushed after "+
		"a red run counts as a real failure. Flake rate is %.2f%% of SHAs against a target of %.2f%%.\n",
		f.SHAs, rate, target)

	writeTableHead(b, "Per workflow, as a share of the head SHAs it reached a verdict on.",
		"<Workflow", "SHAs", "Flaked", "Flake rate", "Real failed", "Real failure rate")

	shas := make(map[string]int, len(f.Workflows))

	for i := range f.Workflows {
		r := &f.Workflows[i]
		shas[r.Name] = r.SHAs

		fmt.Fprintf(b, "| %s | %d | %d | %.2f%% | %d | %.2f%% |\n", r.Name, r.SHAs, r.Flaked,
			sharePercent(r.Flaked, r.SHAs), r.Failed, sharePercent(r.Failed, r.SHAs))
	}

	writeFlakedSHAs(b, f.Workflows)

	writeTableHead(b, "Per job, as a share of the head SHAs its workflow reached a verdict on.",
		"<Job", "Flaked", "Flake rate", "Real failed", "Real failure rate")

	for i := range f.Jobs {
		if i >= topFlakeJobs {
			break
		}

		r := &f.Jobs[i]
		workflow, _, _ := strings.Cut(r.Name, " / ")

		fmt.Fprintf(b, "| %s | %d | %.2f%% | %d | %.2f%% |\n", r.Name, r.Flaked, sharePercent(r.Flaked, shas[workflow]),
			r.Failed, sharePercent(r.Failed, shas[workflow]))
	}

	writeTableHead(b, "Failure classes from job and step conclusions, with no log read. Runner lost is a failed "+
		"job with no failed step. Canceled counts jobs cut short inside failed runs.",
		"<Class", "Flaked jobs", "Real failed jobs")

	for _, class := range []string{classTimedOut, classRunnerLost, classCancelled, classStepFailed} {
		fmt.Fprintf(b, "| %s | %d | %d |\n", class, f.FlakedClasses[class], f.FailedClasses[class])
	}
}
