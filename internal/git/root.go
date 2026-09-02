package git

import (
	"errors"
	"fmt"
	"path/filepath"

	"github.com/kyleking/aragonite/vcs"
)

// ErrNotInRepo indicates no git or jj checkout contains the starting directory.
var ErrNotInRepo = errors.New("not inside a git or jj repository")

// RepoRoot walks up from start to the checkout that holds it. Workflow and
// config paths resolve against the root rather than the working directory, so
// the tool reads the same repository from any subdirectory of it.
func RepoRoot(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", fmt.Errorf("resolving %s: %w", start, err)
	}

	for {
		if vcs.IsRepo(dir) {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("%s: %w", start, ErrNotInRepo)
		}

		dir = parent
	}
}
