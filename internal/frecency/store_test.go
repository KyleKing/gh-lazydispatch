package frecency_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kyleking/gh-lazydispatch/internal/frecency"
)

const testWorkflowDeploy = "deploy.yml"

func TestStore_Record(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", testWorkflowDeploy, "main", map[string]string{"env": "prod"})

	entries := store.Entries["owner/repo"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}

	e := entries[0]
	if e.Workflow != testWorkflowDeploy {
		t.Errorf("expected workflow 'deploy.yml', got %q", e.Workflow)
	}

	if e.Branch != "main" {
		t.Errorf("expected branch 'main', got %q", e.Branch)
	}

	if e.RunCount != 1 {
		t.Errorf("expected run count 1, got %d", e.RunCount)
	}
}

func TestStore_Record_Increment(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", testWorkflowDeploy, "main", map[string]string{"env": "prod"})
	store.Record("owner/repo", testWorkflowDeploy, "main", map[string]string{"env": "prod"})
	store.Record("owner/repo", testWorkflowDeploy, "main", map[string]string{"env": "prod"})

	entries := store.Entries["owner/repo"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry (incremented), got %d", len(entries))
	}

	if entries[0].RunCount != 3 {
		t.Errorf("expected run count 3, got %d", entries[0].RunCount)
	}
}

func TestStore_Record_DifferentInputs(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", testWorkflowDeploy, "main", map[string]string{"env": "prod"})
	store.Record("owner/repo", testWorkflowDeploy, "main", map[string]string{"env": "staging"})

	entries := store.Entries["owner/repo"]
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (different inputs), got %d", len(entries))
	}
}

func TestStore_TopForRepo(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", testWorkflowDeploy, "main", nil)
	store.Record("owner/repo", "ci.yml", "main", nil)
	store.Record("owner/repo", testWorkflowDeploy, "main", nil)
	store.Record("owner/repo", testWorkflowDeploy, "main", nil)

	top := store.TopForRepo("owner/repo", "", 10)
	if len(top) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(top))
	}

	if top[0].Workflow != testWorkflowDeploy {
		t.Errorf("expected deploy.yml first (higher frecency), got %q", top[0].Workflow)
	}
}

func TestStore_TopForRepo_FilterByWorkflow(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	store.Record("owner/repo", testWorkflowDeploy, "main", nil)
	store.Record("owner/repo", "ci.yml", "main", nil)

	top := store.TopForRepo("owner/repo", testWorkflowDeploy, 10)
	if len(top) != 1 {
		t.Fatalf("expected 1 entry (filtered), got %d", len(top))
	}

	if top[0].Workflow != testWorkflowDeploy {
		t.Errorf("expected deploy.yml, got %q", top[0].Workflow)
	}
}

func TestStore_TopForRepo_Limit(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()

	for i := range 10 {
		store.Record("owner/repo", testWorkflowDeploy, "main", map[string]string{"i": string(rune('0' + i))})
	}

	top := store.TopForRepo("owner/repo", "", 5)
	if len(top) != 5 {
		t.Errorf("expected 5 entries (limited), got %d", len(top))
	}
}

func TestStore_SaveLoad(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	path := filepath.Join(tmpDir, "history.json")

	store := frecency.NewStore()
	store.Record("owner/repo", testWorkflowDeploy, "main", map[string]string{"env": "prod"})

	if err := store.SaveTo(path); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	loaded, err := frecency.LoadFrom(path)
	if err != nil {
		t.Fatalf("LoadFrom failed: %v", err)
	}

	entries := loaded.Entries["owner/repo"]
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after load, got %d", len(entries))
	}

	if entries[0].Workflow != testWorkflowDeploy {
		t.Errorf("expected workflow 'deploy.yml', got %q", entries[0].Workflow)
	}
}

func TestLoadFrom_NotFound(t *testing.T) {
	t.Parallel()

	store, err := frecency.LoadFrom("/nonexistent/path/history.json")
	if err != nil {
		t.Fatalf("LoadFrom should not error on missing file: %v", err)
	}

	if store == nil {
		t.Fatal("expected non-nil store")
	}

	if len(store.Entries) != 0 {
		t.Errorf("expected empty entries, got %d", len(store.Entries))
	}
}

