package github_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/testutil"
)

// runTimingJSON builds a workflow_runs page of n synthetic runs, each on its
// own commit, starting the head_sha sequence at startID.
func runTimingJSON(startID, n int) string {
	runs := make([]string, n)
	for i := range n {
		id := startID + i
		runs[i] = fmt.Sprintf(
			`{"id":%d,"head_sha":"sha%d","name":"CI","status":"completed","conclusion":"success",`+
				`"head_commit":{"timestamp":"2025-12-31T23:59:00Z"},`+
				`"created_at":"2026-01-01T00:00:00Z","updated_at":"2026-01-01T00:05:00Z"}`,
			id, id,
		)
	}

	return `{"workflow_runs":[` + strings.Join(runs, ",") + `]}`
}

func runsPath(created string, page int) string {
	return fmt.Sprintf(
		"repos/owner/repo/actions/runs?branch=main&created=%s&event=push&page=%d&per_page=100", created, page,
	)
}

// A CI timing rollup groups runs by commit, so head_sha has to survive the
// trip from the Actions API into RunTiming. Each day is queried on its own
// because a filtered query returns at most 1000 results, a full page means
// ask for the next, and a run on a day boundary is not counted twice.
func TestListRunTimings_PagesEachDayAndDedupesTheBoundary(t *testing.T) {
	t.Parallel()

	const day1 = "2026-01-01T00%3A00%3A00Z..2026-01-02T00%3A00%3A00Z"

	const day2 = "2026-01-02T00%3A00%3A00Z..2026-01-03T00%3A00%3A00Z"

	query := func(created string, page int) []string {
		return []string{"api", runsPath(created, page)}
	}

	mock := testutil.NewMockExecutor()
	mock.AddCommand("gh", query(day1, 1), runTimingJSON(1, 100), "", nil)
	mock.AddCommand("gh", query(day1, 2), runTimingJSON(101, 3), "", nil)
	mock.AddCommand("gh", query(day2, 1), runTimingJSON(103, 2), "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mock)
	if err != nil {
		t.Fatal(err)
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	got, err := client.ListRunTimings(github.RunFilter{Branch: "main", Event: "push"}, from, from.AddDate(0, 0, 2))
	if err != nil {
		t.Fatalf("ListRunTimings: %v", err)
	}

	if len(got.Runs) != 104 {
		t.Fatalf("got %d runs, want 104 (run 103 appears on both days)", len(got.Runs))
	}

	if want := time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC); !got.Runs[0].CommittedAt.Equal(want) {
		t.Errorf("CommittedAt is %v, want %v", got.Runs[0].CommittedAt, want)
	}

	if !github.IsComplete(&got.Runs[0]) {
		t.Error("a completed run is not complete")
	}

	if len(got.Capped) != 0 {
		t.Errorf("Capped is %v, want none", got.Capped)
	}
}

// A day with 1000 or more runs hits the API's cap, so it is halved until each
// half fits rather than silently dropping the rest.
func TestListRunTimings_SplitsARangeThatHitsTheResultCap(t *testing.T) {
	t.Parallel()

	const (
		full = "2026-01-01T00%3A00%3A00Z..2026-01-02T00%3A00%3A00Z"
		low  = "2026-01-01T00%3A00%3A00Z..2026-01-01T12%3A00%3A00Z"
		high = "2026-01-01T12%3A00%3A00Z..2026-01-02T00%3A00%3A00Z"
	)

	mock := testutil.NewMockExecutor()
	addPages := func(created string, firstID, total int) {
		for page := 1; page <= total/100+1; page++ {
			n := min(100, total-(page-1)*100)
			body := runTimingJSON(firstID+(page-1)*100, n)
			mock.AddCommand("gh", []string{"api", runsPath(created, page)}, body, "", nil)
		}
	}

	addPages(full, 1, 1000)
	addPages(low, 1, 400)
	addPages(high, 1001, 300)

	client, err := github.NewClientWithExecutor("owner/repo", mock)
	if err != nil {
		t.Fatal(err)
	}

	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	got, err := client.ListRunTimings(github.RunFilter{Branch: "main", Event: "push"}, from, from.AddDate(0, 0, 1))
	if err != nil {
		t.Fatalf("ListRunTimings: %v", err)
	}

	if len(got.Runs) != 700 || len(got.Capped) != 0 {
		t.Fatalf("got %d runs and %d capped ranges, want 700 and none", len(got.Runs), len(got.Capped))
	}
}
