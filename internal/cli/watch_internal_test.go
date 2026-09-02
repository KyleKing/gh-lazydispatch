package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/github"
)

// fakeRunGetter answers GetWorkflowRun from a queue of runs, one per call, so
// a poll loop's retries are exercised without a live repository or a real
// sleep.
type fakeRunGetter struct {
	runs  []*github.WorkflowRun
	calls int
}

func (f *fakeRunGetter) GetWorkflowRun(int64) (*github.WorkflowRun, error) {
	run := f.runs[min(f.calls, len(f.runs)-1)]
	f.calls++

	return run, nil
}

func TestPollUntilDone_ReturnsImmediatelyWhenAlreadyComplete(t *testing.T) {
	t.Parallel()

	client := &fakeRunGetter{runs: []*github.WorkflowRun{
		{ID: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionSuccess},
	}}

	var stderr bytes.Buffer

	run, err := pollUntilDone(client, 1, time.Hour, &stderr)
	if err != nil {
		t.Fatalf("pollUntilDone: %v", err)
	}

	if run.Conclusion != github.ConclusionSuccess {
		t.Errorf("conclusion: got %q, want %q", run.Conclusion, github.ConclusionSuccess)
	}

	if client.calls != 1 {
		t.Errorf("calls: got %d, want 1: an already-done run should not sleep between polls", client.calls)
	}
}

func TestPollUntilDone_KeepsPollingUntilTheRunFinishes(t *testing.T) {
	t.Parallel()

	client := &fakeRunGetter{runs: []*github.WorkflowRun{
		{ID: 1, Status: github.StatusInProgress},
		{ID: 1, Status: github.StatusInProgress},
		{ID: 1, Status: github.StatusCompleted, Conclusion: github.ConclusionFailure},
	}}

	var stderr bytes.Buffer

	run, err := pollUntilDone(client, 1, time.Millisecond, &stderr)
	if err != nil {
		t.Fatalf("pollUntilDone: %v", err)
	}

	if client.calls != 3 {
		t.Errorf("calls: got %d, want 3", client.calls)
	}

	if run.Conclusion != github.ConclusionFailure {
		t.Errorf("conclusion: got %q, want %q", run.Conclusion, github.ConclusionFailure)
	}

	if !strings.Contains(stderr.String(), "in_progress") {
		t.Errorf("stderr %q reports no progress while waiting", stderr.String())
	}
}

// TestRecorded_WatchWritesTheDigestForAnAlreadyFinishedRun replays a completed
// run, so watch's poll loop exits on its first check and its output is the
// same two gh calls diagnose already makes: one plain view, one --log view.
//
//nolint:paralleltest // runRecordedArgs calls t.Setenv, which rules out t.Parallel
func TestRecorded_WatchWritesTheDigestForAnAlreadyFinishedRun(t *testing.T) {
	digestPath := filepath.Join(t.TempDir(), "digest.md")

	stdout, stderr, code := runRecordedArgs(t, "step-run", "watch", recordedStepRun, "--out", digestPath)
	if code != 0 {
		t.Fatalf("exit %d: %s", code, stderr)
	}

	if got := strings.TrimSpace(stdout); got != digestPath {
		t.Errorf("stdout is %q, want the digest path %q", got, digestPath)
	}

	doc, err := os.ReadFile(digestPath) //nolint:gosec // digestPath is built from t.TempDir() above
	if err != nil {
		t.Fatalf("reading the digest: %v", err)
	}

	if !strings.Contains(string(doc), "Detected issues") {
		t.Errorf("digest has no Detected issues section:\n%s", doc)
	}
}
