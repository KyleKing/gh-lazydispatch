package citiming

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// reasonMerged is the removal reason of an entry that merged.
const reasonMerged = "merged"

var queueBranch = regexp.MustCompile(`/pr-(\d+)-`)

// ciQueue is a span's merge queue traffic: how many entries merged or left
// early, and how long merged entries sat from joining to merging.
type ciQueue struct {
	Reasons      map[string]int `json:"removed_reasons,omitempty"`
	Start        string         `json:"start"`
	Entries      int            `json:"entries"`
	Merged       int            `json:"merged"`
	Evicted      int            `json:"evicted"`
	EvictionRate float64        `json:"eviction_rate"`
	P50Wait      float64        `json:"p50_wait_seconds"`
	P90Wait      float64        `json:"p90_wait_seconds"`
}

// queuePRs lists the pull requests behind the merge groups, from their
// "gh-readonly-queue/<base>/pr-<n>-<sha>" branch names.
func queuePRs(commits []ciTiming) []int {
	seen := make(map[int]bool)

	var prs []int

	for i := range commits {
		m := queueBranch.FindStringSubmatch(commits[i].Branch)
		if m == nil {
			continue
		}

		n, err := strconv.Atoi(m[1])
		if err == nil && !seen[n] {
			seen[n] = true
			prs = append(prs, n)
		}
	}

	sort.Ints(prs)

	return prs
}

type queueSamples struct {
	reasons map[string]int
	waits   []float64
	entries int
	merged  int
	evicted int
}

func summarizeQueue(events map[int][]github.QueueEvent, spans []timeWindow) []ciQueue {
	samples := make([]queueSamples, len(spans))
	for i := range samples {
		samples[i].reasons = make(map[string]int)
	}

	for _, evs := range events {
		sort.Slice(evs, func(i, j int) bool { return evs[i].At.Before(evs[j].At) })

		var added time.Time

		for _, ev := range evs {
			switch ev.Type {
			case github.QueueAdded:
				added = ev.At
			case github.QueueRemoved:
				i := spanIndex(spans, added)
				if i < 0 {
					continue
				}

				samples[i].entries++

				if ev.Reason == reasonMerged {
					samples[i].merged++
					samples[i].waits = append(samples[i].waits, ev.At.Sub(added).Seconds())
				} else {
					samples[i].evicted++
					samples[i].reasons[ev.Reason]++
				}
			}
		}
	}

	out := make([]ciQueue, 0, len(spans))

	for i, span := range spans {
		s := &samples[i]
		out = append(out, ciQueue{
			Start: span.label, Entries: s.entries, Merged: s.merged, Evicted: s.evicted, Reasons: s.reasons,
			EvictionRate: float64(s.evicted) / float64(max(s.entries, 1)),
			P50Wait:      percentile(s.waits, percentileMedian), P90Wait: percentile(s.waits, percentileP90),
		})
	}

	return out
}

func spanIndex(spans []timeWindow, at time.Time) int {
	if at.IsZero() {
		return -1
	}

	for i, span := range spans {
		if !at.Before(span.from) && at.Before(span.to) {
			return i
		}
	}

	return -1
}

func writeQueue(b *strings.Builder, queue []ciQueue) {
	if len(queue) == 0 {
		return
	}

	writeTableHead(b, "Merge queue entries (from each pull request's timeline). Wait is joining the queue to merging. "+
		"An eviction is a removal for any reason other than merging.",
		"<Epoch", "Entries", "Merged", "Evicted", "Eviction rate", "Wait P50", "Wait P90", "<Removal reasons")

	for i := range queue {
		q := &queue[i]

		reasons := make([]string, 0, len(q.Reasons))
		for reason, n := range q.Reasons {
			reasons = append(reasons, fmt.Sprintf("%s %d", strings.ToLower(reason), n))
		}

		sort.Strings(reasons)

		fmt.Fprintf(b, "| %s | %d | %d | %d | %.0f%% | %s | %s | %s |\n", q.Start, q.Entries, q.Merged, q.Evicted,
			q.EvictionRate*percentScale, formatSeconds(q.P50Wait), formatSeconds(q.P90Wait),
			strings.Join(reasons, ", "))
	}
}
