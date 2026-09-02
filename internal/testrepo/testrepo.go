// Package testrepo guards what a recording is allowed to capture.
package testrepo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kyleking/aragonite/ghcassette"
)

// TB is the part of *testing.T this package needs, kept as an interface so a
// caller can pass either a T or a B.
type TB interface {
	Helper()
	Fatalf(format string, args ...any)
}

// RequirePublic fails unless repo is a public GitHub repository, and does
// nothing at all when the run is replaying rather than recording.
//
// A cassette is a verbatim recording of what gh printed, committed to a public
// repository and never read again by a human. Recording against a private
// repository would publish its pull request titles, review bodies, branch
// names, and CI logs, and nothing downstream would notice.
//
// Call this before ghcassette.Start, whose first act when recording is to
// delete the cassette: a guard that runs after it has already destroyed the
// recording it was protecting.
func RequirePublic(tb TB, repo string) {
	tb.Helper()

	if os.Getenv(ghcassette.RecordEnv) != "1" {
		return
	}

	refuseUnlessPublic(tb, repo, ghVisibility)
}

// refuseUnlessPublic is the decision, separated from the subprocess so that what it
// refuses can be tested without reaching GitHub.
func refuseUnlessPublic(tb TB, repo string, read func(repo string) (string, error)) {
	tb.Helper()

	visibility, err := read(repo)
	if err != nil {
		tb.Fatalf("reading %s's visibility before recording it: %v", repo, err)

		return
	}

	if visibility != "PUBLIC" {
		tb.Fatalf("refusing to record %s: it is %s, and a cassette is committed verbatim",
			repo, strings.ToLower(visibility))
	}
}

// ghVisibility asks gh what repo's visibility is. Anything but a repository gh
// can read and report on is an error, so a typo refuses rather than passes.
func ghVisibility(repo string) (string, error) {
	// #nosec G204 -- repo is a constant in the calling test
	cmd := exec.CommandContext(context.Background(), "gh", "repo", "view", repo, "--json", "visibility",
		"--jq", ".visibility")

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("gh repo view %s: %w", repo, err)
	}

	return strings.TrimSpace(string(out)), nil
}
