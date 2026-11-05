package github_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/github"
	"github.com/kyleking/gh-lazydispatch/internal/testutil"
)

func runsPayload(first, count int) string {
	runs := make([]string, 0, count)
	for id := first; id < first+count; id++ {
		runs = append(runs, fmt.Sprintf(`{"id":%d,"conclusion":"success"}`, id))
	}

	return `{"workflow_runs":[` + strings.Join(runs, ",") + `]}`
}

func TestListRunsPaged_ReadsPastOnePageAndStopsAtTheLimit(t *testing.T) {
	t.Parallel()

	mock := testutil.NewMockExecutor()
	base := "repos/o/r/actions/workflows/smoke.yml/runs?"
	mock.AddCommand("gh", []string{"api", base + "page=1&per_page=100"}, runsPayload(1000, 100), "", nil)
	mock.AddCommand("gh", []string{"api", base + "page=2&per_page=100"}, runsPayload(900, 100), "", nil)
	mock.AddCommand("gh", []string{"api", base + "page=3&per_page=100"}, runsPayload(800, 19), "", nil)

	client, err := github.NewClientWithExecutor("o/r", mock)
	if err != nil {
		t.Fatalf("building the client: %v", err)
	}

	all, err := client.ListRunsPaged(github.RunQuery{Workflow: "smoke.yml", Limit: 1000})
	if err != nil || len(all) != 219 {
		t.Fatalf("got %d runs, err %v, want 219 (a short page ends the read)", len(all), err)
	}

	limited, err := client.ListRunsPaged(github.RunQuery{Workflow: "smoke.yml", Limit: 150})
	if err != nil || len(limited) != 150 || limited[149].ID != 949 {
		t.Fatalf("limit 150 returned %d runs, err %v, want 150 ending at run 949", len(limited), err)
	}
}
