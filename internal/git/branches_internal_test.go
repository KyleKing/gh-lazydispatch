package git

import (
	"os/exec"
	"slices"
	"testing"
)

// A repository with no remote still has to offer a branch list, since the
// branch modal is how a dispatch picks its ref. The reads run against the
// working directory, so this one cannot share a process with a parallel test.
//
//nolint:paralleltest // t.Chdir moves the whole process
func TestFetchBranches_FallsBackWhereThereIsNoRemote(t *testing.T) {
	t.Chdir(t.TempDir())

	branches, err := FetchBranches(t.Context())
	if err == nil {
		t.Error("a directory that is no repository reported no error")
	}

	if len(branches) == 0 {
		t.Fatal("a checkout with no remote offered no branches at all")
	}

	for _, want := range DefaultBranches() {
		if !slices.Contains(branches, want) {
			t.Errorf("the fallback list is %v, missing %q", branches, want)
		}
	}
}

// initRepo builds a checkout with one commit and makes it the process's working
// directory, since every read here runs against ".".
func initRepo(t *testing.T) {
	t.Helper()

	dir := t.TempDir()
	t.Chdir(dir)

	for _, args := range [][]string{
		{"init", "--initial-branch=trunk"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test"},
		{"commit", "--allow-empty", "-m", "root"},
	} {
		cmd := exec.CommandContext(t.Context(), "git", args...) //#nosec G204 -- fixed argument lists in this file
		cmd.Dir = dir

		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// The reads answer from a real checkout rather than from a stub, because what
// they are for is the shape of git's own output.
//
//nolint:paralleltest // t.Chdir moves the whole process
func TestBranchReads_AnswerFromARealCheckout(t *testing.T) {
	initRepo(t)

	if got := GetCurrentBranch(t.Context()); got != "trunk" {
		t.Errorf("the checked out branch reads as %q, want trunk", got)
	}

	// No remote, so nothing names a default branch and the caller gets an
	// empty string rather than a guess.
	if got := GetDefaultBranch(t.Context()); got != "" {
		t.Errorf("a repository with no remote named %q as its default", got)
	}
}
