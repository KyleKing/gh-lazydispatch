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

// A CI timing rollup groups runs by commit, so head_sha has to survive the
// trip from the Actions API into RunTiming even though nothing else in this
// package reads it. Requesting more than one page's worth (the Actions API
// caps per_page at 100) is what a repository with real CI history needs.
func TestListRunTimings_FetchesAFullPageThenStopsOnAShortOne(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockExecutor()
	mock.AddCommand("gh", []string{
		"api", "repos/owner/repo/actions/runs?branch=main&event=push&page=1&per_page=100",
	}, runTimingJSON(1, 100), "", nil)
	mock.AddCommand("gh", []string{
		"api", "repos/owner/repo/actions/runs?branch=main&event=push&page=2&per_page=50",
	}, runTimingJSON(101, 3), "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mock)
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.ListRunTimings("main", "push", time.Time{}, 150)
	if err != nil {
		t.Fatalf("ListRunTimings: %v", err)
	}

	if len(got) != 103 {
		t.Fatalf("got %d runs, want 103 (100 from the full page, 3 from the short one)", len(got))
	}

	if got[0].HeadSHA != "sha1" || got[102].HeadSHA != "sha103" {
		t.Errorf("head shas are %q and %q, want sha1 and sha103", got[0].HeadSHA, got[102].HeadSHA)
	}

	if want := time.Date(2025, 12, 31, 23, 59, 0, 0, time.UTC); !got[0].CommittedAt.Equal(want) {
		t.Errorf("CommittedAt is %v, want %v", got[0].CommittedAt, want)
	}

	if !got[0].IsComplete() {
		t.Error("a completed run reports IsComplete false")
	}
}

func TestListRunTimings_StopsOnAnEmptyPageRatherThanLoopingForever(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockExecutor()
	mock.AddCommand("gh", []string{
		"api", "repos/owner/repo/actions/runs?event=push&page=1&per_page=100",
	}, `{"workflow_runs":[]}`, "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mock)
	if err != nil {
		t.Fatal(err)
	}

	got, err := client.ListRunTimings("", "push", time.Time{}, 0)
	if err != nil {
		t.Fatalf("ListRunTimings: %v", err)
	}

	if len(got) != 0 {
		t.Fatalf("got %d runs, want 0", len(got))
	}
}

// The window a rollup compares is bounded server-side, so a week-old window
// is not lost behind the run limit.
func TestListRunTimings_BoundsTheSampleWithACreatedFilter(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockExecutor()
	mock.AddCommand("gh", []string{
		"api",
		"repos/owner/repo/actions/runs?branch=main&created=%3E%3D2026-01-01T00%3A00%3A00Z" +
			"&event=push&page=1&per_page=100",
	}, `{"workflow_runs":[]}`, "", nil)

	client, err := github.NewClientWithExecutor("owner/repo", mock)
	if err != nil {
		t.Fatal(err)
	}

	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if _, err := client.ListRunTimings("main", "push", since, 0); err != nil {
		t.Fatalf("ListRunTimings: %v", err)
	}
}
