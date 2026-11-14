package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kyleking/aragonite/keyusage"
)

func TestRoot_RecordsCommandPathAndFlagNamesButNotValues(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec := keyusage.Open("test", keyusage.WithDir(dir))
	root := Root()

	rec.Invoke(root, []string{"export", "runs", "--branch", "secret-branch", "--limit=5", "positional-arg"})

	if err := rec.Close(); err != nil {
		t.Fatalf("closing the recorder: %v", err)
	}

	km := keyusage.Keymap{keyusage.CLI: keyusage.Commands(root)}

	u, err := keyusage.Load(dir, km)
	if err != nil {
		t.Fatalf("loading the log: %v", err)
	}

	for _, key := range []string{"gh-lazydispatch export runs", "gh-lazydispatch export runs --branch"} {
		if got := u.Stat(keyusage.CLI, key).Count; got != 1 {
			t.Errorf("%q was logged %d times, want 1", key, got)
		}
	}

	if misses := u.Misses(); len(misses) != 0 {
		t.Errorf("declared flags were logged as misses: %+v", misses)
	}

	logs, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil || len(logs) != 1 {
		t.Fatalf("log files = %v, %v", logs, err)
	}

	raw, err := os.ReadFile(logs[0])
	if err != nil {
		t.Fatalf("reading the log: %v", err)
	}

	for _, leaked := range []string{"secret-branch", "positional-arg", "=5"} {
		if strings.Contains(string(raw), leaked) {
			t.Errorf("log holds %q: %s", leaked, raw)
		}
	}
}
