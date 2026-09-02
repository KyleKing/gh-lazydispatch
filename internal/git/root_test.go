package git_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/git"
)

// A subdirectory has to resolve to the same root as the checkout itself, which
// is what lets the tool read .github from anywhere inside a repository.
func TestRepoRoot_ResolvesFromAnyDepth(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".git"), 0o750); err != nil {
		t.Fatalf("failed to create .git: %v", err)
	}

	nested := filepath.Join(root, "api", "src")
	if err := os.MkdirAll(nested, 0o750); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	// The temp dir itself may sit behind a symlink (/var on darwin), so the
	// resolved root is compared after both sides are evaluated.
	want, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolving the temp dir: %v", err)
	}

	for _, start := range []string{root, nested} {
		got, err := git.RepoRoot(start)
		if err != nil {
			t.Fatalf("RepoRoot(%s): %v", start, err)
		}

		resolved, err := filepath.EvalSymlinks(got)
		if err != nil {
			t.Fatalf("resolving %s: %v", got, err)
		}

		if resolved != want {
			t.Errorf("RepoRoot(%s) = %s, want %s", start, resolved, want)
		}
	}
}

func TestRepoRoot_OutsideARepository(t *testing.T) {
	t.Parallel()

	// A temp dir under the system root has no checkout above it.
	if _, err := git.RepoRoot(t.TempDir()); !errors.Is(err, git.ErrNotInRepo) {
		t.Fatalf("expected ErrNotInRepo, got: %v", err)
	}
}
