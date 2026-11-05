package citiming

import (
	"fmt"
	"strings"
	"time"
)

// Defaults for Targets.
const (
	DefaultTargetWall  = 5 * time.Minute
	DefaultTargetQueue = 2 * time.Minute
)

// Targets are what a healthy pipeline meets.
type Targets struct {
	Wall  time.Duration
	Queue time.Duration
	// Flake is the most head SHAs, in percent, that may hold a same-SHA flake.
	Flake float64
}

// DefaultTargets returns the targets used when none are set.
func DefaultTargets() Targets {
	return Targets{Wall: DefaultTargetWall, Queue: DefaultTargetQueue, Flake: DefaultFlakeTarget}
}

// targetSegments cuts the pull request range where the epochs were cut by
// hand, so a comparison across a merge shows one row set per side. Otherwise
// the whole range is one segment.
func targetSegments(pr *reportEvent, explicit bool) []timeWindow {
	from, to := pr.stats.span()

	segments := []timeWindow{{from: from, to: to}}
	if explicit && len(pr.stats.Epochs) > 0 {
		segments = epochSpans(pr.stats.Epochs, to)
	}

	for i := range segments {
		segments[i].label = epochStamp(segments[i].from.Format(timeFormat)) + " to " +
			epochStamp(segments[i].to.Format(timeFormat))
	}

	return segments
}

func targetStatus(value, target float64, unit func(float64) string) string {
	if value <= target {
		return "met"
	}

	return "over by " + unit(value-target)
}

func percentText(v float64) string { return fmt.Sprintf("%.2f%%", v) }

// writeTargets prints the compact block for pull requests: wall clock per
// push, same-SHA flake rate, and merge queue wait, each against its target.
// A segment with fewer pushes than minPushes shows no percentiles.
func writeTargets(b *strings.Builder, events []reportEvent, t Targets, explicit bool, minPushes int) {
	var pr, queue *reportEvent

	for i := range events {
		switch events[i].filter.Event {
		case eventPullRequest:
			pr = &events[i]
		case eventMergeGroup:
			queue = &events[i]
		}
	}

	if pr == nil {
		return
	}

	b.WriteString("\n## Targets\n\nPull requests. Wall clock is first run created to last run finished per push. " +
		"A window with fewer counted pushes than the minimum shows no percentiles.\n\n")
	b.WriteString("| Metric | Window | Pushes | Value | Target | Status |\n| --- | --- | ---: | ---: | ---: | --- |\n")

	segments := targetSegments(pr, explicit)

	for i, seg := range segments {
		pushes := counted(commitsBetween(pr.stats.Commits, seg.from, seg.to))
		row := func(metric string, n int, value, target, status string) {
			fmt.Fprintf(b, "| %s | %s | %d | %s | %s | %s |\n", metric, seg.label, n, value, target, status)
		}

		if len(pushes) < minPushes {
			for _, metric := range []string{"Wall clock P50", "Wall clock P90"} {
				row(metric, len(pushes), "-", formatSeconds(t.Wall.Seconds()),
					fmt.Sprintf("under the %d-push minimum", minPushes))
			}
		} else {
			for _, p := range []struct {
				name string
				q    float64
			}{{"Wall clock P50", percentileMedian}, {"Wall clock P90", percentileP90}} {
				v := secondsPercentile(pushes, p.q)
				row(p.name, len(pushes), formatSeconds(v), formatSeconds(t.Wall.Seconds()),
					targetStatus(v, t.Wall.Seconds(), formatSeconds))
			}
		}

		writeFlakeRows(row, pr, seg, t)
		writeQueueRows(row, queue, i, len(segments), t)
	}
}

func writeFlakeRows(row func(string, int, string, string, string), pr *reportEvent, seg timeWindow, t Targets) {
	f := pr.stats.Flakes
	if f == nil {
		return
	}

	inSegment := make(map[string]bool)

	in := commitsBetween(pr.stats.Commits, seg.from, seg.to)
	for i := range in {
		inSegment[in[i].HeadSHA] = true
	}

	var shas, flaked, failed int

	for sha := range f.shaSet {
		if inSegment[sha] {
			shas++

			if f.flakedSet[sha] {
				flaked++
			}

			if f.failedSet[sha] {
				failed++
			}
		}
	}

	rate := sharePercent(flaked, shas)
	row("Same-SHA flake rate", shas, percentText(rate), percentText(t.Flake), targetStatus(rate, t.Flake, percentText))
	row("Real failure rate", shas, percentText(sharePercent(failed, shas)), "-", "no target")
}

func writeQueueRows(row func(string, int, string, string, string), queue *reportEvent, i, segments int, t Targets) {
	if queue == nil {
		return
	}

	var waits []float64

	for j := range queue.stats.Queue {
		if len(queue.stats.Queue) == segments && j != i {
			continue
		}

		waits = append(waits, queue.stats.Queue[j].waits...)
	}

	if len(waits) == 0 {
		row("Merge queue wait P50", 0, "-", formatSeconds(t.Queue.Seconds()), "no merged entries")

		return
	}

	for _, p := range []struct {
		name string
		q    float64
	}{{"Merge queue wait P50", percentileMedian}, {"Merge queue wait P90", percentileP90}} {
		v := percentile(waits, p.q)
		row(p.name, len(waits), formatSeconds(v), formatSeconds(t.Queue.Seconds()),
			targetStatus(v, t.Queue.Seconds(), formatSeconds))
	}
}
