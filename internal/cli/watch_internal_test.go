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

// A run ID or --pr names the target, in either order, and neither or both is a
// usage error.
func TestParseWatchTarget_AcceptsARunIDOrAPullRequestNotBothOrNeither(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   []string
		runID  int64
		pr     int
		wantOK bool
	}{
		{"run id", []string{"42"}, 42, 0, true},
		{"run id then flag", []string{"42", "--interval", "5"}, 42, 0, true},
		{"pull request", []string{"--pr", "7", "--interval", "5"}, 0, 7, true},
		{"both", []string{"42", "--pr", "7"}, 0, 7, false},
		{"neither", []string{"--interval", "5"}, 0, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fs := newFlagSet("watch")

			var pr, interval int

			fs.IntVar(&pr, "pr", 0, "")
			fs.IntVar(&interval, "interval", 0, "")

			runID, err := parseWatchTarget(fs, tt.args, &pr)
			if (err == nil) != tt.wantOK || runID != tt.runID || pr != tt.pr {
				t.Errorf("parseWatchTarget(%v) = %d, pr %d, err %v; want run %d, pr %d, ok %v",
					tt.args, runID, pr, err, tt.runID, tt.pr, tt.wantOK)
			}
		})
	}
}
