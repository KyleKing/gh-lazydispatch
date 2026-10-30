package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

func insightRec(push *ciTiming, name, runner string, start, seconds int, steps ...github.StepTiming) jobRec {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return base.Add(time.Duration(s) * time.Second) }

	return jobRec{
		push: push, workflow: "CI", name: name, runner: runner, runID: 1, conclusion: github.ConclusionSuccess,
		created: at(start), started: at(start + 2), completed: at(start + 2 + seconds), steps: steps,
	}
}

// A job seen on the trial vendor's runners is compared with its own runs before
// and after, a cache post step that uploads marks a miss, and the chain behind
// the last job follows the needs edges.
func TestInsights_CompareRunnersFindCacheSavesAndTraceTheNeedsChain(t *testing.T) {
	t.Parallel()

	push := &ciTiming{HeadSHA: "a"}
	save := func(seconds int) github.StepTiming {
		start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

		return github.StepTiming{
			Name: "Post Cache deps", Conclusion: github.ConclusionSuccess,
			StartedAt: start, CompletedAt: start.Add(time.Duration(seconds) * time.Second),
		}
	}

	runners := []string{"ubicloud", "blacksmith-2vcpu", "blacksmith-2vcpu", "blacksmith-2vcpu", "ubicloud"}

	recs := make([]jobRec, 0, len(runners))

	for i, runner := range runners {
		recs = append(recs, insightRec(push, "Tests", runner, i*1000, 100))
	}

	cmp := compareRunners(recs, "blacksmith")
	if cmp == nil || len(cmp.Rows) != 3 {
		t.Fatalf("comparison = %+v, want the job before, during, and after the trial", cmp)
	}

	if phases := cmp.Rows[0].Phase + cmp.Rows[1].Phase + cmp.Rows[2].Phase; phases != "1 before2 during3 after" {
		t.Errorf("phases = %q, want before, during, after in order", phases)
	}

	cached := make([]*jobRec, 0, 8)

	for i := range 8 {
		seconds := 2
		if i%2 == 0 {
			seconds = 9
		}

		rec := insightRec(push, "Build", "ubicloud", 0, 50+seconds*10, save(seconds))
		cached = append(cached, &rec)
	}

	if share, kept, saved := cacheSaves(cached); share != 0.5 || kept >= saved {
		t.Errorf("cache saves = %v share, %v without, %v with; want half saving and slower", share, kept, saved)
	}

	build := insightRec(push, "Build", "ubicloud", 0, 100)
	test := insightRec(push, "Test", "ubicloud", 100, 100)
	graph := needsGraph{"CI": {"Build": nil, "Test": {"Build"}}}

	byRun := map[int64][]*jobRec{1: {&build, &test}}
	if got := strings.Join(traceChain(&test, byRun, graph), chainArrow); got != "Build"+chainArrow+"Test" {
		t.Errorf("chain = %q, want Build then Test", got)
	}
}

// Queue events turn into evictions and waits per span, counting only entries
// that ended inside it.
func TestSummarizeQueue_CountsEvictionsAndWaitsPerSpan(t *testing.T) {
	t.Parallel()

	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	spans := []timeWindow{{from: day, to: day.Add(24 * time.Hour), label: "day"}}

	at := day.Add
	events := map[int][]github.QueueEvent{
		1: {{At: at(time.Hour), Type: "added"}, {At: at(time.Hour + 2*time.Minute), Type: "removed", Reason: "merged"}},
		2: {{At: at(2 * time.Hour), Type: "added"}, {At: at(3 * time.Hour), Type: "removed", Reason: "BUILD_FAILURE"}},
	}

	got := summarizeQueue(events, spans)[0]
	if got.Entries != 2 || got.Evicted != 1 || got.EvictionRate != 0.5 || got.P50Wait != 120 {
		t.Errorf("queue = %+v, want 2 entries, 1 evicted, a 2m wait", got)
	}
}

// Findings fire only past their thresholds, and gate jobs are detected as the
// jobs that end pushes with almost no headroom.
func TestReportFindings_FlagLargeChangesAndDetectGateJobs(t *testing.T) {
	t.Parallel()

	stats := ciTimingStats{
		Windows: []ciTimingWindow{
			{Commits: 30, P90Seconds: 100},
			{Commits: 30, P90Seconds: 130, Excluded: map[string]int{excludedCanceled: 20}},
		},
		Flaky: []ciFlaky{{Workflow: "Dashboard", Runs: 100, Failed: 10, RerunGreen: 12, FailedShare: 0.1}},
	}

	got := strings.Join(reportFindings([]reportEvent{{label: "Pull requests", stats: stats}}), "\n")
	for _, want := range []string{"P90 wall clock rose 30%", "40% of pushes", "Dashboard had 12 re-runs"} {
		if !strings.Contains(got, want) {
			t.Errorf("findings missing %q:\n%s", want, got)
		}
	}

	quiet := ciTimingStats{Windows: []ciTimingWindow{{Commits: 30, P90Seconds: 100}, {Commits: 30, P90Seconds: 105}}}
	if got := reportFindings([]reportEvent{{label: "Pull requests", stats: quiet}}); len(got) != 0 {
		t.Errorf("findings = %v, want none under the thresholds", got)
	}
}
