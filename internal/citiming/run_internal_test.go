package citiming

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// runsExecutor answers the Actions run list with two pushes created an hour
// into whatever range is asked for, which is what a repository with steady
// traffic looks like to a time-ranged query.
type runsExecutor struct{}

func (runsExecutor) Execute(_ string, args ...string) (string, string, error) {
	query, err := url.Parse(args[len(args)-1])
	if err != nil {
		return "", "", fmt.Errorf("parsing %q: %w", args[len(args)-1], err)
	}

	from, _, ok := strings.Cut(query.Query().Get("created"), "..")
	if !ok {
		return "", "", fmt.Errorf("%w: no created range in %q", errNoRange, args[len(args)-1])
	}

	start, err := time.Parse(time.RFC3339, from)
	if err != nil {
		return "", "", fmt.Errorf("parsing %q: %w", from, err)
	}

	runs := make([]string, 0, 4)

	for i, finish := range []int{240, 300, 420, 600} {
		created := start.Add(time.Hour + time.Duration(i/2)*time.Hour)
		runs = append(runs, fmt.Sprintf(
			`{"id":%d,"head_sha":"sha%d-%d","name":"CI","status":"completed","conclusion":"success",`+
				`"head_branch":"topic","actor":{"login":"octocat"},"created_at":%q,"updated_at":%q}`,
			start.Unix()+int64(i), start.Unix(), i/2, created.Format(time.RFC3339),
			created.Add(time.Duration(finish)*time.Second).Format(time.RFC3339)))
	}

	return `{"workflow_runs":[` + strings.Join(runs, ",") + `]}`, "", nil
}

var errNoRange = errors.New("missing range")

// Run reads runs through the client, groups them by commit, and writes the
// window comparison as markdown or JSON, without reading a single job.
func TestRun_WritesTheWindowComparisonAsMarkdownAndJSON(t *testing.T) {
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	client, err := github.NewClientWithExecutor("o/r", runsExecutor{})
	if err != nil {
		t.Fatal(err)
	}

	opts := Options{
		Event: "pull_request", Format: FormatMarkdown, WindowDays: 1, BaselineOffsetDays: 1,
	}

	var md bytes.Buffer
	if err := Run(client, opts, &md, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run markdown: %v", err)
	}

	for _, want := range []string{"CI wall clock per commit", "| baseline", "| current", "P90 change", "| CI |"} {
		if !strings.Contains(md.String(), want) {
			t.Errorf("markdown missing %q:\n%s", want, md.String())
		}
	}

	opts.Format = FormatJSON

	var raw bytes.Buffer
	if err := Run(client, opts, &raw, &bytes.Buffer{}); err != nil {
		t.Fatalf("Run json: %v", err)
	}

	var stats struct {
		Commits []struct {
			HeadSHA string  `json:"head_sha"`
			Seconds float64 `json:"seconds"`
		} `json:"commits"`
	}

	if err := json.Unmarshal(raw.Bytes(), &stats); err != nil || len(stats.Commits) == 0 {
		t.Fatalf("json = %v commits, err %v; want commits grouped by head_sha", len(stats.Commits), err)
	}
}

func TestRun_RejectsAnUnknownRepositoryShape(t *testing.T) {
	t.Parallel()

	if _, err := github.NewClientWithExecutor("no-slash", runsExecutor{}); err == nil {
		t.Error("a repository without owner/name was accepted")
	}
}

// Every job-level table the report prints is built from one set of job rows, so
// a populated set has to reach all of them.
func TestWriteInsights_PrintsEveryTableFromOneSetOfJobs(t *testing.T) {
	t.Parallel()

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	at := func(s int) time.Time { return base.Add(time.Duration(s) * time.Second) }
	span := timeWindow{from: base.Add(-time.Hour), to: base.Add(time.Hour), label: "2026-01-01"}

	recs := make([]jobRec, 0, 30)

	for i := range 10 {
		push := &ciTiming{
			HeadSHA: strconv.Itoa(i), Seconds: 300,
			StartedAt: at(0).Format(timeFormat), FinishedAt: at(300).Format(timeFormat),
		}
		runner := "ubicloud"

		if i%2 == 0 {
			runner = "blacksmith-2vcpu"
		}

		build := jobRec{
			push: push, workflow: "CI", name: "Build", runner: runner, runID: int64(i), jobID: int64(i),
			conclusion: github.ConclusionSuccess, created: at(0), started: at(5), completed: at(100 + i*40),
		}
		skipped := jobRec{
			push: push, workflow: "CI", name: "Deploy", runID: int64(i), conclusion: "skipped", created: at(0),
		}
		failed := jobRec{
			push: push, workflow: "CI", name: "Test", runner: runner, runID: int64(i), jobID: int64(100 + i),
			conclusion: github.ConclusionFailure, created: at(100), started: at(105), completed: at(300),
		}
		recs = append(recs, build, skipped, failed)
	}

	cmp := compareRunners(recs, "blacksmith")
	stats := ciTimingStats{
		Insights: &ciInsights{
			Runners: cmp, Families: familyRows(recs, cmp), Variable: variableJobs(recs),
			Spend: summarizeSpend(recs, []timeWindow{span}), Skipped: skippedJobs(recs),
			Handoffs: summarizeHandoffs(recs, []timeWindow{span}, needsGraph{}, nil), Failing: failingJobs(recs),
			Caches: []ciCacheOutcome{
				{Job: "CI / Build", Logs: 4, WithMiss: 1, allHit: []float64{60}, missed: []float64{120}},
			},
		},
		Flaky: []ciFlaky{{Workflow: "CI", Runs: 10, Failed: 2, Reruns: 1, RerunGreen: 1}},
		Queue: []ciQueue{{
			Start: "2026-01-01", Entries: 4, Merged: 3, Evicted: 1, Reasons: map[string]int{"failed_checks": 1},
		}},
	}

	var out bytes.Buffer
	if err := writeCITimingMarkdown(&out, github.RunFilter{Event: "push", Branch: "main"}, "", stats); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"runners were used from", "Handoff between dependent jobs", "widest run-time spread", "Runner time per push",
		"Jobs skipped most often", "Jobs that fail most often", "Cache restores read from job logs",
		"Failures and re-runs by workflow", "Merge queue entries", "failed_checks 1",
	} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("markdown missing %q", want)
		}
	}
}
