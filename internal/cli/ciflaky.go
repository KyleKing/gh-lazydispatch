package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

const topFlaky = 10

// ciFlaky is one workflow's runs that someone re-ran. The API keeps only a run's
// last attempt, so a re-run that ended green is the best sign of a flake, and
// one that failed again is more likely a real failure.
type ciFlaky struct {
	Workflow    string  `json:"workflow"`
	Runs        int     `json:"runs"`
	Failed      int     `json:"failed"`
	Reruns      int     `json:"reruns"`
	RerunGreen  int     `json:"rerun_passed"`
	RerunShare  float64 `json:"rerun_share"`
	FailedShare float64 `json:"failed_share"`
}

// ciJobFailure is a job's failure rate across the sampled pushes.
type ciJobFailure struct {
	Job   string  `json:"job"`
	Jobs  int     `json:"jobs"`
	Share float64 `json:"failed_share"`
}

func (f *ciFlaky) add(run *ciRun) {
	f.Runs++

	if run.Conclusion == github.ConclusionFailure {
		f.Failed++
	}

	if run.Attempt > 1 {
		f.Reruns++

		if run.Conclusion == github.ConclusionSuccess {
			f.RerunGreen++
		}
	}
}

func summarizeFlaky(commits []ciTiming) []ciFlaky {
	byName := make(map[string]*ciFlaky)

	for c := range commits {
		for k := range commits[c].RunDetails {
			run := &commits[c].RunDetails[k]
			if run.Conclusion == "" {
				continue
			}

			if byName[run.Name] == nil {
				byName[run.Name] = &ciFlaky{Workflow: run.Name}
			}

			byName[run.Name].add(run)
		}
	}

	var out []ciFlaky

	for _, f := range byName {
		if f.Reruns == 0 && f.Failed == 0 {
			continue
		}

		f.RerunShare = float64(f.Reruns) / float64(f.Runs)
		f.FailedShare = float64(f.Failed) / float64(f.Runs)
		out = append(out, *f)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Reruns != out[j].Reruns {
			return out[i].Reruns > out[j].Reruns
		}

		return out[i].Workflow < out[j].Workflow
	})

	if len(out) > topFlaky {
		out = out[:topFlaky]
	}

	return out
}

func failingJobs(recs []jobRec) []ciJobFailure {
	counts := make(map[string][2]int)

	for i := range recs {
		if !recs[i].ran() {
			continue
		}

		c := counts[recs[i].label()]
		c[0]++

		if recs[i].conclusion == github.ConclusionFailure {
			c[1]++
		}

		counts[recs[i].label()] = c
	}

	var out []ciJobFailure

	for label, c := range counts {
		if c[1] > 0 && c[0] >= minVarianceRuns {
			out = append(out, ciJobFailure{Job: label, Jobs: c[0], Share: float64(c[1]) / float64(c[0])})
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Share != out[j].Share {
			return out[i].Share > out[j].Share
		}

		return out[i].Job < out[j].Job
	})

	if len(out) > topFlaky {
		out = out[:topFlaky]
	}

	return out
}

func writeFlaky(b *strings.Builder, flaky []ciFlaky, failing []ciJobFailure) {
	if len(flaky) > 0 {
		writeTableHead(b, "Failures and re-runs by workflow. A re-run that passed is the strongest flake signal "+
			"the API gives, but it cannot separate a flake from a fix pushed meanwhile.",
			"<Workflow", "Runs", "Failed", "Re-run", "Re-run passed")

		for i := range flaky {
			f := &flaky[i]
			fmt.Fprintf(b, "| %s | %d | %.0f%% | %d (%.0f%%) | %d |\n", f.Workflow, f.Runs,
				f.FailedShare*percentScale, f.Reruns, f.RerunShare*percentScale, f.RerunGreen)
		}
	}

	if len(failing) > 0 {
		writeTableHead(b, "Jobs that fail most often (sampled pushes).", "<Job", "Jobs", "Failed")

		for i := range failing {
			fmt.Fprintf(b, "| %s | %d | %.0f%% |\n", failing[i].Job, failing[i].Jobs, failing[i].Share*percentScale)
		}
	}
}
