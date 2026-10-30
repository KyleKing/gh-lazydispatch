package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// maxTrendWorkflows caps the workflow trend to the slowest few.
const maxTrendWorkflows = 8

// ciOutcome counts how the pushes of one epoch ended. Left-out pushes are in
// Excluded by reason.
type ciOutcome struct {
	Excluded map[string]int `json:"excluded,omitempty"`
	Start    string         `json:"start"`
	Pushes   int            `json:"pushes"`
	Counted  int            `json:"counted"`
}

// ciTrend is one workflow's runs inside one epoch: how often they failed or
// were canceled, and how long the finished ones took.
type ciTrend struct {
	Workflow    string  `json:"workflow"`
	Start       string  `json:"start"`
	Runs        int     `json:"runs"`
	FailureRate float64 `json:"failure_rate"`
	CancelRate  float64 `json:"cancel_rate"`
	P50Seconds  float64 `json:"p50_seconds"`
	P90Seconds  float64 `json:"p90_seconds"`
}

// epochSpans turns epochs into time spans, the last one ending at now.
func epochSpans(epochs []ciEpoch, now time.Time) []timeWindow {
	spans := make([]timeWindow, 0, len(epochs))

	for i := range epochs {
		from, err := time.Parse(timeFormat, epochs[i].Start)
		if err != nil {
			continue
		}

		to := now
		if i+1 < len(epochs) {
			if next, err := time.Parse(timeFormat, epochs[i+1].Start); err == nil {
				to = next
			}
		}

		spans = append(spans, timeWindow{from: from, to: to, label: epochs[i].Start[:len("2006-01-02")]})
	}

	return spans
}

func summarizeOutcomes(commits []ciTiming, spans []timeWindow) []ciOutcome {
	out := make([]ciOutcome, 0, len(spans))

	for _, span := range spans {
		in := commitsBetween(commits, span.from, span.to)
		out = append(out, ciOutcome{
			Start:    span.label,
			Pushes:   len(in),
			Counted:  len(counted(in)),
			Excluded: excludedCounts(in),
		})
	}

	return out
}

type trendSamples struct {
	durations           []float64
	runs, failed, abort int
}

// trendsIn collects each workflow's run outcomes and durations in commits.
func trendsIn(commits []ciTiming) map[string]*trendSamples {
	byName := make(map[string]*trendSamples)

	for c := range commits {
		for _, run := range commits[c].RunDetails {
			if run.Retrigger {
				continue
			}

			s := byName[run.Name]
			if s == nil {
				s = &trendSamples{}
				byName[run.Name] = s
			}

			s.runs++

			switch run.Conclusion {
			case conclusionCanceled:
				s.abort++
			case github.ConclusionFailure, "timed_out", "startup_failure":
				s.failed++
			}

			if countable(run.Conclusion) {
				s.durations = append(s.durations, run.Seconds)
			}
		}
	}

	return byName
}

func summarizeTrends(commits []ciTiming, spans []timeWindow) []ciTrend {
	perSpan := make([]map[string]*trendSamples, len(spans))
	overall := make(map[string][]float64)

	for i, span := range spans {
		perSpan[i] = trendsIn(commitsBetween(commits, span.from, span.to))

		for name, s := range perSpan[i] {
			overall[name] = append(overall[name], s.durations...)
		}
	}

	var out []ciTrend

	for _, name := range slowestWorkflows(overall) {
		for i, span := range spans {
			s := perSpan[i][name]
			if s == nil {
				continue
			}

			out = append(out, ciTrend{
				Workflow:    name,
				Start:       span.label,
				Runs:        s.runs,
				FailureRate: float64(s.failed) / float64(s.runs),
				CancelRate:  float64(s.abort) / float64(s.runs),
				P50Seconds:  percentile(s.durations, percentileMedian),
				P90Seconds:  percentile(s.durations, percentileP90),
			})
		}
	}

	return out
}

// slowestWorkflows returns the names with the highest overall P90.
func slowestWorkflows(durations map[string][]float64) []string {
	p90 := make(map[string]float64, len(durations))
	names := make([]string, 0, len(durations))

	for name, d := range durations {
		p90[name] = percentile(d, percentileP90)
		names = append(names, name)
	}

	sort.Slice(names, func(i, j int) bool {
		if p90[names[i]] != p90[names[j]] {
			return p90[names[i]] > p90[names[j]]
		}

		return names[i] < names[j]
	})

	if len(names) > maxTrendWorkflows {
		names = names[:maxTrendWorkflows]
	}

	return names
}

func writeOutcomeTable(b *strings.Builder, outcomes []ciOutcome) {
	if len(outcomes) == 0 {
		return
	}

	writeTableHead(b, "How pushes ended, by epoch (a push is left out when its span is cut short).",
		"<Epoch", "Pushes", "Counted", "Canceled", "Re-run", "Timed out", "Unfinished")

	for i := range outcomes {
		o := &outcomes[i]
		fmt.Fprintf(b, "| %s | %d | %d | %s | %s | %s | %s |\n", o.Start, o.Pushes, o.Counted,
			share(o.Excluded[excludedCanceled], o.Pushes), share(o.Excluded[excludedRerun], o.Pushes),
			share(o.Excluded[excludedTimedOut], o.Pushes), share(o.Excluded[excludedIncomplete], o.Pushes))
	}
}

func share(n, total int) string {
	if total == 0 {
		return "-"
	}

	return fmt.Sprintf("%.0f%%", float64(n)/float64(total)*percentScale)
}

func writeTrendTable(b *strings.Builder, trends []ciTrend) {
	if len(trends) == 0 {
		return
	}

	writeTableHead(b, "Workflow run time by epoch (slowest workflows; times over finished runs, rates over all runs).",
		"<Workflow", "<Epoch", "Runs", "Failed", "Canceled", "P50", "P90")

	for i := range trends {
		t := &trends[i]

		runs := strconv.Itoa(t.Runs)
		if t.Runs < minEpochCommits {
			runs += " (few)"
		}

		fmt.Fprintf(b, "| %s | %s | %s | %.0f%% | %.0f%% | %s | %s |\n", t.Workflow, t.Start, runs,
			t.FailureRate*percentScale, t.CancelRate*percentScale,
			formatSeconds(t.P50Seconds), formatSeconds(t.P90Seconds))
	}
}

// writeTableHead writes a caption and a markdown header. A column name with a
// leading "<" is left-aligned and the rest are right-aligned.
func writeTableHead(b *strings.Builder, caption string, columns ...string) {
	names := make([]string, len(columns))
	for i, c := range columns {
		names[i] = strings.TrimPrefix(c, "<")
	}

	b.WriteString("\n" + caption + "\n\n| " + strings.Join(names, " | ") + " |\n|")

	for _, c := range columns {
		if strings.HasPrefix(c, "<") {
			b.WriteString(" --- |")
		} else {
			b.WriteString(" ---: |")
		}
	}

	b.WriteString("\n")
}
