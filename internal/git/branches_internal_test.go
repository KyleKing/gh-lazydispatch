package git

import (
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
