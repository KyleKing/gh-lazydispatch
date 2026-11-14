package app

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kyleking/aragonite/keyusage"
)

// TestUsage_LogsKeysByPaneAndNeverCommandBarText is the privacy line: a key is
// logged where it landed, and what is typed after `:` is not logged at all.
func TestUsage_LogsKeysByPaneAndNeverCommandBarText(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rec := keyusage.Open("test", keyusage.WithDir(dir))

	m := resize(t, newRenderModel(), 120, 40)
	m.usage = rec

	m = pressSpecial(t, m, tea.KeyTab)
	pane := m.paneContext()
	m = typeCommand(t, m, "res")
	pressSpecial(t, m, tea.KeyEnter)

	if err := rec.Close(); err != nil {
		t.Fatalf("closing the recorder: %v", err)
	}

	km := Keymap()

	u, err := keyusage.Load(dir, km)
	if err != nil {
		t.Fatalf("loading the log: %v", err)
	}

	if got := u.Stat(ctxWorkflows, "tab").Count; got != 1 {
		t.Errorf("tab in the workflows pane was logged %d times, want 1", got)
	}

	if got := u.Stat(pane, ":").Count; got != 1 {
		t.Errorf(": in the %s pane was logged %d times, want 1", pane, got)
	}

	if misses := u.Misses(); len(misses) != 0 {
		t.Errorf("command bar text reached the log as misses: %+v", misses)
	}
}
