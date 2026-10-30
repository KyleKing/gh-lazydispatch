package cli

import (
	"sort"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// Reasons a commit stays out of the percentiles. A commit whose CI GitHub cut
// short, or that never finished, has no real end-to-end time.
const (
	excludedCanceled   = "canceled"
	excludedIncomplete = "incomplete"
	excludedRerun      = "rerun"
	excludedTimedOut   = "timed_out"
)

// conclusionCanceled is the spelling the GitHub API reports.
const conclusionCanceled = "cancelled" //nolint:misspell // API value

// ciWorkflow is one workflow's share of the counted commits' wall clock.
type ciWorkflow struct {
	Name string `json:"name"`
	// FinishedAfter percentiles are seconds from the push to this workflow's finish.
	P50FinishedAfter float64 `json:"p50_finished_after_seconds"`
	P90FinishedAfter float64 `json:"p90_finished_after_seconds"`
	P50Seconds       float64 `json:"p50_seconds"`
	P90Seconds       float64 `json:"p90_seconds"`
	// LastShare is the fraction of the window's commits where this workflow finished last.
	LastShare float64 `json:"last_share"`
	Commits   int     `json:"commits"`
}

// exclusion returns why a commit's span cannot be trusted, or "". A timed-out
// or startup-failed run was cut short by GitHub, and a canceled run that
// finishes last may have hidden when the commit's CI would have ended. A
// canceled run that finishes before a completed one leaves the span intact,
// as the usual cause is a newer push superseding that one workflow.
func exclusion(group []github.RunTiming) string {
	if !allComplete(group) {
		return excludedIncomplete
	}

	var lastFinished, lastCanceled time.Time

	for i := range group {
		run := &group[i]

		if run.RunAttempt > 1 {
			return excludedRerun
		}

		switch run.Conclusion {
		case "timed_out", "startup_failure":
			return excludedTimedOut
		case conclusionCanceled:
			if run.UpdatedAt.After(lastCanceled) {
				lastCanceled = run.UpdatedAt
			}
		default:
			if run.UpdatedAt.After(lastFinished) {
				lastFinished = run.UpdatedAt
			}
		}
	}

	if !lastCanceled.IsZero() && !lastCanceled.Before(lastFinished) {
		return excludedCanceled
	}

	return ""
}

func counted(commits []ciTiming) []ciTiming {
	kept := make([]ciTiming, 0, len(commits))

	for i := range commits {
		if commits[i].Excluded == "" {
			kept = append(kept, commits[i])
		}
	}

	return kept
}

func excludedCounts(commits []ciTiming) map[string]int {
	counts := make(map[string]int)

	for i := range commits {
		if commits[i].Excluded != "" {
			counts[commits[i].Excluded]++
		}
	}

	if len(counts) == 0 {
		return nil
	}

	return counts
}

// summarizeWorkflows rolls the counted commits up by workflow name, slowest
// P90 finish first.
func summarizeWorkflows(commits []ciTiming) []ciWorkflow {
	type samples struct{ finished, duration []float64 }

	byName := make(map[string]*samples)
	last := make(map[string]int)

	for i := range commits {
		var latest *ciRun

		for j := range commits[i].RunDetails {
			run := &commits[i].RunDetails[j]
			if run.Retrigger {
				continue
			}

			s := byName[run.Name]
			if s == nil {
				s = &samples{}
				byName[run.Name] = s
			}

			s.finished = append(s.finished, run.FinishedAfter)
			s.duration = append(s.duration, run.Seconds)

			if latest == nil || run.FinishedAfter > latest.FinishedAfter {
				latest = run
			}
		}

		if latest != nil {
			last[latest.Name]++
		}
	}

	out := make([]ciWorkflow, 0, len(byName))

	for name, s := range byName {
		out = append(out, ciWorkflow{
			Name:             name,
			Commits:          len(s.finished),
			LastShare:        float64(last[name]) / float64(len(commits)),
			P50FinishedAfter: percentile(s.finished, percentileMedian),
			P90FinishedAfter: percentile(s.finished, percentileP90),
			P50Seconds:       percentile(s.duration, percentileMedian),
			P90Seconds:       percentile(s.duration, percentileP90),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].P90FinishedAfter != out[j].P90FinishedAfter {
			return out[i].P90FinishedAfter > out[j].P90FinishedAfter
		}

		return out[i].Name < out[j].Name
	})

	return out
}
