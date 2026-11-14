package citiming

import (
	"sort"

	"github.com/kyleking/gh-lazydispatch/v2/internal/github"
	"github.com/kyleking/gh-lazydispatch/v2/internal/runcache"
)

// rerunAttemptKeys lists the earlier attempts of re-run runs that were not
// canceled, whose jobs the run's own job list no longer holds.
func rerunAttemptKeys(runs []github.PagedRun) []string {
	var keys []string

	for i := range runs {
		if runs[i].Conclusion == conclusionCanceled {
			continue
		}

		for a := 1; a < runs[i].RunAttempt; a++ {
			keys = append(keys, runcache.AttemptKey(runs[i].ID, a))
		}
	}

	return keys
}

// markShaVerdicts sets, on every job row, the head SHAs where the job failed
// and a later attempt or run of the same workflow passed (flaked), and the ones
// where its last outcome was a failure (persisted).
func markShaVerdicts(
	report *jobFailureReport, runs []github.PagedRun, jobs runcache.Jobs, attempts runcache.Attempts,
) {
	outcomes := shaOutcomes(runs, jobs, attempts)

	for i := range report.Jobs {
		row := &report.Jobs[i]

		for sha, seq := range outcomes[row.Job] {
			switch {
			case seq[len(seq)-1] == github.ConclusionFailure:
				row.PersistedSHAs = append(row.PersistedSHAs, sha)
			case containsFailure(seq):
				row.FlakedSHAs = append(row.FlakedSHAs, sha)
			}
		}

		sort.Strings(row.FlakedSHAs)
		sort.Strings(row.PersistedSHAs)
	}
}

// shaOutcomes maps job name, then head SHA, to the job's pass and fail
// conclusions oldest first. A re-run run's earlier attempts come before its
// latest.
func shaOutcomes(
	runs []github.PagedRun, jobs runcache.Jobs, attempts runcache.Attempts,
) map[string]map[string][]string {
	ordered := append([]github.PagedRun(nil), runs...)
	sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].CreatedAt.Before(ordered[j].CreatedAt) })

	outcomes := make(map[string]map[string][]string)

	add := func(sha, job, conclusion string) {
		if !countable(conclusion) {
			return
		}

		if outcomes[job] == nil {
			outcomes[job] = make(map[string][]string)
		}

		outcomes[job][sha] = append(outcomes[job][sha], conclusion)
	}

	for i := range ordered {
		run := &ordered[i]
		if run.Conclusion == conclusionCanceled || run.HeadSHA == "" {
			continue
		}

		for a := 1; a < run.RunAttempt; a++ {
			earlier := attempts[runcache.AttemptKey(run.ID, a)]
			for k := range earlier {
				add(run.HeadSHA, earlier[k].Name, earlier[k].Conclusion)
			}
		}

		for k := range jobs[run.ID] {
			add(run.HeadSHA, jobs[run.ID][k].Name, jobs[run.ID][k].Conclusion)
		}
	}

	return outcomes
}

func containsFailure(seq []string) bool {
	for _, conclusion := range seq {
		if conclusion == github.ConclusionFailure {
			return true
		}
	}

	return false
}