func TestScore(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   frecency.HistoryEntry
		wantMin float64
		wantMax float64
	}{
		{
			name: "recent high frequency",
			entry: frecency.HistoryEntry{
				RunCount:  10,
				LastRunAt: time.Now().Add(-30 * time.Minute),
			},
			wantMin: 35.0,
			wantMax: 45.0,
		},
		{
			name: "old low frequency",
			entry: frecency.HistoryEntry{
				RunCount:  1,
				LastRunAt: time.Now().Add(-30 * 24 * time.Hour),
			},
			wantMin: 0.4,
			wantMax: 0.6,
		},
		{
			name: "today medium frequency",
			entry: frecency.HistoryEntry{
				RunCount:  5,
				LastRunAt: time.Now().Add(-6 * time.Hour),
			},
			wantMin: 9.0,
			wantMax: 11.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			score := frecency.Score(tt.entry)
			if score < tt.wantMin || score > tt.wantMax {
				t.Errorf("frecency.Score() = %v, want between %v and %v", score, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestSortByFrecency(t *testing.T) {
	t.Parallel()

	now := time.Now()
	entries := []frecency.HistoryEntry{
		{Workflow: "low", RunCount: 1, LastRunAt: now.Add(-30 * 24 * time.Hour)},
		{Workflow: "high", RunCount: 10, LastRunAt: now.Add(-1 * time.Hour)},
		{Workflow: "medium", RunCount: 5, LastRunAt: now.Add(-6 * time.Hour)},
	}

	frecency.SortByFrecency(entries)

	if entries[0].Workflow != "high" {
		t.Errorf("expected 'high' first, got %q", entries[0].Workflow)
	}

	if entries[1].Workflow != "medium" {
		t.Errorf("expected 'medium' second, got %q", entries[1].Workflow)
	}

	if entries[2].Workflow != "low" {
		t.Errorf("expected 'low' third, got %q", entries[2].Workflow)
	}
}

// The cache moved from gh-wfd to lazydispatch, so a reader upgrading keeps the
// history they had. The copy runs once: a store already at the new path is
// never overwritten by the old one.
func TestCachePath_CarriesTheOldCacheForwardExactlyOnce(t *testing.T) {
	cache := t.TempDir()
	t.Setenv("XDG_CACHE_HOME", cache)

	old := filepath.Join(cache, "gh-wfd", "history.json")
	if err := os.MkdirAll(filepath.Dir(old), 0o750); err != nil {
		t.Fatal(err)
	}

	previous := frecency.NewStore()
	previous.Record("owner/repo", "ci.yml", "main", nil)

	if err := previous.SaveTo(old); err != nil {
		t.Fatal(err)
	}

	path := frecency.CachePath()
	if want := filepath.Join(cache, "lazydispatch", "history.json"); path != want {
		t.Fatalf("cache path is %s, want %s", path, want)
	}

	migrated, err := frecency.Load()
	if err != nil {
		t.Fatal(err)
	}

	if got := len(migrated.TopForRepo("owner/repo", "", 0)); got != 1 {
		t.Fatalf("the migrated store holds %d entries, want the one that was there", got)
	}

	// A second pass must not undo work done since the migration.
	migrated.Record("owner/repo", "release.yml", "main", nil)

	if err := migrated.Save(); err != nil {
		t.Fatal(err)
	}

	frecency.CachePath()

	after, err := frecency.Load()
	if err != nil {
		t.Fatal(err)
	}

	if got := len(after.TopForRepo("owner/repo", "", 0)); got != 2 {
		t.Errorf("a second migration left %d entries, want the newer store untouched", got)
	}
}

// A chain is one history entry rather than one per step, and re-running it
// counts up and replaces the step results rather than appending a row.
func TestRecordChain_CountsUpAndKeepsTheLatestStepResults(t *testing.T) {
	t.Parallel()

	store := frecency.NewStore()
	inputs := map[string]string{"env": "staging"}

	store.RecordChain("owner/repo", "deploy", "main", inputs, []frecency.ChainStepResult{{Workflow: "build.yml"}})
	store.RecordChain("owner/repo", "deploy", "main", inputs, []frecency.ChainStepResult{{Workflow: "test.yml"}})
	store.RecordChain("owner/repo", "deploy", "topic", inputs, nil)

	entries := store.TopForRepo("owner/repo", "", 0)
	if len(entries) != 2 {
		t.Fatalf("the same chain on two branches made %d entries", len(entries))
	}

	chains := frecency.FilterByType(entries, frecency.EntryTypeChain)
	if len(chains) != 2 {
		t.Fatalf("%d of the entries read as chains", len(chains))
	}

	var main frecency.HistoryEntry

	for _, entry := range chains {
		if entry.Branch == "main" {
			main = entry
		}
	}

	if main.RunCount != 2 {
		t.Errorf("re-running the chain counted %d, want 2", main.RunCount)
	}

	if len(main.StepResults) != 1 || main.StepResults[0].Workflow != "test.yml" {
		t.Errorf("the entry kept %v, want only the latest run's steps", main.StepResults)
	}

	// An entry written before chains existed has no type and reads as a
	// workflow, so the two never mix in one list.
	if got := frecency.FilterByType(entries, frecency.EntryTypeWorkflow); len(got) != 0 {
		t.Errorf("%d chain entries read as workflows", len(got))
	}
}
