package ghcassette

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// RecordEnv is the variable that flips a run from replay to record. Recording
// runs the real gh against a real repository, so it is opt-in and never what
// CI does.
const RecordEnv = "LAZYDISPATCH_RECORD"

const stubPackage = "github.com/kyleking/gh-lazydispatch/internal/ghcassette/ghstub"

// TB is the part of *testing.T this package needs, kept as an interface so the
// package itself does not import testing.
type TB interface {
	Helper()
	Cleanup(f func())
	TempDir() string
	Fatalf(format string, args ...any)
}

//nolint:gochecknoglobals // the stub is built once per test process and shared
var (
	stubOnce sync.Once
	stubDir  string
	errStub  error
)

// Session is one cassette bound to one test.
type Session struct {
	cassette string
	journal  string
	binDir   string
	record   bool
}

// Start prepares a cassette for the test. Replay is the default; setting
// LAZYDISPATCH_RECORD=1 re-records the cassette against the real gh,
// discarding what was there.
func Start(t TB, cassette string) *Session {
	t.Helper()

	dir, err := stub()
	if err != nil {
		t.Fatalf("building the gh stub: %v", err)
	}

	s := &Session{
		cassette: cassette,
		journal:  filepath.Join(t.TempDir(), "journal"),
		binDir:   dir,
		record:   os.Getenv(RecordEnv) == "1",
	}

	if s.record {
		if err := os.Remove(cassette); err != nil && !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("clearing the cassette: %v", err)
		}
	}

	return s
}

// Replay prepares a cassette that is never recorded, for a case derived from a
// recording rather than observed: a run that failed, a job that vanished.
func Replay(t TB, cassette string) *Session {
	t.Helper()

	dir, err := stub()
	if err != nil {
		t.Fatalf("building the gh stub: %v", err)
	}

	return &Session{
		cassette: cassette,
		journal:  filepath.Join(t.TempDir(), "journal"),
		binDir:   dir,
	}
}

// Recording reports whether this run talks to GitHub.
func (s *Session) Recording() bool { return s.record }

// GH is the path to the stand-in binary. Programs under test find it through
// PATH; a test that calls it directly needs the path.
func (s *Session) GH() string { return filepath.Join(s.binDir, "gh") }

// Env is the environment a subprocess needs for its gh calls to reach the
// cassette. PATH is prefixed rather than replaced, since gh itself shells out
// to git.
func (s *Session) Env(t TB) []string {
	t.Helper()

	env := append(os.Environ(),
		"PATH="+s.binDir+string(os.PathListSeparator)+os.Getenv("PATH"),
		"GH_CASSETTE="+s.cassette,
		"GH_CASSETTE_JOURNAL="+s.journal,
	)

	if !s.record {
		return append(env, "GH_CASSETTE_MODE=replay")
	}

	ghPath, err := exec.LookPath("gh")
	if err != nil {
		t.Fatalf("recording needs the real gh on PATH: %v", err)
	}

	return append(env, "GH_CASSETTE_MODE=record", "GH_CASSETTE_REAL="+ghPath)
}

// EnvSetter is the part of *testing.T that Apply needs, kept separate from TB
// because only an in-process test sets its own environment.
type EnvSetter interface {
	Helper()
	Setenv(key, value string)
	Fatalf(format string, args ...any)
}

// Apply points this process's own gh calls at the cassette, for a test that
// exercises the packages directly rather than driving the built binary.
// Setenv rules out t.Parallel, which is the trade for not spawning a process.
func (s *Session) Apply(t EnvSetter) {
	t.Helper()

	// Resolved before PATH is rewritten, or LookPath finds the stub.
	ghPath := ""

	if s.record {
		found, err := exec.LookPath("gh")
		if err != nil {
			t.Fatalf("recording needs the real gh on PATH: %v", err)
		}

		ghPath = found
	}

	t.Setenv("PATH", s.binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GH_CASSETTE", s.cassette)
	t.Setenv("GH_CASSETTE_JOURNAL", s.journal)

	if s.record {
		t.Setenv("GH_CASSETTE_MODE", "record")
		t.Setenv("GH_CASSETTE_REAL", ghPath)

		return
	}

	t.Setenv("GH_CASSETTE_MODE", "replay")
}

// Played returns the cassette indices the run consumed, in order.
func (s *Session) Played(t TB) []int {
	t.Helper()

	raw, err := os.ReadFile(s.journal) // #nosec G304 -- a path this package wrote
	if err != nil {
		return nil
	}

	played := []int{}

	for _, field := range strings.Fields(string(raw)) {
		n, err := strconv.Atoi(field)
		if err != nil {
			t.Fatalf("reading the journal: %v", err)
		}

		played = append(played, n)
	}

	return played
}

// RequireAllPlayed fails unless the run consumed every recorded interaction.
// Something left in the cassette means the code stopped making a call it used
// to make, which is the regression worth catching.
func (s *Session) RequireAllPlayed(t TB) {
	t.Helper()

	if s.record {
		return
	}

	c, err := Load(s.cassette)
	if err != nil {
		t.Fatalf("loading the cassette: %v", err)
	}

	if got, want := len(s.Played(t)), len(c.Interactions); got != want {
		t.Fatalf("replayed %d of %d recorded gh calls", got, want)
	}
}

// stub builds the gh stand-in once per test process.
func stub() (string, error) {
	stubOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ghstub")
		if err != nil {
			errStub = fmt.Errorf("creating the stub directory: %w", err)

			return
		}

		// #nosec G204 -- every argument is a constant in this file
		build := exec.CommandContext(context.Background(), "go", "build", "-o", filepath.Join(dir, "gh"), stubPackage)

		out, err := build.CombinedOutput()
		if err != nil {
			errStub = fmt.Errorf("go build ghstub: %w: %s", err, out)

			return
		}

		stubDir = dir
	})

	return stubDir, errStub
}

// RemoveStub deletes the built stub. A TestMain that calls Start should defer it.
func RemoveStub() {
	if stubDir != "" {
		_ = os.RemoveAll(stubDir) //nolint:errcheck // a leftover temp directory is not worth failing a test run
	}
}
