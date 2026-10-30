package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
